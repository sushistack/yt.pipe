package store

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/sushistack/yt.pipe/internal/domain"
)

// CreateManifest inserts a new scene manifest
func (s *Store) CreateManifest(m *domain.SceneManifest) error {
	now := time.Now().UTC()
	_, err := s.db.Exec(
		`INSERT INTO scene_manifests (project_id, scene_num, content_hash, image_hash, audio_hash, subtitle_hash, status, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		m.ProjectID, m.SceneNum, m.ContentHash, m.ImageHash, m.AudioHash, m.SubtitleHash, m.Status,
		now.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("create manifest: %w", err)
	}
	m.UpdatedAt = now
	return nil
}

// GetManifest retrieves a manifest by project ID and scene number
func (s *Store) GetManifest(projectID string, sceneNum int) (*domain.SceneManifest, error) {
	m := &domain.SceneManifest{}
	var updatedAt string
	err := s.db.QueryRow(
		`SELECT project_id, scene_num, content_hash, image_hash, audio_hash, subtitle_hash, status, updated_at
		 FROM scene_manifests WHERE project_id = ? AND scene_num = ?`, projectID, sceneNum,
	).Scan(&m.ProjectID, &m.SceneNum, &m.ContentHash, &m.ImageHash, &m.AudioHash, &m.SubtitleHash, &m.Status, &updatedAt)
	if err == sql.ErrNoRows {
		return nil, &domain.NotFoundError{Resource: "manifest", ID: fmt.Sprintf("%s/scene-%d", projectID, sceneNum)}
	}
	if err != nil {
		return nil, fmt.Errorf("get manifest: %w", err)
	}
	m.UpdatedAt, err = time.Parse(time.RFC3339, updatedAt)
	if err != nil {
		return nil, fmt.Errorf("parse manifest updated_at: %w", err)
	}
	return m, nil
}

// ListManifestsByProject returns all manifests for a project
func (s *Store) ListManifestsByProject(projectID string) ([]*domain.SceneManifest, error) {
	rows, err := s.db.Query(
		`SELECT project_id, scene_num, content_hash, image_hash, audio_hash, subtitle_hash, status, updated_at
		 FROM scene_manifests WHERE project_id = ? ORDER BY scene_num`, projectID,
	)
	if err != nil {
		return nil, fmt.Errorf("list manifests: %w", err)
	}
	defer rows.Close()

	var manifests []*domain.SceneManifest
	for rows.Next() {
		m := &domain.SceneManifest{}
		var updatedAt string
		if err := rows.Scan(&m.ProjectID, &m.SceneNum, &m.ContentHash, &m.ImageHash, &m.AudioHash, &m.SubtitleHash, &m.Status, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan manifest: %w", err)
		}
		m.UpdatedAt, err = time.Parse(time.RFC3339, updatedAt)
		if err != nil {
			return nil, fmt.Errorf("parse manifest updated_at: %w", err)
		}
		manifests = append(manifests, m)
	}
	return manifests, rows.Err()
}

// UpdateManifest updates an existing manifest
func (s *Store) UpdateManifest(m *domain.SceneManifest) error {
	now := time.Now().UTC()
	result, err := s.db.Exec(
		`UPDATE scene_manifests SET content_hash=?, image_hash=?, audio_hash=?, subtitle_hash=?, status=?, updated_at=?
		 WHERE project_id=? AND scene_num=?`,
		m.ContentHash, m.ImageHash, m.AudioHash, m.SubtitleHash, m.Status,
		now.Format(time.RFC3339), m.ProjectID, m.SceneNum,
	)
	if err != nil {
		return fmt.Errorf("update manifest: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return &domain.NotFoundError{Resource: "manifest", ID: fmt.Sprintf("%s/scene-%d", m.ProjectID, m.SceneNum)}
	}
	m.UpdatedAt = now
	return nil
}
