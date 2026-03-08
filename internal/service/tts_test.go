package service

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/sushistack/yt.pipe/internal/domain"
	"github.com/sushistack/yt.pipe/internal/glossary"
	"github.com/sushistack/yt.pipe/internal/mocks"
	"github.com/sushistack/yt.pipe/internal/plugin/tts"
	"github.com/sushistack/yt.pipe/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func newTestTTSService(t *testing.T, mockTTS *mocks.MockTTS, g *glossary.Glossary) (*TTSService, *store.Store) {
	t.Helper()
	s, err := store.New(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { s.Close() })
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	return NewTTSService(mockTTS, g, s, logger), s
}

func TestSynthesizeScene_Success(t *testing.T) {
	mockTTS := mocks.NewMockTTS(t)
	svc, st := newTestTTSService(t, mockTTS, glossary.New())
	ctx := context.Background()
	projectPath := t.TempDir()
	projectID := "tts-proj"
	createTestProject(t, st, projectID)

	mockTTS.On("Synthesize", mock.Anything, "Hello world", "en-US-Neural2-D").
		Return(&tts.SynthesisResult{
			AudioData:   []byte("fake-audio"),
			DurationSec: 1.5,
			WordTimings: []domain.WordTiming{
				{Word: "Hello", StartSec: 0.0, EndSec: 0.5},
				{Word: "world", StartSec: 0.5, EndSec: 1.0},
			},
		}, nil)

	scene := domain.SceneScript{SceneNum: 1, Narration: "Hello world"}
	result, err := svc.SynthesizeScene(ctx, scene, projectID, projectPath, "en-US-Neural2-D")
	require.NoError(t, err)

	assert.Equal(t, 1, result.SceneNum)
	assert.Equal(t, 1.5, result.AudioDuration)
	assert.Len(t, result.WordTimings, 2)
	assert.FileExists(t, filepath.Join(projectPath, "scenes", "1", "audio.mp3"))

	// AC4: Verify manifest updated with audio hash
	manifest, err := st.GetManifest(projectID, 1)
	require.NoError(t, err)
	assert.NotEmpty(t, manifest.AudioHash)
	assert.Equal(t, "audio_generated", manifest.Status)
}

func TestSynthesizeScene_WithGlossary(t *testing.T) {
	mockTTS := mocks.NewMockTTS(t)

	dir := t.TempDir()
	gPath := filepath.Join(dir, "glossary.json")
	require.NoError(t, glossary.WriteToFile(gPath, []glossary.Entry{
		{Term: "SCP-173", Pronunciation: "ess see pee one seven three"},
	}))
	g := glossary.LoadFromFile(gPath)

	svc, st := newTestTTSService(t, mockTTS, g)
	ctx := context.Background()
	projectPath := t.TempDir()
	projectID := "tts-glossary"
	createTestProject(t, st, projectID)

	mockTTS.On("SynthesizeWithOverrides", mock.Anything, "About SCP-173", "voice1", mock.MatchedBy(func(o map[string]string) bool {
		return o["SCP-173"] == "ess see pee one seven three"
	})).Return(&tts.SynthesisResult{
		AudioData:   []byte("audio-data"),
		DurationSec: 2.0,
	}, nil)

	scene := domain.SceneScript{SceneNum: 1, Narration: "About SCP-173"}
	result, err := svc.SynthesizeScene(ctx, scene, projectID, projectPath, "voice1")
	require.NoError(t, err)
	assert.Equal(t, 2.0, result.AudioDuration)
}

func TestSynthesizeAll_Success(t *testing.T) {
	mockTTS := mocks.NewMockTTS(t)
	svc, st := newTestTTSService(t, mockTTS, glossary.New())
	ctx := context.Background()
	projectPath := t.TempDir()
	projectID := "tts-all"
	createTestProject(t, st, projectID)

	mockTTS.On("Synthesize", mock.Anything, mock.Anything, mock.Anything).
		Return(&tts.SynthesisResult{AudioData: []byte("audio"), DurationSec: 1.0}, nil)

	scenes := []domain.SceneScript{
		{SceneNum: 1, Narration: "Scene one"},
		{SceneNum: 2, Narration: "Scene two"},
	}

	results, err := svc.SynthesizeAll(ctx, scenes, projectID, projectPath, "voice1", nil)
	require.NoError(t, err)
	assert.Len(t, results, 2)
}

