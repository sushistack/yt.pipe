package store

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/sushistack/yt.pipe/internal/domain"
)

// VoiceCache represents a cached voice enrollment result for a project.
type VoiceCache struct {
	ProjectID  string    `json:"project_id"`
	VoiceID    string    `json:"voice_id"`
	SamplePath string    `json:"sample_path"`
	CreatedAt  time.Time `json:"created_at"`
}

// GetCachedVoice retrieves the cached voice for a project.
func (s *Store) GetCachedVoice(projectID string) (*VoiceCache, error) {
	vc := &VoiceCache{}
	var createdAt string
	err := s.db.QueryRow(
		`SELECT project_id, voice_id, sample_path, created_at FROM voice_cache WHERE project_id = ?`,
		projectID,
	).Scan(&vc.ProjectID, &vc.VoiceID, &vc.SamplePath, &createdAt)
	if err == sql.ErrNoRows {
		return nil, &domain.NotFoundError{Resource: "voice_cache", ID: projectID}
	}
	if err != nil {
		return nil, fmt.Errorf("get cached voice: %w", err)
	}
	vc.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	return vc, nil
}

// CacheVoice stores or updates the voice cache for a project.
func (s *Store) CacheVoice(projectID, voiceID, samplePath string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.Exec(
		`INSERT INTO voice_cache (project_id, voice_id, sample_path, created_at)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT(project_id) DO UPDATE SET voice_id=excluded.voice_id, sample_path=excluded.sample_path, created_at=excluded.created_at`,
		projectID, voiceID, samplePath, now,
	)
	if err != nil {
		return fmt.Errorf("cache voice: %w", err)
	}
	return nil
}

// DeleteCachedVoice removes the voice cache for a project.
func (s *Store) DeleteCachedVoice(projectID string) error {
	result, err := s.db.Exec(`DELETE FROM voice_cache WHERE project_id = ?`, projectID)
	if err != nil {
		return fmt.Errorf("delete cached voice: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return &domain.NotFoundError{Resource: "voice_cache", ID: projectID}
	}
	return nil
}

// UpdateSelectedImagePath updates the selected character image path.
func (s *Store) UpdateSelectedImagePath(characterID, imagePath string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	result, err := s.db.Exec(
		`UPDATE characters SET selected_image_path = ?, updated_at = ? WHERE id = ?`,
		imagePath, now, characterID,
	)
	if err != nil {
		return fmt.Errorf("update selected image path: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return &domain.NotFoundError{Resource: "character", ID: characterID}
	}
	return nil
}
