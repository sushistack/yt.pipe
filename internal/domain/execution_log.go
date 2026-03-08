package domain

import "time"

// ExecutionLog represents an entry in the execution log
type ExecutionLog struct {
	ID               int
	ProjectID        string
	JobID            string
	Stage            string
	Action           string
	Status           string
	DurationMs       *int
	EstimatedCostUSD *float64
	Details          string
	CreatedAt        time.Time
}
