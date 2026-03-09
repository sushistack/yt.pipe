package cli

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/sushistack/yt.pipe/internal/domain"
	"github.com/sushistack/yt.pipe/internal/service"
	"github.com/sushistack/yt.pipe/internal/store"
	"github.com/spf13/cobra"
)

var scenesCmd = &cobra.Command{
	Use:   "scenes <project-id>",
	Short: "Show scene asset mapping dashboard",
	Long:  "Display per-scene text, image, and TTS status mapping for a project.",
	Args:  cobra.ExactArgs(1),
	RunE:  runScenesCmd,
}

var scenesApproveCmd = &cobra.Command{
	Use:   "approve <project-id>",
	Short: "Approve a scene's asset",
	Args:  cobra.ExactArgs(1),
	RunE:  runScenesApproveCmd,
}

var scenesRejectCmd = &cobra.Command{
	Use:   "reject <project-id>",
	Short: "Reject a scene's asset",
	Args:  cobra.ExactArgs(1),
	RunE:  runScenesRejectCmd,
}

func init() {
	scenesCmd.Flags().Int("scene", 0, "show detail for a specific scene number")
	scenesApproveCmd.Flags().String("type", "", "asset type to approve (image or tts)")
	scenesApproveCmd.Flags().Int("scene", 0, "scene number to approve")
	scenesApproveCmd.Flags().Bool("all", false, "approve all scenes for the asset type")
	scenesRejectCmd.Flags().String("type", "", "asset type to reject (image or tts)")
	scenesRejectCmd.Flags().Int("scene", 0, "scene number to reject")

	scenesCmd.AddCommand(scenesApproveCmd)
	scenesCmd.AddCommand(scenesRejectCmd)
	rootCmd.AddCommand(scenesCmd)
}

func openDB(cmd *cobra.Command) (*store.Store, error) {
	cfg := GetConfig()
	if cfg == nil {
		cmd.SilenceUsage = true
		return nil, fmt.Errorf("configuration not loaded")
	}
	dbPath := cfg.Config.DBPath
	if dbPath == "" {
		dbPath = cfg.Config.WorkspacePath + "/yt-pipe.db"
	}
	return store.New(dbPath)
}

func runScenesCmd(cmd *cobra.Command, args []string) error {
	projectID := args[0]

	db, err := openDB(cmd)
	if err != nil {
		return err
	}
	defer db.Close()

	logger := slog.Default()
	dashSvc := service.NewSceneDashboardService(db, logger)

	sceneNum, _ := cmd.Flags().GetInt("scene")

	dashboard, err := dashSvc.GetDashboard(projectID)
	if err != nil {
		cmd.SilenceUsage = true
		return fmt.Errorf("scenes: %w", err)
	}

	jsonOutput, _ := cmd.Flags().GetBool("json-output")
	if jsonOutput {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(dashboard)
	}

	w := cmd.OutOrStdout()

	if sceneNum > 0 {
		// Detail view for a single scene
		for _, s := range dashboard.Scenes {
			if s.SceneNum == sceneNum {
				fmt.Fprintf(w, "\n=== Scene %d Detail ===\n\n", sceneNum)
				fmt.Fprintf(w, "  Image Status:    %s (attempts: %d)\n", statusLabel(s.ImageStatus), s.ImageAttempts)
				fmt.Fprintf(w, "  Image Path:      %s\n", s.ImagePath)
				fmt.Fprintf(w, "  TTS Status:      %s (attempts: %d)\n", statusLabel(s.TTSStatus), s.TTSAttempts)
				fmt.Fprintf(w, "  TTS Path:        %s\n", s.TTSPath)
				if s.MoodPreset != "" {
					fmt.Fprintf(w, "  Mood Preset:     %s\n", s.MoodPreset)
				}
				if s.BGMName != "" {
					fmt.Fprintf(w, "  BGM:             %s\n", s.BGMName)
				}
				fmt.Fprintln(w)
				return nil
			}
		}
		return fmt.Errorf("scene %d not found", sceneNum)
	}

	// Table view
	fmt.Fprintf(w, "\n=== Scene Dashboard for %s (status: %s) ===\n\n", projectID, dashboard.ProjectStatus)
	fmt.Fprintf(w, "  %-6s %-12s %-12s\n", "Scene", "Image", "TTS")
	fmt.Fprintf(w, "  %-6s %-12s %-12s\n", "-----", "-----", "---")

	for _, s := range dashboard.Scenes {
		fmt.Fprintf(w, "  %-6d %-12s %-12s\n",
			s.SceneNum, statusLabel(s.ImageStatus), statusLabel(s.TTSStatus))
	}

	// Summary
	if dashboard.ImageSummary != nil {
		fmt.Fprintf(w, "\n  Images: %d/%d approved", dashboard.ImageSummary.Approved, dashboard.ImageSummary.Total)
		if dashboard.TTSSummary != nil {
			fmt.Fprintf(w, ", TTS: %d/%d approved", dashboard.TTSSummary.Approved, dashboard.TTSSummary.Total)
		}
		fmt.Fprintln(w)
	}
	fmt.Fprintln(w)

	return nil
}

func runScenesApproveCmd(cmd *cobra.Command, args []string) error {
	projectID := args[0]
	assetType, _ := cmd.Flags().GetString("type")
	sceneNum, _ := cmd.Flags().GetInt("scene")
	approveAll, _ := cmd.Flags().GetBool("all")

	if assetType == "" {
		return fmt.Errorf("--type is required (image or tts)")
	}
	if !approveAll && sceneNum == 0 {
		return fmt.Errorf("--scene or --all is required")
	}

	db, err := openDB(cmd)
	if err != nil {
		return err
	}
	defer db.Close()

	logger := slog.Default()
	approvalSvc := service.NewApprovalService(db, logger)

	if approveAll {
		if err := approvalSvc.AutoApproveAll(projectID, assetType); err != nil {
			cmd.SilenceUsage = true
			return fmt.Errorf("scenes approve all: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "All %s scenes approved for project %s\n", assetType, projectID)
		return nil
	}

	if err := approvalSvc.ApproveScene(projectID, sceneNum, assetType); err != nil {
		cmd.SilenceUsage = true
		return fmt.Errorf("scenes approve: %w", err)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Scene %d %s approved for project %s\n", sceneNum, assetType, projectID)
	return nil
}

func runScenesRejectCmd(cmd *cobra.Command, args []string) error {
	projectID := args[0]
	assetType, _ := cmd.Flags().GetString("type")
	sceneNum, _ := cmd.Flags().GetInt("scene")

	if assetType == "" {
		return fmt.Errorf("--type is required (image or tts)")
	}
	if sceneNum == 0 {
		return fmt.Errorf("--scene is required")
	}

	db, err := openDB(cmd)
	if err != nil {
		return err
	}
	defer db.Close()

	logger := slog.Default()
	approvalSvc := service.NewApprovalService(db, logger)

	if err := approvalSvc.RejectScene(projectID, sceneNum, assetType); err != nil {
		cmd.SilenceUsage = true
		return fmt.Errorf("scenes reject: %w", err)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Scene %d %s rejected for project %s\n", sceneNum, assetType, projectID)
	return nil
}

func statusLabel(status string) string {
	switch status {
	case domain.ApprovalPending:
		return "[PENDING]"
	case domain.ApprovalGenerated:
		return "[GENERATED]"
	case domain.ApprovalApproved:
		return "[APPROVED]"
	case domain.ApprovalRejected:
		return "[REJECTED]"
	default:
		return strings.ToUpper("[" + status + "]")
	}
}
