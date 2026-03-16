package api

import (
	"context"
	"embed"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"time"

	"io/fs"

	"github.com/go-chi/chi/v5"
	"github.com/sushistack/yt.pipe/internal/config"
	"github.com/sushistack/yt.pipe/internal/pipeline"
	"github.com/sushistack/yt.pipe/internal/plugin"
	"github.com/sushistack/yt.pipe/internal/service"
	"github.com/sushistack/yt.pipe/internal/store"
)

//go:embed all:templates
var templatesFS embed.FS

//go:embed static/*
var staticFS embed.FS

// Version can be set at build time via ldflags.
var Version = "dev"

// Server is the HTTP API server.
type Server struct {
	router          chi.Router
	httpServer      *http.Server
	store           *store.Store
	cfg             *config.Config
	projectSvc      *service.ProjectService
	reviewSvc       *service.ReviewService
	scenarioSvc     *service.ScenarioService
	imageGenSvc     *service.ImageGenService
	ttsSvc          *service.TTSService
	assemblerSvc    *service.AssemblerService
	characterSvc    *service.CharacterService
	jobs            *jobManager
	registry        *plugin.Registry
	pipelineRunner  *pipeline.Runner
	webhooks        *WebhookNotifier
	reviewTmpl      *template.Template
	dashboardTmpl   *template.Template
	detailTmpl      *template.Template
	reviewCSS       template.CSS
	pluginStatus    map[string]bool
	version         string
	workspacePath   string
}

// ServerOption configures the server.
type ServerOption func(*Server)

// WithRegistry sets the plugin registry.
func WithRegistry(r *plugin.Registry) ServerOption {
	return func(s *Server) { s.registry = r }
}

// WithScenarioService sets the scenario service.
func WithScenarioService(svc *service.ScenarioService) ServerOption {
	return func(s *Server) { s.scenarioSvc = svc }
}

// WithImageGenService sets the image generation service.
func WithImageGenService(svc *service.ImageGenService) ServerOption {
	return func(s *Server) { s.imageGenSvc = svc }
}

// WithTTSService sets the TTS service.
func WithTTSService(svc *service.TTSService) ServerOption {
	return func(s *Server) { s.ttsSvc = svc }
}

// WithAssemblerService sets the assembler service.
func WithAssemblerService(svc *service.AssemblerService) ServerOption {
	return func(s *Server) { s.assemblerSvc = svc }
}

// WithPipelineRunner sets the pipeline runner for full pipeline execution.
func WithPipelineRunner(r *pipeline.Runner) ServerOption {
	return func(s *Server) { s.pipelineRunner = r }
}

// WithCharacterService sets the character service.
func WithCharacterService(svc *service.CharacterService) ServerOption {
	return func(s *Server) { s.characterSvc = svc }
}

// WithPluginStatus sets the plugin availability status map.
func WithPluginStatus(status map[string]bool) ServerOption {
	return func(s *Server) { s.pluginStatus = status }
}

// NewServer creates a new API server.
func NewServer(st *store.Store, cfg *config.Config, opts ...ServerOption) *Server {
	s := &Server{
		store:         st,
		cfg:           cfg,
		projectSvc:    service.NewProjectService(st),
		reviewSvc:     service.NewReviewService(st, slog.Default()),
		jobs:          newJobManager(),
		registry:      plugin.NewRegistry(),
		webhooks:      NewWebhookNotifier(cfg.Webhooks),
		pluginStatus:  map[string]bool{"llm": false, "imagegen": false, "tts": false, "output": false},
		version:       Version,
		workspacePath: cfg.WorkspacePath,
	}
	for _, opt := range opts {
		opt(s)
	}

	// Parse templates
	s.initReviewTemplate()
	s.initDashboardTemplates()

	s.setupRouter()
	return s
}

