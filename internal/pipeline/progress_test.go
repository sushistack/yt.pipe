package pipeline

import (
	"bytes"
	"testing"

	"github.com/sushistack/yt.pipe/internal/service"
	"github.com/stretchr/testify/assert"
)

func TestProgressTracker_OnProgress_WithScenes(t *testing.T) {
	buf := new(bytes.Buffer)
	pt := NewProgressTracker(buf)
	// Buffer is not a TTY, so simple line output is used

	pt.OnProgress(service.PipelineProgress{
		Stage:          service.StageImageGenerate,
		ScenesTotal:    10,
		ScenesComplete: 3,
	})

	output := buf.String()
	assert.Contains(t, output, "image_generate")
	assert.Contains(t, output, "3/10")
	assert.Contains(t, output, "30%")
}

func TestProgressTracker_OnProgress_NoScenes(t *testing.T) {
	buf := new(bytes.Buffer)
	pt := NewProgressTracker(buf)

	pt.OnProgress(service.PipelineProgress{
		Stage: service.StageDataLoad,
	})

	output := buf.String()
	assert.Contains(t, output, "data_load")
	assert.Contains(t, output, "elapsed")
}

func TestProgressTracker_Finish(t *testing.T) {
	buf := new(bytes.Buffer)
	pt := NewProgressTracker(buf)

	pt.Finish("complete")
	output := buf.String()
	assert.Contains(t, output, "Pipeline complete")
}

func TestProgressTracker_MultipleStages(t *testing.T) {
	buf := new(bytes.Buffer)
	pt := NewProgressTracker(buf)

	pt.OnProgress(service.PipelineProgress{
		Stage:          service.StageImageGenerate,
		ScenesTotal:    10,
		ScenesComplete: 5,
	})
	pt.OnProgress(service.PipelineProgress{
		Stage:          service.StageTTSSynthesize,
		ScenesTotal:    10,
		ScenesComplete: 3,
	})

	output := buf.String()
	assert.Contains(t, output, "image_generate")
	assert.Contains(t, output, "tts_synthesize")
}

func TestProgressTracker_MarkStageDone(t *testing.T) {
	buf := new(bytes.Buffer)
	pt := NewProgressTracker(buf)

	pt.OnProgress(service.PipelineProgress{
		Stage:          service.StageImageGenerate,
		ScenesTotal:    10,
		ScenesComplete: 5,
	})
	pt.MarkStageDone(service.StageImageGenerate)

	// Verify state was updated
	pt.mu.Lock()
	state := pt.stages[service.StageImageGenerate]
	pt.mu.Unlock()
	assert.Equal(t, "done", state.Status)
}

func TestStageIcon(t *testing.T) {
	assert.Equal(t, "1/8", stageIcon(service.StageDataLoad))
	assert.Equal(t, "8/8", stageIcon(service.StageAssemble))
	assert.Equal(t, "...", stageIcon("unknown"))
}

func TestStageName(t *testing.T) {
	assert.Equal(t, "data", stageName(service.StageDataLoad))
	assert.Equal(t, "scenario", stageName(service.StageScenarioGenerate))
	assert.Equal(t, "image", stageName(service.StageImageGenerate))
	assert.Equal(t, "tts", stageName(service.StageTTSSynthesize))
	assert.Equal(t, "assembly", stageName(service.StageAssemble))
}

func TestProgressBar(t *testing.T) {
	bar := progressBar(0.5, 10)
	assert.Equal(t, "█████░░░░░", bar)

	bar = progressBar(1.0, 10)
	assert.Equal(t, "██████████", bar)

	bar = progressBar(0.0, 10)
	assert.Equal(t, "░░░░░░░░░░", bar)
}

func TestIsTerminal_Buffer(t *testing.T) {
	buf := new(bytes.Buffer)
	assert.False(t, isTerminal(buf))
}
