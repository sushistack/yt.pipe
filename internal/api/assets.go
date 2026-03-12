package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/sushistack/yt.pipe/internal/domain"
	"github.com/sushistack/yt.pipe/internal/plugin/imagegen"
	"github.com/sushistack/yt.pipe/internal/service"
	"github.com/sushistack/yt.pipe/internal/workspace"
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

// validImageGenStates defines project states that allow image generation.
var validImageGenStates = map[string]bool{
	domain.StatusApproved:    true,
	domain.StatusImageReview: true,
}

// validTTSGenStates defines project states that allow TTS generation.
var validTTSGenStates = map[string]bool{
	domain.StatusApproved:  true,
	domain.StatusTTSReview: true,
}

// handleGenerateImages enqueues selective image regeneration.
func (s *Server) handleGenerateImages(w http.ResponseWriter, r *http.Request) {
	if !s.requirePlugin(w, r, "imagegen") {
		return
	}

	projectID := chi.URLParam(r, "id")

	project, err := s.store.GetProject(projectID)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}

	// Validate project state
	if !validImageGenStates[project.Status] {
		WriteError(w, r, http.StatusConflict, "CONFLICT",
			fmt.Sprintf("project is in '%s' state; must be in 'approved' or 'image_review' to generate images", project.Status))
		return
	}

	var req generateRequest
	if r.ContentLength > 0 {
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
			WriteError(w, r, http.StatusBadRequest, "INVALID_REQUEST", "invalid JSON body")
			return
		}
	}

	// Validate scene numbers
	if err := validateSceneNumbers(req.Scenes, project.SceneCount); err != nil {
		WriteError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}

	// If scenes is empty, generate all scenes
	scenes := req.Scenes
	if len(scenes) == 0 {
		scenes = makeSceneRange(project.SceneCount)
	}

	// Check for duplicate running job
	if existing := s.jobs.getByType(projectID, "image_generate"); existing != nil && existing.getStatus() == JobStatusRunning {
		WriteError(w, r, http.StatusConflict, "CONFLICT",
			fmt.Sprintf("an image generation job is already running for this project (job_id: %s)", existing.JobID))
		return
	}
	// Also check DB for running jobs (in case server was restarted)
	if dbRunning, err := s.store.GetRunningJobByProjectAndType(projectID, "image_generate"); err == nil && dbRunning != nil {
		WriteError(w, r, http.StatusConflict, "CONFLICT",
			fmt.Sprintf("an image generation job is already running for this project (job_id: %s)", dbRunning.ID))
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

	// Track in job manager
	ctx, cancel := context.WithCancel(context.Background())
	s.jobs.startTyped(projectID, "image_generate", jobID, cancel)

	// Launch background goroutine
	go s.executeImageGeneration(ctx, jobID, project, scenes)

	WriteJSON(w, r, http.StatusAccepted, map[string]interface{}{
		"job_id":     jobID,
		"project_id": projectID,
		"scenes":     scenes,
		"type":       "image_generate",
	})
}

