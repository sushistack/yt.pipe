package cli

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/sushistack/yt.pipe/internal/service"
)

var reviewCmd = &cobra.Command{
	Use:   "review",
	Short: "Scene review and batch approval commands",
}

var reviewBatchCmd = &cobra.Command{
	Use:   "batch <project-id>",
	Short: "Batch preview and approve scenes with selective flagging",
	Long:  "Display all scenes in a table, then approve non-flagged scenes in one pass.",
	Args:  cobra.ExactArgs(1),
	RunE:  runReviewBatchCmd,
}

func init() {
	reviewBatchCmd.Flags().String("asset", "image", "asset type (image or tts)")
	reviewCmd.AddCommand(reviewBatchCmd)
	rootCmd.AddCommand(reviewCmd)
}

func runReviewBatchCmd(cmd *cobra.Command, args []string) error {
	projectID := args[0]
	assetType, _ := cmd.Flags().GetString("asset")

	db, err := openDB(cmd)
	if err != nil {
		return err
	}
	defer db.Close()

	logger := slog.Default()
	approvalSvc := service.NewApprovalService(db, logger)

	// Get batch preview
	items, err := approvalSvc.GetBatchPreview(cmd.Context(), projectID, assetType)
	if err != nil {
		cmd.SilenceUsage = true
		return fmt.Errorf("review batch: %w", err)
	}

	if len(items) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No scenes found for this project.")
		return nil
	}

	// Check for JSON output
	jsonOutput, _ := cmd.Flags().GetBool("json-output")

	// Display batch preview table
	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "\n=== Batch Preview: %s (%s) ===\n\n", projectID, assetType)

	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "Scene\tMood\tStatus\tAI Score\tImage Path")
	fmt.Fprintln(tw, "-----\t----\t------\t--------\t----------")
	for _, item := range items {
		scoreStr := "-"
		if item.ValidationScore != nil {
			scoreStr = strconv.Itoa(*item.ValidationScore)
		}
		fmt.Fprintf(tw, "%d\t%s\t%s\t%s\t%s\n",
			item.SceneNum, item.Mood, item.Status, scoreStr, item.ImagePath)
	}
	tw.Flush()
	fmt.Fprintln(w)

	// Prompt for flagged scenes
	input, err := promptString(os.Stdin, w,
		"Enter flagged scene numbers (comma-separated) or 'none' for full approval", "none")
	if err != nil {
		return fmt.Errorf("review batch: prompt: %w", err)
	}

	var flaggedScenes []int
	if input != "" && input != "none" {
		parts := strings.Split(input, ",")
		for _, p := range parts {
			num, parseErr := strconv.Atoi(strings.TrimSpace(p))
			if parseErr != nil {
				cmd.SilenceUsage = true
				return fmt.Errorf("review batch: invalid scene number %q", strings.TrimSpace(p))
			}
			flaggedScenes = append(flaggedScenes, num)
		}
	}

	// Execute batch approval
	result, err := approvalSvc.BatchApprove(cmd.Context(), projectID, assetType, flaggedScenes)
	if err != nil {
		cmd.SilenceUsage = true
		return fmt.Errorf("review batch: %w", err)
	}

	if jsonOutput {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}

	fmt.Fprintf(w, "\nApproved: %d, Flagged for review: %d\n", result.ApprovedCount, result.FlaggedCount)
	return nil
}
