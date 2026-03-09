package store

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/sushistack/yt.pipe/internal/domain"
)

// InitApproval creates a scene approval record with status=pending, attempts=0.
func (s *Store) InitApproval(projectID string, sceneNum int, assetType string) error {
	if err := domain.ValidateSceneApproval(projectID, sceneNum, assetType); err != nil {
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.Exec(
		`INSERT INTO scene_approvals (project_id, scene_num, asset_type, status, attempts, updated_at)
		 VALUES (?, ?, ?, ?, 0, ?)`,
		projectID, sceneNum, assetType, domain.ApprovalPending, now,
	)
	if err != nil {
		return fmt.Errorf("init approval: %w", err)
	}
	return nil
}

// MarkGenerated sets a scene approval to "generated" and increments attempts.
func (s *Store) MarkGenerated(projectID string, sceneNum int, assetType string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	result, err := s.db.Exec(
		`UPDATE scene_approvals SET status = ?, attempts = attempts + 1, updated_at = ?
		 WHERE project_id = ? AND scene_num = ? AND asset_type = ?`,
		domain.ApprovalGenerated, now, projectID, sceneNum, assetType,
	)
	if err != nil {
		return fmt.Errorf("mark generated: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return &domain.NotFoundError{Resource: "scene_approval", ID: fmt.Sprintf("%s/%d/%s", projectID, sceneNum, assetType)}
	}
	return nil
}

// ApproveScene sets a scene approval to "approved".
func (s *Store) ApproveScene(projectID string, sceneNum int, assetType string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	result, err := s.db.Exec(
		`UPDATE scene_approvals SET status = ?, updated_at = ?
		 WHERE project_id = ? AND scene_num = ? AND asset_type = ?`,
		domain.ApprovalApproved, now, projectID, sceneNum, assetType,
	)
	if err != nil {
		return fmt.Errorf("approve scene: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return &domain.NotFoundError{Resource: "scene_approval", ID: fmt.Sprintf("%s/%d/%s", projectID, sceneNum, assetType)}
	}
	return nil
}

// RejectScene sets a scene approval to "rejected".
func (s *Store) RejectScene(projectID string, sceneNum int, assetType string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	result, err := s.db.Exec(
		`UPDATE scene_approvals SET status = ?, updated_at = ?
		 WHERE project_id = ? AND scene_num = ? AND asset_type = ?`,
		domain.ApprovalRejected, now, projectID, sceneNum, assetType,
	)
	if err != nil {
		return fmt.Errorf("reject scene: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return &domain.NotFoundError{Resource: "scene_approval", ID: fmt.Sprintf("%s/%d/%s", projectID, sceneNum, assetType)}
	}
	return nil
}

// GetApproval retrieves a single scene approval record.
func (s *Store) GetApproval(projectID string, sceneNum int, assetType string) (*domain.SceneApproval, error) {
	a := &domain.SceneApproval{}
	var updatedAt string
	err := s.db.QueryRow(
		`SELECT project_id, scene_num, asset_type, status, attempts, updated_at
		 FROM scene_approvals WHERE project_id = ? AND scene_num = ? AND asset_type = ?`,
		projectID, sceneNum, assetType,
	).Scan(&a.ProjectID, &a.SceneNum, &a.AssetType, &a.Status, &a.Attempts, &updatedAt)
	if err == sql.ErrNoRows {
		return nil, &domain.NotFoundError{Resource: "scene_approval", ID: fmt.Sprintf("%s/%d/%s", projectID, sceneNum, assetType)}
	}
	if err != nil {
		return nil, fmt.Errorf("get approval: %w", err)
	}
	a.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	return a, nil
}

// ListApprovalsByProject returns all approvals for a project filtered by asset type.
func (s *Store) ListApprovalsByProject(projectID, assetType string) ([]*domain.SceneApproval, error) {
	rows, err := s.db.Query(
		`SELECT project_id, scene_num, asset_type, status, attempts, updated_at
		 FROM scene_approvals WHERE project_id = ? AND asset_type = ? ORDER BY scene_num`,
		projectID, assetType,
	)
	if err != nil {
		return nil, fmt.Errorf("list approvals: %w", err)
	}
	defer rows.Close()

	var approvals []*domain.SceneApproval
	for rows.Next() {
		a := &domain.SceneApproval{}
		var updatedAt string
		if err := rows.Scan(&a.ProjectID, &a.SceneNum, &a.AssetType, &a.Status, &a.Attempts, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan approval: %w", err)
		}
		a.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
		approvals = append(approvals, a)
	}
	return approvals, rows.Err()
}

// AllApproved returns true only if every scene for the given asset type has status="approved".
// Returns false if there are no approval records.
func (s *Store) AllApproved(projectID, assetType string) (bool, error) {
	var total, approved int
	err := s.db.QueryRow(
		`SELECT COUNT(*), COUNT(CASE WHEN status = ? THEN 1 END)
		 FROM scene_approvals WHERE project_id = ? AND asset_type = ?`,
		domain.ApprovalApproved, projectID, assetType,
	).Scan(&total, &approved)
	if err != nil {
		return false, fmt.Errorf("all approved: %w", err)
	}
	if total == 0 {
		return false, nil
	}
	return total == approved, nil
}

// BulkApproveAll sets all scene approvals for a project+assetType to "approved".
// Used by --skip-approval mode.
func (s *Store) BulkApproveAll(projectID, assetType string) (int64, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	result, err := s.db.Exec(
		`UPDATE scene_approvals SET status = ?, updated_at = ?
		 WHERE project_id = ? AND asset_type = ?`,
		domain.ApprovalApproved, now, projectID, assetType,
	)
	if err != nil {
		return 0, fmt.Errorf("bulk approve all: %w", err)
	}
	return result.RowsAffected()
}