// handleGenerateTTS enqueues selective TTS regeneration.
func (s *Server) handleGenerateTTS(w http.ResponseWriter, r *http.Request) {
	if !s.requirePlugin(w, r, "tts") {
		return
	}

	projectID := chi.URLParam(r, "id")

	project, err := s.store.GetProject(projectID)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}

	// Validate project state
	if !validTTSGenStates[project.Status] {
		WriteError(w, r, http.StatusConflict, "CONFLICT",
			fmt.Sprintf("project is in '%s' state; must be in 'approved' or 'tts_review' to generate TTS", project.Status))
		return
	}

	var req generateRequest
	if r.ContentLength > 0 {
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
			WriteError(w, r, http.StatusBadRequest, "INVALID_REQUEST", "invalid JSON body")
			return
		}
	}

	if err := validateSceneNumbers(req.Scenes, project.SceneCount); err != nil {
		WriteError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}

	// If scenes is empty, generate all scenes
	scenes := req.Scenes
	if len(scenes) == 0 {
		scenes = makeSceneRange(project.SceneCount)
	}

	// Check for duplicate running job
	if existing := s.jobs.getByType(projectID, "tts_generate"); existing != nil && existing.getStatus() == JobStatusRunning {
		WriteError(w, r, http.StatusConflict, "CONFLICT",
			fmt.Sprintf("a TTS generation job is already running for this project (job_id: %s)", existing.JobID))
		return
	}
	// Also check DB for running jobs
	if dbRunning, err := s.store.GetRunningJobByProjectAndType(projectID, "tts_generate"); err == nil && dbRunning != nil {
		WriteError(w, r, http.StatusConflict, "CONFLICT",
			fmt.Sprintf("a TTS generation job is already running for this project (job_id: %s)", dbRunning.ID))
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

	// Track in job manager
	ctx, cancel := context.WithCancel(context.Background())
	s.jobs.startTyped(projectID, "tts_generate", jobID, cancel)

	// Launch background goroutine
	go s.executeTTSGeneration(ctx, jobID, project, scenes)

	WriteJSON(w, r, http.StatusAccepted, map[string]interface{}{
		"job_id":     jobID,
		"project_id": projectID,
		"scenes":     scenes,
		"type":       "tts_generate",
	})
}

// validAssemblyStates defines project states that allow assembly.
var validAssemblyStates = map[string]bool{
	domain.StatusTTSReview: true,
	domain.StatusApproved:  true,
}

// handleAssemble triggers CapCut project assembly.
func (s *Server) handleAssemble(w http.ResponseWriter, r *http.Request) {
	if !s.requirePlugin(w, r, "output") {
		return
	}

	projectID := chi.URLParam(r, "id")

	project, err := s.store.GetProject(projectID)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}

	// Validate project state allows assembly
	if !validAssemblyStates[project.Status] {
		WriteError(w, r, http.StatusConflict, "INVALID_STATE",
			fmt.Sprintf("project is in '%s' state; must be in 'tts_review' or 'approved' to assemble", project.Status))
		return
	}

	// Check all scenes have approved images and TTS
	imageApproved, err := s.store.AllApproved(projectID, domain.AssetTypeImage)
	if err != nil {
		WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to check image approvals")
		return
	}
	ttsApproved, err := s.store.AllApproved(projectID, domain.AssetTypeTTS)
	if err != nil {
		WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to check TTS approvals")
		return
	}

	if !imageApproved || !ttsApproved {
		// Build a message listing unapproved scenes
		var unapprovedDetails []string
		if !imageApproved {
			unapprovedDetails = append(unapprovedDetails, "images")
		}
		if !ttsApproved {
			unapprovedDetails = append(unapprovedDetails, "TTS")
		}
		WriteError(w, r, http.StatusConflict, "INVALID_STATE",
			fmt.Sprintf("not all scenes have approved assets: %s not fully approved", strings.Join(unapprovedDetails, ", ")))
		return
	}

	// Check for duplicate running assembly job
	if existing := s.jobs.getByType(projectID, "assembly"); existing != nil && existing.getStatus() == JobStatusRunning {
		WriteError(w, r, http.StatusConflict, "CONFLICT",
			fmt.Sprintf("an assembly job is already running for this project (job_id: %s)", existing.JobID))
		return
	}
	if dbRunning, err := s.store.GetRunningJobByProjectAndType(projectID, "assembly"); err == nil && dbRunning != nil {
		WriteError(w, r, http.StatusConflict, "CONFLICT",
			fmt.Sprintf("an assembly job is already running for this project (job_id: %s)", dbRunning.ID))
		return
	}

	// Create async job
	jobID := uuid.New().String()
	dbJob := &domain.Job{
		ID:        jobID,
		ProjectID: projectID,
		Type:      "assembly",
		Status:    JobStatusRunning,
	}
	if err := s.store.CreateJob(dbJob); err != nil {
		WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to create job")
		return
	}

	// Track in job manager
	ctx, cancel := context.WithCancel(context.Background())
	s.jobs.startTyped(projectID, "assembly", jobID, cancel)

	// Launch background goroutine
	go s.executeAssembly(ctx, jobID, project)

	WriteJSON(w, r, http.StatusAccepted, map[string]interface{}{
		"job_id":     jobID,
		"project_id": projectID,
		"type":       "assembly",
	})
}

