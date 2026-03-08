package cli

import (
	"encoding/json"
	"fmt"
	"text/tabwriter"
	"time"

	"github.com/jay/youtube-pipeline/internal/domain"
	"github.com/jay/youtube-pipeline/internal/service"
	"github.com/jay/youtube-pipeline/internal/store"
	"github.com/spf13/cobra"
)

var logsCmd = &cobra.Command{
	Use:   "logs <scp-id>",
	Short: "Show execution history and summary for a project",
	Long:  "Display execution logs, stage summary, API call counts, and estimated costs for an SCP project.",
	Args:  cobra.ExactArgs(1),
	RunE:  runLogsCmd,
}

func init() {
	logsCmd.Flags().Bool("summary", false, "show only the aggregated summary")
	logsCmd.Flags().Int("limit", 50, "maximum number of log entries to display")
	rootCmd.AddCommand(logsCmd)
}

func runLogsCmd(cmd *cobra.Command, args []string) error {
	scpID := args[0]
	cmd.SilenceUsage = true

	cfg := GetConfig()
	if cfg == nil {
		return fmt.Errorf("logs: configuration not loaded")
	}
	c := cfg.Config

	dbPath := c.DBPath
	if dbPath == "" {
		dbPath = c.WorkspacePath + "/yt-pipe.db"
	}
	db, err := store.New(dbPath)
	if err != nil {
		return fmt.Errorf("logs: open database: %w", err)
	}
	defer db.Close()

	project, err := findProjectBySCPID(db, scpID)
	if err != nil {
		return fmt.Errorf("logs: %w", err)
	}

	logs, err := db.ListExecutionLogsByProject(project.ID)
	if err != nil {
		return fmt.Errorf("logs: list execution logs: %w", err)
	}

	summary := service.BuildExecutionSummary(project.ID, logs)

	jsonOutput, _ := cmd.Flags().GetBool("json-output")
	summaryOnly, _ := cmd.Flags().GetBool("summary")
	limit, _ := cmd.Flags().GetInt("limit")

	if jsonOutput {
		return outputLogsJSON(cmd, logs, summary, summaryOnly, limit)
	}
	return outputLogsHuman(cmd, logs, summary, summaryOnly, limit)
}

// LogsOutput is the JSON output format for yt-pipe logs.
type LogsOutput struct {
	Summary *service.ExecutionSummary `json:"summary"`
	Logs    []LogEntry               `json:"logs,omitempty"`
}

// LogEntry is a simplified execution log for JSON output.
type LogEntry struct {
	ID        int     `json:"id"`
	Stage     string  `json:"stage"`
	Action    string  `json:"action"`
	Status    string  `json:"status"`
	Duration  *int    `json:"duration_ms,omitempty"`
	Cost      *float64 `json:"estimated_cost_usd,omitempty"`
	Details   string  `json:"details,omitempty"`
	CreatedAt string  `json:"created_at"`
}

func outputLogsJSON(cmd *cobra.Command, logs []*domain.ExecutionLog, summary *service.ExecutionSummary, summaryOnly bool, limit int) error {
	out := LogsOutput{Summary: summary}

	if !summaryOnly {
		entries := make([]LogEntry, 0, len(logs))
		for i, l := range logs {
			if i >= limit {
				break
			}
			entries = append(entries, LogEntry{
				ID:        l.ID,
				Stage:     l.Stage,
				Action:    l.Action,
				Status:    l.Status,
				Duration:  l.DurationMs,
				Cost:      l.EstimatedCostUSD,
				Details:   l.Details,
				CreatedAt: l.CreatedAt.Format(time.RFC3339),
			})
		}
		out.Logs = entries
	}

	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

func outputLogsHuman(cmd *cobra.Command, logs []*domain.ExecutionLog, summary *service.ExecutionSummary, summaryOnly bool, limit int) error {
	w := cmd.OutOrStdout()

	// Summary section
	fmt.Fprintf(w, "\n=== Execution Summary: %s ===\n\n", summary.ProjectID)
	fmt.Fprintf(w, "  Total Duration:  %s\n", formatDurationMs(summary.TotalDurationMs))
	fmt.Fprintf(w, "  Stages:          %d\n", summary.StageCount)
	fmt.Fprintf(w, "  API Calls:       %d\n", summary.APICallCount)
	fmt.Fprintf(w, "  Estimated Cost:  $%.4f\n", summary.EstimatedCost)
	fmt.Fprintf(w, "  Success/Fail:    %d / %d\n", summary.SuccessCount, summary.FailureCount)
	fmt.Fprintln(w)

	if len(summary.Stages) > 0 {
		fmt.Fprintln(w, "Stage Breakdown:")
		tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
		fmt.Fprintln(tw, "  Stage\tCount\tDuration\tSuccess%\tCost")
		fmt.Fprintln(tw, "  -----\t-----\t--------\t--------\t----")
		for _, s := range summary.Stages {
			fmt.Fprintf(tw, "  %s\t%d\t%s\t%.0f%%\t$%.4f\n",
				s.Stage, s.Count, formatDurationMs(s.TotalMs), s.SuccessRate, s.Cost)
		}
		tw.Flush()
		fmt.Fprintln(w)
	}

	if summaryOnly {
		return nil
	}

	// Log entries
	if len(logs) == 0 {
		fmt.Fprintln(w, "No execution logs found.")
		return nil
	}

	fmt.Fprintln(w, "Recent Logs:")
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "  Time\tStage\tAction\tStatus\tDuration\tDetails")
	fmt.Fprintln(tw, "  ----\t-----\t------\t------\t--------\t-------")
	count := len(logs)
	if count > limit {
		count = limit
	}
	for i := 0; i < count; i++ {
		l := logs[i]
		dur := "-"
		if l.DurationMs != nil {
			dur = formatDurationMs(int64(*l.DurationMs))
		}
		details := l.Details
		if len(details) > 60 {
			details = details[:57] + "..."
		}
		fmt.Fprintf(tw, "  %s\t%s\t%s\t%s\t%s\t%s\n",
			l.CreatedAt.Format("15:04:05"), l.Stage, l.Action, l.Status, dur, details)
	}
	tw.Flush()
	if len(logs) > limit {
		fmt.Fprintf(w, "  ... and %d more entries (use --limit to show more)\n", len(logs)-limit)
	}
	fmt.Fprintln(w)

	return nil
}

func formatDurationMs(ms int64) string {
	if ms < 1000 {
		return fmt.Sprintf("%dms", ms)
	}
	if ms < 60000 {
		return fmt.Sprintf("%.1fs", float64(ms)/1000)
	}
	return fmt.Sprintf("%.1fm", float64(ms)/60000)
}
