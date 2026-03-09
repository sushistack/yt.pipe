package store

import (
	"testing"

	"github.com/sushistack/yt.pipe/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestMoodPreset(id, name string) *domain.MoodPreset {
	return &domain.MoodPreset{
		ID:          id,
		Name:        name,
		Description: "test preset",
		Speed:       1.0,
		Emotion:     "neutral",
		Pitch:       1.0,
		ParamsJSON:  map[string]any{"intensity": 0.5},
	}
}

func TestCreateMoodPreset_Success(t *testing.T) {
	s := setupTestStore(t)
	p := newTestMoodPreset("mp1", "tense")
	err := s.CreateMoodPreset(p)
	require.NoError(t, err)
	assert.False(t, p.CreatedAt.IsZero())
	assert.False(t, p.UpdatedAt.IsZero())
}

func TestCreateMoodPreset_DuplicateName(t *testing.T) {
	s := setupTestStore(t)
	require.NoError(t, s.CreateMoodPreset(newTestMoodPreset("mp1", "tense")))
	err := s.CreateMoodPreset(newTestMoodPreset("mp2", "tense"))
	assert.Error(t, err) // UNIQUE constraint on name
}

func TestGetMoodPreset_Success(t *testing.T) {
	s := setupTestStore(t)
	p := newTestMoodPreset("mp1", "tense")
	p.Speed = 1.2
	p.Emotion = "fearful"
	p.Pitch = 0.9
	require.NoError(t, s.CreateMoodPreset(p))

	got, err := s.GetMoodPreset("mp1")
	require.NoError(t, err)
	assert.Equal(t, "mp1", got.ID)
	assert.Equal(t, "tense", got.Name)
	assert.Equal(t, "test preset", got.Description)
	assert.Equal(t, 1.2, got.Speed)
	assert.Equal(t, "fearful", got.Emotion)
	assert.Equal(t, 0.9, got.Pitch)
	assert.Equal(t, 0.5, got.ParamsJSON["intensity"])
}

func TestGetMoodPreset_NotFound(t *testing.T) {
	s := setupTestStore(t)
	_, err := s.GetMoodPreset("nonexistent")
	assert.IsType(t, &domain.NotFoundError{}, err)
}

func TestGetMoodPresetByName_Success(t *testing.T) {
	s := setupTestStore(t)
	require.NoError(t, s.CreateMoodPreset(newTestMoodPreset("mp1", "tense")))

	got, err := s.GetMoodPresetByName("tense")
	require.NoError(t, err)
	assert.Equal(t, "mp1", got.ID)
	assert.Equal(t, "tense", got.Name)
}

func TestGetMoodPresetByName_NotFound(t *testing.T) {
	s := setupTestStore(t)
	_, err := s.GetMoodPresetByName("nonexistent")
	assert.IsType(t, &domain.NotFoundError{}, err)
}

func TestListMoodPresets_Empty(t *testing.T) {
	s := setupTestStore(t)
	presets, err := s.ListMoodPresets()
	require.NoError(t, err)
	assert.Empty(t, presets)
}

func TestListMoodPresets_Multiple(t *testing.T) {
	s := setupTestStore(t)
	require.NoError(t, s.CreateMoodPreset(newTestMoodPreset("mp1", "calm")))
	require.NoError(t, s.CreateMoodPreset(newTestMoodPreset("mp2", "tense")))

	presets, err := s.ListMoodPresets()
	require.NoError(t, err)
	assert.Len(t, presets, 2)
	assert.Equal(t, "calm", presets[0].Name) // ordered by name
	assert.Equal(t, "tense", presets[1].Name)
}

func TestUpdateMoodPreset_Success(t *testing.T) {
	s := setupTestStore(t)
	p := newTestMoodPreset("mp1", "tense")
	require.NoError(t, s.CreateMoodPreset(p))

	p.Name = "very-tense"
	p.Speed = 1.5
	p.Emotion = "fearful"
	err := s.UpdateMoodPreset(p)
	require.NoError(t, err)

	got, _ := s.GetMoodPreset("mp1")
	assert.Equal(t, "very-tense", got.Name)
	assert.Equal(t, 1.5, got.Speed)
	assert.Equal(t, "fearful", got.Emotion)
}

func TestUpdateMoodPreset_NotFound(t *testing.T) {
	s := setupTestStore(t)
	p := newTestMoodPreset("nonexistent", "tense")
	err := s.UpdateMoodPreset(p)
	assert.IsType(t, &domain.NotFoundError{}, err)
}

func TestDeleteMoodPreset_Success(t *testing.T) {
	s := setupTestStore(t)
	require.NoError(t, s.CreateMoodPreset(newTestMoodPreset("mp1", "tense")))

	err := s.DeleteMoodPreset("mp1")
	require.NoError(t, err)

	_, err = s.GetMoodPreset("mp1")
	assert.IsType(t, &domain.NotFoundError{}, err)
}

func TestDeleteMoodPreset_NotFound(t *testing.T) {
	s := setupTestStore(t)
	err := s.DeleteMoodPreset("nonexistent")
	assert.IsType(t, &domain.NotFoundError{}, err)
}

