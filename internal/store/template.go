package store

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/sushistack/yt.pipe/internal/domain"
)

// CreateTemplate inserts a new prompt template and creates version 1.
func (s *Store) CreateTemplate(t *domain.PromptTemplate) error {
	now := time.Now().UTC()

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("create template: begin tx: %w", err)
	}

	_, err = tx.Exec(
		`INSERT INTO prompt_templates (id, category, name, content, version, is_default, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		t.ID, string(t.Category), t.Name, t.Content, 1, boolToInt(t.IsDefault),
		now.Format(time.RFC3339), now.Format(time.RFC3339),
	)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("create template: %w", err)
	}

	_, err = tx.Exec(
		`INSERT INTO prompt_template_versions (id, template_id, version, content, created_at)
		 VALUES (?, ?, ?, ?, ?)`,
		t.ID+"-v1", t.ID, 1, t.Content, now.Format(time.RFC3339),
	)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("create template version: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("create template: commit: %w", err)
	}

	t.Version = 1
	t.CreatedAt = now
	t.UpdatedAt = now
	return nil
}

// GetTemplate retrieves a prompt template by ID.
func (s *Store) GetTemplate(id string) (*domain.PromptTemplate, error) {
	t := &domain.PromptTemplate{}
	var createdAt, updatedAt, category string
	var isDefault int
	err := s.db.QueryRow(
		`SELECT id, category, name, content, version, is_default, created_at, updated_at
		 FROM prompt_templates WHERE id = ?`, id,
	).Scan(&t.ID, &category, &t.Name, &t.Content, &t.Version, &isDefault, &createdAt, &updatedAt)
	if err == sql.ErrNoRows {
		return nil, &domain.NotFoundError{Resource: "template", ID: id}
	}
	if err != nil {
		return nil, fmt.Errorf("get template: %w", err)
	}
	t.Category = domain.TemplateCategory(category)
	t.IsDefault = isDefault != 0
	t.CreatedAt = parseTime(createdAt)
	t.UpdatedAt = parseTime(updatedAt)
	return t, nil
}

// ListTemplates returns templates optionally filtered by category.
func (s *Store) ListTemplates(category string) ([]*domain.PromptTemplate, error) {
	query := `SELECT id, category, name, content, version, is_default, created_at, updated_at
		 FROM prompt_templates`
	var args []interface{}
	if category != "" {
		query += " WHERE category = ?"
		args = append(args, category)
	}
	query += " ORDER BY category, name"

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list templates: %w", err)
	}
	defer rows.Close()
	return scanTemplateRows(rows)
}

// UpdateTemplate increments version, saves new content, and creates a version record.
// Automatically prunes versions beyond the 10 most recent.
func (s *Store) UpdateTemplate(id, content, versionID string) error {
	now := time.Now().UTC()

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("update template: begin tx: %w", err)
	}

	// Get current version
	var currentVersion int
	err = tx.QueryRow(`SELECT version FROM prompt_templates WHERE id = ?`, id).Scan(&currentVersion)
	if err == sql.ErrNoRows {
		tx.Rollback()
		return &domain.NotFoundError{Resource: "template", ID: id}
	}
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("update template: get version: %w", err)
	}

	newVersion := currentVersion + 1

	// Update template
	_, err = tx.Exec(
		`UPDATE prompt_templates SET content = ?, version = ?, updated_at = ? WHERE id = ?`,
		content, newVersion, now.Format(time.RFC3339), id,
	)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("update template: %w", err)
	}

	// Create version record
	_, err = tx.Exec(
		`INSERT INTO prompt_template_versions (id, template_id, version, content, created_at)
		 VALUES (?, ?, ?, ?, ?)`,
		versionID, id, newVersion, content, now.Format(time.RFC3339),
	)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("update template: create version: %w", err)
	}

	// Prune versions beyond 10
	_, err = tx.Exec(
		`DELETE FROM prompt_template_versions
		 WHERE template_id = ? AND id NOT IN (
		   SELECT id FROM prompt_template_versions
		   WHERE template_id = ?
		   ORDER BY version DESC
		   LIMIT 10
		 )`, id, id,
	)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("update template: prune versions: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("update template: commit: %w", err)
	}
	return nil
}

// DeleteTemplate removes a template and all its version records and project overrides.
func (s *Store) DeleteTemplate(id string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("delete template: begin tx: %w", err)
	}

	// Delete child records first
	for _, q := range []string{
		`DELETE FROM project_template_overrides WHERE template_id = ?`,
		`DELETE FROM prompt_template_versions WHERE template_id = ?`,
	} {
		if _, err := tx.Exec(q, id); err != nil {
			tx.Rollback()
			return fmt.Errorf("delete template children: %w", err)
		}
	}

	result, err := tx.Exec(`DELETE FROM prompt_templates WHERE id = ?`, id)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("delete template: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		tx.Rollback()
		return &domain.NotFoundError{Resource: "template", ID: id}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("delete template: commit: %w", err)
	}
	return nil
}

// GetTemplateVersion retrieves a specific version of a template.
func (s *Store) GetTemplateVersion(templateID string, version int) (*domain.TemplateVersion, error) {
	v := &domain.TemplateVersion{}
	var createdAt string
	err := s.db.QueryRow(
		`SELECT id, template_id, version, content, created_at
		 FROM prompt_template_versions WHERE template_id = ? AND version = ?`,
		templateID, version,
	).Scan(&v.ID, &v.TemplateID, &v.Version, &v.Content, &createdAt)
	if err == sql.ErrNoRows {
		return nil, &domain.NotFoundError{Resource: "template version", ID: fmt.Sprintf("%s@v%d", templateID, version)}
	}
	if err != nil {
		return nil, fmt.Errorf("get template version: %w", err)
	}
	v.CreatedAt = parseTime(createdAt)
	return v, nil
}

// ListTemplateVersions returns all version records for a template.
func (s *Store) ListTemplateVersions(templateID string) ([]*domain.TemplateVersion, error) {
	rows, err := s.db.Query(
		`SELECT id, template_id, version, content, created_at
		 FROM prompt_template_versions WHERE template_id = ?
		 ORDER BY version DESC`, templateID,
	)
	if err != nil {
		return nil, fmt.Errorf("list template versions: %w", err)
	}
	defer rows.Close()

	var versions []*domain.TemplateVersion
	for rows.Next() {
		v := &domain.TemplateVersion{}
		var createdAt string
		if err := rows.Scan(&v.ID, &v.TemplateID, &v.Version, &v.Content, &createdAt); err != nil {
			return nil, fmt.Errorf("scan template version: %w", err)
		}
		v.CreatedAt = parseTime(createdAt)
		versions = append(versions, v)
	}
	return versions, rows.Err()
}

// RollbackTemplate restores template content to a specified version.
// This creates a new version record (version number increments).
func (s *Store) RollbackTemplate(id string, version int, versionID string) error {
	// Get the target version content
	v, err := s.GetTemplateVersion(id, version)
	if err != nil {
		return fmt.Errorf("rollback template: %w", err)
	}

	return s.UpdateTemplate(id, v.Content, versionID)
}

// SetOverride stores a project-specific template override.
func (s *Store) SetOverride(projectID, templateID, content string) error {
	now := time.Now().UTC()
	_, err := s.db.Exec(
		`INSERT INTO project_template_overrides (project_id, template_id, content, created_at)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT (project_id, template_id)
		 DO UPDATE SET content = excluded.content, created_at = excluded.created_at`,
		projectID, templateID, content, now.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("set override: %w", err)
	}
	return nil
}

// GetOverride returns a project-specific template override.
func (s *Store) GetOverride(projectID, templateID string) (*domain.ProjectTemplateOverride, error) {
	o := &domain.ProjectTemplateOverride{}
	var createdAt string
	err := s.db.QueryRow(
		`SELECT project_id, template_id, content, created_at
		 FROM project_template_overrides WHERE project_id = ? AND template_id = ?`,
		projectID, templateID,
	).Scan(&o.ProjectID, &o.TemplateID, &o.Content, &createdAt)
	if err == sql.ErrNoRows {
		return nil, &domain.NotFoundError{Resource: "template override", ID: fmt.Sprintf("%s/%s", projectID, templateID)}
	}
	if err != nil {
		return nil, fmt.Errorf("get override: %w", err)
	}
	o.CreatedAt = parseTime(createdAt)
	return o, nil
}

// DeleteOverride removes a project-specific template override.
func (s *Store) DeleteOverride(projectID, templateID string) error {
	result, err := s.db.Exec(
		`DELETE FROM project_template_overrides WHERE project_id = ? AND template_id = ?`,
		projectID, templateID,
	)
	if err != nil {
		return fmt.Errorf("delete override: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return &domain.NotFoundError{Resource: "template override", ID: fmt.Sprintf("%s/%s", projectID, templateID)}
	}
	return nil
}

func scanTemplateRows(rows *sql.Rows) ([]*domain.PromptTemplate, error) {
	var templates []*domain.PromptTemplate
	for rows.Next() {
		t := &domain.PromptTemplate{}
		var createdAt, updatedAt, category string
		var isDefault int
		if err := rows.Scan(&t.ID, &category, &t.Name, &t.Content, &t.Version, &isDefault, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan template: %w", err)
		}
		t.Category = domain.TemplateCategory(category)
		t.IsDefault = isDefault != 0
		t.CreatedAt = parseTime(createdAt)
		t.UpdatedAt = parseTime(updatedAt)
		templates = append(templates, t)
	}
	return templates, rows.Err()
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
