package api

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/sushistack/yt.pipe/internal/domain"
	"github.com/sushistack/yt.pipe/internal/service"
)

// --- Review Token Validation ---

// validateReviewToken validates the review token from query params against the project's stored token.
// Returns the project and true on success; writes error response and returns false on failure.
func validateReviewToken(s *Server, w http.ResponseWriter, r *http.Request, projectID string) (*domain.Project, bool) {
	project, err := s.store.GetProject(projectID)
	if err != nil {
		if _, ok := err.(*domain.NotFoundError); ok {
			WriteError(w, r, http.StatusNotFound, "NOT_FOUND", "project not found")
			return nil, false
		}
		WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to load project")
		return nil, false
	}

	token := r.URL.Query().Get("token")
	if token == "" {
		WriteError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "missing review token")
		return nil, false
	}

	if project.ReviewToken == "" || subtle.ConstantTimeCompare([]byte(token), []byte(project.ReviewToken)) != 1 {
		WriteError(w, r, http.StatusForbidden, "FORBIDDEN", "invalid review token")
		return nil, false
	}

	return project, true
}

// --- CSRF Verification ---

// verifyCsrfToken checks that the X-Review-Token header matches the ?token= query parameter.
func verifyCsrfToken(w http.ResponseWriter, r *http.Request) bool {
	headerToken := r.Header.Get("X-Review-Token")
	queryToken := r.URL.Query().Get("token")
	if headerToken == "" || subtle.ConstantTimeCompare([]byte(headerToken), []byte(queryToken)) != 1 {
		WriteError(w, r, http.StatusForbidden, "FORBIDDEN", "CSRF token mismatch: X-Review-Token header must match ?token= query parameter")
		return false
	}
	return true
}

// --- Rate Limiting ---

type rateLimiter struct {
	mu       sync.Mutex
	counters map[string]*rateBucket
}

type rateBucket struct {
	count    int
	resetAt  time.Time
}

var (
	mutationLimiter = &rateLimiter{counters: make(map[string]*rateBucket)}
	readLimiter     = &rateLimiter{counters: make(map[string]*rateBucket)}
)

func (rl *rateLimiter) allow(ip string, limit int, window time.Duration) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	bucket, ok := rl.counters[ip]
	if !ok || now.After(bucket.resetAt) {
		rl.counters[ip] = &rateBucket{count: 1, resetAt: now.Add(window)}
		return true
	}
	bucket.count++
	return bucket.count <= limit
}

func checkMutationRateLimit(w http.ResponseWriter, r *http.Request) bool {
	ip := clientIP(r)
	if !mutationLimiter.allow(ip, 30, time.Minute) {
		WriteError(w, r, http.StatusTooManyRequests, "RATE_LIMITED", "too many requests; try again later")
		return false
	}
	return true
}

func checkReadRateLimit(w http.ResponseWriter, r *http.Request) bool {
	ip := clientIP(r)
	if !readLimiter.allow(ip, 120, time.Minute) {
		WriteError(w, r, http.StatusTooManyRequests, "RATE_LIMITED", "too many requests; try again later")
		return false
	}
	return true
}

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return strings.Split(xff, ",")[0]
	}
	return r.RemoteAddr
}

// --- Review Page Handler ---

// handleReviewPage renders the review dashboard HTML page.
// GET /review/{project_id}?token=xxx
func (s *Server) handleReviewPage(w http.ResponseWriter, r *http.Request) {
	if !checkReadRateLimit(w, r) {
		return
	}

	projectID := chi.URLParam(r, "project_id")
	project, ok := validateReviewToken(s, w, r, projectID)
	if !ok {
		return
	}

	dashSvc := service.NewSceneDashboardService(s.store, slog.Default())
	dashboard, err := dashSvc.GetDashboard(projectID)
	if err != nil {
		WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to load dashboard")
		return
	}

	if s.reviewTmpl == nil {
		WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "review template not loaded")
		return
	}

	token := r.URL.Query().Get("token")

	data := map[string]interface{}{
		"Project":   project,
		"Dashboard": dashboard,
		"Token":     token,
		"ProjectID": projectID,
		"StylesCSS": s.reviewCSS,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.reviewTmpl.Execute(w, data); err != nil {
		slog.Error("failed to render review template", "error", err)
	}
}

