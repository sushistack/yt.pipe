package service

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/sushistack/yt.pipe/internal/domain"
	"github.com/sushistack/yt.pipe/internal/mocks"
	"github.com/sushistack/yt.pipe/internal/plugin/llm"
	"github.com/sushistack/yt.pipe/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func newTestMoodService(t *testing.T, mockLLM *mocks.MockLLM) (*MoodService, *store.Store) {
	t.Helper()
	s, err := store.New(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { s.Close() })
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	if mockLLM == nil {
		return NewMoodService(s, nil, logger), s
	}
	return NewMoodService(s, mockLLM, logger), s
}

func TestCreatePreset_Success(t *testing.T) {
	svc, _ := newTestMoodService(t, nil)

	p, err := svc.CreatePreset("tense", "Tense mood", 1.2, "fearful", 0.9, map[string]any{"intensity": 0.8})
	require.NoError(t, err)
	assert.NotEmpty(t, p.ID)
	assert.Equal(t, "tense", p.Name)
	assert.Equal(t, 1.2, p.Speed)
	assert.Equal(t, "fearful", p.Emotion)
	assert.Equal(t, 0.9, p.Pitch)
}

func TestCreatePreset_ValidationError(t *testing.T) {
	svc, _ := newTestMoodService(t, nil)

	_, err := svc.CreatePreset("", "desc", 1.0, "neutral", 1.0, nil)
	assert.Error(t, err)
	assert.IsType(t, &domain.ValidationError{}, err)
}

func TestCreatePreset_DuplicateName(t *testing.T) {
	svc, _ := newTestMoodService(t, nil)

	_, err := svc.CreatePreset("tense", "desc", 1.0, "neutral", 1.0, nil)
	require.NoError(t, err)

	_, err = svc.CreatePreset("tense", "desc2", 1.0, "neutral", 1.0, nil)
	assert.Error(t, err)
}

func TestGetPreset_Success(t *testing.T) {
	svc, _ := newTestMoodService(t, nil)

	created, _ := svc.CreatePreset("tense", "Tense mood", 1.2, "fearful", 0.9, nil)
	got, err := svc.GetPreset(created.ID)
	require.NoError(t, err)
	assert.Equal(t, "tense", got.Name)
}

func TestListPresets(t *testing.T) {
	svc, _ := newTestMoodService(t, nil)

	svc.CreatePreset("calm", "Calm", 0.9, "calm", 1.0, nil)
	svc.CreatePreset("tense", "Tense", 1.2, "fearful", 0.9, nil)

	presets, err := svc.ListPresets()
	require.NoError(t, err)
	assert.Len(t, presets, 2)
}

func TestUpdatePreset_Success(t *testing.T) {
	svc, _ := newTestMoodService(t, nil)

	created, _ := svc.CreatePreset("tense", "desc", 1.0, "neutral", 1.0, nil)
	newName := "very-tense"
	newSpeed := 1.5
	updated, err := svc.UpdatePreset(created.ID, &newName, nil, &newSpeed, nil, nil)
	require.NoError(t, err)
	assert.Equal(t, "very-tense", updated.Name)
	assert.Equal(t, 1.5, updated.Speed)
}

func TestUpdatePreset_NotFound(t *testing.T) {
	svc, _ := newTestMoodService(t, nil)

	name := "x"
	_, err := svc.UpdatePreset("nonexistent", &name, nil, nil, nil, nil)
	assert.IsType(t, &domain.NotFoundError{}, err)
}

func TestDeletePreset(t *testing.T) {
	svc, _ := newTestMoodService(t, nil)

	created, _ := svc.CreatePreset("tense", "desc", 1.0, "neutral", 1.0, nil)
	err := svc.DeletePreset(created.ID)
	require.NoError(t, err)

	_, err = svc.GetPreset(created.ID)
	assert.IsType(t, &domain.NotFoundError{}, err)
}

