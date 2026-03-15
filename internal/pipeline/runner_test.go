package pipeline

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sushistack/yt.pipe/internal/domain"
	"github.com/sushistack/yt.pipe/internal/service"
	"github.com/sushistack/yt.pipe/internal/store"
)

func TestStageResult(t *testing.T) {
	t.Run("pass result", func(t *testing.T) {
		sr := stageResult("test_stage", timeNow(), nil)
		assert.Equal(t, "test_stage", sr.Name)
		assert.Equal(t, "pass", sr.Status)
		assert.Empty(t, sr.Error)
	})

	t.Run("fail result", func(t *testing.T) {
		sr := stageResult("test_stage", timeNow(), assert.AnError)
		assert.Equal(t, "fail", sr.Status)
		assert.NotEmpty(t, sr.Error)
	})
}

func TestMergeSceneData(t *testing.T) {
	imageScenes := []*domain.Scene{
		{SceneNum: 1, ImagePath: "/img/1.png"},
		{SceneNum: 2, ImagePath: "/img/2.png"},
	}
	ttsScenes := []*domain.Scene{
		{SceneNum: 1, Narration: "narration 1", AudioPath: "/audio/1.mp3", AudioDuration: 10.0},
		{SceneNum: 2, Narration: "narration 2", AudioPath: "/audio/2.mp3", AudioDuration: 15.0},
	}
	timings := []service.SceneTiming{
		{SceneNum: 1, StartSec: 0, EndSec: 10},
		{SceneNum: 2, StartSec: 10, EndSec: 25},
	}

	merged := mergeSceneData(imageScenes, ttsScenes, timings)
	assert.Len(t, merged, 2)

	// Find scene 1
	var scene1 *domain.Scene
	for _, s := range merged {
		if s.SceneNum == 1 {
			scene1 = s
			break
		}
	}
	assert.NotNil(t, scene1)
	assert.Equal(t, "/img/1.png", scene1.ImagePath)
	assert.Equal(t, "/audio/1.mp3", scene1.AudioPath)
	assert.Equal(t, "narration 1", scene1.Narration)
}

func TestToDomainScenes(t *testing.T) {
	scenes := []*domain.Scene{
		{SceneNum: 1, ImagePath: "/a.png"},
		{SceneNum: 2, ImagePath: "/b.png"},
	}
	result := toDomainScenes(scenes)
	assert.Len(t, result, 2)
	assert.Equal(t, 1, result[0].SceneNum)
}

func TestParseSceneManifest(t *testing.T) {
	data := []byte(`{"scene_num":3,"narration":"test","image_path":"/img.png","audio_path":"/audio.mp3","audio_duration":5.5,"subtitle_path":"/sub.json"}`)
	scene, err := parseSceneManifest(data)
	assert.NoError(t, err)
	assert.Equal(t, 3, scene.SceneNum)
	assert.Equal(t, "test", scene.Narration)
	assert.Equal(t, "/img.png", scene.ImagePath)
	assert.InDelta(t, 5.5, scene.AudioDuration, 0.01)
}

func TestParseSceneManifest_invalid(t *testing.T) {
	_, err := parseSceneManifest([]byte("not json"))
	assert.Error(t, err)
}

func TestRunnerPipelineError(t *testing.T) {
	r := &Runner{}
	pe := r.pipelineError(service.StageDataLoad, 0, assert.AnError, "SCP-173")
	assert.Equal(t, service.StageDataLoad, pe.Stage)
	assert.Contains(t, pe.RecoverCmd, "yt-pipe run SCP-173")
}

func TestRunnerProgressFunc(t *testing.T) {
	var called bool
	r := &Runner{
		ProgressFunc: func(p service.PipelineProgress) {
			called = true
		},
	}
	r.reportProgress(service.PipelineProgress{Stage: service.StageDataLoad})
	assert.True(t, called)
}

func TestRunnerProgressFunc_nil(t *testing.T) {
	r := &Runner{}
	// Should not panic
	r.reportProgress(service.PipelineProgress{Stage: service.StageDataLoad})
}

func TestRunStage_unknownStage(t *testing.T) {
	r := &Runner{logger: slog.Default()}
	err := r.RunStage(context.Background(), "SCP-173", "nonexistent_stage")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown stage")
}

func TestRunStage_ImageGenerate_CharacterGate_NoCharacter(t *testing.T) {
	s, err := store.New(":memory:")
	require.NoError(t, err)
	defer s.Close()

	// Create a project so findProject works
	require.NoError(t, s.CreateProject(&domain.Project{
		ID: "p1", SCPID: "SCP-173", Status: domain.StageCharacter, WorkspacePath: t.TempDir(),
	}))

	characterSvc := service.NewCharacterService(s)
	r := &Runner{
		store:        s,
		logger:       slog.Default(),
		characterSvc: characterSvc,
	}

	err = r.RunStage(context.Background(), "SCP-173", service.StageImageGenerate)
	require.Error(t, err)
	var depErr *domain.DependencyError
	assert.ErrorAs(t, err, &depErr)
	assert.Equal(t, "image_generate", depErr.Action)
	assert.Contains(t, depErr.Missing, "character")
}

func TestRunStage_ImageGenerate_CharacterGate_NilService(t *testing.T) {
	s, err := store.New(":memory:")
	require.NoError(t, err)
	defer s.Close()

	require.NoError(t, s.CreateProject(&domain.Project{
		ID: "p1", SCPID: "SCP-173", Status: domain.StageImages, WorkspacePath: t.TempDir(),
	}))

	// No characterSvc — gate should be skipped (nil-safe)
	r := &Runner{
		store:  s,
		logger: slog.Default(),
	}

	// Will fail later (no imageGen plugin) but should NOT fail on character gate
	err = r.RunStage(context.Background(), "SCP-173", service.StageImageGenerate)
	// If it's a DependencyError for character, that's wrong
	var depErr *domain.DependencyError
	if errors.As(err, &depErr) {
		assert.NotEqual(t, "image_generate", depErr.Action,
			"nil characterSvc should skip gate, not produce DependencyError")
	}
}

func timeNow() time.Time {
	return time.Now()
}
