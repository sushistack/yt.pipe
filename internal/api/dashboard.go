package api

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/sushistack/yt.pipe/internal/domain"
	"github.com/sushistack/yt.pipe/internal/service"
	"github.com/sushistack/yt.pipe/internal/workspace"
)

const defaultPageSize = 20

// initDashboardTemplates parses the dashboard templates with shared layout and partials.
func (s *Server) initDashboardTemplates() {
	funcMap := template.FuncMap{
		"truncID": func(id string) string {
			if len(id) > 8 {
				return id[:8] + "..."
			}
			return id
		},
		"formatTime": func(t time.Time) string {
			if t.IsZero() {
				return ""
			}
			return t.Format("2006-01-02")
		},
		"add": func(a, b int) int { return a + b },
		"sub": func(a, b int) int { return a - b },
	}

	// Parse layout + partials + each page template
	layoutData, err := templatesFS.ReadFile("templates/_layout.html")
	if err != nil {
		slog.Error("failed to load _layout.html", "error", err)
		return
	}

	// Read all partials
	partialFiles := []string{
		"templates/_partials/progress_bar.html",
		"templates/_partials/project_card.html",
		"templates/_partials/scene_card.html",
		"templates/_partials/toast.html",
		"templates/_partials/character_section.html",
	}
	var partials []string
	for _, pf := range partialFiles {
		data, readErr := templatesFS.ReadFile(pf)
		if readErr != nil {
			slog.Warn("failed to load partial", "file", pf, "error", readErr)
			continue
		}
		partials = append(partials, string(data))
	}

	// Parse dashboard list page
	dashData, err := templatesFS.ReadFile("templates/dashboard.html")
	if err != nil {
		slog.Error("failed to load dashboard.html", "error", err)
		return
	}
	dashTmpl, err := template.New("_layout.html").Funcs(funcMap).Parse(string(layoutData))
	if err != nil {
		slog.Error("failed to parse layout for dashboard", "error", err)
		return
	}
	for _, p := range partials {
		if _, err := dashTmpl.Parse(p); err != nil {
			slog.Warn("failed to parse partial for dashboard", "error", err)
		}
	}
	if _, err := dashTmpl.Parse(string(dashData)); err != nil {
		slog.Error("failed to parse dashboard.html", "error", err)
		return
	}
	s.dashboardTmpl = dashTmpl

	// Parse project detail page
	detailData, err := templatesFS.ReadFile("templates/project_detail.html")
	if err != nil {
		slog.Error("failed to load project_detail.html", "error", err)
		return
	}
	detailTmpl, err := template.New("_layout.html").Funcs(funcMap).Parse(string(layoutData))
	if err != nil {
		slog.Error("failed to parse layout for detail", "error", err)
		return
	}
	for _, p := range partials {
		if _, err := detailTmpl.Parse(p); err != nil {
			slog.Warn("failed to parse partial for detail", "error", err)
		}
	}
	if _, err := detailTmpl.Parse(string(detailData)); err != nil {
		slog.Error("failed to parse project_detail.html", "error", err)
		return
	}
	s.detailTmpl = detailTmpl

	slog.Info("dashboard templates loaded successfully")
}

const scpGroupPageSize = 20
const projectsPerGroup = 3

// scpGroupViewData is the view data for a single SCP accordion group.
type scpGroupViewData struct {
	SCPID        string
	Count        int
	LatestUpdate time.Time
	Projects     []*domain.Project
	HasMore      bool
	CurrentStage string
	NextPage     int
}

// dashboardListData is the template data for the dashboard list page.
type dashboardListData struct {
	APIKey       string
	Groups       []scpGroupViewData
	Stages       []string
	CurrentStage string
	CurrentSCP   string
	Total        int
	HasMore      bool
	NextPage     int
}

