package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jay/youtube-pipeline/internal/domain"
)

// projectResponse is the JSON representation of a project.
type projectResponse struct {
	ID            string `json:"id"`
	SCPID         string `json:"scp_id"`
	Status        string `json:"status"`
	SceneCount    int    `json:"scene_count"`
	WorkspacePath string `json:"workspace_path"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
}

func toProjectResponse(p *domain.Project) projectResponse {
	return projectResponse{
		ID:            p.ID,
		SCPID:         p.SCPID,
		Status:        p.Status,
		SceneCount:    p.SceneCount,
		WorkspacePath: p.WorkspacePath,
		CreatedAt:     p.CreatedAt.Format(time.RFC3339),
		UpdatedAt:     p.UpdatedAt.Format(time.RFC3339),
	}
}

type createProjectRequest struct {
	SCPID string `json:"scp_id"`
}

type listProjectsResponse struct {
	Projects []projectResponse `json:"projects"`
	Total    int               `json:"total"`
	Limit    int               `json:"limit"`
	Offset   int               `json:"offset"`
}

func (s *Server) handleCreateProject(w http.ResponseWriter, r *http.Request) {
	var req createProjectRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		WriteError(w, r, http.StatusBadRequest, "INVALID_REQUEST", "invalid JSON body")
		return
	}
	if req.SCPID == "" {
		WriteError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "scp_id is required")
		return
	}

	p, err := s.projectSvc.CreateProject(r.Context(), req.SCPID, s.workspacePath)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}

	WriteJSON(w, r, http.StatusCreated, toProjectResponse(p))
}

func (s *Server) handleListProjects(w http.ResponseWriter, r *http.Request) {
	state := r.URL.Query().Get("state")
	scpID := r.URL.Query().Get("scp_id")
	limit := parseIntQuery(r, "limit", 20)
	offset := parseIntQuery(r, "offset", 0)

	if limit < 1 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	projects, total, err := s.store.ListProjectsFiltered(state, scpID, limit, offset)
	if err != nil {
		WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to list projects")
		return
	}

	items := make([]projectResponse, 0, len(projects))
	for _, p := range projects {
		items = append(items, toProjectResponse(p))
	}

	WriteJSON(w, r, http.StatusOK, listProjectsResponse{
		Projects: items,
		Total:    total,
		Limit:    limit,
		Offset:   offset,
	})
}

func (s *Server) handleGetProject(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	p, err := s.store.GetProject(id)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}

	WriteJSON(w, r, http.StatusOK, toProjectResponse(p))
}

func (s *Server) handleDeleteProject(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	p, err := s.store.GetProject(id)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}

	// Only allow deletion of completed or pending projects
	if p.Status != domain.StatusComplete && p.Status != domain.StatusPending {
		WriteError(w, r, http.StatusConflict, "CONFLICT",
			"cannot delete project in '"+p.Status+"' state; only 'complete' or 'pending' projects can be deleted")
		return
	}

	if err := s.store.DeleteProject(id); err != nil {
		writeServiceError(w, r, err)
		return
	}

	WriteJSON(w, r, http.StatusOK, map[string]string{"deleted": id})
}

// writeServiceError maps domain errors to HTTP error responses.
func writeServiceError(w http.ResponseWriter, r *http.Request, err error) {
	var notFound *domain.NotFoundError
	if errors.As(err, &notFound) {
		WriteError(w, r, http.StatusNotFound, "NOT_FOUND", err.Error())
		return
	}
	var validationErr *domain.ValidationError
	if errors.As(err, &validationErr) {
		WriteError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	var transitionErr *domain.TransitionError
	if errors.As(err, &transitionErr) {
		WriteError(w, r, http.StatusConflict, "CONFLICT", err.Error())
		return
	}
	WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
}

func parseIntQuery(r *http.Request, key string, defaultVal int) int {
	s := r.URL.Query().Get(key)
	if s == "" {
		return defaultVal
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return defaultVal
	}
	return v
}
