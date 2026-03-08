package cli

import (
	"fmt"
	"log/slog"

	"github.com/jay/youtube-pipeline/internal/glossary"
	"github.com/jay/youtube-pipeline/internal/pipeline"
	"github.com/jay/youtube-pipeline/internal/plugin/imagegen"
	"github.com/jay/youtube-pipeline/internal/plugin/output"
	"github.com/jay/youtube-pipeline/internal/plugin/output/capcut"
	"github.com/jay/youtube-pipeline/internal/service"
	"github.com/jay/youtube-pipeline/internal/store"
	"github.com/spf13/cobra"
)

// scenarioCmd groups scenario-related subcommands.
var scenarioCmd = &cobra.Command{
	Use:   "scenario",
	Short: "Manage scenario generation and approval",
}

var scenarioGenerateCmd = &cobra.Command{
	Use:   "generate <scp-id>",
	Short: "Generate a scenario from SCP data",
	Args:  cobra.ExactArgs(1),
	RunE:  runStageCmd(service.StageScenarioGenerate),
}

var scenarioApproveCmd = &cobra.Command{
	Use:   "approve <project-id>",
	Short: "Approve a generated scenario",
	Args:  cobra.ExactArgs(1),
	RunE:  runScenarioApproveCmd,
}

// imageCmd groups image-related subcommands.
var imageCmd = &cobra.Command{
	Use:   "image",
	Short: "Manage image generation",
}

var imageGenerateCmd = &cobra.Command{
	Use:   "generate <scp-id>",
	Short: "Generate images for all scenes",
	Args:  cobra.ExactArgs(1),
	RunE:  runStageCmd(service.StageImageGenerate),
}

// ttsCmd groups TTS-related subcommands.
var ttsCmd = &cobra.Command{
	Use:   "tts",
	Short: "Manage TTS narration synthesis",
}

var ttsGenerateCmd = &cobra.Command{
	Use:   "generate <scp-id>",
	Short: "Synthesize TTS narration for all scenes",
	Args:  cobra.ExactArgs(1),
	RunE:  runStageCmd(service.StageTTSSynthesize),
}

// subtitleCmd groups subtitle-related subcommands.
var subtitleCmd = &cobra.Command{
	Use:   "subtitle",
	Short: "Manage subtitle generation",
}

var subtitleGenerateCmd = &cobra.Command{
	Use:   "generate <scp-id>",
	Short: "Generate subtitles for all scenes",
	Args:  cobra.ExactArgs(1),
	RunE:  runStageCmd(service.StageSubtitleGenerate),
}

func init() {
	scenarioCmd.AddCommand(scenarioGenerateCmd)
	scenarioCmd.AddCommand(scenarioApproveCmd)
	rootCmd.AddCommand(scenarioCmd)

	imageCmd.AddCommand(imageGenerateCmd)
	rootCmd.AddCommand(imageCmd)

	ttsCmd.AddCommand(ttsGenerateCmd)
	rootCmd.AddCommand(ttsCmd)

	subtitleCmd.AddCommand(subtitleGenerateCmd)
	rootCmd.AddCommand(subtitleCmd)
}

// runStageCmd returns a cobra RunE function that runs a specific pipeline stage.
func runStageCmd(stage service.PipelineStage) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		scpID := args[0]
		cmd.SilenceUsage = true

		runner, cleanup, err := buildRunner(cmd)
		if err != nil {
			return err
		}
		defer cleanup()

		if err := runner.RunStage(cmd.Context(), scpID, stage); err != nil {
			return err
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Stage %s completed for %s\n", stage, scpID)
		return nil
	}
}

func runScenarioApproveCmd(cmd *cobra.Command, args []string) error {
	projectID := args[0]
	cmd.SilenceUsage = true

	cfg := GetConfig()
	if cfg == nil {
		return fmt.Errorf("scenario approve: configuration not loaded")
	}
	c := cfg.Config

	dbPath := c.DBPath
	if dbPath == "" {
		dbPath = c.WorkspacePath + "/yt-pipe.db"
	}
	db, err := store.New(dbPath)
	if err != nil {
		return fmt.Errorf("scenario approve: open database: %w", err)
	}
	defer db.Close()

	projectSvc := service.NewProjectService(db)
	scenarioSvc := service.NewScenarioService(db, nil, projectSvc)

	project, err := scenarioSvc.ApproveScenario(cmd.Context(), projectID)
	if err != nil {
		return fmt.Errorf("scenario approve: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Scenario approved for project %s (SCP: %s)\n", project.ID, project.SCPID)
	fmt.Fprintf(cmd.OutOrStdout(), "Resume pipeline with: yt-pipe run %s --resume %s\n", project.SCPID, project.ID)
	return nil
}

// buildRunner creates a pipeline.Runner from the current config.
func buildRunner(cmd *cobra.Command) (*pipeline.Runner, func(), error) {
	cfg := GetConfig()
	if cfg == nil {
		return nil, nil, fmt.Errorf("configuration not loaded")
	}
	c := cfg.Config

	dbPath := c.DBPath
	if dbPath == "" {
		dbPath = c.WorkspacePath + "/yt-pipe.db"
	}
	db, err := store.New(dbPath)
	if err != nil {
		return nil, nil, fmt.Errorf("open database: %w", err)
	}

	llmPlugin, imgPlugin, ttsPlugin, err := createPlugins(cfg)
	if err != nil {
		db.Close()
		return nil, nil, err
	}

	assembler := capcut.New()

	var g *glossary.Glossary
	if c.GlossaryPath != "" {
		g = glossary.LoadFromFile(c.GlossaryPath)
	}

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

	runner := pipeline.NewRunner(db, llmPlugin, imgPlugin, ttsPlugin, assembler, g, slog.Default(), pipeline.RunnerConfig{
		SCPDataPath:   c.SCPDataPath,
		WorkspacePath: c.WorkspacePath,
		Voice:         c.TTS.Voice,
		ImageOpts:     imagegen.GenerateOptions{},
		Canvas:        canvas,
		TemplatePath:  c.Output.TemplatePath,
		MetaPath:      c.Output.MetaPath,
	})

	cleanup := func() { db.Close() }
	return runner, cleanup, nil
}