// --- Asset Serving Handlers ---

// handleServeImage serves a scene's image file.
// GET /api/v1/projects/{id}/scenes/{num}/image?token=xxx
func (s *Server) handleServeImage(w http.ResponseWriter, r *http.Request) {
	if !checkReadRateLimit(w, r) {
		return
	}
	s.serveSceneAsset(w, r, "image.png")
}

// handleServeAudio serves a scene's audio file.
// GET /api/v1/projects/{id}/scenes/{num}/audio?token=xxx
func (s *Server) handleServeAudio(w http.ResponseWriter, r *http.Request) {
	if !checkReadRateLimit(w, r) {
		return
	}
	s.serveSceneAsset(w, r, "audio.wav")
}

func (s *Server) serveSceneAsset(w http.ResponseWriter, r *http.Request, filename string) {
	projectID := chi.URLParam(r, "id")
	project, ok := validateReviewToken(s, w, r, projectID)
	if !ok {
		return
	}

	numStr := chi.URLParam(r, "num")
	sceneNum, err := strconv.Atoi(numStr)
	if err != nil || sceneNum < 1 {
		WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", "invalid scene number")
		return
	}

	projectPath := project.WorkspacePath
	if projectPath == "" {
		projectPath = filepath.Join(s.workspacePath, project.ID)
	}

	assetPath := filepath.Join(projectPath, "scenes", strconv.Itoa(sceneNum), filename)

	// Path traversal prevention
	cleaned := filepath.Clean(assetPath)
	if !strings.HasPrefix(cleaned, filepath.Clean(projectPath)) {
		WriteError(w, r, http.StatusForbidden, "FORBIDDEN", "path traversal detected")
		return
	}

	if _, err := os.Stat(cleaned); os.IsNotExist(err) {
		WriteError(w, r, http.StatusNotFound, "NOT_FOUND", "asset not found")
		return
	}

	http.ServeFile(w, r, cleaned)
}

// --- Narration Edit Handler ---

type updateNarrationRequest struct {
	Narration string `json:"narration"`
}

// handleUpdateNarration updates narration text for a scene.
// PATCH /api/v1/projects/{id}/scenes/{num}/narration?token=xxx
func (s *Server) handleUpdateNarration(w http.ResponseWriter, r *http.Request) {
	if !checkMutationRateLimit(w, r) {
		return
	}

	projectID := chi.URLParam(r, "id")
	if _, ok := validateReviewToken(s, w, r, projectID); !ok {
		return
	}
	if !verifyCsrfToken(w, r) {
		return
	}

	numStr := chi.URLParam(r, "num")
	sceneNum, err := strconv.Atoi(numStr)
	if err != nil || sceneNum < 1 {
		WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", "invalid scene number")
		return
	}

	var req updateNarrationRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		WriteError(w, r, http.StatusBadRequest, "INVALID_REQUEST", "invalid JSON body")
		return
	}

	if err := s.reviewSvc.UpdateNarration(projectID, sceneNum, req.Narration); err != nil {
		writeServiceError(w, r, err)
		return
	}

	WriteJSON(w, r, http.StatusOK, map[string]interface{}{
		"project_id": projectID,
		"scene_num":  sceneNum,
		"narration":  req.Narration,
		"updated":    true,
	})
}

// --- Scene Add/Delete Handlers ---

type addSceneRequest struct {
	Narration string `json:"narration"`
}

// handleAddScene appends a new scene to the project.
// POST /api/v1/projects/{id}/scenes?token=xxx
func (s *Server) handleAddScene(w http.ResponseWriter, r *http.Request) {
	if !checkMutationRateLimit(w, r) {
		return
	}

	projectID := chi.URLParam(r, "id")
	if _, ok := validateReviewToken(s, w, r, projectID); !ok {
		return
	}
	if !verifyCsrfToken(w, r) {
		return
	}

	var req addSceneRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		WriteError(w, r, http.StatusBadRequest, "INVALID_REQUEST", "invalid JSON body")
		return
	}

	afterParam := r.URL.Query().Get("after")
	if afterParam != "" {
		// Positional insert mode
		afterNum, err := strconv.Atoi(afterParam)
		if err != nil {
			WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", "invalid 'after' parameter: must be an integer")
			return
		}
		newSceneNum, err := s.reviewSvc.InsertScene(projectID, afterNum, req.Narration)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		WriteJSON(w, r, http.StatusCreated, map[string]interface{}{
			"project_id": projectID,
			"scene_num":  newSceneNum,
			"narration":  req.Narration,
			"inserted":   true,
		})
		return
	}

	// Append mode (backward compatible)
	newSceneNum, err := s.reviewSvc.AddScene(projectID, req.Narration)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}

	WriteJSON(w, r, http.StatusCreated, map[string]interface{}{
		"project_id": projectID,
		"scene_num":  newSceneNum,
		"narration":  req.Narration,
	})
}

