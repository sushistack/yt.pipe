package store

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/sushistack/yt.pipe/internal/domain"
)

// scanProject scans a project row into a domain.Project.
func scanProject(scanner interface{ Scan(...interface{}) error }) (*domain.Project, error) {
	p := &domain.Project{}
	var createdAt, updatedAt string
	var reviewToken sql.NullString
	err := scanner.Scan(&p.ID, &p.SCPID, &p.Status, &p.SceneCount, &p.WorkspacePath, &reviewToken, &createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}
	if reviewToken.Valid {
		p.ReviewToken = reviewToken.String
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

const projectColumns = `id, scp_id, status, scene_count, workspace_path, review_token, created_at, updated_at`

// CreateProject inserts a new project
func (s *Store) CreateProject(p *domain.Project) error {
	now := time.Now().UTC()
	var reviewToken interface{}
	if p.ReviewToken != "" {
		reviewToken = p.ReviewToken
	}
	_, err := s.db.Exec(
		`INSERT INTO projects (id, scp_id, status, scene_count, workspace_path, review_token, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		p.ID, p.SCPID, p.Status, p.SceneCount, p.WorkspacePath, reviewToken,
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
	p, err := scanProject(s.db.QueryRow(
		`SELECT `+projectColumns+` FROM projects WHERE id = ?`, id,
	))
	if err == sql.ErrNoRows {
		return nil, &domain.NotFoundError{Resource: "project", ID: id}
	}
	if err != nil {
		return nil, fmt.Errorf("get project: %w", err)
	}
	return p, nil
}

// ListProjects returns all projects
func (s *Store) ListProjects() ([]*domain.Project, error) {
	rows, err := s.db.Query(
		`SELECT `+projectColumns+` FROM projects ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}
	defer rows.Close()

	var projects []*domain.Project
	for rows.Next() {
		p, err := scanProject(rows)
		if err != nil {
			return nil, fmt.Errorf("scan project: %w", err)
		}
		projects = append(projects, p)
	}
	return projects, rows.Err()
}

// DeleteProject deletes a project and all its child records (jobs, manifests, execution logs).
func (s *Store) DeleteProject(id string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin delete project tx: %w", err)
	}

	// Delete child records first (no ON DELETE CASCADE in schema)
	for _, q := range []string{
		`DELETE FROM execution_logs WHERE project_id = ?`,
		`DELETE FROM scene_manifests WHERE project_id = ?`,
		`DELETE FROM scene_approvals WHERE project_id = ?`,
		`DELETE FROM jobs WHERE project_id = ?`,
	} {
		if _, err := tx.Exec(q, id); err != nil {
			tx.Rollback()
			return fmt.Errorf("delete project children: %w", err)
		}
	}

	result, err := tx.Exec(`DELETE FROM projects WHERE id = ?`, id)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("delete project: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		tx.Rollback()
		return &domain.NotFoundError{Resource: "project", ID: id}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit delete project: %w", err)
	}
	return nil
}

// ListSCPGroups returns distinct SCP IDs with their project count, ordered by most recent activity.
func (s *Store) ListSCPGroups(stage string, scpFilter string, limit, offset int) ([]SCPGroup, int, error) {
	where := ""
	var args []interface{}
	if stage != "" {
		where += " AND status = ?"
		args = append(args, stage)
	}
	if scpFilter != "" {
		where += " AND scp_id LIKE ?"
		args = append(args, "%"+scpFilter+"%")
	}

	var total int
	countQ := "SELECT COUNT(DISTINCT scp_id) FROM projects WHERE 1=1" + where
	if err := s.db.QueryRow(countQ, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count scp groups: %w", err)
	}

	query := `SELECT scp_id, COUNT(*) as cnt, MAX(updated_at) as latest
		FROM projects WHERE 1=1` + where + `
		GROUP BY scp_id ORDER BY latest DESC LIMIT ? OFFSET ?`
	qArgs := append(args, limit, offset)
	rows, err := s.db.Query(query, qArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list scp groups: %w", err)
	}
	defer rows.Close()

	var groups []SCPGroup
	for rows.Next() {
		var g SCPGroup
		var latest string
		if err := rows.Scan(&g.SCPID, &g.Count, &latest); err != nil {
			return nil, 0, fmt.Errorf("scan scp group: %w", err)
		}
		g.LatestUpdate, _ = time.Parse(time.RFC3339, latest)
		groups = append(groups, g)
	}
	return groups, total, rows.Err()
}

// SCPGroup represents a group of projects sharing the same SCP ID.
type SCPGroup struct {
	SCPID        string
	Count        int
	LatestUpdate time.Time
}

// ListProjectsBySCP returns projects for a specific SCP ID, ordered by most recent first.
func (s *Store) ListProjectsBySCP(scpID, stage string, limit, offset int) ([]*domain.Project, int, error) {
	where := " AND scp_id = ?"
	args := []interface{}{scpID}
	if stage != "" {
		where += " AND status = ?"
		args = append(args, stage)
	}

	var total int
	countQ := "SELECT COUNT(*) FROM projects WHERE 1=1" + where
	if err := s.db.QueryRow(countQ, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count projects by scp: %w", err)
	}

	query := "SELECT " + projectColumns + " FROM projects WHERE 1=1" + where + " ORDER BY created_at DESC LIMIT ? OFFSET ?"
	qArgs := append(args, limit, offset)
	rows, err := s.db.Query(query, qArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list projects by scp: %w", err)
	}
	defer rows.Close()

	var projects []*domain.Project
	for rows.Next() {
		p, scanErr := scanProject(rows)
		if scanErr != nil {
			return nil, 0, fmt.Errorf("scan project: %w", scanErr)
		}
		projects = append(projects, p)
	}
	return projects, total, rows.Err()
}

// ListProjectsFiltered returns projects matching optional filters with pagination.
func (s *Store) ListProjectsFiltered(state, scpID string, limit, offset int) ([]*domain.Project, int, error) {
	where := ""
	var args []interface{}

	if state != "" {
		where += " AND status = ?"
		args = append(args, state)
	}
	if scpID != "" {
		where += " AND scp_id = ?"
		args = append(args, scpID)
	}

	// Count total
	var total int
	countQuery := "SELECT COUNT(*) FROM projects WHERE 1=1" + where
	if err := s.db.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count projects: %w", err)
	}

	// Query with pagination
	query := "SELECT " + projectColumns + " FROM projects WHERE 1=1" + where + " ORDER BY created_at DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list projects filtered: %w", err)
	}
	defer rows.Close()

	var projects []*domain.Project
	for rows.Next() {
		p, err := scanProject(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("scan project: %w", err)
		}
		projects = append(projects, p)
	}
	return projects, total, rows.Err()
}

