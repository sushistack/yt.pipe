package cli

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jay/youtube-pipeline/internal/api"
	"github.com/jay/youtube-pipeline/internal/store"
	"github.com/spf13/cobra"
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

	// Create and start server
	srv := api.NewServer(db, c)

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