// handleDeleteScene removes a scene from the project.
// DELETE /api/v1/projects/{id}/scenes/{num}?token=xxx
func (s *Server) handleDeleteScene(w http.ResponseWriter, r *http.Request) {
	if !checkMutationRateLimit(w, r) {
		return
	}

	projectID := chi.URLParam(r, "id")
	if _, ok := validateReviewToken(s, w, r, projectID); !ok {
		return
	}
	if !verifyCsrfToken(w, r) {
		return
	}

	numStr := chi.URLParam(r, "num")
	sceneNum, err := strconv.Atoi(numStr)
	if err != nil || sceneNum < 1 {
		WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", "invalid scene number")
		return
	}

	// Cancel running jobs for this scene
	for _, jobType := range []string{"image_generate", "tts_generate"} {
		if job := s.jobs.getByType(projectID, jobType); job != nil && job.getStatus() == JobStatusRunning {
			job.Cancel()
		}
	}

	if err := s.reviewSvc.DeleteScene(projectID, sceneNum); err != nil {
		writeServiceError(w, r, err)
		return
	}

	WriteJSON(w, r, http.StatusOK, map[string]interface{}{
		"project_id": projectID,
		"scene_num":  sceneNum,
		"deleted":    true,
	})
}

// --- Reject with Regeneration Handler ---

// handleRejectAndRegenerate rejects a scene's asset and triggers regeneration.
// POST /api/v1/projects/{id}/scenes/{num}/reject?type=image|tts&token=xxx
// This extends the existing reject handler for review-token access.
func (s *Server) handleReviewRejectScene(w http.ResponseWriter, r *http.Request) {
	if !checkMutationRateLimit(w, r) {
		return
	}

	projectID := chi.URLParam(r, "id")
	project, ok := validateReviewToken(s, w, r, projectID)
	if !ok {
		return
	}
	if !verifyCsrfToken(w, r) {
		return
	}

	numStr := chi.URLParam(r, "num")
	sceneNum, err := strconv.Atoi(numStr)
	if err != nil || sceneNum < 1 {
		WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", "invalid scene number")
		return
	}

	assetType := r.URL.Query().Get("type")
	if assetType != domain.AssetTypeImage && assetType != domain.AssetTypeTTS {
		WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", "type must be 'image' or 'tts'")
		return
	}

	// Reject the scene
	approvalSvc := service.NewApprovalService(s.store, slog.Default())
	if err := approvalSvc.RejectScene(projectID, sceneNum, assetType); err != nil {
		writeServiceError(w, r, err)
		return
	}

	// Check project state allows regeneration
	regenStarted := false
	var jobID string

	canRegen := false
	if assetType == domain.AssetTypeImage {
		canRegen = validImageGenStates[project.Status]
	} else {
		canRegen = validTTSGenStates[project.Status]
	}

	if canRegen {
		jobType := assetType + "_generate"
		// Check for duplicate running job
		if existing := s.jobs.getByType(projectID, jobType); existing != nil && existing.getStatus() == JobStatusRunning {
			jobID = existing.JobID
		} else if dbRunning, err := s.store.GetRunningJobByProjectAndType(projectID, jobType); err == nil && dbRunning != nil {
			jobID = dbRunning.ID
		} else {
			// Create new regen job
			jobID = uuid.New().String()
			dbJob := &domain.Job{
				ID:        jobID,
				ProjectID: projectID,
				Type:      jobType,
				Status:    JobStatusRunning,
			}
			if err := s.store.CreateJob(dbJob); err == nil {
				ctx, cancel := context.WithCancel(context.Background())
				s.jobs.startTyped(projectID, jobType, jobID, cancel)
				regenStarted = true

				scenes := []int{sceneNum}
				if assetType == domain.AssetTypeImage && s.imageGenSvc != nil {
					go s.executeImageGeneration(ctx, jobID, project, scenes)
				} else if assetType == domain.AssetTypeTTS && s.ttsSvc != nil {
					go s.executeTTSGeneration(ctx, jobID, project, scenes)
				}
			}
		}
	}

	WriteJSON(w, r, http.StatusOK, map[string]interface{}{
		"project_id":           projectID,
		"scene_num":            sceneNum,
		"asset_type":           assetType,
		"status":               "rejected",
		"regeneration_started": regenStarted,
		"job_id":               jobID,
	})
}

