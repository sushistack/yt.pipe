package store

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/sushistack/yt.pipe/internal/domain"
)

// CreateGlossarySuggestion inserts a new glossary suggestion with status=pending.
func (s *Store) CreateGlossarySuggestion(sg *domain.GlossarySuggestion) error {
	if err := domain.ValidateGlossarySuggestion(sg.ProjectID, sg.Term, sg.Pronunciation); err != nil {
		return err
	}
	now := time.Now().UTC()
	nowStr := now.Format(time.RFC3339)
	result, err := s.db.Exec(
		`INSERT INTO glossary_suggestions (project_id, term, pronunciation, definition, category, status, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		sg.ProjectID, sg.Term, sg.Pronunciation, sg.Definition, sg.Category, domain.SuggestionPending, nowStr, nowStr,
	)
	if err != nil {
		return fmt.Errorf("create glossary suggestion: %w", err)
	}
	id, _ := result.LastInsertId()
	sg.ID = int(id)
	sg.Status = domain.SuggestionPending
	sg.CreatedAt = now
	sg.UpdatedAt = now
	return nil
}

// GetGlossarySuggestion retrieves a suggestion by ID.
func (s *Store) GetGlossarySuggestion(id int) (*domain.GlossarySuggestion, error) {
	sg := &domain.GlossarySuggestion{}
	var createdAt, updatedAt string
	err := s.db.QueryRow(
		`SELECT id, project_id, term, pronunciation, definition, category, status, created_at, updated_at
		 FROM glossary_suggestions WHERE id = ?`, id,
	).Scan(&sg.ID, &sg.ProjectID, &sg.Term, &sg.Pronunciation, &sg.Definition, &sg.Category, &sg.Status, &createdAt, &updatedAt)
	if err == sql.ErrNoRows {
		return nil, &domain.NotFoundError{Resource: "glossary_suggestion", ID: fmt.Sprintf("%d", id)}
	}
	if err != nil {
		return nil, fmt.Errorf("get glossary suggestion: %w", err)
	}
	sg.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	sg.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	return sg, nil
}

// ListGlossarySuggestionsByProject returns suggestions for a project, optionally filtered by status.
// Pass empty string for status to return all suggestions.
func (s *Store) ListGlossarySuggestionsByProject(projectID, status string) ([]*domain.GlossarySuggestion, error) {
	var rows *sql.Rows
	var err error
	if status != "" {
		rows, err = s.db.Query(
			`SELECT id, project_id, term, pronunciation, definition, category, status, created_at, updated_at
			 FROM glossary_suggestions WHERE project_id = ? AND status = ? ORDER BY id`,
			projectID, status,
		)
	} else {
		rows, err = s.db.Query(
			`SELECT id, project_id, term, pronunciation, definition, category, status, created_at, updated_at
			 FROM glossary_suggestions WHERE project_id = ? ORDER BY id`,
			projectID,
		)
	}
	if err != nil {
		return nil, fmt.Errorf("list glossary suggestions: %w", err)
	}
	defer rows.Close()

	var suggestions []*domain.GlossarySuggestion
	for rows.Next() {
		sg := &domain.GlossarySuggestion{}
		var createdAt, updatedAt string
		if err := rows.Scan(&sg.ID, &sg.ProjectID, &sg.Term, &sg.Pronunciation, &sg.Definition, &sg.Category, &sg.Status, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan glossary suggestion: %w", err)
		}
		sg.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		sg.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
		suggestions = append(suggestions, sg)
	}
	return suggestions, rows.Err()
}

// UpdateGlossarySuggestionStatus transitions a suggestion to a new status.
func (s *Store) UpdateGlossarySuggestionStatus(id int, newStatus string) error {
	// Load current status for transition validation
	var currentStatus string
	err := s.db.QueryRow(`SELECT status FROM glossary_suggestions WHERE id = ?`, id).Scan(&currentStatus)
	if err == sql.ErrNoRows {
		return &domain.NotFoundError{Resource: "glossary_suggestion", ID: fmt.Sprintf("%d", id)}
	}
	if err != nil {
		return fmt.Errorf("update glossary suggestion status: %w", err)
	}

	if !domain.CanSuggestionTransition(currentStatus, newStatus) {
		return &domain.TransitionError{Current: currentStatus, Requested: newStatus, Allowed: allowedForSuggestion(currentStatus)}
	}

	now := time.Now().UTC().Format(time.RFC3339)
	_, err = s.db.Exec(
		`UPDATE glossary_suggestions SET status = ?, updated_at = ? WHERE id = ?`,
		newStatus, now, id,
	)
	if err != nil {
		return fmt.Errorf("update glossary suggestion status: %w", err)
	}
	return nil
}

// DeleteGlossarySuggestion removes a suggestion by ID.
func (s *Store) DeleteGlossarySuggestion(id int) error {
	result, err := s.db.Exec(`DELETE FROM glossary_suggestions WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete glossary suggestion: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return &domain.NotFoundError{Resource: "glossary_suggestion", ID: fmt.Sprintf("%d", id)}
	}
	return nil
}

func allowedForSuggestion(current string) []string {
	transitions := map[string][]string{
		domain.SuggestionPending:  {domain.SuggestionApproved, domain.SuggestionRejected},
		domain.SuggestionApproved: {},
		domain.SuggestionRejected: {},
	}
	return transitions[current]
}
