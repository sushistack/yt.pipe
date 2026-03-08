package store

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/sushistack/yt.pipe/internal/domain"
)

// CreateExecutionLog inserts a new execution log entry
func (s *Store) CreateExecutionLog(log *domain.ExecutionLog) error {
	now := time.Now().UTC()
	var jobID interface{}
	if log.JobID != "" {
		jobID = log.JobID
	}
	result, err := s.db.Exec(
		`INSERT INTO execution_logs (project_id, job_id, stage, action, status, duration_ms, estimated_cost_usd, details, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		log.ProjectID, jobID, log.Stage, log.Action, log.Status,
		log.DurationMs, log.EstimatedCostUSD, log.Details, now.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("create execution log: %w", err)
	}
	id, _ := result.LastInsertId()
	log.ID = int(id)
	log.CreatedAt = now
	return nil
}

// ListExecutionLogsByProject returns execution logs for a project
func (s *Store) ListExecutionLogsByProject(projectID string) ([]*domain.ExecutionLog, error) {
	rows, err := s.db.Query(
		`SELECT id, project_id, job_id, stage, action, status, duration_ms, estimated_cost_usd, details, created_at
		 FROM execution_logs WHERE project_id = ? ORDER BY created_at DESC`, projectID,
	)
	if err != nil {
		return nil, fmt.Errorf("list execution logs: %w", err)
	}
	defer rows.Close()

	var logs []*domain.ExecutionLog
	for rows.Next() {
		l := &domain.ExecutionLog{}
		var createdAt string
		var jobID sql.NullString
		if err := rows.Scan(&l.ID, &l.ProjectID, &jobID, &l.Stage, &l.Action, &l.Status,
			&l.DurationMs, &l.EstimatedCostUSD, &l.Details, &createdAt); err != nil {
			return nil, fmt.Errorf("scan execution log: %w", err)
		}
		if jobID.Valid {
			l.JobID = jobID.String
		}
		l.CreatedAt = parseTime(createdAt)
		logs = append(logs, l)
	}
	return logs, rows.Err()
}

// ListAllExecutionLogs returns all execution logs across all projects.
func (s *Store) ListAllExecutionLogs() ([]*domain.ExecutionLog, error) {
	rows, err := s.db.Query(
		`SELECT id, project_id, job_id, stage, action, status, duration_ms, estimated_cost_usd, details, created_at
		 FROM execution_logs ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("list all execution logs: %w", err)
	}
	defer rows.Close()

	var logs []*domain.ExecutionLog
	for rows.Next() {
		l := &domain.ExecutionLog{}
		var createdAt string
		var jobID sql.NullString
		if err := rows.Scan(&l.ID, &l.ProjectID, &jobID, &l.Stage, &l.Action, &l.Status,
			&l.DurationMs, &l.EstimatedCostUSD, &l.Details, &createdAt); err != nil {
			return nil, fmt.Errorf("scan execution log: %w", err)
		}
		if jobID.Valid {
			l.JobID = jobID.String
		}
		l.CreatedAt = parseTime(createdAt)
		logs = append(logs, l)
	}
	return logs, rows.Err()
}
