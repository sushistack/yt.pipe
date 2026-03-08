package service

import (
	"testing"

	"github.com/sushistack/yt.pipe/internal/domain"
	"github.com/stretchr/testify/assert"
)

func TestBuildPipelineMetrics_Empty(t *testing.T) {
	m := BuildPipelineMetrics(nil, nil, nil)
	assert.Zero(t, m.TotalProjects)
	assert.Zero(t, m.TotalExecutions)
	assert.Zero(t, m.SuccessRate)
	assert.Nil(t, m.FeedbackSummary)
}

func TestBuildPipelineMetrics_WithData(t *testing.T) {
	projects := []*domain.Project{
		{ID: "p1"}, {ID: "p2"},
	}
	logs := []*domain.ExecutionLog{
		{Action: "generate_image", Status: "completed", EstimatedCostUSD: floatPtr(0.05)},
		{Action: "generate_image", Status: "failed", EstimatedCostUSD: floatPtr(0.02)},
		{Action: "synthesize_audio", Status: "completed", EstimatedCostUSD: floatPtr(0.03)},
		{Action: "transition", Status: "completed"},
	}
	feedbacks := []*domain.Feedback{
		{AssetType: "image", Rating: "good"},
		{AssetType: "image", Rating: "bad"},
		{AssetType: "audio", Rating: "good"},
	}

	m := BuildPipelineMetrics(projects, logs, feedbacks)

	assert.Equal(t, 2, m.TotalProjects)
	assert.Equal(t, 4, m.TotalExecutions)
	assert.Equal(t, 3, m.TotalAPICalls)
	assert.InDelta(t, 0.10, m.TotalCost, 0.001)
	assert.InDelta(t, 75.0, m.SuccessRate, 0.1) // 3/4 = 75%

	assert.NotNil(t, m.FeedbackSummary)
	assert.Equal(t, 3, m.FeedbackSummary.Total)
	assert.Equal(t, 2, m.FeedbackSummary.ByRating["good"])
	assert.Equal(t, 1, m.FeedbackSummary.ByRating["bad"])
	assert.Equal(t, 1, m.FeedbackSummary.ByType["image"].Good)
	assert.Equal(t, 1, m.FeedbackSummary.ByType["image"].Bad)
	assert.Equal(t, 1, m.FeedbackSummary.ByType["audio"].Good)
}

func TestBuildPipelineMetrics_NoFailures(t *testing.T) {
	logs := []*domain.ExecutionLog{
		{Action: "generate_image", Status: "completed"},
		{Action: "generate_image", Status: "completed"},
	}
	m := BuildPipelineMetrics(nil, logs, nil)
	assert.InDelta(t, 100.0, m.SuccessRate, 0.1)
}
