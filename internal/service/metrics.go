package service

import (
	"github.com/sushistack/yt.pipe/internal/domain"
)

// PipelineMetrics holds aggregated pipeline metrics.
type PipelineMetrics struct {
	TotalProjects   int              `json:"total_projects"`
	TotalExecutions int              `json:"total_executions"`
	TotalAPICalls   int              `json:"total_api_calls"`
	TotalCost       float64          `json:"total_cost_usd"`
	SuccessRate     float64          `json:"success_rate"`
	FeedbackSummary *FeedbackSummary `json:"feedback,omitempty"`
}

// FeedbackSummary summarizes feedback data.
type FeedbackSummary struct {
	Total      int                       `json:"total"`
	ByRating   map[string]int            `json:"by_rating"`
	ByType     map[string]TypeFeedback   `json:"by_type"`
}

// TypeFeedback holds feedback counts for a specific asset type.
type TypeFeedback struct {
	Good    int `json:"good"`
	Bad     int `json:"bad"`
	Neutral int `json:"neutral"`
}

// BuildPipelineMetrics creates aggregated metrics from projects, execution logs, and feedback.
func BuildPipelineMetrics(
	projects []*domain.Project,
	logs []*domain.ExecutionLog,
	feedbacks []*domain.Feedback,
) *PipelineMetrics {
	m := &PipelineMetrics{
		TotalProjects: len(projects),
	}

	var successCount, failCount int
	for _, log := range logs {
		m.TotalExecutions++
		if log.EstimatedCostUSD != nil {
			m.TotalCost += *log.EstimatedCostUSD
		}
		if isAPICall(log.Action) {
			m.TotalAPICalls++
		}
		if log.Status == "completed" || log.Status == "success" {
			successCount++
		} else if log.Status == "failed" || log.Status == "error" {
			failCount++
		}
	}

	total := successCount + failCount
	if total > 0 {
		m.SuccessRate = float64(successCount) / float64(total) * 100
	}

	if len(feedbacks) > 0 {
		m.FeedbackSummary = buildFeedbackSummary(feedbacks)
	}

	return m
}

func buildFeedbackSummary(feedbacks []*domain.Feedback) *FeedbackSummary {
	fs := &FeedbackSummary{
		Total:    len(feedbacks),
		ByRating: make(map[string]int),
		ByType:   make(map[string]TypeFeedback),
	}

	for _, f := range feedbacks {
		fs.ByRating[f.Rating]++

		tf := fs.ByType[f.AssetType]
		switch f.Rating {
		case "good":
			tf.Good++
		case "bad":
			tf.Bad++
		case "neutral":
			tf.Neutral++
		}
		fs.ByType[f.AssetType] = tf
	}

	return fs
}
