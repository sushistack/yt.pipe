package cli

import (
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/sushistack/yt.pipe/internal/glossary"
	"github.com/sushistack/yt.pipe/internal/service"
	"github.com/sushistack/yt.pipe/internal/store"
)

var glossaryCmd = &cobra.Command{
	Use:   "glossary",
	Short: "Manage glossary and term suggestions",
}

var glossarySuggestCmd = &cobra.Command{
	Use:   "suggest <scp-id>",
	Short: "Extract and suggest new glossary terms from scenario using LLM",
	Args:  cobra.ExactArgs(1),
	RunE:  runGlossarySuggest,
}

var glossaryApproveCmd = &cobra.Command{
	Use:   "approve <scp-id>",
	Short: "Approve or reject pending glossary suggestions",
	Long:  "Lists pending suggestions and approves selected ones. Use --all to approve all, or comma-separated indices.",
	Args:  cobra.ExactArgs(1),
	RunE:  runGlossaryApprove,
}

func init() {
	glossaryApproveCmd.Flags().Bool("all", false, "approve all pending suggestions")
	glossaryApproveCmd.Flags().String("select", "", "comma-separated indices to approve (e.g., '1,3,5')")
	glossaryCmd.AddCommand(glossarySuggestCmd, glossaryApproveCmd)
	rootCmd.AddCommand(glossaryCmd)
}

func runGlossarySuggest(cmd *cobra.Command, args []string) error {
	scpID := args[0]
	cmd.SilenceUsage = true

	cfg := GetConfig()
	if cfg == nil {
		return fmt.Errorf("glossary suggest: configuration not loaded")
	}

	c := cfg.Config
	projectDir := filepath.Join(c.WorkspacePath, scpID)

	// Load scenario
	scenarioPath := filepath.Join(projectDir, "scenario.json")
	scenario, err := service.LoadScenarioFromFile(scenarioPath)
	if err != nil {
		return fmt.Errorf("glossary suggest: %w", err)
	}

	// Build scenario text from all scenes
	var textParts []string
	for _, scene := range scenario.Scenes {
		if scene.Narration != "" {
			textParts = append(textParts, scene.Narration)
		}
	}
	scenarioText := strings.Join(textParts, "\n\n")
	if scenarioText == "" {
		fmt.Fprintln(cmd.OutOrStdout(), "No narration text found in scenario.")
		return nil
	}

	// Load existing glossary
	existingGlossary := glossary.LoadFromFile(c.GlossaryPath)

	// Create LLM plugin
	llmPlugin, _, _, err := createPlugins(cfg)
	if err != nil {
		return fmt.Errorf("glossary suggest: %w", err)
	}

	// Open store
	s, err := store.New(c.DBPath)
	if err != nil {
		return fmt.Errorf("glossary suggest: open store: %w", err)
	}
	defer s.Close()

	// Run suggestion
	svc := service.NewGlossaryService(s, llmPlugin, slog.Default())
	suggestions, err := svc.SuggestTerms(cmd.Context(), scpID, scenarioText, existingGlossary)
	if err != nil {
		return err
	}

	if len(suggestions) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No new terms found.")
		return nil
	}

	// Display results
	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "\n=== Glossary Suggestions (%d new terms) ===\n\n", len(suggestions))
	for i, sg := range suggestions {
		fmt.Fprintf(w, "  %d. %s [%s]\n", i+1, sg.Term, sg.Category)
		fmt.Fprintf(w, "     Pronunciation: %s\n", sg.Pronunciation)
		if sg.Definition != "" {
			fmt.Fprintf(w, "     Definition: %s\n", sg.Definition)
		}
		fmt.Fprintln(w)
	}
	fmt.Fprintf(w, "Run 'yt-pipe glossary approve %s' to approve or reject suggestions.\n", scpID)

	return nil
}

func runGlossaryApprove(cmd *cobra.Command, args []string) error {
	scpID := args[0]
	cmd.SilenceUsage = true

	cfg := GetConfig()
	if cfg == nil {
		return fmt.Errorf("glossary approve: configuration not loaded")
	}
	c := cfg.Config

	// Open store
	s, err := store.New(c.DBPath)
	if err != nil {
		return fmt.Errorf("glossary approve: open store: %w", err)
	}
	defer s.Close()

	svc := service.NewGlossaryService(s, nil, slog.Default())

	// List pending suggestions
	pending, err := svc.ListPendingSuggestions(cmd.Context(), scpID)
	if err != nil {
		return fmt.Errorf("glossary approve: %w", err)
	}

	if len(pending) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No pending suggestions.")
		return nil
	}

	// Display pending suggestions
	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "\n=== Pending Suggestions for %s (%d) ===\n\n", scpID, len(pending))
	for i, sg := range pending {
		fmt.Fprintf(w, "  %d. %s [%s]\n", i+1, sg.Term, sg.Category)
		fmt.Fprintf(w, "     Pronunciation: %s\n", sg.Pronunciation)
		if sg.Definition != "" {
			fmt.Fprintf(w, "     Definition: %s\n", sg.Definition)
		}
		fmt.Fprintln(w)
	}

	// Load existing glossary
	existingGlossary := glossary.LoadFromFile(c.GlossaryPath)

	// Determine which to approve
	approveAll, _ := cmd.Flags().GetBool("all")
	selectStr, _ := cmd.Flags().GetString("select")

	var toApprove []int // indices (0-based)
	if approveAll {
		for i := range pending {
			toApprove = append(toApprove, i)
		}
	} else if selectStr != "" {
		parts := strings.Split(selectStr, ",")
		for _, p := range parts {
			idx, err := strconv.Atoi(strings.TrimSpace(p))
			if err != nil || idx < 1 || idx > len(pending) {
				return fmt.Errorf("glossary approve: invalid index %q (valid: 1-%d)", p, len(pending))
			}
			toApprove = append(toApprove, idx-1) // convert 1-based to 0-based
		}
	} else {
		// No flags: show list and exit, user must re-run with --all or --select
		fmt.Fprintf(w, "Use --all to approve all, or --select '1,3' to approve specific suggestions.\n")
		return nil
	}

	// Approve selected
	approvedCount := 0
	for _, idx := range toApprove {
		sg := pending[idx]
		if err := svc.ApproveSuggestion(cmd.Context(), sg.ID, existingGlossary); err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to approve %q: %v\n", sg.Term, err)
			continue
		}
		approvedCount++
	}

	// Reject the rest
	rejectedCount := 0
	if approveAll || selectStr != "" {
		approvedSet := make(map[int]bool)
		for _, idx := range toApprove {
			approvedSet[idx] = true
		}
		for i, sg := range pending {
			if !approvedSet[i] {
				if err := svc.RejectSuggestion(cmd.Context(), sg.ID); err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to reject %q: %v\n", sg.Term, err)
					continue
				}
				rejectedCount++
			}
		}
	}

	// Save glossary to file (preserving existing entries)
	if c.GlossaryPath != "" && approvedCount > 0 {
		if err := glossary.WriteToFile(c.GlossaryPath, existingGlossary.Entries()); err != nil {
			return fmt.Errorf("glossary approve: save glossary: %w", err)
		}
		fmt.Fprintf(w, "Glossary saved to %s\n", c.GlossaryPath)
	}

	fmt.Fprintf(w, "\nApproved: %d, Rejected: %d\n", approvedCount, rejectedCount)
	return nil
}
