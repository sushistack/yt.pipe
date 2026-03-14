package api

import (
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

// projectDetailData is the template data for the project detail page.
type projectDetailData struct {
	APIKey         string
	Project        *domain.Project
	Scenes         []sceneViewData
	StageOrder     []string
	CurrentStage   string
	ProjectID      string
	DependenciesMet map[string]bool
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
				})
			}
		}
	}

	deps := computeDependencies(project, s.workspacePath)

	data := projectDetailData{
		APIKey:          s.cfg.API.Auth.Key,
		Project:         project,
		Scenes:          scenes,
		StageOrder:      domain.StageOrder,
		CurrentStage:    project.Status,
		ProjectID:       project.ID,
		DependenciesMet: deps,
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

	// Check images: all scenes have image files
	allImages := true
	allTTS := true
	for i := 1; i <= project.SceneCount; i++ {
		sceneDir := filepath.Join(project.WorkspacePath, "scenes", fmt.Sprintf("%d", i))
		if !fileExistsWithExtensions(sceneDir, "image", []string{"png", "jpg", "webp"}) {
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

func (s *Server) handleDashboardImage(w http.ResponseWriter, r *http.Request) {
	s.handleDashboardAsset(w, r, "image.png")
}

func (s *Server) handleDashboardAudio(w http.ResponseWriter, r *http.Request) {
	s.handleDashboardAsset(w, r, "audio.wav")
}

// handleListAvailableSCPs returns available SCP entries from the data directory as JSON.
func (s *Server) handleListAvailableSCPs(w http.ResponseWriter, r *http.Request) {
	scps, err := workspace.ListAvailableSCPs(s.cfg.SCPDataPath)
	if err != nil {
		slog.Error("failed to list available scps", "error", err)
		WriteJSON(w, r, http.StatusOK, []workspace.SCPListEntry{})
		return
	}
	WriteJSON(w, r, http.StatusOK, scps)
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
