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

// validImageGenStates defines project stages that allow image generation.
// Generation actions are gated by dependency checks, not state — permit all non-pending stages.
var validImageGenStates = map[string]bool{
	domain.StageScenario:  true,
	domain.StageCharacter: true,
	domain.StageImages:    true,
	domain.StageTTS:       true,
	domain.StageComplete:  true,
}

// validTTSGenStates defines project stages that allow TTS generation.
var validTTSGenStates = map[string]bool{
	domain.StageScenario:  true,
	domain.StageCharacter: true,
	domain.StageImages:    true,
	domain.StageTTS:       true,
	domain.StageComplete:  true,
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

	// If a TTS job is already running, return it (idempotent)
	if existing := s.jobs.getByType(projectID, "tts_generate"); existing != nil && existing.getStatus() == JobStatusRunning {
		WriteJSON(w, r, http.StatusAccepted, map[string]interface{}{
			"job_id":     existing.JobID,
			"project_id": projectID,
			"type":       "tts_generate",
			"already_running": true,
		})
		return
	}
	if dbRunning, err := s.store.GetRunningJobByProjectAndType(projectID, "tts_generate"); err == nil && dbRunning != nil {
		WriteJSON(w, r, http.StatusAccepted, map[string]interface{}{
			"job_id":     dbRunning.ID,
			"project_id": projectID,
			"type":       "tts_generate",
			"already_running": true,
		})
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
		slog.Error("failed to create TTS job", "error", err, "project_id", projectID, "job_id", jobID)
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

// validAssemblyStates defines project stages that allow assembly.
var validAssemblyStates = map[string]bool{
	domain.StageScenario:  true,
	domain.StageCharacter: true,
	domain.StageImages:    true,
	domain.StageTTS:       true,
	domain.StageComplete:  true,
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

	// If an assembly job is already running, return it (idempotent)
	if existing := s.jobs.getByType(projectID, "assembly"); existing != nil && existing.getStatus() == JobStatusRunning {
		WriteJSON(w, r, http.StatusAccepted, map[string]interface{}{
			"job_id":          existing.JobID,
			"project_id":     projectID,
			"type":           "assembly",
			"already_running": true,
		})
		return
	}
	if dbRunning, err := s.store.GetRunningJobByProjectAndType(projectID, "assembly"); err == nil && dbRunning != nil {
		WriteJSON(w, r, http.StatusAccepted, map[string]interface{}{
			"job_id":          dbRunning.ID,
			"project_id":     projectID,
			"type":           "assembly",
			"already_running": true,
		})
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
		s.webhooks.NotifyJobFailed(project.ID, project.SCPID, jobID, "assembly", err.Error(), 0, "", BuildReviewURL(project.ID, project.ReviewToken))
		return
	}

	// Transition project to complete
	previousState := project.Status
	if _, err := s.projectSvc.SetProjectStage(ctx, project.ID, domain.StageComplete); err != nil {
		slog.Warn("failed to set stage to complete", "project_id", project.ID, "err", err)
	}

	s.updateJobRecord(jobID, JobStatusComplete, 100, result.OutputPath, "")
	s.webhooks.NotifyStateChange(project.ID, project.SCPID, previousState, domain.StageComplete, BuildReviewURL(project.ID, project.ReviewToken))
	s.webhooks.NotifyJobComplete(project.ID, project.SCPID, jobID, "assembly", result.OutputPath, "", BuildReviewURL(project.ID, project.ReviewToken))
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
			slog.Debug("scene manifest not found", "dir", entry.Name(), "path", manifestPath, "err", err)
			continue
		}
		scene, err := parseSceneManifestJSON(data)
		if err != nil {
			slog.Warn("scene manifest parse failed", "dir", entry.Name(), "err", err)
			continue
		}
		scenes = append(scenes, *scene)
	}
	slog.Info("loaded scenes from workspace", "count", len(scenes), "dir", scenesDir)
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

// writeSceneManifest writes a manifest.json file to the scene directory for assembly.
func writeSceneManifest(sceneDir string, scene *domain.Scene) error {
	m := struct {
		SceneNum      int                 `json:"scene_num"`
		Narration     string              `json:"narration"`
		ImagePath     string              `json:"image_path"`
		AudioPath     string              `json:"audio_path"`
		AudioDuration float64             `json:"audio_duration"`
		SubtitlePath  string              `json:"subtitle_path,omitempty"`
		WordTimings   []domain.WordTiming `json:"word_timings,omitempty"`
	}{
		SceneNum:      scene.SceneNum,
		Narration:     scene.Narration,
		ImagePath:     scene.ImagePath,
		AudioPath:     scene.AudioPath,
		AudioDuration: scene.AudioDuration,
		SubtitlePath:  scene.SubtitlePath,
		WordTimings:   scene.WordTimings,
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}
	return workspace.WriteFileAtomic(filepath.Join(sceneDir, "manifest.json"), data)
}

// subtitleEntry represents a single subtitle cue for JSON serialization.
type subtitleEntry struct {
	Index    int     `json:"index"`
	StartSec float64 `json:"start_sec"`
	EndSec   float64 `json:"end_sec"`
	Text     string  `json:"text"`
}

// generateSubtitleJSON creates a subtitle.json from word timings, grouping words into lines.
func generateSubtitleJSON(path string, timings []domain.WordTiming) error {
	const maxWordsPerLine = 8
	var entries []subtitleEntry
	idx := 1

	for i := 0; i < len(timings); {
		end := i + maxWordsPerLine
		if end > len(timings) {
			end = len(timings)
		}
		var words []string
		for j := i; j < end; j++ {
			words = append(words, timings[j].Word)
		}
		entry := subtitleEntry{
			Index:    idx,
			StartSec: timings[i].StartSec,
			EndSec:   timings[end-1].EndSec,
			Text:     strings.Join(words, " "),
		}
		entries = append(entries, entry)
		idx++
		i = end
	}

	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal subtitle: %w", err)
	}
	return workspace.WriteFileAtomic(path, data)
}

