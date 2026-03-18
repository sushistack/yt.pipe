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

// DeleteSceneApprovals deletes all approval records for a specific scene.
func (s *Store) DeleteSceneApprovals(projectID string, sceneNum int) error {
	_, err := s.db.Exec(
		`DELETE FROM scene_approvals WHERE project_id = ? AND scene_num = ?`,
		projectID, sceneNum,
	)
	if err != nil {
		return fmt.Errorf("delete scene approvals: %w", err)
	}
	return nil
}

// ListAllSceneValidationScores returns the minimum validation score per scene for ALL scenes
// regardless of approval status. Used by batch preview which shows all scenes.
func (s *Store) ListAllSceneValidationScores(projectID, assetType string) ([]SceneValidationScore, error) {
	rows, err := s.db.Query(
		`SELECT sa.scene_num,
		        CASE WHEN COUNT(sm.id) = 0 THEN NULL
		             WHEN COUNT(sm.validation_score) < COUNT(sm.id) THEN NULL
		             ELSE MIN(sm.validation_score)
		        END AS min_score
		 FROM scene_approvals sa
		 LEFT JOIN shot_manifests sm ON sa.project_id = sm.project_id AND sa.scene_num = sm.scene_num
		 WHERE sa.project_id = ? AND sa.asset_type = ?
		 GROUP BY sa.scene_num
		 ORDER BY sa.scene_num`,
		projectID, assetType,
	)
	if err != nil {
		return nil, fmt.Errorf("list all scene validation scores: %w", err)
	}
	defer rows.Close()

	var results []SceneValidationScore
	for rows.Next() {
		var svs SceneValidationScore
		var score sql.NullInt64
		if err := rows.Scan(&svs.SceneNum, &score); err != nil {
			return nil, fmt.Errorf("scan scene validation score: %w", err)
		}
		if score.Valid {
			v := int(score.Int64)
			svs.ValidationScore = &v
		}
		results = append(results, svs)
	}
	return results, rows.Err()
}

// DeleteSceneManifest deletes the manifest record for a specific scene.
func (s *Store) DeleteSceneManifest(projectID string, sceneNum int) error {
	_, err := s.db.Exec(
		`DELETE FROM scene_manifests WHERE project_id = ? AND scene_num = ?`,
		projectID, sceneNum,
	)
	if err != nil {
		return fmt.Errorf("delete scene manifest: %w", err)
	}
	return nil
}

// BulkApproveGenerated sets all "generated" scene approvals to "approved" for a project+assetType.
// Returns the count of approved and skipped scenes.
func (s *Store) BulkApproveGenerated(projectID, assetType string) (approved int64, err error) {
	now := time.Now().UTC().Format(time.RFC3339)
	result, err := s.db.Exec(
		`UPDATE scene_approvals SET status = ?, updated_at = ?
		 WHERE project_id = ? AND asset_type = ? AND status = ?`,
		domain.ApprovalApproved, now, projectID, assetType, domain.ApprovalGenerated,
	)
	if err != nil {
		return 0, fmt.Errorf("bulk approve generated: %w", err)
	}
	approved, _ = result.RowsAffected()
	return approved, nil
}

// MaxSceneNum returns the maximum scene number for a project's approvals.
func (s *Store) MaxSceneNum(projectID string) (int, error) {
	var maxNum sql.NullInt64
	err := s.db.QueryRow(
		`SELECT MAX(scene_num) FROM scene_approvals WHERE project_id = ?`,
		projectID,
	).Scan(&maxNum)
	if err != nil {
		return 0, fmt.Errorf("max scene num: %w", err)
	}
	if !maxNum.Valid {
		return 0, nil
	}
	return int(maxNum.Int64), nil
}

// RenumberSceneApprovalsTx shifts scene_num by delta for all approvals where scene_num > afterNum.
// Uses a two-pass negative-temp approach to avoid unique constraint violations without ORDER BY.
func RenumberSceneApprovalsTx(tx *sql.Tx, projectID string, afterNum int, delta int) error {
	// Pass 1: negate scene_num to temp values (no collision possible with positive values)
	_, err := tx.Exec(
		`UPDATE scene_approvals SET scene_num = -(scene_num + ?)
		 WHERE project_id = ? AND scene_num > ?`,
		delta, projectID, afterNum,
	)
	if err != nil {
		return fmt.Errorf("renumber scene approvals (pass 1): %w", err)
	}
	// Pass 2: flip negative back to positive final values
	_, err = tx.Exec(
		`UPDATE scene_approvals SET scene_num = -scene_num
		 WHERE project_id = ? AND scene_num < 0`,
		projectID,
	)
	if err != nil {
		return fmt.Errorf("renumber scene approvals (pass 2): %w", err)
	}
	return nil
}

// RenumberSceneManifestsTx shifts scene_num by delta for all manifests where scene_num > afterNum.
func RenumberSceneManifestsTx(tx *sql.Tx, projectID string, afterNum int, delta int) error {
	_, err := tx.Exec(
		`UPDATE scene_manifests SET scene_num = -(scene_num + ?)
		 WHERE project_id = ? AND scene_num > ?`,
		delta, projectID, afterNum,
	)
	if err != nil {
		return fmt.Errorf("renumber scene manifests (pass 1): %w", err)
	}
	_, err = tx.Exec(
		`UPDATE scene_manifests SET scene_num = -scene_num
		 WHERE project_id = ? AND scene_num < 0`,
		projectID,
	)
	if err != nil {
		return fmt.Errorf("renumber scene manifests (pass 2): %w", err)
	}
	return nil
}

// SceneValidationScore holds a scene number and its minimum validation score across all shots.
type SceneValidationScore struct {
	SceneNum        int
	ValidationScore *int // nil if any shot has NULL score or no shots exist
}

// ListSceneValidationScores returns the minimum validation score per scene for all scenes
// with "generated" approval status. A scene's score is the minimum across all its shots.
// If any shot in a scene has NULL validation_score, the scene score is nil.
func (s *Store) ListSceneValidationScores(projectID, assetType string) ([]SceneValidationScore, error) {
	rows, err := s.db.Query(
		`SELECT sa.scene_num,
		        CASE WHEN COUNT(sm.id) = 0 THEN NULL
		             WHEN COUNT(sm.validation_score) < COUNT(sm.id) THEN NULL
		             ELSE MIN(sm.validation_score)
		        END AS min_score
		 FROM scene_approvals sa
		 LEFT JOIN shot_manifests sm ON sa.project_id = sm.project_id AND sa.scene_num = sm.scene_num
		 WHERE sa.project_id = ? AND sa.asset_type = ? AND sa.status = ?
		 GROUP BY sa.scene_num
		 ORDER BY sa.scene_num`,
		projectID, assetType, domain.ApprovalGenerated,
	)
	if err != nil {
		return nil, fmt.Errorf("list scene validation scores: %w", err)
	}
	defer rows.Close()

	var results []SceneValidationScore
	for rows.Next() {
		var svs SceneValidationScore
		var score sql.NullInt64
		if err := rows.Scan(&svs.SceneNum, &score); err != nil {
			return nil, fmt.Errorf("scan scene validation score: %w", err)
		}
		if score.Valid {
			v := int(score.Int64)
			svs.ValidationScore = &v
		}
		results = append(results, svs)
	}
	return results, rows.Err()
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
