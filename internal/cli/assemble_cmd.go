package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/jay/youtube-pipeline/internal/domain"
	"github.com/jay/youtube-pipeline/internal/plugin/output"
	"github.com/jay/youtube-pipeline/internal/plugin/output/capcut"
	"github.com/jay/youtube-pipeline/internal/service"
	"github.com/jay/youtube-pipeline/internal/workspace"
	"github.com/spf13/cobra"
)

var assembleCmd = &cobra.Command{
	Use:   "assemble <scp-id>",
	Short: "Assemble a CapCut project from generated assets",
	Long:  "Creates a CapCut project (draft_content.json + draft_meta_info.json) from all generated scene assets (images, audio, subtitles).",
	Args:  cobra.ExactArgs(1),
	RunE:  runAssembleCmd,
}

func init() {
	rootCmd.AddCommand(assembleCmd)
}

func runAssembleCmd(cmd *cobra.Command, args []string) error {
	scpID := args[0]
	cmd.SilenceUsage = true

	cfg := GetConfig()
	if cfg == nil {
		return fmt.Errorf("assemble: configuration not loaded")
	}

	// Create the CapCut assembler plugin
	assembler := capcut.New()

	c := cfg.Config

	// Build canvas config from configuration
	canvas := output.DefaultCanvasConfig()
	if c.Output.CanvasWidth > 0 && c.Output.CanvasHeight > 0 {
		canvas.Width = c.Output.CanvasWidth
		canvas.Height = c.Output.CanvasHeight
	}
	if c.Output.FPS > 0 {
		canvas.FPS = float64(c.Output.FPS)
	}

	// Load SCP data for copyright
	scpData, err := workspace.LoadSCPData(c.SCPDataPath, scpID)
	if err != nil {
		return fmt.Errorf("assemble: load SCP data: %w", err)
	}

	// Load scenes from workspace
	projectDir := filepath.Join(c.WorkspacePath, scpID)
	scenes, err := loadScenesFromWorkspace(projectDir)
	if err != nil {
		return fmt.Errorf("assemble: load scenes: %w", err)
	}
	if len(scenes) == 0 {
		return fmt.Errorf("assemble: no scenes found for %s", scpID)
	}

	// Run assembly
	outputDir := filepath.Join(projectDir, "output")
	input := output.AssembleInput{
		Project:   domain.Project{SCPID: scpID, WorkspacePath: c.WorkspacePath},
		Scenes:    scenes,
		OutputDir: outputDir,
		Canvas:    canvas,
	}

	result, err := assembler.Assemble(cmd.Context(), input)
	if err != nil {
		return fmt.Errorf("assemble: %w", err)
	}

	// Validate the assembled output
	if err := assembler.Validate(cmd.Context(), result.OutputPath); err != nil {
		return fmt.Errorf("assemble: validation failed: %w", err)
	}

	// Generate copyright notice
	author := ""
	if scpData.Meta != nil {
		author = scpData.Meta.Author
	}
	asmSvc := service.NewAssemblerService(assembler, nil)
	if err := asmSvc.GenerateCopyrightNotice(projectDir, scpID, author); err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Warning: copyright notice generation failed: %v\n", err)
	}

	// Log special copyright if applicable
	if scpData.Meta != nil {
		if err := service.LogSpecialCopyright(projectDir, scpID, scpData.Meta); err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Warning: special copyright check failed: %v\n", err)
		}
	}

	// Output results
	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "\n=== CapCut Project Assembly Complete ===\n\n")
	fmt.Fprintf(w, "  SCP ID:       %s\n", scpID)
	fmt.Fprintf(w, "  Output:       %s\n", result.OutputPath)
	fmt.Fprintf(w, "  Scenes:       %d\n", result.SceneCount)
	fmt.Fprintf(w, "  Duration:     %.1fs\n", result.TotalDuration)
	fmt.Fprintf(w, "  Images:       %d\n", result.ImageCount)
	fmt.Fprintf(w, "  Audio clips:  %d\n", result.AudioCount)
	fmt.Fprintf(w, "  Subtitles:    %d\n", result.SubtitleCount)
	fmt.Fprintln(w)

	return nil
}

// loadScenesFromWorkspace scans scene directories for manifest.json files
// and reconstructs domain.Scene objects from them.
func loadScenesFromWorkspace(projectDir string) ([]domain.Scene, error) {
	scenesDir := filepath.Join(projectDir, "scenes")

	entries, err := os.ReadDir(scenesDir)
	if err != nil {
		return nil, fmt.Errorf("read scenes directory: %w", err)
	}

	var scenes []domain.Scene
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		manifestPath := filepath.Join(scenesDir, entry.Name(), "manifest.json")
		data, err := workspace.ReadFile(manifestPath)
		if err != nil {
			continue // skip scenes without manifest
		}

		var manifest sceneManifest
		if err := json.Unmarshal(data, &manifest); err != nil {
			continue
		}

		scenes = append(scenes, domain.Scene{
			SceneNum:      manifest.SceneNum,
			Narration:     manifest.Narration,
			ImagePath:     manifest.ImagePath,
			AudioPath:     manifest.AudioPath,
			AudioDuration: manifest.AudioDuration,
			SubtitlePath:  manifest.SubtitlePath,
			WordTimings:   manifest.WordTimings,
		})
	}

	// Sort by scene number
	sort.Slice(scenes, func(i, j int) bool {
		return scenes[i].SceneNum < scenes[j].SceneNum
	})

	return scenes, nil
}

// sceneManifest is the JSON structure stored per scene in the workspace.
type sceneManifest struct {
	SceneNum      int                 `json:"scene_num"`
	Narration     string              `json:"narration"`
	ImagePath     string              `json:"image_path"`
	AudioPath     string              `json:"audio_path"`
	AudioDuration float64             `json:"audio_duration"`
	SubtitlePath  string              `json:"subtitle_path"`
	WordTimings   []domain.WordTiming `json:"word_timings"`
}