// executeAssembly runs assembly in a background goroutine.
func (s *Server) executeAssembly(ctx context.Context, jobID string, project *domain.Project) {
	defer func() {
		s.jobs.removeTyped(project.ID, "assembly")
		if r := recover(); r != nil {
			slog.Error("assembly panic", "error", r, "project_id", project.ID)
			s.updateJobRecord(jobID, JobStatusFailed, 0, "", fmt.Sprintf("panic: %v", r))
		}
	}()

	projectPath := project.WorkspacePath
	if projectPath == "" {
		projectPath = filepath.Join(s.workspacePath, project.ID)
	}

	slog.Info("assembly started", "project_id", project.ID, "job_id", jobID)

	// Load scenes from workspace directory
	scenes, err := loadScenesFromWorkspace(projectPath)
	if err != nil {
		slog.Error("assembly failed: load scenes", "project_id", project.ID, "error", err)
		s.updateJobRecord(jobID, JobStatusFailed, 0, "", fmt.Sprintf("load scenes: %s", err.Error()))
		return
	}

	// Check context cancellation
	if err := ctx.Err(); err != nil {
		slog.Info("assembly cancelled", "project_id", project.ID, "job_id", jobID)
		s.updateJobRecord(jobID, JobStatusCancelled, 0, "", "cancelled")
		return
	}

	s.updateJobRecord(jobID, JobStatusRunning, 10, "", "")

	// Call assembler service
	result, err := s.assemblerSvc.Assemble(ctx, project.ID, scenes)
	if err != nil {
		slog.Error("assembly failed", "project_id", project.ID, "job_id", jobID, "error", err)
		s.updateJobRecord(jobID, JobStatusFailed, 0, "", err.Error())
		s.webhooks.NotifyJobFailed(project.ID, project.SCPID, jobID, "assembly", err.Error(), 0, BuildReviewURL(project.ID, project.ReviewToken))
		return
	}

	s.updateJobRecord(jobID, JobStatusComplete, 100, result.OutputPath, "")
	s.webhooks.NotifyJobComplete(project.ID, project.SCPID, jobID, "assembly", result.OutputPath, BuildReviewURL(project.ID, project.ReviewToken))
	slog.Info("assembly complete",
		"project_id", project.ID,
		"job_id", jobID,
		"output_path", result.OutputPath,
		"scene_count", result.SceneCount)
}

// loadScenesFromWorkspace loads scene data from workspace scene directories.
func loadScenesFromWorkspace(projectPath string) ([]domain.Scene, error) {
	scenesDir := filepath.Join(projectPath, "scenes")
	entries, err := os.ReadDir(scenesDir)
	if err != nil {
		return nil, fmt.Errorf("read scenes directory: %w", err)
	}

	var scenes []domain.Scene
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		manifestPath := filepath.Join(scenesDir, entry.Name(), "manifest.json")
		data, err := os.ReadFile(manifestPath)
		if err != nil {
			continue
		}
		scene, err := parseSceneManifestJSON(data)
		if err != nil {
			continue
		}
		scenes = append(scenes, *scene)
	}
	return scenes, nil
}