func (s *Server) initReviewTemplate() {
	funcMap := template.FuncMap{
		"progressPercent": func(d *service.SceneDashboard) int {
			if d.TotalScenes == 0 {
				return 0
			}
			total := d.TotalScenes * 2 // image + TTS per scene
			approved := d.ApprovedImageCount + d.ApprovedTTSCount
			return approved * 100 / total
		},
	}

	// Load CSS content for inline embedding
	cssData, err := templatesFS.ReadFile("templates/styles.css")
	if err != nil {
		slog.Warn("failed to load review styles.css", "error", err)
		cssData = []byte("")
	}

	htmlData, err := templatesFS.ReadFile("templates/review.html")
	if err != nil {
		slog.Warn("failed to load review.html template", "error", err)
		return
	}

	// The template uses {{.StylesCSS}} to inline the CSS
	tmpl, err := template.New("review.html").Funcs(funcMap).Parse(string(htmlData))
	if err != nil {
		slog.Error("failed to parse review template", "error", err)
		return
	}

	s.reviewTmpl = tmpl
	s.reviewCSS = template.CSS(cssData)
	slog.Info("review template loaded successfully")
}

func (s *Server) setupRouter() {
	r := chi.NewRouter()

	// Middleware stack: Recovery → RequestID → Logging → Auth
	r.Use(RecoveryMiddleware)
	r.Use(RequestIDMiddleware)
	r.Use(LoggingMiddleware)
	r.Use(AuthMiddleware(s.cfg.API.Auth.Enabled, s.cfg.API.Auth.Key))

	if s.cfg.API.Auth.Enabled {
		slog.Info("api authentication enabled")
	} else {
		slog.Warn("api authentication disabled; all endpoints are publicly accessible")
	}

	// Public endpoints (auth exempt via AuthMiddleware path check)
	r.Get("/health", s.handleHealth)
	r.Get("/ready", s.handleReady)
	r.Get("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	// Static files (auth exempt via path check in AuthMiddleware)
	staticSub, _ := fs.Sub(staticFS, "static")
	r.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.FS(staticSub))))

	// Review page (auth via review token, exempt from Bearer in AuthMiddleware)
	r.Get("/review/{project_id}", s.handleReviewPage)

	// Dashboard pages (auth via Bearer, handled by global middleware)
	r.Get("/dashboard/", s.handleDashboardList)
	r.Get("/dashboard/scp/{scpID}/projects", s.handleSCPProjects)
	r.Get("/dashboard/projects/{id}", s.handleProjectDetail)
	r.Get("/dashboard/projects/{id}/characters/image", s.handleCharacterImage)
	r.Get("/dashboard/projects/{id}/characters/uploaded/image", s.handleUploadedCharacterImage)
	r.Get("/dashboard/projects/{id}/characters/candidates/{num}/image", s.handleCandidateImage)
	r.Get("/dashboard/projects/{id}/scenes/{num}/image", s.handleDashboardImage)
	r.Get("/dashboard/projects/{id}/scenes/{num}/shots/{shotNum}/image", s.handleDashboardShotImage)
	r.Get("/dashboard/projects/{id}/scenes/{num}/audio", s.handleDashboardAudio)
	r.Get("/dashboard/projects/{id}/output/*", s.handleDashboardOutputFile)
	r.Get("/dashboard/scps", s.handleListAvailableSCPs)

	// API v1 routes
	r.Route("/api/v1", func(r chi.Router) {
		// Projects
		r.Post("/projects", s.handleCreateProject)
		r.Get("/projects", s.handleListProjects)
		r.Get("/projects/{id}", s.handleGetProject)
		r.Delete("/projects/{id}", s.handleDeleteProject)
		r.Patch("/projects/{id}/stage", s.handleSetStage)

		// Pipeline control
		r.Post("/projects/{id}/run", s.handleRunPipeline)
		r.Get("/projects/{id}/status", s.handleGetStatus)
		r.Post("/projects/{id}/cancel", s.handleCancelPipeline)
		r.Post("/projects/{id}/approve", s.handleApprovePipeline)

		// Scene dashboard & approval (dual auth: Bearer or review token)
		r.Get("/projects/{id}/scenes", s.handleGetScenes)
		r.Post("/projects/{id}/scenes/{num}/approve", s.handleReviewApproveScene)
		r.Post("/projects/{id}/scenes/{num}/reject", s.handleRejectScene)

		// Review-specific routes (review token auth)
		r.Get("/projects/{id}/scenes/{num}/image", s.handleServeImage)
		r.Get("/projects/{id}/scenes/{num}/shots/{shotNum}/image", s.handleServeShotImage)
		r.Get("/projects/{id}/scenes/{num}/audio", s.handleServeAudio)
		r.Patch("/projects/{id}/scenes/{num}/narration", s.handleUpdateNarration)
		r.Post("/projects/{id}/scenes", s.handleAddScene)
		r.Delete("/projects/{id}/scenes/{num}", s.handleDeleteScene)
		r.Post("/projects/{id}/approve-all", s.handleApproveAll)

		// Token rotation (Bearer auth only, NOT review-token accessible)
		r.Post("/projects/{id}/review-token/rotate", s.handleRotateReviewToken)

		// Character management
		r.Post("/projects/{id}/characters/generate", s.handleGenerateCharacters)
		r.Get("/projects/{id}/characters/candidates", s.handleListCandidates)
		r.Post("/projects/{id}/characters/select", s.handleSelectCharacter)
		r.Post("/projects/{id}/characters/deselect", s.handleDeselectCharacter)
		r.Post("/projects/{id}/characters/upload", s.handleUploadCharacterImage)
		r.Post("/projects/{id}/characters/select-uploaded", s.handleSelectUploadedImage)
		r.Delete("/projects/{id}/characters/uploaded", s.handleDeleteUploadedImage)
		r.Get("/projects/{id}/characters", s.handleGetCharacter)

		// Asset management
		r.Post("/projects/{id}/images/generate", s.handleGenerateImages)
		r.Post("/projects/{id}/tts/generate", s.handleGenerateTTS)
		r.Post("/projects/{id}/assemble", s.handleAssemble)
		r.Put("/projects/{id}/scenes/{num}/prompt", s.handleUpdatePrompt)
		r.Post("/projects/{id}/feedback", s.handleCreateFeedback)

		// Jobs
		r.Get("/jobs/{jobId}", s.handleGetJob)

		// Configuration & plugins
		r.Get("/config", s.handleGetConfig)
		r.Patch("/config", s.handlePatchConfig)
		r.Get("/plugins", s.handleListPlugins)
		r.Put("/plugins/{type}/active", s.handleSetActivePlugin)
	})

	s.router = r
}

// Start begins listening on the configured address. It blocks until the server stops.
func (s *Server) Start() error {
	addr := fmt.Sprintf("%s:%d", s.cfg.API.Host, s.cfg.API.Port)
	s.httpServer = &http.Server{
		Addr:         addr,
		Handler:      s.router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
	slog.Info("api server starting", "addr", addr, "version", s.version)
	return s.httpServer.ListenAndServe()
}

// Shutdown gracefully shuts down the server with the given context.
func (s *Server) Shutdown(ctx context.Context) error {
	slog.Info("api server shutting down")
	return s.httpServer.Shutdown(ctx)
}

// Router returns the chi router for testing.
func (s *Server) Router() chi.Router {
	return s.router
}

// requirePlugin checks if a plugin type is available. Returns true if available,
// or writes a 502 error response and returns false if the plugin is unavailable.
func (s *Server) requirePlugin(w http.ResponseWriter, r *http.Request, pluginType string) bool {
	if s.pluginStatus != nil && s.pluginStatus[pluginType] {
		return true
	}
	WriteError(w, r, http.StatusBadGateway, "API_UPSTREAM_ERROR",
		pluginType+" plugin is not available")
	return false
}
