package store

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/jay/youtube-pipeline/internal/domain"
)

// CreateProject inserts a new project
func (s *Store) CreateProject(p *domain.Project) error {
	now := time.Now().UTC()
	_, err := s.db.Exec(
		`INSERT INTO projects (id, scp_id, status, scene_count, workspace_path, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		p.ID, p.SCPID, p.Status, p.SceneCount, p.WorkspacePath,
		now.Format(time.RFC3339), now.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("create project: %w", err)
	}
	p.CreatedAt = now
	p.UpdatedAt = now
	return nil
}

// GetProject retrieves a project by ID
func (s *Store) GetProject(id string) (*domain.Project, error) {
	p := &domain.Project{}
	var createdAt, updatedAt string
	err := s.db.QueryRow(
		`SELECT id, scp_id, status, scene_count, workspace_path, created_at, updated_at
		 FROM projects WHERE id = ?`, id,
	).Scan(&p.ID, &p.SCPID, &p.Status, &p.SceneCount, &p.WorkspacePath, &createdAt, &updatedAt)
	if err == sql.ErrNoRows {
		return nil, &domain.NotFoundError{Resource: "project", ID: id}
	}
	if err != nil {
		return nil, fmt.Errorf("get project: %w", err)
	}
	p.CreatedAt, err = time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return nil, fmt.Errorf("parse project created_at: %w", err)
	}
	p.UpdatedAt, err = time.Parse(time.RFC3339, updatedAt)
	if err != nil {
		return nil, fmt.Errorf("parse project updated_at: %w", err)
	}
	return p, nil
}

// ListProjects returns all projects
func (s *Store) ListProjects() ([]*domain.Project, error) {
	rows, err := s.db.Query(
		`SELECT id, scp_id, status, scene_count, workspace_path, created_at, updated_at
		 FROM projects ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}
	defer rows.Close()

	var projects []*domain.Project
	for rows.Next() {
		p := &domain.Project{}
		var createdAt, updatedAt string
		if err := rows.Scan(&p.ID, &p.SCPID, &p.Status, &p.SceneCount, &p.WorkspacePath, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan project: %w", err)
		}
		p.CreatedAt, err = time.Parse(time.RFC3339, createdAt)
		if err != nil {
			return nil, fmt.Errorf("parse project created_at: %w", err)
		}
		p.UpdatedAt, err = time.Parse(time.RFC3339, updatedAt)
		if err != nil {
			return nil, fmt.Errorf("parse project updated_at: %w", err)
		}
		projects = append(projects, p)
	}
	return projects, rows.Err()
}

// UpdateProject updates an existing project
func (s *Store) UpdateProject(p *domain.Project) error {
	now := time.Now().UTC()
	result, err := s.db.Exec(
		`UPDATE projects SET scp_id=?, status=?, scene_count=?, workspace_path=?, updated_at=?
		 WHERE id=?`,
		p.SCPID, p.Status, p.SceneCount, p.WorkspacePath, now.Format(time.RFC3339), p.ID,
	)
	if err != nil {
		return fmt.Errorf("update project: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return &domain.NotFoundError{Resource: "project", ID: p.ID}
	}
	p.UpdatedAt = now
	return nil
}
