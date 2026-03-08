package cli

import (
	"encoding/json"
	"fmt"
	"text/tabwriter"

	"github.com/jay/youtube-pipeline/internal/service"
	"github.com/jay/youtube-pipeline/internal/store"
	"github.com/spf13/cobra"
)

var cleanCmd = &cobra.Command{
	Use:   "clean <scp-id>",
	Short: "Clean up project workspace files",
	Long: `Remove intermediate artifacts from a project workspace to free disk space.

By default, preserves final outputs (scenario, images, audio, subtitles, output).
Use --all to remove everything and archive the project.
Use --dry-run to preview what would be deleted.
Use --status to show disk usage without deleting anything.`,
	Args: cobra.ExactArgs(1),
	RunE: runCleanCmd,
}

func init() {
	cleanCmd.Flags().Bool("all", false, "remove all files and archive the project")
	cleanCmd.Flags().Bool("dry-run", false, "show what would be deleted without actually deleting")
	cleanCmd.Flags().Bool("status", false, "show disk usage for the project workspace")
	rootCmd.AddCommand(cleanCmd)
}

func runCleanCmd(cmd *cobra.Command, args []string) error {
	scpID := args[0]
	cmd.SilenceUsage = true

	cfg := GetConfig()
	if cfg == nil {
		return fmt.Errorf("clean: configuration not loaded")
	}
	c := cfg.Config

	dbPath := c.DBPath
	if dbPath == "" {
		dbPath = c.WorkspacePath + "/yt-pipe.db"
	}
	db, err := store.New(dbPath)
	if err != nil {
		return fmt.Errorf("clean: open database: %w", err)
	}
	defer db.Close()

	project, err := findProjectBySCPID(db, scpID)
	if err != nil {
		return fmt.Errorf("clean: %w", err)
	}

	showStatus, _ := cmd.Flags().GetBool("status")
	if showStatus {
		return showDiskUsage(cmd, project.WorkspacePath, project.ID)
	}

	allFiles, _ := cmd.Flags().GetBool("all")
	dryRun, _ := cmd.Flags().GetBool("dry-run")

	result, err := service.CleanProject(project.WorkspacePath, c.WorkspacePath, allFiles, dryRun)
	if err != nil {
		return fmt.Errorf("clean: %w", err)
	}
	result.ProjectID = project.ID

	// If --all and not dry-run, update project status to archived
	if allFiles && !dryRun {
		// Set status directly via store (archived is a terminal state)
		project.Status = "archived"
		if err := db.UpdateProject(project); err != nil {
			return fmt.Errorf("clean: archive project: %w", err)
		}
	}

	jsonOutput, _ := cmd.Flags().GetBool("json-output")
	if jsonOutput {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}

	return outputCleanHuman(cmd, result, dryRun)
}

func outputCleanHuman(cmd *cobra.Command, result *service.CleanupResult, dryRun bool) error {
	w := cmd.OutOrStdout()

	if dryRun {
		fmt.Fprintf(w, "\n=== Dry Run: Files to be removed ===\n\n")
	} else {
		fmt.Fprintf(w, "\n=== Cleanup Complete ===\n\n")
	}

	if len(result.FilesRemoved) == 0 {
		fmt.Fprintln(w, "  No files to clean.")
		return nil
	}

	for _, f := range result.FilesRemoved {
		if dryRun {
			fmt.Fprintf(w, "  [would remove] %s\n", f)
		} else {
			fmt.Fprintf(w, "  [removed] %s\n", f)
		}
	}
	fmt.Fprintln(w)
	fmt.Fprintf(w, "  Files: %d\n", len(result.FilesRemoved))
	fmt.Fprintf(w, "  Space: %s\n", formatBytes(result.BytesFreed))
	fmt.Fprintln(w)

	return nil
}

func showDiskUsage(cmd *cobra.Command, workspacePath, projectID string) error {
	usage, err := service.GetDiskUsage(workspacePath)
	if err != nil {
		return fmt.Errorf("clean: disk usage: %w", err)
	}
	usage.ProjectID = projectID

	jsonOutput, _ := cmd.Flags().GetBool("json-output")
	if jsonOutput {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(usage)
	}

	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "\n=== Disk Usage: %s ===\n\n", projectID)
	fmt.Fprintf(w, "  Path:        %s\n", usage.WorkspacePath)
	fmt.Fprintf(w, "  Total Size:  %s\n", formatBytes(usage.TotalBytes))
	fmt.Fprintf(w, "  Total Files: %d\n", usage.TotalFiles)
	fmt.Fprintln(w)

	if len(usage.Categories) > 0 {
		tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
		fmt.Fprintln(tw, "  Category\tFiles\tSize")
		fmt.Fprintln(tw, "  --------\t-----\t----")
		for _, c := range usage.Categories {
			fmt.Fprintf(tw, "  %s\t%d\t%s\n", c.Category, c.Files, formatBytes(c.Bytes))
		}
		tw.Flush()
		fmt.Fprintln(w)
	}

	return nil
}

func formatBytes(b int64) string {
	switch {
	case b < 1024:
		return fmt.Sprintf("%d B", b)
	case b < 1024*1024:
		return fmt.Sprintf("%.1f KB", float64(b)/1024)
	case b < 1024*1024*1024:
		return fmt.Sprintf("%.1f MB", float64(b)/(1024*1024))
	default:
		return fmt.Sprintf("%.1f GB", float64(b)/(1024*1024*1024))
	}
}
