package service

import (
	"github.com/jay/youtube-pipeline/internal/domain"
)

// ExecutionSummary aggregates execution log entries into a summary.
type ExecutionSummary struct {
	ProjectID      string         `json:"project_id"`
	TotalDurationMs int64         `json:"total_duration_ms"`
	StageCount     int            `json:"stage_count"`
	APICallCount   int            `json:"api_call_count"`
	EstimatedCost  float64        `json:"estimated_cost_usd"`
	Stages         []StageSummary `json:"stages"`
	SuccessCount   int            `json:"success_count"`
	FailureCount   int            `json:"failure_count"`
}

// StageSummary summarizes execution logs for a specific stage.
type StageSummary struct {
	Stage       string  `json:"stage"`
	Count       int     `json:"count"`
	TotalMs     int64   `json:"total_ms"`
	SuccessRate float64 `json:"success_rate"`
	Cost        float64 `json:"estimated_cost_usd"`
}

// stageSummaryBuilder is an internal builder for computing StageSummary.
type stageSummaryBuilder struct {
	StageSummary
	successCount int
}

// BuildExecutionSummary creates an ExecutionSummary from a list of execution logs.
func BuildExecutionSummary(projectID string, logs []*domain.ExecutionLog) *ExecutionSummary {
	summary := &ExecutionSummary{
		ProjectID: projectID,
	}

	stageMap := make(map[string]*stageSummaryBuilder)
	stageOrder := make([]string, 0)

	for _, log := range logs {
		// Total duration
		if log.DurationMs != nil {
			summary.TotalDurationMs += int64(*log.DurationMs)
		}

		// Estimated cost
		if log.EstimatedCostUSD != nil {
			summary.EstimatedCost += *log.EstimatedCostUSD
		}

		// Count API calls (actions that involve external services)
		if isAPICall(log.Action) {
			summary.APICallCount++
		}

		// Success/failure counts
		if log.Status == "completed" || log.Status == "success" {
			summary.SuccessCount++
		} else if log.Status == "failed" || log.Status == "error" {
			summary.FailureCount++
		}

		// Per-stage aggregation
		ss, ok := stageMap[log.Stage]
		if !ok {
			ss = &stageSummaryBuilder{StageSummary: StageSummary{Stage: log.Stage}}
			stageMap[log.Stage] = ss
			stageOrder = append(stageOrder, log.Stage)
		}
		ss.Count++
		if log.DurationMs != nil {
			ss.TotalMs += int64(*log.DurationMs)
		}
		if log.EstimatedCostUSD != nil {
			ss.Cost += *log.EstimatedCostUSD
		}
		if log.Status == "completed" || log.Status == "success" {
			ss.successCount++
		}
	}

	// Compute success rates
	for _, name := range stageOrder {
		ss := stageMap[name]
		if ss.Count > 0 {
			ss.SuccessRate = float64(ss.successCount) / float64(ss.Count) * 100
		}
		summary.Stages = append(summary.Stages, ss.StageSummary)
	}

	summary.StageCount = len(stageOrder)
	return summary
}

func isAPICall(action string) bool {
	switch action {
	case "generate_scenario", "regenerate_section",
		"generate_image", "synthesize_audio",
		"assemble":
		return true
	}
	return false
}
