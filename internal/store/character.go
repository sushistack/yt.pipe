package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/sushistack/yt.pipe/internal/domain"
)

// CreateCharacter inserts a new character into the database.
func (s *Store) CreateCharacter(c *domain.Character) error {
	now := time.Now().UTC()
	aliasesJSON, err := json.Marshal(c.Aliases)
	if err != nil {
		return fmt.Errorf("marshal aliases: %w", err)
	}

	_, err = s.db.Exec(
		`INSERT INTO characters (id, scp_id, canonical_name, aliases, visual_descriptor, style_guide, image_prompt_base, selected_image_path, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		c.ID, c.SCPID, c.CanonicalName, string(aliasesJSON),
		c.VisualDescriptor, c.StyleGuide, c.ImagePromptBase, c.SelectedImagePath,
		now.Format(time.RFC3339), now.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("create character: %w", err)
	}
	c.CreatedAt = now
	c.UpdatedAt = now
	return nil
}

// GetCharacter retrieves a character by ID.
func (s *Store) GetCharacter(id string) (*domain.Character, error) {
	c := &domain.Character{}
	var aliasesJSON, createdAt, updatedAt string
	err := s.db.QueryRow(
		`SELECT id, scp_id, canonical_name, aliases, visual_descriptor, style_guide, image_prompt_base, selected_image_path, created_at, updated_at
		 FROM characters WHERE id = ?`, id,
	).Scan(&c.ID, &c.SCPID, &c.CanonicalName, &aliasesJSON,
		&c.VisualDescriptor, &c.StyleGuide, &c.ImagePromptBase, &c.SelectedImagePath,
		&createdAt, &updatedAt)
	if err == sql.ErrNoRows {
		return nil, &domain.NotFoundError{Resource: "character", ID: id}
	}
	if err != nil {
		return nil, fmt.Errorf("get character: %w", err)
	}
	if err := json.Unmarshal([]byte(aliasesJSON), &c.Aliases); err != nil {
		return nil, fmt.Errorf("unmarshal aliases: %w", err)
	}
	c.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	c.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	return c, nil
}

// ListCharactersBySCPID returns all characters for a given SCP entity.
func (s *Store) ListCharactersBySCPID(scpID string) ([]*domain.Character, error) {
	rows, err := s.db.Query(
		`SELECT id, scp_id, canonical_name, aliases, visual_descriptor, style_guide, image_prompt_base, selected_image_path, created_at, updated_at
		 FROM characters WHERE scp_id = ? ORDER BY canonical_name`, scpID,
	)
	if err != nil {
		return nil, fmt.Errorf("list characters by scp_id: %w", err)
	}
	defer rows.Close()
	return scanCharacters(rows)
}

// ListAllCharacters returns all characters (for global preset reuse).
func (s *Store) ListAllCharacters() ([]*domain.Character, error) {
	rows, err := s.db.Query(
		`SELECT id, scp_id, canonical_name, aliases, visual_descriptor, style_guide, image_prompt_base, selected_image_path, created_at, updated_at
		 FROM characters ORDER BY scp_id, canonical_name`,
	)
	if err != nil {
		return nil, fmt.Errorf("list all characters: %w", err)
	}
	defer rows.Close()
	return scanCharacters(rows)
}

// UpdateCharacter updates all fields of an existing character.
func (s *Store) UpdateCharacter(c *domain.Character) error {
	now := time.Now().UTC()
	aliasesJSON, err := json.Marshal(c.Aliases)
	if err != nil {
		return fmt.Errorf("marshal aliases: %w", err)
	}

	result, err := s.db.Exec(
		`UPDATE characters SET scp_id=?, canonical_name=?, aliases=?, visual_descriptor=?, style_guide=?, image_prompt_base=?, selected_image_path=?, updated_at=?
		 WHERE id=?`,
		c.SCPID, c.CanonicalName, string(aliasesJSON),
		c.VisualDescriptor, c.StyleGuide, c.ImagePromptBase, c.SelectedImagePath,
		now.Format(time.RFC3339), c.ID,
	)
	if err != nil {
		return fmt.Errorf("update character: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return &domain.NotFoundError{Resource: "character", ID: c.ID}
	}
	c.UpdatedAt = now
	return nil
}

// DeleteCharacter removes a character by ID.
func (s *Store) DeleteCharacter(id string) error {
	result, err := s.db.Exec(`DELETE FROM characters WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete character: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return &domain.NotFoundError{Resource: "character", ID: id}
	}
	return nil
}

