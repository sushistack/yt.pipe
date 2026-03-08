package cli

import (
	"encoding/json"
	"fmt"

	"github.com/sushistack/yt.pipe/internal/domain"
	"github.com/sushistack/yt.pipe/internal/store"
	"github.com/spf13/cobra"
)

var feedbackCmd = &cobra.Command{
	Use:   "feedback <scp-id>",
	Short: "Submit quality feedback for a scene asset",
	Long:  "Record quality feedback (good/bad/neutral) for a specific scene's asset (image, audio, subtitle, scenario).",
	Args:  cobra.ExactArgs(1),
	RunE:  runFeedbackCmd,
}

func init() {
	feedbackCmd.Flags().Int("scene", 0, "scene number (required)")
	feedbackCmd.Flags().String("type", "", "asset type: image, audio, subtitle, scenario (required)")
	feedbackCmd.Flags().String("rating", "", "rating: good, bad, neutral (required)")
	feedbackCmd.Flags().String("comment", "", "optional comment")
	_ = feedbackCmd.MarkFlagRequired("scene")
	_ = feedbackCmd.MarkFlagRequired("type")
	_ = feedbackCmd.MarkFlagRequired("rating")
	rootCmd.AddCommand(feedbackCmd)
}

func runFeedbackCmd(cmd *cobra.Command, args []string) error {
	scpID := args[0]
	cmd.SilenceUsage = true

	scene, _ := cmd.Flags().GetInt("scene")
	assetType, _ := cmd.Flags().GetString("type")
	rating, _ := cmd.Flags().GetString("rating")
	comment, _ := cmd.Flags().GetString("comment")

	// Validate
	if scene < 1 {
		return fmt.Errorf("feedback: --scene must be >= 1")
	}
	if !domain.ValidAssetTypes[assetType] {
		return fmt.Errorf("feedback: --type must be one of: image, audio, subtitle, scenario")
	}
	if !domain.ValidRatings[rating] {
		return fmt.Errorf("feedback: --rating must be one of: good, bad, neutral")
	}

	cfg := GetConfig()
	if cfg == nil {
		return fmt.Errorf("feedback: configuration not loaded")
	}
	c := cfg.Config

	dbPath := c.DBPath
	if dbPath == "" {
		dbPath = c.WorkspacePath + "/yt-pipe.db"
	}
	db, err := store.New(dbPath)
	if err != nil {
		return fmt.Errorf("feedback: open database: %w", err)
	}
	defer db.Close()

	project, err := findProjectBySCPID(db, scpID)
	if err != nil {
		return fmt.Errorf("feedback: %w", err)
	}

	f := &domain.Feedback{
		ProjectID: project.ID,
		SceneNum:  scene,
		AssetType: assetType,
		Rating:    rating,
		Comment:   comment,
	}

	if err := db.CreateFeedback(f); err != nil {
		return fmt.Errorf("feedback: %w", err)
	}

	jsonOutput, _ := cmd.Flags().GetBool("json-output")
	if jsonOutput {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(map[string]any{
			"status":     "recorded",
			"project_id": project.ID,
			"scene":      scene,
			"type":       assetType,
			"rating":     rating,
		})
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Feedback recorded: scene %d %s = %s (project %s)\n",
		scene, assetType, rating, project.SCPID)
	return nil
}
