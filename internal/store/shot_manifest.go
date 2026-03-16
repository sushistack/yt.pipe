package store

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/sushistack/yt.pipe/internal/domain"
)

// CreateShotManifest inserts a new shot manifest.
func (s *Store) CreateShotManifest(m *domain.ShotManifest) error {
	now := time.Now().UTC()
	_, err := s.db.Exec(
		`INSERT INTO shot_manifests (project_id, scene_num, shot_num, content_hash, image_hash, gen_method, status, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		m.ProjectID, m.SceneNum, m.ShotNum, m.ContentHash, m.ImageHash, m.GenMethod, m.Status,
		now.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("create shot manifest: %w", err)
	}
	m.UpdatedAt = now
	return nil
}

// GetShotManifest retrieves a shot manifest by composite key.
func (s *Store) GetShotManifest(projectID string, sceneNum, shotNum int) (*domain.ShotManifest, error) {
	m := &domain.ShotManifest{}
	var updatedAt string
	err := s.db.QueryRow(
		`SELECT project_id, scene_num, shot_num, content_hash, image_hash, gen_method, status, updated_at
		 FROM shot_manifests WHERE project_id = ? AND scene_num = ? AND shot_num = ?`,
		projectID, sceneNum, shotNum,
	).Scan(&m.ProjectID, &m.SceneNum, &m.ShotNum, &m.ContentHash, &m.ImageHash, &m.GenMethod, &m.Status, &updatedAt)
	if err == sql.ErrNoRows {
		return nil, &domain.NotFoundError{Resource: "shot_manifest", ID: fmt.Sprintf("%s/scene-%d/shot-%d", projectID, sceneNum, shotNum)}
	}
	if err != nil {
		return nil, fmt.Errorf("get shot manifest: %w", err)
	}
	m.UpdatedAt, err = time.Parse(time.RFC3339, updatedAt)
	if err != nil {
		return nil, fmt.Errorf("parse shot manifest updated_at: %w", err)
	}
	return m, nil
}

// ListShotManifestsByScene returns all shot manifests for a scene, ordered by shot_num.
func (s *Store) ListShotManifestsByScene(projectID string, sceneNum int) ([]*domain.ShotManifest, error) {
	rows, err := s.db.Query(
		`SELECT project_id, scene_num, shot_num, content_hash, image_hash, gen_method, status, updated_at
		 FROM shot_manifests WHERE project_id = ? AND scene_num = ? ORDER BY shot_num`,
		projectID, sceneNum,
	)
	if err != nil {
		return nil, fmt.Errorf("list shot manifests: %w", err)
	}
	defer rows.Close()

	var manifests []*domain.ShotManifest
	for rows.Next() {
		m := &domain.ShotManifest{}
		var updatedAt string
		if err := rows.Scan(&m.ProjectID, &m.SceneNum, &m.ShotNum, &m.ContentHash, &m.ImageHash, &m.GenMethod, &m.Status, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan shot manifest: %w", err)
		}
		m.UpdatedAt, err = time.Parse(time.RFC3339, updatedAt)
		if err != nil {
			return nil, fmt.Errorf("parse shot manifest updated_at: %w", err)
		}
		manifests = append(manifests, m)
	}
	return manifests, rows.Err()
}

// UpdateShotManifest updates an existing shot manifest.
func (s *Store) UpdateShotManifest(m *domain.ShotManifest) error {
	now := time.Now().UTC()
	result, err := s.db.Exec(
		`UPDATE shot_manifests SET content_hash=?, image_hash=?, gen_method=?, status=?, updated_at=?
		 WHERE project_id=? AND scene_num=? AND shot_num=?`,
		m.ContentHash, m.ImageHash, m.GenMethod, m.Status,
		now.Format(time.RFC3339), m.ProjectID, m.SceneNum, m.ShotNum,
	)
	if err != nil {
		return fmt.Errorf("update shot manifest: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return &domain.NotFoundError{Resource: "shot_manifest", ID: fmt.Sprintf("%s/scene-%d/shot-%d", m.ProjectID, m.SceneNum, m.ShotNum)}
	}
	m.UpdatedAt = now
	return nil
}

// DeleteShotManifestsByScene removes all shot manifests for a given scene.
func (s *Store) DeleteShotManifestsByScene(projectID string, sceneNum int) error {
	_, err := s.db.Exec(
		`DELETE FROM shot_manifests WHERE project_id = ? AND scene_num = ?`,
		projectID, sceneNum,
	)
	if err != nil {
		return fmt.Errorf("delete shot manifests by scene: %w", err)
	}
	return nil
}
