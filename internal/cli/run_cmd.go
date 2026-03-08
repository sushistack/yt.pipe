package cli

import (
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/jay/youtube-pipeline/internal/glossary"
	"github.com/jay/youtube-pipeline/internal/pipeline"
	"github.com/jay/youtube-pipeline/internal/plugin/imagegen"
	"github.com/jay/youtube-pipeline/internal/plugin/output"
	"github.com/jay/youtube-pipeline/internal/plugin/output/capcut"
	"github.com/jay/youtube-pipeline/internal/store"
	"github.com/spf13/cobra"
)

var runCmd = &cobra.Command{
	Use:   "run <scp-id>",
	Short: "Run the content pipeline for an SCP",
	Long:  "Execute the full content generation pipeline for a given SCP ID. Use --dry-run to verify configuration without making API calls.",
	Args:  cobra.ExactArgs(1),
	RunE:  runRunCmd,
}

func init() {
	runCmd.Flags().Bool("dry-run", false, "verify pipeline flow without making real API calls")
	runCmd.Flags().String("resume", "", "resume pipeline from project ID (after scenario approval)")
	rootCmd.AddCommand(runCmd)
}

func runRunCmd(cmd *cobra.Command, args []string) error {
	scpID := args[0]

	dryRun, _ := cmd.Flags().GetBool("dry-run")
	if dryRun {
		return runDryRun(cmd, scpID)
	}

	cfg := GetConfig()
	if cfg == nil {
		cmd.SilenceUsage = true
		return fmt.Errorf("run: configuration not loaded")
	}
	c := cfg.Config

	// Open database
	dbPath := c.DBPath
	if dbPath == "" {
		dbPath = c.WorkspacePath + "/yt-pipe.db"
	}
	db, err := store.New(dbPath)
	if err != nil {
		cmd.SilenceUsage = true
		return fmt.Errorf("run: open database: %w", err)
	}
	defer db.Close()

	// Build plugin instances (using registry or direct creation)
	// For now, plugins must be available — real implementations expected
	llmPlugin, imgPlugin, ttsPlugin, err := createPlugins(cfg)
	if err != nil {
		cmd.SilenceUsage = true
		return fmt.Errorf("run: %w", err)
	}

	assembler := capcut.New()

	// Load glossary if configured
	var g *glossary.Glossary
	if c.GlossaryPath != "" {
		g = glossary.LoadFromFile(c.GlossaryPath)
	}

	logger := slog.Default()

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

	runner := pipeline.NewRunner(db, llmPlugin, imgPlugin, ttsPlugin, assembler, g, logger, pipeline.RunnerConfig{
		SCPDataPath:   c.SCPDataPath,
		WorkspacePath: c.WorkspacePath,
		Voice:         c.TTS.Voice,
		ImageOpts:     imagegen.GenerateOptions{},
		Canvas:        canvas,
		TemplatePath:  c.Output.TemplatePath,
		MetaPath:      c.Output.MetaPath,
	})

	// Set progress callback for stderr output
	tracker := pipeline.NewProgressTracker(cmd.ErrOrStderr())
	runner.ProgressFunc = tracker.OnProgress

	// Check if resuming from approval
	resumeID, _ := cmd.Flags().GetString("resume")
	if resumeID != "" {
		result, err := runner.Resume(cmd.Context(), resumeID)
		if err != nil {
			cmd.SilenceUsage = true
			return err
		}
		return outputRunResult(cmd, result)
	}

	// Fresh run
	result, err := runner.Run(cmd.Context(), scpID)
	if err != nil {
		cmd.SilenceUsage = true
		return err
	}

	return outputRunResult(cmd, result)
}

