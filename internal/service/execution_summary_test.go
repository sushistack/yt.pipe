package service

import (
	"testing"

	"github.com/sushistack/yt.pipe/internal/domain"
	"github.com/stretchr/testify/assert"
)

func intPtr(v int) *int          { return &v }
func floatPtr(v float64) *float64 { return &v }

func TestBuildExecutionSummary_Empty(t *testing.T) {
	summary := BuildExecutionSummary("proj-1", nil)
	assert.Equal(t, "proj-1", summary.ProjectID)
	assert.Zero(t, summary.TotalDurationMs)
	assert.Zero(t, summary.StageCount)
	assert.Zero(t, summary.APICallCount)
}

func TestBuildExecutionSummary_WithLogs(t *testing.T) {
	logs := []*domain.ExecutionLog{
		{
			Stage:            "image_generate",
			Action:           "generate_image",
			Status:           "completed",
			DurationMs:       intPtr(1500),
			EstimatedCostUSD: floatPtr(0.05),
		},
		{
			Stage:            "image_generate",
			Action:           "generate_image",
			Status:           "failed",
			DurationMs:       intPtr(500),
			EstimatedCostUSD: floatPtr(0.02),
		},
		{
			Stage:            "tts_synthesize",
			Action:           "synthesize_audio",
			Status:           "completed",
			DurationMs:       intPtr(2000),
			EstimatedCostUSD: floatPtr(0.03),
		},
		{
			Stage:  "state_machine",
			Action: "transition",
			Status: "completed",
		},
	}

	summary := BuildExecutionSummary("proj-1", logs)

	assert.Equal(t, int64(4000), summary.TotalDurationMs)
	assert.Equal(t, 3, summary.APICallCount) // generate_image x2 + synthesize_audio
	assert.InDelta(t, 0.10, summary.EstimatedCost, 0.001)
	assert.Equal(t, 3, summary.SuccessCount)
	assert.Equal(t, 1, summary.FailureCount)

	// Stage summaries
	assert.Len(t, summary.Stages, 3) // image_generate, tts_synthesize, state_machine

	imgStage := summary.Stages[0]
	assert.Equal(t, "image_generate", imgStage.Stage)
	assert.Equal(t, 2, imgStage.Count)
	assert.InDelta(t, 50.0, imgStage.SuccessRate, 0.1) // 1/2 = 50%
	assert.Equal(t, int64(2000), imgStage.TotalMs)

	ttsStage := summary.Stages[1]
	assert.Equal(t, "tts_synthesize", ttsStage.Stage)
	assert.Equal(t, 1, ttsStage.Count)
	assert.InDelta(t, 100.0, ttsStage.SuccessRate, 0.1)
}

func TestIsAPICall(t *testing.T) {
	assert.True(t, isAPICall("generate_scenario"))
	assert.True(t, isAPICall("generate_image"))
	assert.True(t, isAPICall("synthesize_audio"))
	assert.True(t, isAPICall("assemble"))
	assert.False(t, isAPICall("transition"))
	assert.False(t, isAPICall(""))
}
