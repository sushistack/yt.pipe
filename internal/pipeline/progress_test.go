package pipeline

import (
	"bytes"
	"testing"

	"github.com/jay/youtube-pipeline/internal/service"
	"github.com/stretchr/testify/assert"
)

func TestProgressTracker_OnProgress_WithScenes(t *testing.T) {
	buf := new(bytes.Buffer)
	pt := NewProgressTracker(buf)

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

func TestStageIcon(t *testing.T) {
	assert.Equal(t, "1/8", stageIcon(service.StageDataLoad))
	assert.Equal(t, "8/8", stageIcon(service.StageAssemble))
	assert.Equal(t, "...", stageIcon("unknown"))
}