// --- Approve All Handler ---

// handleApproveAll bulk-approves all generated scenes for an asset type.
// POST /api/v1/projects/{id}/approve-all?type=image|tts&token=xxx
func (s *Server) handleApproveAll(w http.ResponseWriter, r *http.Request) {
	if !checkMutationRateLimit(w, r) {
		return
	}

	projectID := chi.URLParam(r, "id")
	project, ok := validateReviewToken(s, w, r, projectID)
	if !ok {
		return
	}
	if !verifyCsrfToken(w, r) {
		return
	}

	assetType := r.URL.Query().Get("type")
	if assetType != domain.AssetTypeImage && assetType != domain.AssetTypeTTS {
		WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", "type must be 'image' or 'tts'")
		return
	}

	// Bulk approve only "generated" scenes
	approved, err := s.store.BulkApproveGenerated(projectID, assetType)
	if err != nil {
		WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to bulk approve")
		return
	}

	// Count skipped (pending scenes)
	approvals, _ := s.store.ListApprovalsByProject(projectID, assetType)
	skipped := 0
	for _, a := range approvals {
		if a.Status == domain.ApprovalPending {
			skipped++
		}
	}

	allApproved, _ := s.store.AllApproved(projectID, assetType)
	if allApproved {
		s.webhooks.NotifyAllApproved(projectID, project.SCPID, assetType, BuildReviewURL(projectID, project.ReviewToken))
	}

	// When all images are approved: transition state immediately and auto-trigger TTS generation
	ttsTriggered := false
	if allApproved && assetType == domain.AssetTypeImage {
		// Transition to tts_review immediately so the UI reflects the new state
		if _, err := s.projectSvc.TransitionProject(r.Context(), projectID, domain.StatusTTSReview); err != nil {
			slog.Warn("failed to transition to tts_review after image approval",
				"project_id", projectID, "err", err)
		} else {
			slog.Info("transitioned to tts_review after all images approved", "project_id", projectID)
			s.webhooks.NotifyStateChange(projectID, project.SCPID, domain.StatusImageReview, domain.StatusTTSReview, BuildReviewURL(projectID, project.ReviewToken))
		}

		// Auto-trigger TTS generation if plugin available
		if s.ttsSvc != nil {
			existingJob := s.jobs.getByType(projectID, "tts_generate")
			dbRunning, _ := s.store.GetRunningJobByProjectAndType(projectID, "tts_generate")
			if (existingJob == nil || existingJob.getStatus() != JobStatusRunning) && dbRunning == nil {
				jobID := uuid.New().String()
				dbJob := &domain.Job{
					ID:        jobID,
					ProjectID: projectID,
					Type:      "tts_generate",
					Status:    JobStatusRunning,
				}
				if err := s.store.CreateJob(dbJob); err == nil {
					ctx, cancel := context.WithCancel(context.Background())
					s.jobs.startTyped(projectID, "tts_generate", jobID, cancel)
					scenes := makeSceneRange(project.SceneCount)
					go s.executeTTSGeneration(ctx, jobID, project, scenes)
					ttsTriggered = true
					slog.Info("auto-triggered TTS generation after image approval",
						"project_id", projectID, "job_id", jobID)
				}
			}
		}
	}

	// When all TTS are approved: transition to assembling and auto-trigger assembly
	assemblyTriggered := false
	if allApproved && assetType == domain.AssetTypeTTS {
		// Transition to assembling immediately
		if _, err := s.projectSvc.TransitionProject(r.Context(), projectID, domain.StatusAssembling); err != nil {
			slog.Warn("failed to transition to assembling after TTS approval",
				"project_id", projectID, "err", err)
		} else {
			slog.Info("transitioned to assembling after all TTS approved", "project_id", projectID)
			s.webhooks.NotifyStateChange(projectID, project.SCPID, domain.StatusTTSReview, domain.StatusAssembling, BuildReviewURL(projectID, project.ReviewToken))
		}

		// Auto-trigger assembly if output plugin available
		if s.pluginStatus != nil && s.pluginStatus["output"] {
			existingJob := s.jobs.getByType(projectID, "assembly")
			dbRunning, _ := s.store.GetRunningJobByProjectAndType(projectID, "assembly")
			if (existingJob == nil || existingJob.getStatus() != JobStatusRunning) && dbRunning == nil {
				jobID := uuid.New().String()
				dbJob := &domain.Job{
					ID:        jobID,
					ProjectID: projectID,
					Type:      "assembly",
					Status:    JobStatusRunning,
				}
				if err := s.store.CreateJob(dbJob); err == nil {
					ctx, cancel := context.WithCancel(context.Background())
					s.jobs.startTyped(projectID, "assembly", jobID, cancel)
					go s.executeAssembly(ctx, jobID, project)
					assemblyTriggered = true
					slog.Info("auto-triggered assembly after TTS approval",
						"project_id", projectID, "job_id", jobID)
				}
			}
		}
	}

	WriteJSON(w, r, http.StatusOK, map[string]interface{}{
		"approved":           approved,
		"skipped":            skipped,
		"all_approved":       allApproved,
		"tts_triggered":      ttsTriggered,
		"assembly_triggered": assemblyTriggered,
	})
}