func outputRunResult(cmd *cobra.Command, result *pipeline.RunResult) error {
	w := cmd.OutOrStdout()

	jsonOutput, _ := cmd.Flags().GetBool("json-output")
	if jsonOutput {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}

	fmt.Fprintf(w, "\n=== Pipeline Result for %s ===\n\n", result.SCPID)
	fmt.Fprintf(w, "  Project ID:   %s\n", result.ProjectID)
	fmt.Fprintf(w, "  Status:       %s\n", result.Status)
	fmt.Fprintf(w, "  Scene Count:  %d\n", result.SceneCount)
	fmt.Fprintf(w, "  Elapsed:      %s\n", result.TotalElapsed)
	fmt.Fprintln(w)

	fmt.Fprintln(w, "Stages:")
	for i, stage := range result.Stages {
		icon := "✓"
		if stage.Status == "fail" {
			icon = "✗"
		} else if stage.Status == "paused" {
			icon = "⏸"
		}
		fmt.Fprintf(w, "  %d. [%s] %s (%dms)\n", i+1, icon, stage.Name, stage.DurationMs)
		if stage.Error != "" {
			fmt.Fprintf(w, "     Error: %s\n", stage.Error)
		}
	}
	fmt.Fprintln(w)

	if result.PausedAt != "" {
		fmt.Fprintf(w, "Pipeline paused at %s.\n", result.PausedAt)
		fmt.Fprintf(w, "Review the scenario, then resume with:\n")
		fmt.Fprintf(w, "  yt-pipe scenario approve <project-id>\n")
		fmt.Fprintf(w, "  yt-pipe run %s --resume %s\n\n", result.SCPID, result.ProjectID)
	} else if result.Status == "complete" {
		fmt.Fprintln(w, "Pipeline completed successfully.")
	}

	return nil
}

func runDryRun(cmd *cobra.Command, scpID string) error {
	cfg := GetConfig()
	if cfg == nil {
		cmd.SilenceUsage = true
		return fmt.Errorf("run: configuration not loaded")
	}

	result, err := pipeline.RunDryRun(cmd.Context(), cfg.Config, scpID)
	if err != nil {
		cmd.SilenceUsage = true
		return fmt.Errorf("run dry-run: %w", err)
	}

	jsonOutput, _ := cmd.Flags().GetBool("json-output")
	if jsonOutput {
		return outputDryRunJSON(cmd, result)
	}
	return outputDryRunHuman(cmd, result)
}

func outputDryRunJSON(cmd *cobra.Command, result *pipeline.DryRunResult) error {
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	if err := enc.Encode(result); err != nil {
		return fmt.Errorf("run dry-run: encoding JSON: %w", err)
	}
	if !result.Success {
		cmd.SilenceUsage = true
		return fmt.Errorf("dry-run failed: %d configuration error(s)", len(result.Errors))
	}
	return nil
}

func outputDryRunHuman(cmd *cobra.Command, result *pipeline.DryRunResult) error {
	w := cmd.OutOrStdout()

	fmt.Fprintf(w, "\n=== Dry-Run Results for %s ===\n\n", result.SCPID)

	fmt.Fprintln(w, "Configuration:")
	fmt.Fprintf(w, "  SCP Data Path:      %s\n", result.Config.SCPDataPath)
	fmt.Fprintf(w, "  Workspace Path:     %s\n", result.Config.WorkspacePath)
	fmt.Fprintf(w, "  LLM Provider:       %s (key: %s)\n", result.Config.LLMProvider, displayKey(result.Config.LLMAPIKey))
	fmt.Fprintf(w, "  ImageGen Provider:  %s (key: %s)\n", result.Config.ImageGenProvider, displayKey(result.Config.ImageGenAPIKey))
	fmt.Fprintf(w, "  TTS Provider:       %s (key: %s)\n", result.Config.TTSProvider, displayKey(result.Config.TTSAPIKey))
	fmt.Fprintf(w, "  Output Provider:    %s\n", result.Config.OutputProvider)
	fmt.Fprintln(w)

	fmt.Fprintln(w, "Pipeline Stages:")
	for i, stage := range result.Stages {
		icon := "✓"
		if stage.Status == "fail" {
			icon = "✗"
		} else if stage.Status == "skip" {
			icon = "○"
		}
		fmt.Fprintf(w, "  %d. [%s] %s (%dms)\n", i+1, icon, stage.Name, stage.DurationMs)
		if stage.InputSummary != "" {
			fmt.Fprintf(w, "     Input:  %s\n", stage.InputSummary)
		}
		if stage.OutputSummary != "" {
			fmt.Fprintf(w, "     Output: %s\n", stage.OutputSummary)
		}
		if stage.Error != "" {
			fmt.Fprintf(w, "     Error:  %s\n", stage.Error)
		}
	}
	fmt.Fprintln(w)

	if result.Success {
		fmt.Fprintln(w, "Result: ✓ All stages passed. Pipeline configuration is valid.")
		return nil
	}

	fmt.Fprintf(w, "Result: ✗ %d error(s) found:\n", len(result.Errors))
	for _, e := range result.Errors {
		fmt.Fprintf(w, "  - %s\n", e)
	}
	cmd.SilenceUsage = true
	return fmt.Errorf("dry-run failed: %d configuration error(s)", len(result.Errors))
}

func displayKey(masked string) string {
	if masked == "" {
		return "not set"
	}
	return masked
}