// handleDashboardList renders the project list page grouped by SCP ID.
func (s *Server) handleDashboardList(w http.ResponseWriter, r *http.Request) {
	stage := r.URL.Query().Get("stage")
	scp := r.URL.Query().Get("scp")
	pageStr := r.URL.Query().Get("page")
	page := 1
	if pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	offset := (page - 1) * scpGroupPageSize
	groups, total, err := s.store.ListSCPGroups(stage, scp, scpGroupPageSize, offset)
	if err != nil {
		slog.Error("failed to list scp groups", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// For each SCP group, load first N projects
	var viewGroups []scpGroupViewData
	for _, g := range groups {
		projects, projTotal, projErr := s.store.ListProjectsBySCP(g.SCPID, stage, projectsPerGroup, 0)
		if projErr != nil {
			slog.Warn("failed to list projects for scp group", "scp", g.SCPID, "error", projErr)
			continue
		}
		viewGroups = append(viewGroups, scpGroupViewData{
			SCPID:        g.SCPID,
			Count:        g.Count,
			LatestUpdate: g.LatestUpdate,
			Projects:     projects,
			HasMore:      projTotal > projectsPerGroup,
			CurrentStage: stage,
			NextPage:     2,
		})
	}

	data := dashboardListData{
		APIKey:       s.cfg.API.Auth.Key,
		Groups:       viewGroups,
		Stages:       domain.StageOrder,
		CurrentStage: stage,
		CurrentSCP:   scp,
		Total:        total,
		HasMore:      offset+len(groups) < total,
		NextPage:     page + 1,
	}

	// HTMX partial: return just the group list
	if isHTMX(r) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := s.dashboardTmpl.ExecuteTemplate(w, "scp_group_list", data); err != nil {
			slog.Error("failed to render group list partial", "error", err)
		}
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.dashboardTmpl.Execute(w, data); err != nil {
		slog.Error("failed to render dashboard page", "error", err)
	}
}

// handleSCPProjects returns projects for a specific SCP (HTMX partial for "load more" within accordion).
func (s *Server) handleSCPProjects(w http.ResponseWriter, r *http.Request) {
	scpID := chi.URLParam(r, "scpID")
	stage := r.URL.Query().Get("stage")
	pageStr := r.URL.Query().Get("page")
	page := 1
	if pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	offset := (page - 1) * projectsPerGroup
	projects, total, err := s.store.ListProjectsBySCP(scpID, stage, projectsPerGroup, offset)
	if err != nil {
		slog.Error("failed to list projects by scp", "scp", scpID, "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	data := struct {
		SCPID        string
		Projects     []*domain.Project
		HasMore      bool
		NextPage     int
		CurrentStage string
	}{
		SCPID:        scpID,
		Projects:     projects,
		HasMore:      offset+len(projects) < total,
		NextPage:     page + 1,
		CurrentStage: stage,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.dashboardTmpl.ExecuteTemplate(w, "scp_project_rows", data); err != nil {
		slog.Error("failed to render scp project rows", "error", err)
	}
}

// jobStatusData holds running job information for the template.
type jobStatusData struct {
	IsRunning      bool
	JobID          string
	JobStatus      string // running, complete, failed, waiting_approval
	Stage          string // pipeline stage: scenario_generate, image_generate, etc.
	StageLabel     string // human-readable: "Generating Scenario", "Generating Images", etc.
	ProgressPct    int
	ScenesTotal    int
	ScenesComplete int
	ElapsedSec     int
}

// UIStageOrder is the 6-step UI representation (images+tts merged as "assets").
var UIStageOrder = []string{"pending", "scenario", "character", "assets", "assemble", "complete"}

// projectDetailData is the template data for the project detail page.
type projectDetailData struct {
	APIKey              string
	Project             *domain.Project
	Scenes              []sceneViewData
	StageOrder          []string
	CurrentStage        string // mapped to UI stage: "assets" for images/tts
	ProjectID           string
	DependenciesMet     map[string]bool
	Job                 jobStatusData
	OutputFiles         []outputFileData
	Character           *domain.Character
	CharacterCandidates []*domain.CharacterCandidate
	CharacterStatus     string
	HasUploadedImage    bool
	Now                 int64 // Unix timestamp for cache-busting image URLs
	ScenarioPipeline    string // "4-stage" or "legacy-single-prompt" or ""
	ScenarioFormatGuide string // "applied" or "none" or ""
}

type outputFileData struct {
	Name string
	Size string
	Path string // relative path within workspace for download URL
}

type shotViewData struct {
	ShotNum  int
	ImageURL string
}

type sceneViewData struct {
	ProjectID   string
	SceneNum    int
	Prompt      string
	ImagePrompt string
	ImageStatus string
	TTSStatus   string
	HasImage    bool
	HasAudio    bool
	ImageURL    string
	AudioURL    string
	Shots       []shotViewData
}

// handleProjectDetail renders the project detail page.
func (s *Server) handleProjectDetail(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "id")
	project, err := s.store.GetProject(projectID)
	if err != nil {
		http.Error(w, "Project not found", http.StatusNotFound)
		return
	}

	// Build scene data from dashboard service
	scenes := make([]sceneViewData, 0, project.SceneCount)
	if project.SceneCount > 0 {
		dashSvc := service.NewSceneDashboardService(s.store, slog.Default())
		dashboard, dashErr := dashSvc.GetDashboard(projectID)
		if dashErr != nil {
			slog.Warn("failed to build scene dashboard", "project_id", projectID, "error", dashErr)
		} else {
			for _, sc := range dashboard.Scenes {
				hasImage := sc.ImagePath != "" || sc.ImageStatus == "generated" || sc.ImageStatus == "approved"
				hasAudio := sc.TTSPath != "" || sc.TTSStatus == "generated" || sc.TTSStatus == "approved"

				// Discover shot/cut images for this scene
				var shots []shotViewData
				projectPath := project.WorkspacePath
				if projectPath == "" {
					projectPath = filepath.Join(s.workspacePath, project.ID)
				}
				sceneDir := filepath.Join(projectPath, "scenes", strconv.Itoa(sc.SceneNum))
				// Try legacy shot_N naming first, then new cut_N_M naming
				for shotNum := 1; shotNum <= 50; shotNum++ {
					found := false
					for _, ext := range []string{"png", "jpg", "webp"} {
						p := filepath.Join(sceneDir, fmt.Sprintf("shot_%d.%s", shotNum, ext))
						if _, statErr := os.Stat(p); statErr == nil {
							found = true
							break
						}
					}
					if !found {
						// Try cut naming: cut_{sentenceStart}_{cutNum}
						for _, ext := range []string{"png", "jpg", "webp"} {
							p := filepath.Join(sceneDir, fmt.Sprintf("cut_%d_1.%s", shotNum, ext))
							if _, statErr := os.Stat(p); statErr == nil {
								found = true
								break
							}
						}
					}
					if !found {
						break
					}
					shots = append(shots, shotViewData{
						ShotNum:  shotNum,
						ImageURL: fmt.Sprintf("/dashboard/projects/%s/scenes/%d/shots/%d/image", projectID, sc.SceneNum, shotNum),
					})
				}
				if len(shots) > 0 {
					hasImage = true
				}

				scenes = append(scenes, sceneViewData{
					ProjectID:   projectID,
					SceneNum:    sc.SceneNum,
					Prompt:      sc.Prompt,
					ImagePrompt: sc.ImagePrompt,
					ImageStatus: sc.ImageStatus,
					TTSStatus:   sc.TTSStatus,
					HasImage:    hasImage,
					HasAudio:    hasAudio,
					ImageURL:    fmt.Sprintf("/dashboard/projects/%s/scenes/%d/image", projectID, sc.SceneNum),
					AudioURL:    fmt.Sprintf("/dashboard/projects/%s/scenes/%d/audio", projectID, sc.SceneNum),
					Shots:       shots,
				})
			}
		}
	}

	// Load character data
	var character *domain.Character
	var charCandidates []*domain.CharacterCandidate
	var charStatus string
	if s.characterSvc != nil {
		character, _ = s.characterSvc.CheckExistingCharacter(project.SCPID)
		charCandidates, _ = s.characterSvc.ListCandidates(project.ID)
		charStatus, _ = s.characterSvc.GetCandidateGenerationStatus(project.ID)
	}

	// Load scenario metadata to show pipeline mode
	var scenarioPipeline, scenarioFormatGuide string
	if project.WorkspacePath != "" {
		if scenarioData, err := workspace.ReadFile(filepath.Join(project.WorkspacePath, "scenario.json")); err == nil {
			var scenarioMeta struct {
				Metadata map[string]any `json:"metadata"`
			}
			if json.Unmarshal(scenarioData, &scenarioMeta) == nil && scenarioMeta.Metadata != nil {
				if v, ok := scenarioMeta.Metadata["pipeline_mode"].(string); ok {
					scenarioPipeline = v
				}
				if v, ok := scenarioMeta.Metadata["format_guide"].(string); ok {
					scenarioFormatGuide = v
				}
			}
		}
	}

	deps := computeDependencies(project, s.workspacePath)

	// Check for running job
	var jobData jobStatusData
	if job := s.jobs.get(projectID); job != nil && job.getStatus() == JobStatusRunning {
		progress := job.getProgress()
		jobData = jobStatusData{
			IsRunning:      true,
			JobID:          job.JobID,
			JobStatus:      job.getStatus(),
			Stage:          string(progress.Stage),
			StageLabel:     pipelineStageLabel(string(progress.Stage)),
			ProgressPct:    int(progress.ProgressPct),
			ScenesTotal:    progress.ScenesTotal,
			ScenesComplete: progress.ScenesComplete,
			ElapsedSec:     int(time.Since(job.StartedAt).Seconds()),
		}
	} else {
		// Check typed jobs (image_generate, tts_generate)
		for _, jt := range []string{"character_generate", "image_generate", "tts_generate", "assembly"} {
			if tj := s.jobs.getByType(projectID, jt); tj != nil && tj.getStatus() == JobStatusRunning {
				progress := tj.getProgress()
				jobData = jobStatusData{
					IsRunning:      true,
					JobID:          tj.JobID,
					JobStatus:      tj.getStatus(),
					Stage:          jt,
					StageLabel:     pipelineStageLabel(jt),
					ProgressPct:    int(progress.ProgressPct),
					ScenesTotal:    progress.ScenesTotal,
					ScenesComplete: progress.ScenesComplete,
					ElapsedSec:     int(time.Since(tj.StartedAt).Seconds()),
				}
				break
			}
		}
	}

	// Character dependency (outside computeDependencies which is filesystem-based)
	deps["character"] = character != nil && character.SelectedImagePath != ""

	// Map backend stage to UI stage (images/tts → assets)
	uiStage := project.Status
	if uiStage == domain.StageImages || uiStage == domain.StageTTS {
		uiStage = "assets"
	}

	// Add UI-level dependencies
	deps["assets"] = deps["images"] && deps["tts"]
	deps["assemble"] = deps["assets"] // assemble is ready when all assets are done

	// Detect output files (CapCut project files)
	var outputFiles []outputFileData
	if project.Status == domain.StageComplete {
		outputFiles = detectOutputFiles(project.WorkspacePath)
	}

	data := projectDetailData{
		APIKey:              s.cfg.API.Auth.Key,
		Project:             project,
		Scenes:              scenes,
		StageOrder:          UIStageOrder,
		CurrentStage:        uiStage,
		ProjectID:           project.ID,
		DependenciesMet:     deps,
		Job:                 jobData,
		OutputFiles:         outputFiles,
		Character:           character,
		CharacterCandidates: charCandidates,
		CharacterStatus:     charStatus,
		HasUploadedImage:    hasUploadedImage(project),
		Now:                 time.Now().Unix(),
		ScenarioPipeline:    scenarioPipeline,
		ScenarioFormatGuide: scenarioFormatGuide,
	}

	// HTMX partial: return just the content section (for stage changes)
	if isHTMX(r) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := s.detailTmpl.ExecuteTemplate(w, "project_detail_content", data); err != nil {
			slog.Error("failed to render detail partial", "error", err)
		}
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.detailTmpl.Execute(w, data); err != nil {
		slog.Error("failed to render detail page", "error", err)
	}
}

// pipelineStageLabel returns a human-readable label for a pipeline stage.
func pipelineStageLabel(stage string) string {
	labels := map[string]string{
		"data_load":           "Loading data...",
		"scenario_generate":   "Generating scenario...",
		"scenario_approval":   "Waiting for approval...",
		"character_select":    "Generating Characters...",
		"character_generate":  "Generating character candidates...",
		"image_generate":      "Generating images...",
		"tts_synthesize":     "Generating TTS...",
		"timing_resolve":     "Resolving timing...",
		"subtitle_generate":  "Generating subtitles...",
		"assemble":           "Assembling output...",
		"assembly":           "Assembling output...",
	}
	if label, ok := labels[stage]; ok {
		return label
	}
	return "Processing..."
}

// computeDependencies checks filesystem to determine which stage dependencies are met.
func computeDependencies(project *domain.Project, basePath string) map[string]bool {
	deps := map[string]bool{
		"pending":  true,
		"scenario": false,
		"images":   false,
		"tts":      false,
		"complete": false,
	}

	if project.WorkspacePath == "" {
		return deps
	}

	// Check scenario: scene count > 0 means scenario was generated
	if project.SceneCount > 0 {
		deps["scenario"] = true
	}

	if project.SceneCount == 0 {
		return deps
	}

	// Check images: all scenes have image files (image.*, shot_*, or cut_* naming)
	allImages := true
	allTTS := true
	for i := 1; i <= project.SceneCount; i++ {
		sceneDir := filepath.Join(project.WorkspacePath, "scenes", fmt.Sprintf("%d", i))
		hasImage := fileExistsWithExtensions(sceneDir, "image", []string{"png", "jpg", "webp"})
		if !hasImage {
			// Check for shot_N or cut_N_M naming (multi-shot / cut decomposition)
			shotMatches, _ := filepath.Glob(filepath.Join(sceneDir, "shot_*.png"))
			cutMatches, _ := filepath.Glob(filepath.Join(sceneDir, "cut_*.png"))
			hasImage = len(shotMatches) > 0 || len(cutMatches) > 0
		}
		if !hasImage {
			allImages = false
		}
		if !fileExistsWithExtensions(sceneDir, "audio", []string{"wav", "mp3", "ogg"}) {
			allTTS = false
		}
	}
	deps["images"] = allImages
	deps["tts"] = allTTS
	deps["complete"] = allImages && allTTS

	return deps
}

// detectOutputFiles scans the workspace for assembly output files.
func detectOutputFiles(workspacePath string) []outputFileData {
	var files []outputFileData
	// Look for CapCut output directory
	outputDir := filepath.Join(workspacePath, "output")
	entries, err := os.ReadDir(outputDir)
	if err != nil {
		// Also check root for draft_content.json
		if info, err := os.Stat(filepath.Join(workspacePath, "draft_content.json")); err == nil {
			files = append(files, outputFileData{
				Name: "draft_content.json",
				Size: formatFileSize(info.Size()),
				Path: "draft_content.json",
			})
		}
		return files
	}
	for _, e := range entries {
		if e.IsDir() {
			// Check for draft_content.json inside subdirectories (CapCut project structure)
			subEntries, err := os.ReadDir(filepath.Join(outputDir, e.Name()))
			if err != nil {
				continue
			}
			for _, se := range subEntries {
				if !se.IsDir() {
					info, _ := se.Info()
					size := ""
					if info != nil {
						size = formatFileSize(info.Size())
					}
					files = append(files, outputFileData{
						Name: e.Name() + "/" + se.Name(),
						Size: size,
						Path: "output/" + e.Name() + "/" + se.Name(),
					})
				}
			}
		} else {
			info, _ := e.Info()
			size := ""
			if info != nil {
				size = formatFileSize(info.Size())
			}
			files = append(files, outputFileData{
				Name: e.Name(),
				Size: size,
				Path: "output/" + e.Name(),
			})
		}
	}
	return files
}

func formatFileSize(bytes int64) string {
	switch {
	case bytes >= 1024*1024:
		return fmt.Sprintf("%.1f MB", float64(bytes)/1024/1024)
	case bytes >= 1024:
		return fmt.Sprintf("%.1f KB", float64(bytes)/1024)
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

// handleDashboardOutputFile serves output files (CapCut JSON, etc.) for download.
func (s *Server) handleDashboardOutputFile(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "id")
	project, err := s.store.GetProject(projectID)
	if err != nil {
		http.Error(w, "Project not found", http.StatusNotFound)
		return
	}

	// Extract the file path from the wildcard
	filePath := chi.URLParam(r, "*")
	if filePath == "" {
		http.Error(w, "File path required", http.StatusBadRequest)
		return
	}

	// Prevent directory traversal
	if strings.Contains(filePath, "..") {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	projectPath := project.WorkspacePath
	if projectPath == "" {
		projectPath = filepath.Join(s.workspacePath, project.ID)
	}

	fullPath := filepath.Join(projectPath, filePath)
	info, err := os.Stat(fullPath)
	if err != nil || info.IsDir() {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	// Set download headers
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filepath.Base(fullPath)))
	http.ServeFile(w, r, fullPath)
}

// handleDashboardAsset serves scene image/audio files for the dashboard (Bearer auth, no review token).
func (s *Server) handleDashboardAsset(w http.ResponseWriter, r *http.Request, filename string) {
	projectID := chi.URLParam(r, "id")
	project, err := s.store.GetProject(projectID)
	if err != nil {
		http.Error(w, "Project not found", http.StatusNotFound)
		return
	}

	numStr := chi.URLParam(r, "num")
	sceneNum, convErr := strconv.Atoi(numStr)
	if convErr != nil || sceneNum < 1 {
		http.Error(w, "Invalid scene number", http.StatusBadRequest)
		return
	}

	projectPath := project.WorkspacePath
	if projectPath == "" {
		projectPath = filepath.Join(s.workspacePath, project.ID)
	}

	assetPath := filepath.Join(projectPath, "scenes", strconv.Itoa(sceneNum), filename)
	cleaned := filepath.Clean(assetPath)
	if !strings.HasPrefix(cleaned, filepath.Clean(projectPath)) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	if _, statErr := os.Stat(cleaned); os.IsNotExist(statErr) {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	http.ServeFile(w, r, cleaned)
}

// handleCharacterImage serves the selected character image for a project's SCP ID.
func (s *Server) handleCharacterImage(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "id")
	project, err := s.store.GetProject(projectID)
	if err != nil {
		http.Error(w, "Project not found", http.StatusNotFound)
		return
	}
	if s.characterSvc == nil {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
	char, err := s.characterSvc.CheckExistingCharacter(project.SCPID)
	if err != nil || char == nil || char.SelectedImagePath == "" {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
	cleaned := filepath.Clean(char.SelectedImagePath)
	if _, statErr := os.Stat(cleaned); os.IsNotExist(statErr) {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
	http.ServeFile(w, r, cleaned)
}

// handleCandidateImage serves a candidate character image with path safety validation.
func (s *Server) handleCandidateImage(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "id")
	num, err := strconv.Atoi(chi.URLParam(r, "num"))
	if err != nil || num < 1 || num > 10 {
		WriteError(w, r, http.StatusBadRequest, "INVALID_CANDIDATE",
			"candidate number must be 1-10")
		return
	}

	project, err := s.store.GetProject(projectID)
	if err != nil {
		WriteError(w, r, http.StatusNotFound, "NOT_FOUND", "project not found")
		return
	}

	imgPath := filepath.Join(project.WorkspacePath, project.SCPID,
		"characters", fmt.Sprintf("candidate_%d.png", num))
	cleaned := filepath.Clean(imgPath)
	if !strings.HasPrefix(cleaned, filepath.Clean(project.WorkspacePath)) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	http.ServeFile(w, r, cleaned)
}

// handleUploadedCharacterImage serves the user-uploaded character image.
func (s *Server) handleUploadedCharacterImage(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "id")
	project, err := s.store.GetProject(projectID)
	if err != nil {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
	imgPath := service.UploadedImagePath(project.WorkspacePath, project.SCPID)
	if _, err := os.Stat(imgPath); os.IsNotExist(err) {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
	http.ServeFile(w, r, imgPath)
}

// hasUploadedImage checks if a user-uploaded character image exists for the project.
func hasUploadedImage(project *domain.Project) bool {
	imgPath := filepath.Join(project.WorkspacePath, project.SCPID, "characters", "uploaded.png")
	_, err := os.Stat(imgPath)
	return err == nil
}

func (s *Server) handleDashboardImage(w http.ResponseWriter, r *http.Request) {
	s.handleDashboardAsset(w, r, "image.png")
}

func (s *Server) handleDashboardShotImage(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "id")
	project, err := s.store.GetProject(projectID)
	if err != nil {
		http.Error(w, "Project not found", http.StatusNotFound)
		return
	}

	numStr := chi.URLParam(r, "num")
	sceneNum, convErr := strconv.Atoi(numStr)
	if convErr != nil || sceneNum < 1 {
		http.Error(w, "Invalid scene number", http.StatusBadRequest)
		return
	}

	shotStr := chi.URLParam(r, "shotNum")
	shotNum, shotErr := strconv.Atoi(shotStr)
	if shotErr != nil || shotNum < 1 {
		http.Error(w, "Invalid shot number", http.StatusBadRequest)
		return
	}

	projectPath := project.WorkspacePath
	if projectPath == "" {
		projectPath = filepath.Join(s.workspacePath, project.ID)
	}

	sceneDir := filepath.Join(projectPath, "scenes", strconv.Itoa(sceneNum))
	// Try legacy shot_N naming, then new cut_N_1 naming
	patterns := []string{
		fmt.Sprintf("shot_%d", shotNum),
		fmt.Sprintf("cut_%d_1", shotNum),
	}
	for _, pattern := range patterns {
		for _, ext := range []string{"png", "jpg", "webp"} {
			assetPath := filepath.Join(sceneDir, pattern+"."+ext)
			cleaned := filepath.Clean(assetPath)
			if !strings.HasPrefix(cleaned, filepath.Clean(projectPath)) {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}
			if _, statErr := os.Stat(cleaned); statErr == nil {
				http.ServeFile(w, r, cleaned)
				return
			}
		}
	}
	http.Error(w, "Not found", http.StatusNotFound)
}

func (s *Server) handleDashboardAudio(w http.ResponseWriter, r *http.Request) {
	s.handleDashboardAsset(w, r, "audio.wav")
}

// handleListAvailableSCPs returns paginated SCP entries from the data directory as JSON.
// Query params: q (search), offset (default 0), limit (default 50).
func (s *Server) handleListAvailableSCPs(w http.ResponseWriter, r *http.Request) {
	// Build set of SCP IDs that already have projects
	existing := make(map[string]bool)
	projects, listErr := s.store.ListProjects()
	if listErr == nil {
		for _, p := range projects {
			existing[p.SCPID] = true
		}
	}

	// Rebuild cache each request (filesystem-based, no long-lived state needed)
	cache, err := workspace.NewSCPListCache(s.cfg.SCPDataPath, existing)
	if err != nil {
		slog.Error("failed to list available scps", "error", err)
		WriteJSON(w, r, http.StatusOK, workspace.SCPListResult{Items: []workspace.SCPListEntry{}})
		return
	}

	q := r.URL.Query().Get("q")
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	result := cache.Query(q, offset, limit)
	WriteJSON(w, r, http.StatusOK, result)
}

// fileExistsWithExtensions checks if a file with the given base name and any of the extensions exists.
func fileExistsWithExtensions(dir, baseName string, exts []string) bool {
	for _, ext := range exts {
		path := filepath.Join(dir, baseName+"."+ext)
		if _, err := os.Stat(path); err == nil {
			return true
		}
	}
	// Also check without extension pattern — some files may use different naming
	matches, _ := filepath.Glob(filepath.Join(dir, baseName+".*"))
	return len(matches) > 0
}