// SearchCharactersByName returns characters where canonical_name or any alias
// matches the search term (case-insensitive).
func (s *Store) SearchCharactersByName(name string) ([]*domain.Character, error) {
	lowerName := strings.ToLower(name)
	rows, err := s.db.Query(
		`SELECT id, scp_id, canonical_name, aliases, visual_descriptor, style_guide, image_prompt_base, selected_image_path, created_at, updated_at
		 FROM characters
		 WHERE LOWER(canonical_name) = ? OR LOWER(aliases) LIKE ?
		 ORDER BY canonical_name`,
		lowerName, "%"+lowerName+"%",
	)
	if err != nil {
		return nil, fmt.Errorf("search characters by name: %w", err)
	}
	defer rows.Close()
	return scanCharacters(rows)
}

// CreateCandidateBatch inserts N candidate rows with status "pending" in a single transaction.
func (s *Store) CreateCandidateBatch(projectID, scpID string, count int) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("create candidate batch: begin tx: %w", err)
	}
	for i := 1; i <= count; i++ {
		id := uuid.New().String()
		_, err := tx.Exec(
			`INSERT INTO character_candidates (id, project_id, scp_id, candidate_num, status)
			 VALUES (?, ?, ?, ?, 'pending')`,
			id, projectID, scpID, i,
		)
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("create candidate batch: %w", err)
		}
	}
	return tx.Commit()
}

// ListCandidatesByProject returns all candidates for a project ordered by candidate_num.
func (s *Store) ListCandidatesByProject(projectID string) ([]*domain.CharacterCandidate, error) {
	rows, err := s.db.Query(
		`SELECT id, project_id, scp_id, candidate_num, image_path, description, status, error_detail, created_at
		 FROM character_candidates WHERE project_id = ? ORDER BY candidate_num`, projectID,
	)
	if err != nil {
		return nil, fmt.Errorf("list candidates by project: %w", err)
	}
	defer rows.Close()
	return scanCandidates(rows)
}

// UpdateCandidateStatus updates the status, image path, description, and error detail for a candidate.
func (s *Store) UpdateCandidateStatus(id, status, imagePath, description, errorDetail string) error {
	_, err := s.db.Exec(
		`UPDATE character_candidates SET status=?, image_path=?, description=?, error_detail=? WHERE id=?`,
		status, imagePath, description, errorDetail, id,
	)
	if err != nil {
		return fmt.Errorf("update candidate status: %w", err)
	}
	return nil
}

// DeleteCandidatesByProject removes all candidates for a project.
func (s *Store) DeleteCandidatesByProject(projectID string) error {
	_, err := s.db.Exec(`DELETE FROM character_candidates WHERE project_id = ?`, projectID)
	if err != nil {
		return fmt.Errorf("delete candidates by project: %w", err)
	}
	return nil
}

func scanCandidates(rows *sql.Rows) ([]*domain.CharacterCandidate, error) {
	var candidates []*domain.CharacterCandidate
	for rows.Next() {
		c := &domain.CharacterCandidate{}
		var createdAt string
		if err := rows.Scan(&c.ID, &c.ProjectID, &c.SCPID, &c.CandidateNum,
			&c.ImagePath, &c.Description, &c.Status, &c.ErrorDetail, &createdAt); err != nil {
			return nil, fmt.Errorf("scan candidate: %w", err)
		}
		c.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		candidates = append(candidates, c)
	}
	return candidates, rows.Err()
}

func scanCharacters(rows *sql.Rows) ([]*domain.Character, error) {
	var characters []*domain.Character
	for rows.Next() {
		c := &domain.Character{}
		var aliasesJSON, createdAt, updatedAt string
		if err := rows.Scan(&c.ID, &c.SCPID, &c.CanonicalName, &aliasesJSON,
			&c.VisualDescriptor, &c.StyleGuide, &c.ImagePromptBase, &c.SelectedImagePath,
			&createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan character: %w", err)
		}
		if err := json.Unmarshal([]byte(aliasesJSON), &c.Aliases); err != nil {
			return nil, fmt.Errorf("unmarshal aliases: %w", err)
		}
		c.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		c.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
		characters = append(characters, c)
	}
	return characters, rows.Err()
}
