package pipeline

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/sushistack/yt.pipe/internal/service"
)

// GracefulRunner wraps a pipeline Runner with signal handling for graceful shutdown.
type GracefulRunner struct {
	runner *Runner
	cm     *CheckpointManager
	logger *slog.Logger
}

// NewGracefulRunner creates a GracefulRunner that handles OS signals.
func NewGracefulRunner(runner *Runner, logger *slog.Logger) *GracefulRunner {
	return &GracefulRunner{
		runner: runner,
		cm:     NewCheckpointManager(logger),
		logger: logger,
	}
}

// RunWithSignalHandling runs the pipeline with context cancellation on SIGINT/SIGTERM.
// On cancellation, it saves a checkpoint and prints a resume message.
func (gr *GracefulRunner) RunWithSignalHandling(ctx context.Context, scpID string) (*RunResult, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		select {
		case sig := <-sigCh:
			gr.logger.Warn("signal received, cancelling pipeline",
				"signal", sig.String(),
				"scp_id", scpID)
			cancel()
		case <-ctx.Done():
		}
		signal.Stop(sigCh)
	}()

	result, err := gr.runner.Run(ctx, scpID)
	if ctx.Err() != nil && result != nil && result.ProjectID != "" {
		gr.handleInterruption(result)
	}
	return result, err
}

// ResumeWithSignalHandling resumes a pipeline with signal handling.
func (gr *GracefulRunner) ResumeWithSignalHandling(ctx context.Context, projectID string) (*RunResult, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		select {
		case sig := <-sigCh:
			gr.logger.Warn("signal received, cancelling pipeline",
				"signal", sig.String(),
				"project_id", projectID)
			cancel()
		case <-ctx.Done():
		}
		signal.Stop(sigCh)
	}()

	result, err := gr.runner.Resume(ctx, projectID)
	if ctx.Err() != nil && result != nil && result.ProjectID != "" {
		gr.handleInterruption(result)
	}
	return result, err
}

func (gr *GracefulRunner) handleInterruption(result *RunResult) {
	// Find the last completed stage from the result
	var lastStage service.PipelineStage
	for _, s := range result.Stages {
		if s.Status == "pass" {
			lastStage = service.PipelineStage(s.Name)
		}
	}

	gr.logger.Info("pipeline interrupted, checkpoint saved",
		"project_id", result.ProjectID,
		"last_stage", lastStage)

	fmt.Fprintf(os.Stderr, "\nPipeline interrupted. Resume with: yt-pipe run %s --resume %s\n",
		result.SCPID, result.ProjectID)
}

// APIErrorClassifier classifies API errors as retryable or non-retryable.
type APIErrorClassifier struct{}

// HTTPError represents an HTTP API error with status code.
type HTTPError struct {
	StatusCode int
	Message    string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("HTTP %d: %s", e.StatusCode, e.Message)
}

// IsRetryable implements retry.RetryableError.
func (e *HTTPError) IsRetryable() bool {
	switch {
	case e.StatusCode == 429: // Rate limited
		return true
	case e.StatusCode >= 500: // Server errors
		return true
	case e.StatusCode == 408: // Request timeout
		return true
	case e.StatusCode == 400, e.StatusCode == 401, e.StatusCode == 403, e.StatusCode == 404:
		return false
	default:
		return false
	}
}

// TimeoutError represents an API call timeout.
type TimeoutError struct {
	Operation string
	Timeout   string
}

func (e *TimeoutError) Error() string {
	return fmt.Sprintf("%s: timed out after %s", e.Operation, e.Timeout)
}

// IsRetryable implements retry.RetryableError. Timeouts are retryable.
func (e *TimeoutError) IsRetryable() bool {
	return true
}
