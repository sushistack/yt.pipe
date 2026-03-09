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
	j.CreatedAt, err = time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return nil, fmt.Errorf("parse job created_at: %w", err)
	}
	j.UpdatedAt, err = time.Parse(time.RFC3339, updatedAt)
	if err != nil {
		return nil, fmt.Errorf("parse job updated_at: %w", err)
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
		j.CreatedAt, err = time.Parse(time.RFC3339, createdAt)
		if err != nil {
			return nil, fmt.Errorf("parse job created_at: %w", err)
		}
		j.UpdatedAt, err = time.Parse(time.RFC3339, updatedAt)
		if err != nil {
			return nil, fmt.Errorf("parse job updated_at: %w", err)
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

// GetLatestJobByProject returns the most recent job for a project, or nil if none exists.
func (s *Store) GetLatestJobByProject(projectID string) (*domain.Job, error) {
	j := &domain.Job{}
	var createdAt, updatedAt string
	err := s.db.QueryRow(
		`SELECT id, project_id, type, status, progress, result, error, created_at, updated_at
		 FROM jobs WHERE project_id = ? ORDER BY created_at DESC LIMIT 1`, projectID,
	).Scan(&j.ID, &j.ProjectID, &j.Type, &j.Status, &j.Progress, &j.Result, &j.Error, &createdAt, &updatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get latest job by project: %w", err)
	}
	j.CreatedAt, err = time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return nil, fmt.Errorf("parse job created_at: %w", err)
	}
	j.UpdatedAt, err = time.Parse(time.RFC3339, updatedAt)
	if err != nil {
		return nil, fmt.Errorf("parse job updated_at: %w", err)
	}
	return j, nil
}

// MarkStaleJobsFailed marks all jobs with "running" status as "failed" with the given error message.
// This is used on server startup to handle jobs that were interrupted by a server restart.
func (s *Store) MarkStaleJobsFailed(errMsg string) (int64, error) {
	now := time.Now().UTC()
	result, err := s.db.Exec(
		`UPDATE jobs SET status=?, error=?, updated_at=? WHERE status=?`,
		"failed", errMsg, now.Format(time.RFC3339), "running",
	)
	if err != nil {
		return 0, fmt.Errorf("mark stale jobs failed: %w", err)
	}
	rows, _ := result.RowsAffected()
	return rows, nil
}

// GetRunningJobByProjectAndType returns a running job for the given project and type, or nil if none exists.
func (s *Store) GetRunningJobByProjectAndType(projectID, jobType string) (*domain.Job, error) {
	j := &domain.Job{}
	var createdAt, updatedAt string
	err := s.db.QueryRow(
		`SELECT id, project_id, type, status, progress, result, error, created_at, updated_at
		 FROM jobs WHERE project_id = ? AND type = ? AND status = ? ORDER BY created_at DESC LIMIT 1`,
		projectID, jobType, "running",
	).Scan(&j.ID, &j.ProjectID, &j.Type, &j.Status, &j.Progress, &j.Result, &j.Error, &createdAt, &updatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get running job by project and type: %w", err)
	}
	j.CreatedAt, err = time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return nil, fmt.Errorf("parse job created_at: %w", err)
	}
	j.UpdatedAt, err = time.Parse(time.RFC3339, updatedAt)
	if err != nil {
		return nil, fmt.Errorf("parse job updated_at: %w", err)
	}
	return j, nil
}

// PurgeOldJobs deletes completed/failed jobs older than the given retention period.
// Jobs with "running" status are never purged.
func (s *Store) PurgeOldJobs(retentionDays int) (int64, error) {
	cutoff := time.Now().UTC().Add(-time.Duration(retentionDays) * 24 * time.Hour)
	result, err := s.db.Exec(
		`DELETE FROM jobs WHERE status IN (?, ?, ?) AND created_at < ?`,
		"complete", "failed", "cancelled", cutoff.Format(time.RFC3339),
	)
	if err != nil {
		return 0, fmt.Errorf("purge old jobs: %w", err)
	}
	rows, _ := result.RowsAffected()
	return rows, nil
}
