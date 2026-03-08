package api

import (
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/sushistack/yt.pipe/internal/domain"
)

type generateRequest struct {
	Scenes []int `json:"scenes"`
}

type updatePromptRequest struct {
	Prompt string `json:"prompt"`
}

type feedbackRequest struct {
	SceneNum  int    `json:"scene_num"`
	AssetType string `json:"asset_type"`
	Rating    string `json:"rating"`
	Comment   string `json:"comment"`
}

type feedbackResponse struct {
	ID        int    `json:"id"`
	ProjectID string `json:"project_id"`
	SceneNum  int    `json:"scene_num"`
	AssetType string `json:"asset_type"`
	Rating    string `json:"rating"`
	Comment   string `json:"comment"`
	CreatedAt string `json:"created_at"`
}

// handleGenerateImages enqueues selective image regeneration.
func (s *Server) handleGenerateImages(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "id")

	project, err := s.store.GetProject(projectID)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}

	var req generateRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		WriteError(w, r, http.StatusBadRequest, "INVALID_REQUEST", "invalid JSON body")
		return
	}

	// Validate scene numbers
	if err := validateSceneNumbers(req.Scenes, project.SceneCount); err != nil {
		WriteError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}

	// Create async job
	jobID := uuid.New().String()
	dbJob := &domain.Job{
		ID:        jobID,
		ProjectID: projectID,
		Type:      "image_generate",
		Status:    JobStatusRunning,
	}
	if err := s.store.CreateJob(dbJob); err != nil {
		WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to create job")
		return
	}

	WriteJSON(w, r, http.StatusAccepted, map[string]interface{}{
		"job_id":     jobID,
		"project_id": projectID,
		"scenes":     req.Scenes,
		"type":       "image_generate",
	})
}

// handleGenerateTTS enqueues selective TTS regeneration.
func (s *Server) handleGenerateTTS(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "id")

	project, err := s.store.GetProject(projectID)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}

	var req generateRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		WriteError(w, r, http.StatusBadRequest, "INVALID_REQUEST", "invalid JSON body")
		return
	}

	if err := validateSceneNumbers(req.Scenes, project.SceneCount); err != nil {
		WriteError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}

	jobID := uuid.New().String()
	dbJob := &domain.Job{
		ID:        jobID,
		ProjectID: projectID,
		Type:      "tts_generate",
		Status:    JobStatusRunning,
	}
	if err := s.store.CreateJob(dbJob); err != nil {
		WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to create job")
		return
	}

	WriteJSON(w, r, http.StatusAccepted, map[string]interface{}{
		"job_id":     jobID,
		"project_id": projectID,
		"scenes":     req.Scenes,
		"type":       "tts_generate",
	})
}

// handleUpdatePrompt updates the image prompt for a specific scene.
func (s *Server) handleUpdatePrompt(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "id")
	sceneNum := parseIntParam(r, "num")

	project, err := s.store.GetProject(projectID)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}

	if sceneNum < 1 || sceneNum > project.SceneCount {
		WriteError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "invalid scene number")
		return
	}

	var req updatePromptRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		WriteError(w, r, http.StatusBadRequest, "INVALID_REQUEST", "invalid JSON body")
		return
	}

	if req.Prompt == "" {
		WriteError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "prompt is required")
		return
	}

	WriteJSON(w, r, http.StatusOK, map[string]interface{}{
		"project_id": projectID,
		"scene_num":  sceneNum,
		"prompt":     req.Prompt,
		"updated":    true,
	})
}

// handleCreateFeedback submits feedback for a scene asset.
func (s *Server) handleCreateFeedback(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "id")

	if _, err := s.store.GetProject(projectID); err != nil {
		writeServiceError(w, r, err)
		return
	}

	var req feedbackRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		WriteError(w, r, http.StatusBadRequest, "INVALID_REQUEST", "invalid JSON body")
		return
	}

	if !domain.ValidAssetTypes[req.AssetType] {
		WriteError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "invalid asset_type; must be one of: image, audio, subtitle, scenario")
		return
	}
	if !domain.ValidRatings[req.Rating] {
		WriteError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "invalid rating; must be one of: good, bad, neutral")
		return
	}

	f := &domain.Feedback{
		ProjectID: projectID,
		SceneNum:  req.SceneNum,
		AssetType: req.AssetType,
		Rating:    req.Rating,
		Comment:   req.Comment,
	}
	if err := s.store.CreateFeedback(f); err != nil {
		WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to create feedback")
		return
	}

	WriteJSON(w, r, http.StatusCreated, feedbackResponse{
		ID:        f.ID,
		ProjectID: f.ProjectID,
		SceneNum:  f.SceneNum,
		AssetType: f.AssetType,
		Rating:    f.Rating,
		Comment:   f.Comment,
		CreatedAt: f.CreatedAt.Format(time.RFC3339),
	})
}

func validateSceneNumbers(scenes []int, maxScene int) error {
	for _, n := range scenes {
		if n < 1 || n > maxScene {
			return &domain.ValidationError{
				Field:   "scenes",
				Message: "scene number out of range",
			}
		}
	}
	return nil
}

func parseIntParam(r *http.Request, name string) int {
	s := chi.URLParam(r, name)
	v := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0
		}
		v = v*10 + int(c-'0')
	}
	return v
}
