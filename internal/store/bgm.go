package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/sushistack/yt.pipe/internal/domain"
)

// CreateBGM inserts a new BGM into the database.
func (s *Store) CreateBGM(b *domain.BGM) error {
	now := time.Now().UTC()
	tagsJSON, err := json.Marshal(b.MoodTags)
	if err != nil {
		return fmt.Errorf("marshal mood_tags: %w", err)
	}

	_, err = s.db.Exec(
		`INSERT INTO bgms (id, name, file_path, mood_tags, duration_ms, license_type, license_source, credit_text, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		b.ID, b.Name, b.FilePath, string(tagsJSON),
		b.DurationMs, string(b.LicenseType), b.LicenseSource, b.CreditText,
		now.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("create bgm: %w", err)
	}
	b.CreatedAt = now
	return nil
}

// GetBGM retrieves a BGM by ID.
func (s *Store) GetBGM(id string) (*domain.BGM, error) {
	b := &domain.BGM{}
	var tagsJSON, licenseType, createdAt string
	err := s.db.QueryRow(
		`SELECT id, name, file_path, mood_tags, duration_ms, license_type, license_source, credit_text, created_at
		 FROM bgms WHERE id = ?`, id,
	).Scan(&b.ID, &b.Name, &b.FilePath, &tagsJSON,
		&b.DurationMs, &licenseType, &b.LicenseSource, &b.CreditText,
		&createdAt)
	if err == sql.ErrNoRows {
		return nil, &domain.NotFoundError{Resource: "bgm", ID: id}
	}
	if err != nil {
		return nil, fmt.Errorf("get bgm: %w", err)
	}
	if err := json.Unmarshal([]byte(tagsJSON), &b.MoodTags); err != nil {
		return nil, fmt.Errorf("unmarshal mood_tags: %w", err)
	}
	b.LicenseType = domain.LicenseType(licenseType)
	b.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	return b, nil
}

// ListBGMs returns all BGMs ordered by name.
func (s *Store) ListBGMs() ([]*domain.BGM, error) {
	rows, err := s.db.Query(
		`SELECT id, name, file_path, mood_tags, duration_ms, license_type, license_source, credit_text, created_at
		 FROM bgms ORDER BY name`,
	)
	if err != nil {
		return nil, fmt.Errorf("list bgms: %w", err)
	}
	defer rows.Close()
	return scanBGMs(rows)
}

// UpdateBGM updates all fields of an existing BGM.
func (s *Store) UpdateBGM(b *domain.BGM) error {
	tagsJSON, err := json.Marshal(b.MoodTags)
	if err != nil {
		return fmt.Errorf("marshal mood_tags: %w", err)
	}

	result, err := s.db.Exec(
		`UPDATE bgms SET name=?, file_path=?, mood_tags=?, duration_ms=?, license_type=?, license_source=?, credit_text=?
		 WHERE id=?`,
		b.Name, b.FilePath, string(tagsJSON),
		b.DurationMs, string(b.LicenseType), b.LicenseSource, b.CreditText,
		b.ID,
	)
	if err != nil {
		return fmt.Errorf("update bgm: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return &domain.NotFoundError{Resource: "bgm", ID: b.ID}
	}
	return nil
}

// DeleteBGM removes a BGM by ID. Fails if scene assignments reference it.
func (s *Store) DeleteBGM(id string) error {
	// Check for existing scene assignments
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM scene_bgm_assignments WHERE bgm_id = ?`, id).Scan(&count)
	if err != nil {
		return fmt.Errorf("check bgm assignments: %w", err)
	}
	if count > 0 {
		return fmt.Errorf("cannot delete bgm %s: %d scene assignment(s) reference it", id, count)
	}

	result, err := s.db.Exec(`DELETE FROM bgms WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete bgm: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return &domain.NotFoundError{Resource: "bgm", ID: id}
	}
	return nil
}

// SearchByMoodTags returns BGMs matching any of the given tags, ranked by match count.
func (s *Store) SearchByMoodTags(tags []string) ([]*domain.BGM, error) {
	if len(tags) == 0 {
		return nil, nil
	}

	// Build query that counts matches across all tags using JSON LIKE
	// Each tag is checked with LIKE on the mood_tags JSON column
	conditions := make([]string, len(tags))
	args := make([]interface{}, len(tags))
	for i, tag := range tags {
		conditions[i] = fmt.Sprintf(`(mood_tags LIKE '%%"%s"%%')`, tag)
		args[i] = tag
	}

	// Sum matching conditions for ranking, then filter rows with at least one match
	matchExpr := ""
	for i, cond := range conditions {
		if i > 0 {
			matchExpr += " + "
		}
		matchExpr += fmt.Sprintf("CASE WHEN %s THEN 1 ELSE 0 END", cond)
	}

	query := fmt.Sprintf(
		`SELECT id, name, file_path, mood_tags, duration_ms, license_type, license_source, credit_text, created_at
		 FROM bgms
		 WHERE (%s) > 0
		 ORDER BY (%s) DESC, name`,
		matchExpr, matchExpr,
	)

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("search bgms by mood tags: %w", err)
	}
	defer rows.Close()
	return scanBGMs(rows)
}

// AssignBGMToScene creates or updates a BGM assignment for a scene.
func (s *Store) AssignBGMToScene(a *domain.SceneBGMAssignment) error {
	_, err := s.db.Exec(
		`INSERT INTO scene_bgm_assignments (project_id, scene_num, bgm_id, volume_db, fade_in_ms, fade_out_ms, ducking_db, auto_recommended, confirmed)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(project_id, scene_num) DO UPDATE SET
		   bgm_id=excluded.bgm_id, volume_db=excluded.volume_db,
		   fade_in_ms=excluded.fade_in_ms, fade_out_ms=excluded.fade_out_ms,
		   ducking_db=excluded.ducking_db, auto_recommended=excluded.auto_recommended,
		   confirmed=excluded.confirmed`,
		a.ProjectID, a.SceneNum, a.BGMID,
		a.VolumeDB, a.FadeInMs, a.FadeOutMs, a.DuckingDB,
		boolToInt(a.AutoRecommended), boolToInt(a.Confirmed),
	)
	if err != nil {
		return fmt.Errorf("assign bgm to scene: %w", err)
	}
	return nil
}

// ConfirmSceneBGM sets confirmed=true for a scene assignment.
func (s *Store) ConfirmSceneBGM(projectID string, sceneNum int) error {
	result, err := s.db.Exec(
		`UPDATE scene_bgm_assignments SET confirmed = 1 WHERE project_id = ? AND scene_num = ?`,
		projectID, sceneNum,
	)
	if err != nil {
		return fmt.Errorf("confirm scene bgm: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return &domain.NotFoundError{Resource: "scene_bgm_assignment", ID: fmt.Sprintf("%s/%d", projectID, sceneNum)}
	}
	return nil
}

// GetSceneBGMAssignment returns the BGM assignment for a specific scene.
func (s *Store) GetSceneBGMAssignment(projectID string, sceneNum int) (*domain.SceneBGMAssignment, error) {
	a := &domain.SceneBGMAssignment{}
	var autoRec, confirmed int
	err := s.db.QueryRow(
		`SELECT project_id, scene_num, bgm_id, volume_db, fade_in_ms, fade_out_ms, ducking_db, auto_recommended, confirmed
		 FROM scene_bgm_assignments WHERE project_id = ? AND scene_num = ?`,
		projectID, sceneNum,
	).Scan(&a.ProjectID, &a.SceneNum, &a.BGMID,
		&a.VolumeDB, &a.FadeInMs, &a.FadeOutMs, &a.DuckingDB,
		&autoRec, &confirmed)
	if err == sql.ErrNoRows {
		return nil, &domain.NotFoundError{Resource: "scene_bgm_assignment", ID: fmt.Sprintf("%s/%d", projectID, sceneNum)}
	}
	if err != nil {
		return nil, fmt.Errorf("get scene bgm assignment: %w", err)
	}
	a.AutoRecommended = autoRec == 1
	a.Confirmed = confirmed == 1
	return a, nil
}

// ListSceneBGMAssignments returns all BGM assignments for a project.
func (s *Store) ListSceneBGMAssignments(projectID string) ([]*domain.SceneBGMAssignment, error) {
	rows, err := s.db.Query(
		`SELECT project_id, scene_num, bgm_id, volume_db, fade_in_ms, fade_out_ms, ducking_db, auto_recommended, confirmed
		 FROM scene_bgm_assignments WHERE project_id = ? ORDER BY scene_num`,
		projectID,
	)
	if err != nil {
		return nil, fmt.Errorf("list scene bgm assignments: %w", err)
	}
	defer rows.Close()

	var assignments []*domain.SceneBGMAssignment
	for rows.Next() {
		a := &domain.SceneBGMAssignment{}
		var autoRec, confirmed int
		if err := rows.Scan(&a.ProjectID, &a.SceneNum, &a.BGMID,
			&a.VolumeDB, &a.FadeInMs, &a.FadeOutMs, &a.DuckingDB,
			&autoRec, &confirmed); err != nil {
			return nil, fmt.Errorf("scan scene bgm assignment: %w", err)
		}
		a.AutoRecommended = autoRec == 1
		a.Confirmed = confirmed == 1
		assignments = append(assignments, a)
	}
	return assignments, rows.Err()
}

// RenumberSceneBGMTx shifts scene_num by delta for all BGM assignments where scene_num > afterNum.
func RenumberSceneBGMTx(tx *sql.Tx, projectID string, afterNum int, delta int) error {
	_, err := tx.Exec(
		`UPDATE scene_bgm_assignments SET scene_num = -(scene_num + ?)
		 WHERE project_id = ? AND scene_num > ?`,
		delta, projectID, afterNum,
	)
	if err != nil {
		return fmt.Errorf("renumber scene bgm (pass 1): %w", err)
	}
	_, err = tx.Exec(
		`UPDATE scene_bgm_assignments SET scene_num = -scene_num
		 WHERE project_id = ? AND scene_num < 0`,
		projectID,
	)
	if err != nil {
		return fmt.Errorf("renumber scene bgm (pass 2): %w", err)
	}
	return nil
}

// DeleteSceneBGM removes the BGM assignment for a specific scene.
func (s *Store) DeleteSceneBGM(projectID string, sceneNum int) error {
	_, err := s.db.Exec(
		`DELETE FROM scene_bgm_assignments WHERE project_id = ? AND scene_num = ?`,
		projectID, sceneNum,
	)
	if err != nil {
		return fmt.Errorf("delete scene bgm: %w", err)
	}
	return nil
}

func scanBGMs(rows *sql.Rows) ([]*domain.BGM, error) {
	var bgms []*domain.BGM
	for rows.Next() {
		b := &domain.BGM{}
		var tagsJSON, licenseType, createdAt string
		if err := rows.Scan(&b.ID, &b.Name, &b.FilePath, &tagsJSON,
			&b.DurationMs, &licenseType, &b.LicenseSource, &b.CreditText,
			&createdAt); err != nil {
			return nil, fmt.Errorf("scan bgm: %w", err)
		}
		if err := json.Unmarshal([]byte(tagsJSON), &b.MoodTags); err != nil {
			return nil, fmt.Errorf("unmarshal mood_tags: %w", err)
		}
		b.LicenseType = domain.LicenseType(licenseType)
		b.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		bgms = append(bgms, b)
	}
	return bgms, rows.Err()
}
