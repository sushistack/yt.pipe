package pipeline

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/jay/youtube-pipeline/internal/domain"
	"github.com/jay/youtube-pipeline/internal/service"
	"github.com/stretchr/testify/assert"
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

func timeNow() time.Time {
	return time.Now()
}
