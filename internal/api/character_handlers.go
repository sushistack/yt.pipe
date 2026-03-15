package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/sushistack/yt.pipe/internal/domain"
	"github.com/sushistack/yt.pipe/internal/service"
)

// handleGenerateCharacters starts async character candidate generation.
// POST /api/v1/projects/{id}/characters/generate
func (s *Server) handleGenerateCharacters(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "id")

	project, err := s.store.GetProject(projectID)
	if err != nil {
		WriteError(w, r, http.StatusNotFound, "NOT_FOUND", "project not found")
		return
	}

	// Stage guard: only allow from scenario or character stage
	if project.Status != domain.StageScenario && project.Status != domain.StageCharacter {
		WriteError(w, r, http.StatusConflict, "INVALID_STAGE",
			"character generation requires project in scenario or character stage")
		return
	}

	if !s.requirePlugin(w, r, "llm") || !s.requirePlugin(w, r, "imagegen") {
		return
	}

	if s.characterSvc == nil {
		WriteError(w, r, http.StatusInternalServerError, "SERVICE_UNAVAILABLE",
			"character service not configured")
		return
	}

	// Check for duplicate running job
	if existing := s.jobs.getByType(projectID, "character_generate"); existing != nil && existing.getStatus() == JobStatusRunning {
		WriteError(w, r, http.StatusConflict, "CONFLICT", "character generation already running")
		return
	}

	// Set project stage to character
	projectSvc := service.NewProjectService(s.store)
	if _, err := projectSvc.SetProjectStage(r.Context(), project.ID, domain.StageCharacter); err != nil {
		WriteError(w, r, http.StatusInternalServerError, "STAGE_UPDATE_FAILED", err.Error())
		return
	}

	// Create async job
	jobID := uuid.New().String()
	dbJob := &domain.Job{
		ID:        jobID,
		ProjectID: projectID,
		Type:      "character_generate",
		Status:    JobStatusRunning,
	}
	if err := s.store.CreateJob(dbJob); err != nil {
		WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to create job")
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	s.jobs.startTyped(projectID, "character_generate", jobID, cancel)

	go func() {
		_, genErr := s.characterSvc.GenerateCandidates(ctx, project.ID, project.SCPID, 4, project.WorkspacePath)
		if genErr != nil {
			slog.Error("character generation failed", "project_id", projectID, "job_id", jobID, "error", genErr)
			s.updateJobRecord(jobID, JobStatusFailed, 0, "", genErr.Error())
		} else {
			s.updateJobRecord(jobID, JobStatusComplete, 100, "", "")
		}
		s.jobs.removeTyped(projectID, "character_generate")
	}()

	WriteJSON(w, r, http.StatusAccepted, map[string]interface{}{
		"job_id":     jobID,
		"project_id": projectID,
		"type":       "character_generate",
	})
}

// handleListCandidates returns candidates and their generation status.
// GET /api/v1/projects/{id}/characters/candidates
func (s *Server) handleListCandidates(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "id")

	// Validate project exists
	if _, err := s.store.GetProject(projectID); err != nil {
		WriteError(w, r, http.StatusNotFound, "NOT_FOUND", "project not found")
		return
	}

	if s.characterSvc == nil {
		WriteJSON(w, r, http.StatusOK, map[string]interface{}{
			"status":     "empty",
			"candidates": []*domain.CharacterCandidate{},
		})
		return
	}

	candidates, err := s.characterSvc.ListCandidates(projectID)
	if err != nil {
		WriteError(w, r, http.StatusInternalServerError, "LIST_FAILED", err.Error())
		return
	}

	status, err := s.characterSvc.GetCandidateGenerationStatus(projectID)
	if err != nil {
		status = "empty"
	}

	WriteJSON(w, r, http.StatusOK, map[string]interface{}{
		"status":     status,
		"candidates": candidates,
	})
}

// handleSelectCharacter selects a candidate as the project's character.
// POST /api/v1/projects/{id}/characters/select
func (s *Server) handleSelectCharacter(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "id")

	project, err := s.store.GetProject(projectID)
	if err != nil {
		WriteError(w, r, http.StatusNotFound, "NOT_FOUND", "project not found")
		return
	}

	if s.characterSvc == nil {
		WriteError(w, r, http.StatusInternalServerError, "SERVICE_UNAVAILABLE",
			"character service not configured")
		return
	}

	// Stage guard: only allow from scenario or character stage
	if project.Status != domain.StageScenario && project.Status != domain.StageCharacter {
		WriteError(w, r, http.StatusConflict, "INVALID_STAGE",
			"character selection requires project in scenario or character stage")
		return
	}

	var body struct {
		CandidateNum int `json:"candidate_num"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		WriteError(w, r, http.StatusBadRequest, "INVALID_BODY", "expected {candidate_num: N}")
		return
	}
	if body.CandidateNum < 1 || body.CandidateNum > 10 {
		WriteError(w, r, http.StatusBadRequest, "INVALID_CANDIDATE",
			"candidate_num must be between 1 and 10")
		return
	}

	char, err := s.characterSvc.SelectCandidate(project.SCPID, body.CandidateNum, project.WorkspacePath)
	if err != nil {
		WriteError(w, r, http.StatusInternalServerError, "SELECT_FAILED", err.Error())
		return
	}

	// Confirm stage
	projectSvc := service.NewProjectService(s.store)
	_, _ = projectSvc.SetProjectStage(r.Context(), project.ID, domain.StageCharacter)

	WriteJSON(w, r, http.StatusOK, char)
}

// handleDeselectCharacter clears the selected character so user can pick again.
// POST /api/v1/projects/{id}/characters/deselect
func (s *Server) handleDeselectCharacter(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "id")

	project, err := s.store.GetProject(projectID)
	if err != nil {
		WriteError(w, r, http.StatusNotFound, "NOT_FOUND", "project not found")
		return
	}

	if s.characterSvc == nil {
		WriteError(w, r, http.StatusInternalServerError, "SERVICE_UNAVAILABLE",
			"character service not configured")
		return
	}

	char, err := s.characterSvc.CheckExistingCharacter(project.SCPID)
	if err != nil || char == nil {
		WriteError(w, r, http.StatusNotFound, "NOT_FOUND", "no character to deselect")
		return
	}

	char.SelectedImagePath = ""
	if err := s.store.UpdateCharacter(char); err != nil {
		WriteError(w, r, http.StatusInternalServerError, "UPDATE_FAILED", err.Error())
		return
	}

	WriteJSON(w, r, http.StatusOK, map[string]string{"status": "deselected"})
}

// handleGetCharacter returns the current character for a project's SCP ID.
// GET /api/v1/projects/{id}/characters
func (s *Server) handleGetCharacter(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "id")

	project, err := s.store.GetProject(projectID)
	if err != nil {
		WriteError(w, r, http.StatusNotFound, "NOT_FOUND", "project not found")
		return
	}

	if s.characterSvc == nil {
		WriteError(w, r, http.StatusNotFound, "NOT_FOUND", "no character found")
		return
	}

	char, err := s.characterSvc.CheckExistingCharacter(project.SCPID)
	if err != nil {
		WriteError(w, r, http.StatusInternalServerError, "CHECK_FAILED", err.Error())
		return
	}
	if char == nil {
		WriteError(w, r, http.StatusNotFound, "NOT_FOUND", "no character found")
		return
	}

	WriteJSON(w, r, http.StatusOK, char)
}
