package service

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/sushistack/yt.pipe/internal/domain"
	"github.com/sushistack/yt.pipe/internal/store"
)

// ProjectService manages project lifecycle and state transitions.
type ProjectService struct {
	store *store.Store
}

// NewProjectService creates a new ProjectService.
func NewProjectService(s *store.Store) *ProjectService {
	return &ProjectService{store: s}
}

// CreateProject creates a new project with pending status.
func (ps *ProjectService) CreateProject(_ context.Context, scpID, workspacePath string) (*domain.Project, error) {
	if scpID == "" {
		return nil, &domain.ValidationError{Field: "scp_id", Message: "must not be empty"}
	}
	if workspacePath == "" {
		return nil, &domain.ValidationError{Field: "workspace_path", Message: "must not be empty"}
	}

	p := &domain.Project{
		ID:            uuid.New().String(),
		SCPID:         scpID,
		Status:        domain.StatusPending,
		WorkspacePath: workspacePath,
		ReviewToken:   uuid.New().String(),
	}
	if err := ps.store.CreateProject(p); err != nil {
		return nil, fmt.Errorf("service: create project: %w", err)
	}
	return p, nil
}

// BackfillReviewTokens generates review tokens for existing projects that have none.
func (ps *ProjectService) BackfillReviewTokens(_ context.Context) (int, error) {
	ids, err := ps.store.ListProjectsWithNullToken()
	if err != nil {
		return 0, fmt.Errorf("service: list null token projects: %w", err)
	}
	for _, id := range ids {
		token := uuid.New().String()
		if err := ps.store.SetReviewToken(id, token); err != nil {
			return 0, fmt.Errorf("service: backfill token for %s: %w", id, err)
		}
	}
	return len(ids), nil
}

// GetProject retrieves a project by ID.
func (ps *ProjectService) GetProject(_ context.Context, id string) (*domain.Project, error) {
	return ps.store.GetProject(id)
}

// ListProjects returns all projects.
func (ps *ProjectService) ListProjects(_ context.Context) ([]*domain.Project, error) {
	return ps.store.ListProjects()
}

// TransitionProject atomically transitions a project to a new state within a transaction.
// It validates the transition, updates the project, and records an execution log entry.
func (ps *ProjectService) TransitionProject(_ context.Context, projectID, newStatus string) (*domain.Project, error) {
	db := ps.store.DB()

	tx, err := db.Begin()
	if err != nil {
		return nil, fmt.Errorf("service: begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Load project within transaction for serialized access
	var p domain.Project
	var createdAt, updatedAt string
	var reviewToken sql.NullString
	err = tx.QueryRow(
		`SELECT id, scp_id, status, scene_count, workspace_path, review_token, created_at, updated_at
		 FROM projects WHERE id = ?`, projectID,
	).Scan(&p.ID, &p.SCPID, &p.Status, &p.SceneCount, &p.WorkspacePath, &reviewToken, &createdAt, &updatedAt)
	if reviewToken.Valid {
		p.ReviewToken = reviewToken.String
	}
	if err != nil {
		return nil, &domain.NotFoundError{Resource: "project", ID: projectID}
	}
	p.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	p.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)

	// Validate and apply transition
	previousStatus := p.Status
	if err := p.Transition(newStatus); err != nil {
		return nil, err
	}

	// Update project in transaction
	now := time.Now().UTC()
	_, err = tx.Exec(
		`UPDATE projects SET status=?, updated_at=? WHERE id=?`,
		p.Status, now.Format(time.RFC3339), p.ID,
	)
	if err != nil {
		return nil, fmt.Errorf("service: update project status: %w", err)
	}
	p.UpdatedAt = now

	// Record transition in execution log
	_, err = tx.Exec(
		`INSERT INTO execution_logs (project_id, stage, action, status, details, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		p.ID, "state_machine", "transition",
		"completed",
		fmt.Sprintf("%s -> %s", previousStatus, newStatus),
		now.Format(time.RFC3339),
	)
	if err != nil {
		return nil, fmt.Errorf("service: record transition log: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("service: commit transition: %w", err)
	}

	return &p, nil
}