// parseSceneManifestJSON parses a scene manifest JSON into a domain.Scene.
func parseSceneManifestJSON(data []byte) (*domain.Scene, error) {
	var m struct {
		SceneNum      int                 `json:"scene_num"`
		Narration     string              `json:"narration"`
		ImagePath     string              `json:"image_path"`
		AudioPath     string              `json:"audio_path"`
		AudioDuration float64             `json:"audio_duration"`
		SubtitlePath  string              `json:"subtitle_path"`
		WordTimings   []domain.WordTiming `json:"word_timings"`
	}
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return &domain.Scene{
		SceneNum:      m.SceneNum,
		Narration:     m.Narration,
		ImagePath:     m.ImagePath,
		AudioPath:     m.AudioPath,
		AudioDuration: m.AudioDuration,
		SubtitlePath:  m.SubtitlePath,
		WordTimings:   m.WordTimings,
	}, nil
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

	// Persist prompt to workspace directory via workspace manager
	projectPath := project.WorkspacePath
	if projectPath == "" {
		projectPath = filepath.Join(s.workspacePath, project.ID)
	}

	sceneDir := filepath.Join(projectPath, "scenes", fmt.Sprintf("%d", sceneNum))
	promptPath := filepath.Join(sceneDir, "prompt.txt")
	if err := workspace.WriteFileAtomic(promptPath, []byte(req.Prompt)); err != nil {
		slog.Error("failed to persist prompt",
			"project_id", projectID,
			"scene_num", sceneNum,
			"error", err)
		WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to persist prompt")
		return
	}

	// Invalidate content hash in manifest for incremental build detection
	manifest, err := s.store.GetManifest(projectID, sceneNum)
	if err == nil {
		manifest.ContentHash = ""
		if updateErr := s.store.UpdateManifest(manifest); updateErr != nil {
			slog.Warn("failed to invalidate content hash",
				"project_id", projectID,
				"scene_num", sceneNum,
				"error", updateErr)
		}
	}

	now := time.Now().UTC()
	WriteJSON(w, r, http.StatusOK, map[string]interface{}{
		"project_id": projectID,
		"scene_num":  sceneNum,
		"prompt":     req.Prompt,
		"updated":    true,
		"updated_at": now.Format(time.RFC3339),
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

// executeImageGeneration runs image generation in a background goroutine.
func (s *Server) executeImageGeneration(ctx context.Context, jobID string, project *domain.Project, scenes []int) {
	defer func() {
		s.jobs.removeTyped(project.ID, "image_generate")
		if r := recover(); r != nil {
			slog.Error("image generation panic", "error", r, "project_id", project.ID)
			s.updateJobRecord(jobID, JobStatusFailed, 0, "", fmt.Sprintf("panic: %v", r))
		}
	}()

	total := len(scenes)
	completed := 0
	var resultPaths []string
	projectPath := project.WorkspacePath
	if projectPath == "" {
		projectPath = filepath.Join(s.workspacePath, project.ID)
	}

	slog.Info("image generation started",
		"project_id", project.ID,
		"job_id", jobID,
		"scenes", scenes,
		"total", total,
	)

	for _, sceneNum := range scenes {
		// Check context cancellation
		if err := ctx.Err(); err != nil {
			slog.Info("image generation cancelled", "project_id", project.ID, "job_id", jobID)
			s.updateJobRecord(jobID, JobStatusCancelled, completed*100/total, "", "cancelled")
			return
		}

		// Build a minimal prompt for the scene
		prompt := service.ImagePromptResult{
			SceneNum:        sceneNum,
			SanitizedPrompt: fmt.Sprintf("Scene %d image for project %s", sceneNum, project.SCPID),
			SCPID:           project.SCPID,
		}

		// Try to read a manual prompt from the scene directory
		if manualPrompt, exists, err := s.imageGenSvc.ReadManualPrompt(projectPath, sceneNum); err == nil && exists {
			prompt.SanitizedPrompt = manualPrompt
		}

		scene, err := s.imageGenSvc.GenerateSceneImage(ctx, prompt, project.ID, projectPath, imagegen.GenerateOptions{})
		if err != nil {
			slog.Error("image generation failed for scene",
				"project_id", project.ID,
				"job_id", jobID,
				"scene_num", sceneNum,
				"error", err,
			)
			errMsg := fmt.Sprintf("scene %d: %s", sceneNum, err.Error())
			s.updateJobRecord(jobID, JobStatusFailed, completed*100/total, "", errMsg)
			s.webhooks.NotifyJobFailed(project.ID, project.SCPID, jobID, "image_generate", errMsg, sceneNum, BuildReviewURL(project.ID, project.ReviewToken))
			return
		}

		completed++
		progress := completed * 100 / total
		if scene != nil && scene.ImagePath != "" {
			resultPaths = append(resultPaths, scene.ImagePath)
		}
		s.updateJobRecord(jobID, JobStatusRunning, progress, "", "")

		slog.Info("image generation scene complete",
			"project_id", project.ID,
			"job_id", jobID,
			"scene_num", sceneNum,
			"progress", fmt.Sprintf("%d/%d", completed, total),
		)
	}

	result := strings.Join(resultPaths, ",")
	s.updateJobRecord(jobID, JobStatusComplete, 100, result, "")
	s.webhooks.NotifyJobComplete(project.ID, project.SCPID, jobID, "image_generate", result, BuildReviewURL(project.ID, project.ReviewToken))
	slog.Info("image generation complete", "project_id", project.ID, "job_id", jobID, "scenes_completed", completed)
}

// executeTTSGeneration runs TTS generation in a background goroutine.
func (s *Server) executeTTSGeneration(ctx context.Context, jobID string, project *domain.Project, scenes []int) {
	defer func() {
		s.jobs.removeTyped(project.ID, "tts_generate")
		if r := recover(); r != nil {
			slog.Error("tts generation panic", "error", r, "project_id", project.ID)
			s.updateJobRecord(jobID, JobStatusFailed, 0, "", fmt.Sprintf("panic: %v", r))
		}
	}()

	total := len(scenes)
	completed := 0
	var resultPaths []string
	projectPath := project.WorkspacePath
	if projectPath == "" {
		projectPath = filepath.Join(s.workspacePath, project.ID)
	}

	slog.Info("tts generation started",
		"project_id", project.ID,
		"job_id", jobID,
		"scenes", scenes,
		"total", total,
	)

	// Default voice (can be extended to read from project config)
	voice := "default"

	for _, sceneNum := range scenes {
		// Check context cancellation
		if err := ctx.Err(); err != nil {
			slog.Info("tts generation cancelled", "project_id", project.ID, "job_id", jobID)
			s.updateJobRecord(jobID, JobStatusCancelled, completed*100/total, "", "cancelled")
			return
		}

		sceneScript := domain.SceneScript{
			SceneNum:  sceneNum,
			Narration: fmt.Sprintf("Narration for scene %d", sceneNum),
		}

		scene, err := s.ttsSvc.SynthesizeScene(ctx, sceneScript, project.ID, projectPath, voice)
		if err != nil {
			slog.Error("tts generation failed for scene",
				"project_id", project.ID,
				"job_id", jobID,
				"scene_num", sceneNum,
				"error", err,
			)
			errMsg := fmt.Sprintf("scene %d: %s", sceneNum, err.Error())
			s.updateJobRecord(jobID, JobStatusFailed, completed*100/total, "", errMsg)
			s.webhooks.NotifyJobFailed(project.ID, project.SCPID, jobID, "tts_generate", errMsg, sceneNum, BuildReviewURL(project.ID, project.ReviewToken))
			return
		}

		completed++
		progress := completed * 100 / total
		if scene != nil && scene.AudioPath != "" {
			resultPaths = append(resultPaths, scene.AudioPath)
		}
		s.updateJobRecord(jobID, JobStatusRunning, progress, "", "")

		slog.Info("tts generation scene complete",
			"project_id", project.ID,
			"job_id", jobID,
			"scene_num", sceneNum,
			"progress", fmt.Sprintf("%d/%d", completed, total),
		)
	}

	result := strings.Join(resultPaths, ",")
	s.updateJobRecord(jobID, JobStatusComplete, 100, result, "")
	s.webhooks.NotifyJobComplete(project.ID, project.SCPID, jobID, "tts_generate", result, BuildReviewURL(project.ID, project.ReviewToken))
	slog.Info("tts generation complete", "project_id", project.ID, "job_id", jobID, "scenes_completed", completed)
}

// makeSceneRange generates a slice of scene numbers from 1 to count.
func makeSceneRange(count int) []int {
	scenes := make([]int, count)
	for i := range scenes {
		scenes[i] = i + 1
	}
	return scenes
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