// --- Token Rotation Handler ---

// handleRotateReviewToken generates a new review token, invalidating the old one.
// POST /api/v1/projects/{id}/review-token/rotate (Bearer auth only)
func (s *Server) handleRotateReviewToken(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "id")

	project, err := s.store.GetProject(projectID)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}

	newToken := uuid.New().String()
	project.ReviewToken = newToken
	if err := s.store.UpdateProject(project); err != nil {
		WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to rotate token")
		return
	}

	WriteJSON(w, r, http.StatusOK, map[string]interface{}{
		"project_id":   projectID,
		"review_token": newToken,
	})
}

// --- Review Approve Scene Handler (with review token) ---

// handleReviewApproveScene approves a scene's asset via Bearer or review token.
// POST /api/v1/projects/{id}/scenes/{num}/approve?type=image|tts[&token=xxx]
func (s *Server) handleReviewApproveScene(w http.ResponseWriter, r *http.Request) {
	if !checkMutationRateLimit(w, r) {
		return
	}

	projectID := chi.URLParam(r, "id")
	project, ok := requireReviewAuth(s, w, r, projectID)
	if !ok {
		return
	}
	// CSRF is required only for review-token requests (not Bearer, not auth-disabled)
	if r.Header.Get("Authorization") == "" && r.URL.Query().Get("token") != "" {
		if !verifyCsrfToken(w, r) {
			return
		}
	}

	numStr := chi.URLParam(r, "num")
	sceneNum, err := strconv.Atoi(numStr)
	if err != nil || sceneNum < 1 {
		WriteError(w, r, http.StatusBadRequest, "bad_request", "invalid scene number")
		return
	}

	assetType := r.URL.Query().Get("type")
	if assetType != domain.AssetTypeImage && assetType != domain.AssetTypeTTS {
		WriteError(w, r, http.StatusBadRequest, "bad_request", "type must be 'image' or 'tts'")
		return
	}

	approvalSvc := service.NewApprovalService(s.store, slog.Default())
	if err := approvalSvc.ApproveScene(projectID, sceneNum, assetType); err != nil {
		writeServiceError(w, r, err)
		return
	}

	reviewURL := BuildReviewURL(projectID, project.ReviewToken)
	s.webhooks.NotifySceneApproved(projectID, project.SCPID, sceneNum, assetType, reviewURL)

	allApproved, err := s.store.AllApproved(projectID, assetType)
	if err == nil && allApproved {
		s.webhooks.NotifyAllApproved(projectID, project.SCPID, assetType, reviewURL)
	}

	// Auto-transition and auto-trigger when all scenes of an asset type are approved
	ttsTriggered := false
	assemblyTriggered := false

	if allApproved && assetType == domain.AssetTypeImage {
		if _, err := s.projectSvc.TransitionProject(r.Context(), projectID, domain.StatusTTSReview); err != nil {
			slog.Warn("failed to transition to tts_review after image approval",
				"project_id", projectID, "err", err)
		} else {
			slog.Info("transitioned to tts_review after all images approved (single approve)", "project_id", projectID)
			s.webhooks.NotifyStateChange(projectID, project.SCPID, domain.StatusImageReview, domain.StatusTTSReview, reviewURL)
		}

		if s.ttsSvc != nil {
			existingJob := s.jobs.getByType(projectID, "tts_generate")
			dbRunning, _ := s.store.GetRunningJobByProjectAndType(projectID, "tts_generate")
			if (existingJob == nil || existingJob.getStatus() != JobStatusRunning) && dbRunning == nil {
				jobID := uuid.New().String()
				dbJob := &domain.Job{
					ID:        jobID,
					ProjectID: projectID,
					Type:      "tts_generate",
					Status:    JobStatusRunning,
				}
				if err := s.store.CreateJob(dbJob); err == nil {
					ctx, cancel := context.WithCancel(context.Background())
					s.jobs.startTyped(projectID, "tts_generate", jobID, cancel)
					scenes := makeSceneRange(project.SceneCount)
					go s.executeTTSGeneration(ctx, jobID, project, scenes)
					ttsTriggered = true
					slog.Info("auto-triggered TTS generation after image approval (single approve)",
						"project_id", projectID, "job_id", jobID)
				}
			}
		}
	}

	if allApproved && assetType == domain.AssetTypeTTS {
		if _, err := s.projectSvc.TransitionProject(r.Context(), projectID, domain.StatusAssembling); err != nil {
			slog.Warn("failed to transition to assembling after TTS approval",
				"project_id", projectID, "err", err)
		} else {
			slog.Info("transitioned to assembling after all TTS approved (single approve)", "project_id", projectID)
			s.webhooks.NotifyStateChange(projectID, project.SCPID, domain.StatusTTSReview, domain.StatusAssembling, reviewURL)
		}

		if s.pluginStatus != nil && s.pluginStatus["output"] {
			existingJob := s.jobs.getByType(projectID, "assembly")
			dbRunning, _ := s.store.GetRunningJobByProjectAndType(projectID, "assembly")
			if (existingJob == nil || existingJob.getStatus() != JobStatusRunning) && dbRunning == nil {
				jobID := uuid.New().String()
				dbJob := &domain.Job{
					ID:        jobID,
					ProjectID: projectID,
					Type:      "assembly",
					Status:    JobStatusRunning,
				}
				if err := s.store.CreateJob(dbJob); err == nil {
					ctx, cancel := context.WithCancel(context.Background())
					s.jobs.startTyped(projectID, "assembly", jobID, cancel)
					go s.executeAssembly(ctx, jobID, project)
					assemblyTriggered = true
					slog.Info("auto-triggered assembly after TTS approval (single approve)",
						"project_id", projectID, "job_id", jobID)
				}
			}
		}
	}

	WriteJSON(w, r, http.StatusOK, map[string]interface{}{
		"project_id":         projectID,
		"scene_num":          sceneNum,
		"asset_type":         assetType,
		"status":             "approved",
		"all_approved":       allApproved,
		"tts_triggered":      ttsTriggered,
		"assembly_triggered": assemblyTriggered,
	})
}

// requireReviewAuth is a helper that dispatches between Bearer auth (already validated
// by middleware) and review token auth. Returns the project on success.
// For routes that support both auth modes, handlers call this instead of validateReviewToken directly.
func requireReviewAuth(s *Server, w http.ResponseWriter, r *http.Request, projectID string) (*domain.Project, bool) {
	// If Bearer auth header is present (already validated by middleware), just load the project.
	// Also allow through if no token param is present — the request already passed the auth
	// middleware, meaning either auth is disabled or Bearer was validated.
	if r.Header.Get("Authorization") != "" || r.URL.Query().Get("token") == "" {
		project, err := s.store.GetProject(projectID)
		if err != nil {
			writeServiceError(w, r, err)
			return nil, false
		}
		return project, true
	}
	// Review token present — validate it
	return validateReviewToken(s, w, r, projectID)
}