// generateSubtitleFromNarration creates a subtitle.json by splitting narration into
// roughly equal segments when word timings are unavailable (e.g. Qwen3 TTS).
func generateSubtitleFromNarration(path, narration string, durationSec float64) error {
	if narration == "" || durationSec <= 0 {
		return nil
	}

	// Split Korean text by sentences (periods, exclamation, question marks)
	// then group into subtitle entries
	sentences := splitSentences(narration)
	if len(sentences) == 0 {
		return nil
	}

	// Distribute duration proportionally by character count
	totalChars := 0
	for _, s := range sentences {
		totalChars += len([]rune(s))
	}

	var entries []subtitleEntry
	offset := 0.0
	for i, sentence := range sentences {
		charLen := len([]rune(sentence))
		segDuration := durationSec * float64(charLen) / float64(totalChars)
		entries = append(entries, subtitleEntry{
			Index:    i + 1,
			StartSec: offset,
			EndSec:   offset + segDuration,
			Text:     sentence,
		})
		offset += segDuration
	}

	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal subtitle: %w", err)
	}
	return workspace.WriteFileAtomic(path, data)
}

// splitSentences splits text into sentences by Korean/common punctuation.
func splitSentences(text string) []string {
	var sentences []string
	var current strings.Builder
	for _, r := range text {
		current.WriteRune(r)
		if r == '.' || r == '!' || r == '?' || r == '。' {
			s := strings.TrimSpace(current.String())
			if s != "" {
				sentences = append(sentences, s)
			}
			current.Reset()
		}
	}
	// Remaining text
	if s := strings.TrimSpace(current.String()); s != "" {
		sentences = append(sentences, s)
	}
	return sentences
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

	// Wire character service and selected image for Edit() support
	if s.characterSvc != nil {
		s.imageGenSvc.SetCharacterService(s.characterSvc)
		char, _ := s.characterSvc.CheckExistingCharacter(project.SCPID)
		if char != nil && char.SelectedImagePath != "" {
			if err := s.imageGenSvc.SetSelectedCharacterImage(char.SelectedImagePath); err != nil {
				slog.Warn("failed to load character image for edit", "project_id", project.ID, "err", err)
			} else {
				slog.Info("character image loaded for image-edit",
					"project_id", project.ID, "scp_id", project.SCPID, "path", char.SelectedImagePath)
			}
		}
	}

	// Initialize approval records for each scene
	for _, sceneNum := range scenes {
		if err := s.store.InitApproval(project.ID, sceneNum, domain.AssetTypeImage); err != nil {
			slog.Warn("failed to init image approval", "project_id", project.ID, "scene_num", sceneNum, "err", err)
		}
	}

	// Load scenario to get visual descriptions for image prompts
	scenarioPath := filepath.Join(projectPath, "scenario.json")
	scenario, scenarioErr := service.LoadScenarioFromFile(scenarioPath)
	if scenarioErr != nil {
		slog.Warn("could not load scenario for image prompts, using fallback",
			"project_id", project.ID, "err", scenarioErr)
	}

	// Build scene visual description lookup
	sceneVisuals := make(map[int]string)
	sceneNarrations := make(map[int]string)
	sceneEntityVisible := make(map[int]bool)
	if scenario != nil {
		for _, sc := range scenario.Scenes {
			sceneVisuals[sc.SceneNum] = sc.VisualDescription
			sceneNarrations[sc.SceneNum] = sc.Narration
			sceneEntityVisible[sc.SceneNum] = sc.EntityVisible
		}
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

		// Build prompt from scenario visual description
		promptText := sceneVisuals[sceneNum]
		if promptText == "" {
			promptText = fmt.Sprintf("Scene %d image for project %s", sceneNum, project.SCPID)
		}
		prompt := service.ImagePromptResult{
			SceneNum:        sceneNum,
			SanitizedPrompt: promptText,
			SCPID:           project.SCPID,
			SceneText:       sceneNarrations[sceneNum],
			EntityVisible:   sceneEntityVisible[sceneNum],
		}

		// Try to read a manual prompt from the scene directory (overrides scenario)
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
			s.webhooks.NotifyJobFailed(project.ID, project.SCPID, jobID, "image_generate", errMsg, sceneNum, "", BuildReviewURL(project.ID, project.ReviewToken))
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

	// Mark all scenes as generated+approved (no manual approval step) and transition project
	for _, sceneNum := range scenes {
		if err := s.store.MarkGenerated(project.ID, sceneNum, domain.AssetTypeImage); err != nil {
			slog.Warn("failed to mark image generated", "project_id", project.ID, "scene_num", sceneNum, "err", err)
		}
		if err := s.store.ApproveScene(project.ID, sceneNum, domain.AssetTypeImage); err != nil {
			slog.Warn("failed to auto-approve image", "project_id", project.ID, "scene_num", sceneNum, "err", err)
		}
	}
	if _, err := s.projectSvc.SetProjectStage(ctx, project.ID, domain.StageImages); err != nil {
		slog.Warn("failed to set stage to images", "project_id", project.ID, "err", err)
	}
	s.webhooks.NotifyStateChange(project.ID, project.SCPID, domain.StageScenario, domain.StageImages, BuildReviewURL(project.ID, project.ReviewToken))

	result := strings.Join(resultPaths, ",")
	s.updateJobRecord(jobID, JobStatusComplete, 100, result, "")
	s.webhooks.NotifyJobComplete(project.ID, project.SCPID, jobID, "image_generate", result, "", BuildReviewURL(project.ID, project.ReviewToken))
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

	// Initialize approval records for each scene
	for _, sceneNum := range scenes {
		if err := s.store.InitApproval(project.ID, sceneNum, domain.AssetTypeTTS); err != nil {
			slog.Warn("failed to init tts approval", "project_id", project.ID, "scene_num", sceneNum, "err", err)
		}
	}

	// Load scenario to get actual narration text for each scene
	scenarioPath := filepath.Join(projectPath, "scenario.json")
	scenario, scenarioErr := service.LoadScenarioFromFile(scenarioPath)
	if scenarioErr != nil {
		slog.Error("tts generation failed: load scenario", "project_id", project.ID, "error", scenarioErr)
		errMsg := fmt.Sprintf("load scenario: %s", scenarioErr.Error())
		s.updateJobRecord(jobID, JobStatusFailed, 0, "", errMsg)
		s.webhooks.NotifyJobFailed(project.ID, project.SCPID, jobID, "tts_generate", errMsg, 0, "", BuildReviewURL(project.ID, project.ReviewToken))
		return
	}

	// Build narration lookup by scene number
	narrationMap := make(map[int]domain.SceneScript, len(scenario.Scenes))
	for _, sc := range scenario.Scenes {
		narrationMap[sc.SceneNum] = sc
	}

	slog.Info("tts generation started",
		"project_id", project.ID,
		"job_id", jobID,
		"scenes", scenes,
		"total", total,
	)

	// Read voice from config
	voice := s.cfg.TTS.Voice

	for _, sceneNum := range scenes {
		// Check context cancellation
		if err := ctx.Err(); err != nil {
			slog.Info("tts generation cancelled", "project_id", project.ID, "job_id", jobID)
			s.updateJobRecord(jobID, JobStatusCancelled, completed*100/total, "", "cancelled")
			return
		}

		sceneScript, ok := narrationMap[sceneNum]
		if !ok {
			slog.Error("tts generation failed: no narration for scene",
				"project_id", project.ID, "scene_num", sceneNum)
			errMsg := fmt.Sprintf("scene %d: narration not found in scenario.json", sceneNum)
			s.updateJobRecord(jobID, JobStatusFailed, completed*100/total, "", errMsg)
			s.webhooks.NotifyJobFailed(project.ID, project.SCPID, jobID, "tts_generate", errMsg, sceneNum, "", BuildReviewURL(project.ID, project.ReviewToken))
			return
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
			s.webhooks.NotifyJobFailed(project.ID, project.SCPID, jobID, "tts_generate", errMsg, sceneNum, "", BuildReviewURL(project.ID, project.ReviewToken))
			return
		}

		completed++
		progress := completed * 100 / total
		if scene != nil && scene.AudioPath != "" {
			resultPaths = append(resultPaths, scene.AudioPath)
		}

		// Generate subtitle and write manifest.json for assembly
		if scene != nil {
			scene.Narration = sceneScript.Narration
			sceneDir := filepath.Join(projectPath, "scenes", fmt.Sprintf("%d", sceneNum))
			scene.ImagePath = filepath.Join(sceneDir, "image.png")

			// Generate subtitle.json
			subtitlePath := filepath.Join(sceneDir, "subtitle.json")
			var subtitleErr error
			if len(scene.WordTimings) > 0 {
				subtitleErr = generateSubtitleJSON(subtitlePath, scene.WordTimings)
			} else {
				// No word timings (e.g. Qwen3 TTS) — generate from narration text + duration
				subtitleErr = generateSubtitleFromNarration(subtitlePath, sceneScript.Narration, scene.AudioDuration)
			}
			if subtitleErr != nil {
				slog.Warn("failed to generate subtitle", "project_id", project.ID, "scene_num", sceneNum, "err", subtitleErr)
			} else {
				scene.SubtitlePath = subtitlePath
			}

			if err := writeSceneManifest(sceneDir, scene); err != nil {
				slog.Warn("failed to write scene manifest", "project_id", project.ID, "scene_num", sceneNum, "err", err)
			}
		}

		s.updateJobRecord(jobID, JobStatusRunning, progress, "", "")

		slog.Info("tts generation scene complete",
			"project_id", project.ID,
			"job_id", jobID,
			"scene_num", sceneNum,
			"progress", fmt.Sprintf("%d/%d", completed, total),
		)
	}

	// Mark all scenes as generated+approved (no manual approval step) and transition project
	for _, sceneNum := range scenes {
		if err := s.store.MarkGenerated(project.ID, sceneNum, domain.AssetTypeTTS); err != nil {
			slog.Warn("failed to mark tts generated", "project_id", project.ID, "scene_num", sceneNum, "err", err)
		}
		if err := s.store.ApproveScene(project.ID, sceneNum, domain.AssetTypeTTS); err != nil {
			slog.Warn("failed to auto-approve tts", "project_id", project.ID, "scene_num", sceneNum, "err", err)
		}
	}
	// Only set stage if not already at tts (handleApproveAll may have set it early)
	currentProject, _ := s.store.GetProject(project.ID)
	if currentProject == nil || currentProject.Status != domain.StageTTS {
		if _, err := s.projectSvc.SetProjectStage(ctx, project.ID, domain.StageTTS); err != nil {
			slog.Warn("failed to set stage to tts", "project_id", project.ID, "err", err)
		}
	}
	// Always notify after TTS generation completes (not on stage transition)
	s.webhooks.NotifyStateChange(project.ID, project.SCPID, domain.StageImages, domain.StageTTS, BuildReviewURL(project.ID, project.ReviewToken))

	result := strings.Join(resultPaths, ",")
	s.updateJobRecord(jobID, JobStatusComplete, 100, result, "")
	s.webhooks.NotifyJobComplete(project.ID, project.SCPID, jobID, "tts_generate", result, "", BuildReviewURL(project.ID, project.ReviewToken))
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