func TestDeleteMoodPreset_FailsWithAssignment(t *testing.T) {
	s := setupTestStore(t)
	require.NoError(t, s.CreateMoodPreset(newTestMoodPreset("mp1", "tense")))
	require.NoError(t, s.AssignMoodToScene("proj1", 1, "mp1", false))

	err := s.DeleteMoodPreset("mp1")
	assert.Error(t, err) // FK constraint prevents deletion
}

// Scene Mood Assignment tests

func TestAssignMoodToScene_Success(t *testing.T) {
	s := setupTestStore(t)
	require.NoError(t, s.CreateMoodPreset(newTestMoodPreset("mp1", "tense")))

	err := s.AssignMoodToScene("proj1", 1, "mp1", true)
	require.NoError(t, err)

	a, err := s.GetSceneMoodAssignment("proj1", 1)
	require.NoError(t, err)
	assert.Equal(t, "proj1", a.ProjectID)
	assert.Equal(t, 1, a.SceneNum)
	assert.Equal(t, "mp1", a.PresetID)
	assert.True(t, a.AutoMapped)
	assert.False(t, a.Confirmed)
}

func TestAssignMoodToScene_Upsert(t *testing.T) {
	s := setupTestStore(t)
	require.NoError(t, s.CreateMoodPreset(newTestMoodPreset("mp1", "tense")))
	require.NoError(t, s.CreateMoodPreset(newTestMoodPreset("mp2", "calm")))

	require.NoError(t, s.AssignMoodToScene("proj1", 1, "mp1", true))
	require.NoError(t, s.AssignMoodToScene("proj1", 1, "mp2", false)) // upsert

	a, err := s.GetSceneMoodAssignment("proj1", 1)
	require.NoError(t, err)
	assert.Equal(t, "mp2", a.PresetID)
	assert.False(t, a.AutoMapped)
	assert.False(t, a.Confirmed) // reset on upsert
}

func TestConfirmSceneMood_Success(t *testing.T) {
	s := setupTestStore(t)
	require.NoError(t, s.CreateMoodPreset(newTestMoodPreset("mp1", "tense")))
	require.NoError(t, s.AssignMoodToScene("proj1", 1, "mp1", true))

	err := s.ConfirmSceneMood("proj1", 1)
	require.NoError(t, err)

	a, _ := s.GetSceneMoodAssignment("proj1", 1)
	assert.True(t, a.Confirmed)
}

func TestConfirmSceneMood_NotFound(t *testing.T) {
	s := setupTestStore(t)
	err := s.ConfirmSceneMood("proj1", 99)
	assert.IsType(t, &domain.NotFoundError{}, err)
}

func TestGetSceneMoodAssignment_NotFound(t *testing.T) {
	s := setupTestStore(t)
	_, err := s.GetSceneMoodAssignment("proj1", 99)
	assert.IsType(t, &domain.NotFoundError{}, err)
}

func TestListSceneMoodAssignments_Success(t *testing.T) {
	s := setupTestStore(t)
	require.NoError(t, s.CreateMoodPreset(newTestMoodPreset("mp1", "tense")))
	require.NoError(t, s.CreateMoodPreset(newTestMoodPreset("mp2", "calm")))
	require.NoError(t, s.AssignMoodToScene("proj1", 1, "mp1", true))
	require.NoError(t, s.AssignMoodToScene("proj1", 2, "mp2", false))
	require.NoError(t, s.AssignMoodToScene("proj2", 1, "mp1", true)) // different project

	assignments, err := s.ListSceneMoodAssignments("proj1")
	require.NoError(t, err)
	assert.Len(t, assignments, 2)
	assert.Equal(t, 1, assignments[0].SceneNum)
	assert.Equal(t, 2, assignments[1].SceneNum)
}

func TestListSceneMoodAssignments_Empty(t *testing.T) {
	s := setupTestStore(t)
	assignments, err := s.ListSceneMoodAssignments("proj1")
	require.NoError(t, err)
	assert.Empty(t, assignments)
}

func TestDeleteSceneMoodAssignment_Success(t *testing.T) {
	s := setupTestStore(t)
	require.NoError(t, s.CreateMoodPreset(newTestMoodPreset("mp1", "tense")))
	require.NoError(t, s.AssignMoodToScene("proj1", 1, "mp1", false))

	err := s.DeleteSceneMoodAssignment("proj1", 1)
	require.NoError(t, err)

	_, err = s.GetSceneMoodAssignment("proj1", 1)
	assert.IsType(t, &domain.NotFoundError{}, err)
}

func TestDeleteSceneMoodAssignment_NotFound(t *testing.T) {
	s := setupTestStore(t)
	err := s.DeleteSceneMoodAssignment("proj1", 99)
	assert.IsType(t, &domain.NotFoundError{}, err)
}

func TestNilParamsJSON(t *testing.T) {
	s := setupTestStore(t)
	p := &domain.MoodPreset{
		ID:      "mp1",
		Name:    "test",
		Speed:   1.0,
		Emotion: "neutral",
		Pitch:   1.0,
	}
	require.NoError(t, s.CreateMoodPreset(p))

	got, err := s.GetMoodPreset("mp1")
	require.NoError(t, err)
	assert.NotNil(t, got.ParamsJSON) // should be empty map, not nil
}
