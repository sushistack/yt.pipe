package domain

import "time"

// Job represents an async pipeline job
type Job struct {
	ID        string
	ProjectID string
	Type      string
	Status    string
	Progress  int
	Result    string
	Error     string
	CreatedAt time.Time
	UpdatedAt time.Time
}
