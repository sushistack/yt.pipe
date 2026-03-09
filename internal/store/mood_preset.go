package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/sushistack/yt.pipe/internal/domain"
)

// CreateMoodPreset inserts a new mood preset.
func (s *Store) CreateMoodPreset(p *domain.MoodPreset) error {
	now := time.Now().UTC()
	params := p.ParamsJSON
	if params == nil {
		params = map[string]any{}
	}
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return fmt.Errorf("marshal params_json: %w", err)
	}

	_, err = s.db.Exec(
		`INSERT INTO mood_presets (id, name, description, speed, emotion, pitch, params_json, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.ID, p.Name, p.Description, p.Speed, p.Emotion, p.Pitch, string(paramsJSON),
		now.Format(time.RFC3339), now.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("create mood preset: %w", err)
	}
	p.CreatedAt = now
	p.UpdatedAt = now
	return nil
}

// GetMoodPreset retrieves a mood preset by ID.
func (s *Store) GetMoodPreset(id string) (*domain.MoodPreset, error) {
	p := &domain.MoodPreset{}
	var paramsJSON, createdAt, updatedAt string
	err := s.db.QueryRow(
		`SELECT id, name, description, speed, emotion, pitch, params_json, created_at, updated_at
		 FROM mood_presets WHERE id = ?`, id,
	).Scan(&p.ID, &p.Name, &p.Description, &p.Speed, &p.Emotion, &p.Pitch, &paramsJSON, &createdAt, &updatedAt)
	if err == sql.ErrNoRows {
		return nil, &domain.NotFoundError{Resource: "mood_preset", ID: id}
	}
	if err != nil {
		return nil, fmt.Errorf("get mood preset: %w", err)
	}
	if err := json.Unmarshal([]byte(paramsJSON), &p.ParamsJSON); err != nil {
		return nil, fmt.Errorf("unmarshal params_json: %w", err)
	}
	p.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	p.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	return p, nil
}

// GetMoodPresetByName retrieves a mood preset by unique name.
func (s *Store) GetMoodPresetByName(name string) (*domain.MoodPreset, error) {
	p := &domain.MoodPreset{}
	var paramsJSON, createdAt, updatedAt string
	err := s.db.QueryRow(
		`SELECT id, name, description, speed, emotion, pitch, params_json, created_at, updated_at
		 FROM mood_presets WHERE name = ?`, name,
	).Scan(&p.ID, &p.Name, &p.Description, &p.Speed, &p.Emotion, &p.Pitch, &paramsJSON, &createdAt, &updatedAt)
	if err == sql.ErrNoRows {
		return nil, &domain.NotFoundError{Resource: "mood_preset", ID: name}
	}
	if err != nil {
		return nil, fmt.Errorf("get mood preset by name: %w", err)
	}
	if err := json.Unmarshal([]byte(paramsJSON), &p.ParamsJSON); err != nil {
		return nil, fmt.Errorf("unmarshal params_json: %w", err)
	}
	p.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	p.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	return p, nil
}

// ListMoodPresets returns all mood presets ordered by name.
func (s *Store) ListMoodPresets() ([]*domain.MoodPreset, error) {
	rows, err := s.db.Query(
		`SELECT id, name, description, speed, emotion, pitch, params_json, created_at, updated_at
		 FROM mood_presets ORDER BY name`,
	)
	if err != nil {
		return nil, fmt.Errorf("list mood presets: %w", err)
	}
	defer rows.Close()
	return scanMoodPresets(rows)
}

// UpdateMoodPreset updates all fields of an existing mood preset.
func (s *Store) UpdateMoodPreset(p *domain.MoodPreset) error {
	now := time.Now().UTC()
	paramsJSON, err := json.Marshal(p.ParamsJSON)
	if err != nil {
		return fmt.Errorf("marshal params_json: %w", err)
	}

	result, err := s.db.Exec(
		`UPDATE mood_presets SET name=?, description=?, speed=?, emotion=?, pitch=?, params_json=?, updated_at=?
		 WHERE id=?`,
		p.Name, p.Description, p.Speed, p.Emotion, p.Pitch, string(paramsJSON),
		now.Format(time.RFC3339), p.ID,
	)
	if err != nil {
		return fmt.Errorf("update mood preset: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return &domain.NotFoundError{Resource: "mood_preset", ID: p.ID}
	}
	p.UpdatedAt = now
	return nil
}

// DeleteMoodPreset removes a mood preset by ID.
// Fails if scene assignments reference the preset (FK constraint).
func (s *Store) DeleteMoodPreset(id string) error {
	result, err := s.db.Exec(`DELETE FROM mood_presets WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete mood preset: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return &domain.NotFoundError{Resource: "mood_preset", ID: id}
	}
	return nil
}

