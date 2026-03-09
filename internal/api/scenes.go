package api

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/sushistack/yt.pipe/internal/domain"
	"github.com/sushistack/yt.pipe/internal/service"
)

// handleGetScenes returns the scene dashboard for a project.
// GET /api/v1/projects/{id}/scenes
func (s *Server) handleGetScenes(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "id")

	dashSvc := service.NewSceneDashboardService(s.store, slog.Default())
	dashboard, err := dashSvc.GetDashboard(projectID)
	if err != nil {
		if _, ok := err.(*domain.NotFoundError); ok {
			WriteError(w, r, http.StatusNotFound, "not_found", err.Error())
			return
		}
		WriteError(w, r, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	WriteJSON(w, r, http.StatusOK, dashboard)
}

// handleApproveScene approves a scene's asset.
// POST /api/v1/projects/{id}/scenes/{num}/approve?type=image|tts
func (s *Server) handleApproveScene(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "id")
	numStr := chi.URLParam(r, "num")
	sceneNum, err := strconv.Atoi(numStr)
	if err != nil {
		WriteError(w, r, http.StatusBadRequest, "bad_request", "invalid scene number")
		return
	}

	assetType := r.URL.Query().Get("type")
	if assetType != domain.AssetTypeImage && assetType != domain.AssetTypeTTS {
		WriteError(w, r, http.StatusBadRequest, "bad_request", "type must be 'image' or 'tts'")
		return
	}

	// Fetch project for SCP ID (needed for webhook payloads)
	project, err := s.store.GetProject(projectID)
	if err != nil {
		if _, ok := err.(*domain.NotFoundError); ok {
			WriteError(w, r, http.StatusNotFound, "not_found", err.Error())
			return
		}
		WriteError(w, r, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	approvalSvc := service.NewApprovalService(s.store, slog.Default())
	if err := approvalSvc.ApproveScene(projectID, sceneNum, assetType); err != nil {
		if _, ok := err.(*domain.NotFoundError); ok {
			WriteError(w, r, http.StatusNotFound, "not_found", err.Error())
			return
		}
		if _, ok := err.(*domain.TransitionError); ok {
			WriteError(w, r, http.StatusConflict, "invalid_transition", err.Error())
			return
		}
		WriteError(w, r, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	// Fire scene_approved webhook
	s.webhooks.NotifySceneApproved(projectID, project.SCPID, sceneNum, assetType)

	// Check if all scenes of this asset type are now approved
	allApproved, err := s.store.AllApproved(projectID, assetType)
	if err == nil && allApproved {
		s.webhooks.NotifyAllApproved(projectID, project.SCPID, assetType)
	}

	WriteJSON(w, r, http.StatusOK, map[string]interface{}{
		"project_id": projectID,
		"scene_num":  sceneNum,
		"asset_type": assetType,
		"status":     "approved",
	})
}

// handleRejectScene rejects a scene's asset.
// POST /api/v1/projects/{id}/scenes/{num}/reject?type=image|tts
func (s *Server) handleRejectScene(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "id")
	numStr := chi.URLParam(r, "num")
	sceneNum, err := strconv.Atoi(numStr)
	if err != nil {
		WriteError(w, r, http.StatusBadRequest, "bad_request", "invalid scene number")
		return
	}

	assetType := r.URL.Query().Get("type")
	if assetType != domain.AssetTypeImage && assetType != domain.AssetTypeTTS {
		WriteError(w, r, http.StatusBadRequest, "bad_request", "type must be 'image' or 'tts'")
		return
	}

	approvalSvc := service.NewApprovalService(s.store, slog.Default())
	if err := approvalSvc.RejectScene(projectID, sceneNum, assetType); err != nil {
		if _, ok := err.(*domain.NotFoundError); ok {
			WriteError(w, r, http.StatusNotFound, "not_found", err.Error())
			return
		}
		if _, ok := err.(*domain.TransitionError); ok {
			WriteError(w, r, http.StatusConflict, "invalid_transition", err.Error())
			return
		}
		WriteError(w, r, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	WriteJSON(w, r, http.StatusOK, map[string]interface{}{
		"project_id": projectID,
		"scene_num":  sceneNum,
		"asset_type": assetType,
		"status":     "rejected",
	})
}