// UpdateProject updates an existing project
func (s *Store) UpdateProject(p *domain.Project) error {
	now := time.Now().UTC()
	var reviewToken interface{}
	if p.ReviewToken != "" {
		reviewToken = p.ReviewToken
	}
	result, err := s.db.Exec(
		`UPDATE projects SET scp_id=?, status=?, scene_count=?, workspace_path=?, review_token=?, updated_at=?
		 WHERE id=?`,
		p.SCPID, p.Status, p.SceneCount, p.WorkspacePath, reviewToken, now.Format(time.RFC3339), p.ID,
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

// ListProjectsWithNullToken returns IDs of projects that have no review_token.
func (s *Store) ListProjectsWithNullToken() ([]string, error) {
	rows, err := s.db.Query(`SELECT id FROM projects WHERE review_token IS NULL`)
	if err != nil {
		return nil, fmt.Errorf("list null token projects: %w", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan project id: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// SetReviewToken sets the review_token for a project.
func (s *Store) SetReviewToken(projectID, token string) error {
	result, err := s.db.Exec(`UPDATE projects SET review_token=? WHERE id=?`, token, projectID)
	if err != nil {
		return fmt.Errorf("set review token: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return &domain.NotFoundError{Resource: "project", ID: projectID}
	}
	return nil
}