// AssignMoodToScene creates or updates a scene mood assignment.
func (s *Store) AssignMoodToScene(projectID string, sceneNum int, presetID string, autoMapped bool) error {
	autoInt := 0
	if autoMapped {
		autoInt = 1
	}
	_, err := s.db.Exec(
		`INSERT INTO scene_mood_assignments (project_id, scene_num, preset_id, auto_mapped, confirmed)
		 VALUES (?, ?, ?, ?, 0)
		 ON CONFLICT(project_id, scene_num) DO UPDATE SET preset_id=?, auto_mapped=?, confirmed=0`,
		projectID, sceneNum, presetID, autoInt, presetID, autoInt,
	)
	if err != nil {
		return fmt.Errorf("assign mood to scene: %w", err)
	}
	return nil
}

// ConfirmSceneMood sets confirmed=true for a scene mood assignment.
func (s *Store) ConfirmSceneMood(projectID string, sceneNum int) error {
	result, err := s.db.Exec(
		`UPDATE scene_mood_assignments SET confirmed=1 WHERE project_id=? AND scene_num=?`,
		projectID, sceneNum,
	)
	if err != nil {
		return fmt.Errorf("confirm scene mood: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return &domain.NotFoundError{Resource: "scene_mood_assignment", ID: fmt.Sprintf("%s/%d", projectID, sceneNum)}
	}
	return nil
}

// GetSceneMoodAssignment retrieves a scene mood assignment.
func (s *Store) GetSceneMoodAssignment(projectID string, sceneNum int) (*domain.SceneMoodAssignment, error) {
	a := &domain.SceneMoodAssignment{}
	var autoMapped, confirmed int
	err := s.db.QueryRow(
		`SELECT project_id, scene_num, preset_id, auto_mapped, confirmed
		 FROM scene_mood_assignments WHERE project_id=? AND scene_num=?`,
		projectID, sceneNum,
	).Scan(&a.ProjectID, &a.SceneNum, &a.PresetID, &autoMapped, &confirmed)
	if err == sql.ErrNoRows {
		return nil, &domain.NotFoundError{Resource: "scene_mood_assignment", ID: fmt.Sprintf("%s/%d", projectID, sceneNum)}
	}
	if err != nil {
		return nil, fmt.Errorf("get scene mood assignment: %w", err)
	}
	a.AutoMapped = autoMapped == 1
	a.Confirmed = confirmed == 1
	return a, nil
}

// ListSceneMoodAssignments returns all mood assignments for a project.
func (s *Store) ListSceneMoodAssignments(projectID string) ([]*domain.SceneMoodAssignment, error) {
	rows, err := s.db.Query(
		`SELECT project_id, scene_num, preset_id, auto_mapped, confirmed
		 FROM scene_mood_assignments WHERE project_id=? ORDER BY scene_num`,
		projectID,
	)
	if err != nil {
		return nil, fmt.Errorf("list scene mood assignments: %w", err)
	}
	defer rows.Close()

	var assignments []*domain.SceneMoodAssignment
	for rows.Next() {
		a := &domain.SceneMoodAssignment{}
		var autoMapped, confirmed int
		if err := rows.Scan(&a.ProjectID, &a.SceneNum, &a.PresetID, &autoMapped, &confirmed); err != nil {
			return nil, fmt.Errorf("scan scene mood assignment: %w", err)
		}
		a.AutoMapped = autoMapped == 1
		a.Confirmed = confirmed == 1
		assignments = append(assignments, a)
	}
	return assignments, rows.Err()
}

// DeleteSceneMoodAssignment removes a scene mood assignment.
func (s *Store) DeleteSceneMoodAssignment(projectID string, sceneNum int) error {
	result, err := s.db.Exec(
		`DELETE FROM scene_mood_assignments WHERE project_id=? AND scene_num=?`,
		projectID, sceneNum,
	)
	if err != nil {
		return fmt.Errorf("delete scene mood assignment: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return &domain.NotFoundError{Resource: "scene_mood_assignment", ID: fmt.Sprintf("%s/%d", projectID, sceneNum)}
	}
	return nil
}

func scanMoodPresets(rows *sql.Rows) ([]*domain.MoodPreset, error) {
	var presets []*domain.MoodPreset
	for rows.Next() {
		p := &domain.MoodPreset{}
		var paramsJSON, createdAt, updatedAt string
		if err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.Speed, &p.Emotion, &p.Pitch, &paramsJSON, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan mood preset: %w", err)
		}
		if err := json.Unmarshal([]byte(paramsJSON), &p.ParamsJSON); err != nil {
			return nil, fmt.Errorf("unmarshal params_json: %w", err)
		}
		p.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		p.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
		presets = append(presets, p)
	}
	return presets, rows.Err()
}
