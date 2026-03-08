package store

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/sushistack/yt.pipe/internal/domain"
)

// CreateJob inserts a new job
func (s *Store) CreateJob(j *domain.Job) error {
	now := time.Now().UTC()
	_, err := s.db.Exec(
		`INSERT INTO jobs (id, project_id, type, status, progress, result, error, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		j.ID, j.ProjectID, j.Type, j.Status, j.Progress, j.Result, j.Error,
		now.Format(time.RFC3339), now.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("create job: %w", err)
	}
	j.CreatedAt = now
	j.UpdatedAt = now
	return nil
}

// GetJob retrieves a job by ID
func (s *Store) GetJob(id string) (*domain.Job, error) {
	j := &domain.Job{}
	var createdAt, updatedAt string
	err := s.db.QueryRow(
		`SELECT id, project_id, type, status, progress, result, error, created_at, updated_at
		 FROM jobs WHERE id = ?`, id,
	).Scan(&j.ID, &j.ProjectID, &j.Type, &j.Status, &j.Progress, &j.Result, &j.Error, &createdAt, &updatedAt)
	if err == sql.ErrNoRows {
		return nil, &domain.NotFoundError{Resource: "job", ID: id}
	}
	if err != nil {
		return nil, fmt.Errorf("get job: %w", err)
	}
	if parsed, err := time.Parse(time.RFC3339, createdAt); err == nil {
		j.CreatedAt = parsed
	}
	if parsed, err := time.Parse(time.RFC3339, updatedAt); err == nil {
		j.UpdatedAt = parsed
	}
	return j, nil
}

// ListJobsByProject returns all jobs for a given project
func (s *Store) ListJobsByProject(projectID string) ([]*domain.Job, error) {
	rows, err := s.db.Query(
		`SELECT id, project_id, type, status, progress, result, error, created_at, updated_at
		 FROM jobs WHERE project_id = ? ORDER BY created_at DESC`, projectID,
	)
	if err != nil {
		return nil, fmt.Errorf("list jobs: %w", err)
	}
	defer rows.Close()

	var jobs []*domain.Job
	for rows.Next() {
		j := &domain.Job{}
		var createdAt, updatedAt string
		if err := rows.Scan(&j.ID, &j.ProjectID, &j.Type, &j.Status, &j.Progress, &j.Result, &j.Error, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan job: %w", err)
		}
		if parsed, err := time.Parse(time.RFC3339, createdAt); err == nil {
			j.CreatedAt = parsed
		}
		if parsed, err := time.Parse(time.RFC3339, updatedAt); err == nil {
			j.UpdatedAt = parsed
		}
		jobs = append(jobs, j)
	}
	return jobs, rows.Err()
}

// UpdateJob updates an existing job
func (s *Store) UpdateJob(j *domain.Job) error {
	now := time.Now().UTC()
	result, err := s.db.Exec(
		`UPDATE jobs SET status=?, progress=?, result=?, error=?, updated_at=?
		 WHERE id=?`,
		j.Status, j.Progress, j.Result, j.Error, now.Format(time.RFC3339), j.ID,
	)
	if err != nil {
		return fmt.Errorf("update job: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return &domain.NotFoundError{Resource: "job", ID: j.ID}
	}
	j.UpdatedAt = now
	return nil
}
