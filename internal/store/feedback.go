package store

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/jay/youtube-pipeline/internal/domain"
)

// CreateFeedback inserts a new feedback entry.
func (s *Store) CreateFeedback(f *domain.Feedback) error {
	now := time.Now().UTC()
	result, err := s.db.Exec(
		`INSERT INTO feedback (project_id, scene_num, asset_type, rating, comment, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		f.ProjectID, f.SceneNum, f.AssetType, f.Rating, f.Comment, now.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("create feedback: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("create feedback: last insert id: %w", err)
	}
	f.ID = int(id)
	f.CreatedAt = now
	return nil
}

// ListFeedbackByProject returns all feedback for a project.
func (s *Store) ListFeedbackByProject(projectID string) ([]*domain.Feedback, error) {
	rows, err := s.db.Query(
		`SELECT id, project_id, scene_num, asset_type, rating, comment, created_at
		 FROM feedback WHERE project_id = ? ORDER BY created_at DESC`, projectID,
	)
	if err != nil {
		return nil, fmt.Errorf("list feedback: %w", err)
	}
	defer rows.Close()
	return scanFeedbackRows(rows)
}

// ListAllFeedback returns all feedback entries.
func (s *Store) ListAllFeedback() ([]*domain.Feedback, error) {
	rows, err := s.db.Query(
		`SELECT id, project_id, scene_num, asset_type, rating, comment, created_at
		 FROM feedback ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("list all feedback: %w", err)
	}
	defer rows.Close()
	return scanFeedbackRows(rows)
}

func scanFeedbackRows(rows *sql.Rows) ([]*domain.Feedback, error) {
	var feedbacks []*domain.Feedback
	for rows.Next() {
		f := &domain.Feedback{}
		var createdAt string
		var comment *string
		if err := rows.Scan(&f.ID, &f.ProjectID, &f.SceneNum, &f.AssetType, &f.Rating, &comment, &createdAt); err != nil {
			return nil, fmt.Errorf("scan feedback: %w", err)
		}
		if comment != nil {
			f.Comment = *comment
		}
		f.CreatedAt = parseTime(createdAt)
		feedbacks = append(feedbacks, f)
	}
	return feedbacks, rows.Err()
}

// parseTime tries RFC3339 first, then SQLite's default datetime format.
func parseTime(s string) time.Time {
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t
	}
	if t, err := time.Parse("2006-01-02 15:04:05", s); err == nil {
		return t
	}
	return time.Time{}
}
