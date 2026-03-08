package api

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jay/youtube-pipeline/internal/config"
	"github.com/jay/youtube-pipeline/internal/plugin"
	"github.com/jay/youtube-pipeline/internal/service"
	"github.com/jay/youtube-pipeline/internal/store"
)

// Version can be set at build time via ldflags.
var Version = "dev"

// Server is the HTTP API server.
type Server struct {
	router        chi.Router
	httpServer    *http.Server
	store         *store.Store
	cfg           *config.Config
	projectSvc    *service.ProjectService
	jobs          *jobManager
	registry      *plugin.Registry
	webhooks      *WebhookNotifier
	version       string
	workspacePath string
}

// ServerOption configures the server.
type ServerOption func(*Server)

// WithRegistry sets the plugin registry.
func WithRegistry(r *plugin.Registry) ServerOption {
	return func(s *Server) { s.registry = r }
}

// NewServer creates a new API server.
func NewServer(st *store.Store, cfg *config.Config, opts ...ServerOption) *Server {
	s := &Server{
		store:         st,
		cfg:           cfg,
		projectSvc:    service.NewProjectService(st),
		jobs:          newJobManager(),
		registry:      plugin.NewRegistry(),
		webhooks:      NewWebhookNotifier(cfg.Webhooks),
		version:       Version,
		workspacePath: cfg.WorkspacePath,
	}
	for _, opt := range opts {
		opt(s)
	}
	s.setupRouter()
	return s
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

	// API v1 routes
	r.Route("/api/v1", func(r chi.Router) {
		// Projects
		r.Post("/projects", s.handleCreateProject)
		r.Get("/projects", s.handleListProjects)
		r.Get("/projects/{id}", s.handleGetProject)
		r.Delete("/projects/{id}", s.handleDeleteProject)

		// Pipeline control
		r.Post("/projects/{id}/run", s.handleRunPipeline)
		r.Get("/projects/{id}/status", s.handleGetStatus)
		r.Post("/projects/{id}/cancel", s.handleCancelPipeline)
		r.Post("/projects/{id}/approve", s.handleApprovePipeline)

		// Asset management
		r.Post("/projects/{id}/images/generate", s.handleGenerateImages)
		r.Post("/projects/{id}/tts/generate", s.handleGenerateTTS)
		r.Put("/projects/{id}/scenes/{num}/prompt", s.handleUpdatePrompt)
		r.Post("/projects/{id}/feedback", s.handleCreateFeedback)

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
