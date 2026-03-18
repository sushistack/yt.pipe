package cli

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/sushistack/yt.pipe/internal/api"
	"github.com/sushistack/yt.pipe/internal/glossary"
	"github.com/sushistack/yt.pipe/internal/pipeline"
	"github.com/sushistack/yt.pipe/internal/plugin/imagegen"
	"github.com/sushistack/yt.pipe/internal/plugin/output"
	"github.com/sushistack/yt.pipe/internal/service"
	"github.com/sushistack/yt.pipe/internal/store"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the HTTP API server",
	Long:  "Start the REST API server for managing projects and pipelines remotely.",
	RunE:  runServeCmd,
}

func init() {
	serveCmd.Flags().Int("port", 0, "override API server port")
	rootCmd.AddCommand(serveCmd)
}

func runServeCmd(cmd *cobra.Command, _ []string) error {
	cmd.SilenceUsage = true

	cfg := GetConfig()
	if cfg == nil {
		return fmt.Errorf("serve: configuration not loaded")
	}
	c := cfg.Config

	// Override port from flag if provided
	if port, _ := cmd.Flags().GetInt("port"); port > 0 {
		c.API.Port = port
	}

	// Open database
	dbPath := c.DBPath
	if dbPath == "" {
		dbPath = c.WorkspacePath + "/yt-pipe.db"
	}
	db, err := store.New(dbPath)
	if err != nil {
		return fmt.Errorf("serve: open database: %w", err)
	}
	defer db.Close()

	// Initialize plugins with graceful degradation
	plugins := createPluginsGraceful(cfg)

	logger := slog.Default()

	// Load glossary if configured
	var g *glossary.Glossary
	if c.GlossaryPath != "" {
		g = glossary.LoadFromFile(c.GlossaryPath)
	}

	// Shared ProjectService instance for all services
	projectSvc := service.NewProjectService(db)

	// Resolve templates path with fallback for local development
	tplPath := c.TemplatesPath
	if tplPath == "" {
		tplPath = "templates"
	}
	if _, err := os.Stat(filepath.Join(tplPath, "scenario")); os.IsNotExist(err) {
		if _, err2 := os.Stat(filepath.Join("templates", "scenario")); err2 == nil {
			tplPath = "templates"
			slog.Info("templates_path fallback to relative ./templates", "configured", c.TemplatesPath)
		} else {
			slog.Warn("templates_path not found — 4-stage pipeline will be DISABLED", "path", tplPath)
			tplPath = ""
		}
	}

	// Build server options with initialized services
	opts := []api.ServerOption{
		api.WithRegistry(pluginRegistry),
		api.WithPluginStatus(plugins.Status),
	}

	if plugins.LLM != nil {
		scenarioSvc := service.NewScenarioService(db, plugins.LLM, projectSvc)
		if tplPath != "" {
			scenarioSvc.SetTemplatesDir(tplPath)
		}
		scenarioSvc.SetGlossary(g)
		opts = append(opts, api.WithScenarioService(scenarioSvc))
	}
	if plugins.ImageGen != nil {
		imageGenSvc := service.NewImageGenService(plugins.ImageGen, db, logger)
		opts = append(opts, api.WithImageGenService(imageGenSvc))
	}
	if plugins.TTS != nil {
		ttsSvc := service.NewTTSService(plugins.TTS, g, db, logger)
		opts = append(opts, api.WithTTSService(ttsSvc))
	}

	// Create CharacterService and wire plugins
	characterSvc := service.NewCharacterService(db)
	if plugins.LLM != nil {
		characterSvc.SetLLM(plugins.LLM)
	}
	if plugins.ImageGen != nil {
		characterSvc.SetImageGen(plugins.ImageGen)
	}
	opts = append(opts, api.WithCharacterService(characterSvc))

	// Output assembler is always available (built-in CapCut)
	assemblerSvc := service.NewAssemblerService(plugins.Output, projectSvc)
	// Apply canvas config
	canvas := output.DefaultCanvasConfig()
	if c.Output.CanvasWidth > 0 {
		canvas.Width = c.Output.CanvasWidth
	}
	if c.Output.CanvasHeight > 0 {
		canvas.Height = c.Output.CanvasHeight
	}
	if c.Output.FPS > 0 {
		canvas.FPS = float64(c.Output.FPS)
	}
	assemblerSvc.WithConfig(c.Output.TemplatePath, c.Output.MetaPath, canvas)
	// Wire extra assemblers for "both" output mode
	if len(plugins.Outputs) > 1 {
		assemblerSvc.WithExtraAssemblers(plugins.Outputs[1:]...)
	}
	opts = append(opts, api.WithAssemblerService(assemblerSvc))

	// Create pipeline runner (fixes existing gap where executeFullPipeline always fails)
	imgOpts := imagegen.GenerateOptions{}
	if c.ImageGen.Width > 0 {
		imgOpts.Width = c.ImageGen.Width
	}
	if c.ImageGen.Height > 0 {
		imgOpts.Height = c.ImageGen.Height
	}
	runner := pipeline.NewRunner(db, plugins.LLM, plugins.ImageGen, plugins.TTS, plugins.Output, g, logger, pipeline.RunnerConfig{
		SCPDataPath:          c.SCPDataPath,
		WorkspacePath:        c.WorkspacePath,
		Voice:                c.TTS.Voice,
		ImageOpts:            imgOpts,
		Canvas:               canvas,
		TemplatePath:         c.Output.TemplatePath,
		MetaPath:             c.Output.MetaPath,
		TemplatesPath:        tplPath,
		DefaultSceneDuration:  c.Output.DefaultSceneDuration,
		CharacterSvc:          characterSvc,
		AutoApprovalEnabled:   c.AutoApproval.Enabled && c.ImageValidation.Enabled,
		AutoApprovalThreshold: c.AutoApproval.Threshold,
	})
	opts = append(opts, api.WithPipelineRunner(runner))

	// Create and start server
	srv := api.NewServer(db, c, opts...)

	// Graceful shutdown on SIGTERM/SIGINT
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		if err := srv.Start(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case <-ctx.Done():
		slog.Info("shutdown signal received")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("serve: shutdown: %w", err)
		}
		slog.Info("server stopped gracefully")
		return nil
	case err := <-errCh:
		if err != nil {
			return fmt.Errorf("serve: %w", err)
		}
		return nil
	}
}