func TestAutoMapMoods_Success(t *testing.T) {
	mockLLM := mocks.NewMockLLM(t)
	svc, _ := newTestMoodService(t, mockLLM)

	// Create presets
	svc.CreatePreset("tense", "Tense mood", 1.2, "fearful", 0.9, nil)
	svc.CreatePreset("calm", "Calm mood", 0.9, "calm", 1.1, nil)

	scenes := []domain.SceneScript{
		{SceneNum: 1, Narration: "The creature lurked in the shadows..."},
		{SceneNum: 2, Narration: "The sun rose over the peaceful village..."},
	}

	// LLM returns mood name for each scene
	mockLLM.On("Complete", mock.Anything, mock.Anything, mock.Anything).
		Return(&llm.CompletionResult{Content: "tense"}, nil).Once()
	mockLLM.On("Complete", mock.Anything, mock.Anything, mock.Anything).
		Return(&llm.CompletionResult{Content: "calm"}, nil).Once()

	mapped, err := svc.AutoMapMoods(context.Background(), "proj1", scenes)
	require.NoError(t, err)
	assert.Equal(t, 2, mapped)

	// Verify assignments
	a1, _ := svc.GetSceneAssignment("proj1", 1)
	assert.True(t, a1.AutoMapped)
	assert.False(t, a1.Confirmed)

	a2, _ := svc.GetSceneAssignment("proj1", 2)
	assert.True(t, a2.AutoMapped)
}

func TestAutoMapMoods_NoPresets(t *testing.T) {
	mockLLM := mocks.NewMockLLM(t)
	svc, _ := newTestMoodService(t, mockLLM)

	scenes := []domain.SceneScript{{SceneNum: 1, Narration: "text"}}
	_, err := svc.AutoMapMoods(context.Background(), "proj1", scenes)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no mood presets")
}

func TestAutoMapMoods_NoLLM(t *testing.T) {
	svc, _ := newTestMoodService(t, nil)

	scenes := []domain.SceneScript{{SceneNum: 1, Narration: "text"}}
	_, err := svc.AutoMapMoods(context.Background(), "proj1", scenes)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "LLM plugin not configured")
}

func TestAutoMapMoods_UnmatchedMood(t *testing.T) {
	mockLLM := mocks.NewMockLLM(t)
	svc, _ := newTestMoodService(t, mockLLM)

	svc.CreatePreset("tense", "Tense", 1.0, "fearful", 1.0, nil)

	scenes := []domain.SceneScript{{SceneNum: 1, Narration: "text"}}

	// LLM returns a mood that doesn't match any preset
	mockLLM.On("Complete", mock.Anything, mock.Anything, mock.Anything).
		Return(&llm.CompletionResult{Content: "romantic"}, nil)

	mapped, err := svc.AutoMapMoods(context.Background(), "proj1", scenes)
	require.NoError(t, err)
	assert.Equal(t, 0, mapped) // no match
}

func TestGetPendingConfirmations(t *testing.T) {
	svc, st := newTestMoodService(t, nil)

	p1, _ := svc.CreatePreset("tense", "Tense", 1.0, "fearful", 1.0, nil)
	p2, _ := svc.CreatePreset("calm", "Calm", 1.0, "calm", 1.0, nil)

	st.AssignMoodToScene("proj1", 1, p1.ID, true)
	st.AssignMoodToScene("proj1", 2, p2.ID, true)
	st.ConfirmSceneMood("proj1", 1) // confirm scene 1

	pending, err := svc.GetPendingConfirmations("proj1")
	require.NoError(t, err)
	assert.Len(t, pending, 1)
	assert.Equal(t, 2, pending[0].SceneNum)
}

func TestConfirmAll(t *testing.T) {
	svc, st := newTestMoodService(t, nil)

	p1, _ := svc.CreatePreset("tense", "Tense", 1.0, "fearful", 1.0, nil)
	st.AssignMoodToScene("proj1", 1, p1.ID, true)
	st.AssignMoodToScene("proj1", 2, p1.ID, true)

	confirmed, err := svc.ConfirmAll("proj1")
	require.NoError(t, err)
	assert.Equal(t, 2, confirmed)

	pending, _ := svc.GetPendingConfirmations("proj1")
	assert.Empty(t, pending)
}

func TestReassignScene(t *testing.T) {
	svc, st := newTestMoodService(t, nil)

	p1, _ := svc.CreatePreset("tense", "Tense", 1.0, "fearful", 1.0, nil)
	p2, _ := svc.CreatePreset("calm", "Calm", 1.0, "calm", 1.0, nil)

	st.AssignMoodToScene("proj1", 1, p1.ID, true)
	st.ConfirmSceneMood("proj1", 1)

	err := svc.ReassignScene("proj1", 1, p2.ID)
	require.NoError(t, err)

	a, _ := svc.GetSceneAssignment("proj1", 1)
	assert.Equal(t, p2.ID, a.PresetID)
	assert.False(t, a.AutoMapped)
	assert.False(t, a.Confirmed) // reset on reassign
}
