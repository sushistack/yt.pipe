package cli

import (
	"encoding/json"
	"fmt"
	"text/tabwriter"

	"github.com/jay/youtube-pipeline/internal/domain"
	"github.com/jay/youtube-pipeline/internal/service"
	"github.com/jay/youtube-pipeline/internal/store"
	"github.com/spf13/cobra"
)

var metricsCmd = &cobra.Command{
	Use:   "metrics [scp-id]",
	Short: "Show pipeline metrics and feedback summary",
	Long:  "Display aggregated pipeline metrics including success rates, costs, and feedback. Optionally filter by SCP ID.",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runMetricsCmd,
}

func init() {
	rootCmd.AddCommand(metricsCmd)
}

func runMetricsCmd(cmd *cobra.Command, args []string) error {
	cmd.SilenceUsage = true

	cfg := GetConfig()
	if cfg == nil {
		return fmt.Errorf("metrics: configuration not loaded")
	}
	c := cfg.Config

	dbPath := c.DBPath
	if dbPath == "" {
		dbPath = c.WorkspacePath + "/yt-pipe.db"
	}
	db, err := store.New(dbPath)
	if err != nil {
		return fmt.Errorf("metrics: open database: %w", err)
	}
	defer db.Close()

	var projects []*domain.Project
	var logs []*domain.ExecutionLog
	var feedbacks []*domain.Feedback

	if len(args) > 0 {
		// Per-project metrics
		scpID := args[0]
		project, err := findProjectBySCPID(db, scpID)
		if err != nil {
			return fmt.Errorf("metrics: %w", err)
		}
		projects = []*domain.Project{project}
		logs, err = db.ListExecutionLogsByProject(project.ID)
		if err != nil {
			return fmt.Errorf("metrics: %w", err)
		}
		feedbacks, err = db.ListFeedbackByProject(project.ID)
		if err != nil {
			return fmt.Errorf("metrics: %w", err)
		}
	} else {
		// Global metrics
		projects, err = db.ListProjects()
		if err != nil {
			return fmt.Errorf("metrics: list projects: %w", err)
		}
		logs, err = db.ListAllExecutionLogs()
		if err != nil {
			return fmt.Errorf("metrics: list execution logs: %w", err)
		}
		feedbacks, err = db.ListAllFeedback()
		if err != nil {
			return fmt.Errorf("metrics: %w", err)
		}
	}

	metrics := service.BuildPipelineMetrics(projects, logs, feedbacks)

	jsonOutput, _ := cmd.Flags().GetBool("json-output")
	if jsonOutput {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(metrics)
	}

	return outputMetricsHuman(cmd, metrics)
}

func outputMetricsHuman(cmd *cobra.Command, m *service.PipelineMetrics) error {
	w := cmd.OutOrStdout()

	fmt.Fprintf(w, "\n=== Pipeline Metrics ===\n\n")
	fmt.Fprintf(w, "  Projects:      %d\n", m.TotalProjects)
	fmt.Fprintf(w, "  Executions:    %d\n", m.TotalExecutions)
	fmt.Fprintf(w, "  API Calls:     %d\n", m.TotalAPICalls)
	fmt.Fprintf(w, "  Total Cost:    $%.4f\n", m.TotalCost)
	fmt.Fprintf(w, "  Success Rate:  %.1f%%\n", m.SuccessRate)
	fmt.Fprintln(w)

	if m.FeedbackSummary != nil {
		fs := m.FeedbackSummary
		fmt.Fprintf(w, "Feedback (%d total):\n", fs.Total)
		fmt.Fprintf(w, "  Good: %d  Bad: %d  Neutral: %d\n",
			fs.ByRating["good"], fs.ByRating["bad"], fs.ByRating["neutral"])
		fmt.Fprintln(w)

		if len(fs.ByType) > 0 {
			tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
			fmt.Fprintln(tw, "  Type\tGood\tBad\tNeutral")
			fmt.Fprintln(tw, "  ----\t----\t---\t-------")
			for assetType, tf := range fs.ByType {
				fmt.Fprintf(tw, "  %s\t%d\t%d\t%d\n", assetType, tf.Good, tf.Bad, tf.Neutral)
			}
			tw.Flush()
			fmt.Fprintln(w)
		}
	}

	return nil
}
