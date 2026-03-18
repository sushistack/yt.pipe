package cli

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/sushistack/yt.pipe/internal/domain"
	"github.com/sushistack/yt.pipe/internal/service"
	"github.com/sushistack/yt.pipe/internal/workspace"
)

var chaptersCmd = &cobra.Command{
	Use:   "chapters <scp-id>",
	Short: "Generate YouTube chapter timestamps from scene timings",
	Args:  cobra.ExactArgs(1),
	RunE:  runChapters,
}

func init() {
	rootCmd.AddCommand(chaptersCmd)
}

func runChapters(cmd *cobra.Command, args []string) error {
	scpID := args[0]
	cmd.SilenceUsage = true

	cfg := GetConfig()
	if cfg == nil {
		return fmt.Errorf("chapters: configuration not loaded")
	}

	projectDir := filepath.Join(cfg.Config.WorkspacePath, scpID)

	// Check timeline.json exists
	timelinePath := filepath.Join(projectDir, "timeline.json")
	timelineData, err := workspace.ReadFile(timelinePath)
	if err != nil {
		return &domain.DependencyError{
			Action:  "generate chapters",
			Missing: []string{"timeline.json — run pipeline first to resolve timings"},
		}
	}

	var timeline service.Timeline
	if err := json.Unmarshal(timelineData, &timeline); err != nil {
		return fmt.Errorf("chapters: parse timeline: %w", err)
	}

	// Load scenario for scene metadata (Mood, VisualDescription)
	scenarioPath := filepath.Join(projectDir, "scenario.json")
	scenario, err := service.LoadScenarioFromFile(scenarioPath)
	if err != nil {
		return fmt.Errorf("chapters: load scenario: %w", err)
	}

	// Generate chapters
	resolver := service.NewTimingResolver(slog.Default())
	chapters := resolver.GenerateChapters(timeline, scenario.Scenes)
	content := service.FormatChapters(chapters)

	if err := resolver.SaveChaptersFile(content, projectDir); err != nil {
		return err
	}

	outputPath := filepath.Join(projectDir, "output", "chapters.txt")
	fmt.Fprintf(cmd.OutOrStdout(), "Chapters generated: %s\n", outputPath)
	fmt.Fprintf(cmd.OutOrStdout(), "\n%s", content)

	return nil
}