func TestSynthesizeAll_PartialFailure(t *testing.T) {
	mockTTS := mocks.NewMockTTS(t)
	svc, st := newTestTTSService(t, mockTTS, glossary.New())
	ctx := context.Background()
	projectPath := t.TempDir()
	projectID := "tts-partial"
	createTestProject(t, st, projectID)

	mockTTS.On("Synthesize", mock.Anything, "Scene one", mock.Anything).
		Return(&tts.SynthesisResult{AudioData: []byte("audio"), DurationSec: 1.0}, nil)
	mockTTS.On("Synthesize", mock.Anything, "Scene two", mock.Anything).
		Return(nil, assert.AnError)

	scenes := []domain.SceneScript{
		{SceneNum: 1, Narration: "Scene one"},
		{SceneNum: 2, Narration: "Scene two"},
	}

	results, err := svc.SynthesizeAll(ctx, scenes, projectID, projectPath, "voice1", nil)
	require.Error(t, err)
	assert.Len(t, results, 1) // partial results returned

	// Verify failed scene marked
	manifest, getErr := st.GetManifest(projectID, 2)
	require.NoError(t, getErr)
	assert.Equal(t, "audio_failed", manifest.Status)
}

func TestSynthesizeAll_SceneFilter(t *testing.T) {
	mockTTS := mocks.NewMockTTS(t)
	svc, st := newTestTTSService(t, mockTTS, glossary.New())
	ctx := context.Background()
	projectPath := t.TempDir()
	projectID := "tts-filter"
	createTestProject(t, st, projectID)

	mockTTS.On("Synthesize", mock.Anything, "Scene two", mock.Anything).
		Return(&tts.SynthesisResult{AudioData: []byte("audio"), DurationSec: 1.0}, nil)

	scenes := []domain.SceneScript{
		{SceneNum: 1, Narration: "Scene one"},
		{SceneNum: 2, Narration: "Scene two"},
		{SceneNum: 3, Narration: "Scene three"},
	}

	// AC3: Only re-synthesize scene 2
	results, err := svc.SynthesizeAll(ctx, scenes, projectID, projectPath, "voice1", []int{2})
	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, 2, results[0].SceneNum)
}

func TestSynthesizeScene_BackupExistingAudio(t *testing.T) {
	mockTTS := mocks.NewMockTTS(t)
	svc, st := newTestTTSService(t, mockTTS, glossary.New())
	ctx := context.Background()
	projectPath := t.TempDir()
	projectID := "tts-backup"
	createTestProject(t, st, projectID)

	// Create existing audio file
	sceneDir := filepath.Join(projectPath, "scenes", "1")
	require.NoError(t, os.MkdirAll(sceneDir, 0o755))
	existingAudio := filepath.Join(sceneDir, "audio.mp3")
	require.NoError(t, os.WriteFile(existingAudio, []byte("old-audio"), 0o644))

	mockTTS.On("Synthesize", mock.Anything, "New narration", "voice1").
		Return(&tts.SynthesisResult{AudioData: []byte("new-audio"), DurationSec: 2.0}, nil)

	scene := domain.SceneScript{SceneNum: 1, Narration: "New narration"}
	_, err := svc.SynthesizeScene(ctx, scene, projectID, projectPath, "voice1")
	require.NoError(t, err)

	// AC3: Verify backup exists
	assert.FileExists(t, existingAudio+".bak")
	backupData, _ := os.ReadFile(existingAudio + ".bak")
	assert.Equal(t, []byte("old-audio"), backupData)
}
