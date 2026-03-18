package cli

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/sushistack/yt.pipe/internal/glossary"
	"github.com/sushistack/yt.pipe/internal/pipeline"
	"github.com/sushistack/yt.pipe/internal/plugin/imagegen"
	"github.com/sushistack/yt.pipe/internal/plugin/output"
	"github.com/sushistack/yt.pipe/internal/plugin/output/capcut"
	"github.com/sushistack/yt.pipe/internal/service"
	"github.com/sushistack/yt.pipe/internal/store"
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

var imageRegenerateCmd = &cobra.Command{
	Use:   "regenerate <scp-id>",
	Short: "Regenerate images for specific scenes",
	Args:  cobra.ExactArgs(1),
	RunE:  runImageRegenerateCmd,
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
	RunE:  runTTSGenerateCmd,
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

	imageGenerateCmd.Flags().Bool("parallel", false, "enable parallel scene generation (disables shot continuity)")
	imageGenerateCmd.Flags().Bool("force", false, "regenerate all images even if already generated")

	imageRegenerateCmd.Flags().String("scenes", "", "comma-separated scene numbers to regenerate (e.g., 3,5,7)")
	imageRegenerateCmd.Flags().Int("scene", 0, "single scene number to regenerate")
	imageRegenerateCmd.Flags().String("edit-prompt", "", "instruction to modify the prompt before regeneration")

	imageCmd.AddCommand(imageGenerateCmd)
	imageCmd.AddCommand(imageRegenerateCmd)
	rootCmd.AddCommand(imageCmd)

	ttsGenerateCmd.Flags().Bool("force", false, "regenerate all scenes even if already generated")
	ttsGenerateCmd.Flags().String("scenes", "", "comma-separated scene numbers to regenerate (e.g., 3,5)")
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

	characterSvc := service.NewCharacterService(db)
	characterSvc.SetLLM(llmPlugin)
	characterSvc.SetImageGen(imgPlugin)

	runner := pipeline.NewRunner(db, llmPlugin, imgPlugin, ttsPlugin, assembler, g, slog.Default(), pipeline.RunnerConfig{
		SCPDataPath:   c.SCPDataPath,
		WorkspacePath: c.WorkspacePath,
		Voice:         c.TTS.Voice,
		ImageOpts: imagegen.GenerateOptions{
			Width:  c.ImageGen.Width,
			Height: c.ImageGen.Height,
		},
		Canvas:               canvas,
		TemplatePath:         c.Output.TemplatePath,
		MetaPath:             c.Output.MetaPath,
		TemplatesPath:        c.TemplatesPath,
		DefaultSceneDuration:  c.Output.DefaultSceneDuration,
		CharacterSvc:          characterSvc,
		AutoApprovalEnabled:   c.AutoApproval.Enabled && c.ImageValidation.Enabled,
		AutoApprovalThreshold: c.AutoApproval.Threshold,
	})

	cleanup := func() { db.Close() }
	return runner, cleanup, nil
}

// runImageRegenerateCmd handles `yt-pipe image regenerate <scp-id> --scenes 3,5,7`.
func runImageRegenerateCmd(cmd *cobra.Command, args []string) error {
	scpID := args[0]
	cmd.SilenceUsage = true

	scenes, _ := cmd.Flags().GetString("scenes")
	singleScene, _ := cmd.Flags().GetInt("scene")

	var sceneNums []int
	if singleScene > 0 {
		sceneNums = []int{singleScene}
	} else if scenes != "" {
		sceneNums = parseSceneNums(scenes)
	}

	if len(sceneNums) == 0 {
		return fmt.Errorf("image regenerate: specify scenes with --scenes 3,5,7 or --scene 3")
	}

	runner, cleanup, err := buildRunner(cmd)
	if err != nil {
		return err
	}
	defer cleanup()

	if err := runner.RunImageRegenerate(cmd.Context(), scpID, sceneNums); err != nil {
		return err
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Regenerated images for scenes %v of %s\n", sceneNums, scpID)
	return nil
}

// parseSceneNums parses a comma-separated string of scene numbers.
func parseSceneNums(s string) []int {
	var nums []int
	for _, part := range strings.Split(s, ",") {
		p := strings.TrimSpace(part)
		if p == "" {
			continue
		}
		var n int
		if _, err := fmt.Sscanf(p, "%d", &n); err == nil && n > 0 {
			nums = append(nums, n)
		}
	}
	return nums
}

// runTTSGenerateCmd handles `yt-pipe tts generate <scp-id>` with --force and --scenes flags.
func runTTSGenerateCmd(cmd *cobra.Command, args []string) error {
	scpID := args[0]
	cmd.SilenceUsage = true

	force, _ := cmd.Flags().GetBool("force")
	scenesStr, _ := cmd.Flags().GetString("scenes")

	var sceneNums []int
	if scenesStr != "" {
		sceneNums = parseSceneNums(scenesStr)
	}

	runner, cleanup, err := buildRunner(cmd)
	if err != nil {
		return err
	}
	defer cleanup()

	if err := runner.RunTTSGenerate(cmd.Context(), scpID, sceneNums, force); err != nil {
		return err
	}

	fmt.Fprintf(cmd.OutOrStdout(), "TTS generation completed for %s\n", scpID)
	return nil
}
