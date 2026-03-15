package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/sushistack/yt.pipe/internal/service"
	"github.com/sushistack/yt.pipe/internal/store"
)

var characterCmd = &cobra.Command{
	Use:   "character",
	Short: "Manage character ID cards for visual consistency",
}

func init() {
	// Create
	createCmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new character ID card",
		RunE:  runCharacterCreate,
	}
	createCmd.Flags().String("scp-id", "", "SCP entity ID (required)")
	createCmd.Flags().String("name", "", "canonical character name (required)")
	createCmd.Flags().String("aliases", "", "comma-separated aliases")
	createCmd.Flags().String("visual", "", "visual descriptor text or @filepath")
	createCmd.Flags().String("style", "", "style guide text")
	createCmd.Flags().String("prompt-base", "", "base image prompt fragment")
	_ = createCmd.MarkFlagRequired("scp-id")
	_ = createCmd.MarkFlagRequired("name")

	// List
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List character ID cards",
		RunE:  runCharacterList,
	}
	listCmd.Flags().String("scp-id", "", "filter by SCP entity ID")

	// Show
	showCmd := &cobra.Command{
		Use:   "show <character-id>",
		Short: "Show character detail",
		Args:  cobra.ExactArgs(1),
		RunE:  runCharacterShow,
	}

	// Update
	updateCmd := &cobra.Command{
		Use:   "update <character-id>",
		Short: "Update character fields",
		Args:  cobra.ExactArgs(1),
		RunE:  runCharacterUpdate,
	}
	updateCmd.Flags().String("name", "", "new canonical name")
	updateCmd.Flags().String("aliases", "", "new comma-separated aliases")
	updateCmd.Flags().String("visual", "", "new visual descriptor text or @filepath")
	updateCmd.Flags().String("style", "", "new style guide text")
	updateCmd.Flags().String("prompt-base", "", "new base image prompt fragment")

	// Delete
	deleteCmd := &cobra.Command{
		Use:   "delete <character-id>",
		Short: "Delete a character ID card",
		Args:  cobra.ExactArgs(1),
		RunE:  runCharacterDelete,
	}

	// Generate candidates
	generateCmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate character appearance candidates using LLM + image generation",
		RunE:  runCharacterGenerate,
	}
	generateCmd.Flags().String("scp", "", "SCP entity ID (required)")
	generateCmd.Flags().Int("count", 4, "Number of candidate images to generate")
	generateCmd.Flags().Bool("regenerate", false, "Discard existing candidates and regenerate")
	_ = generateCmd.MarkFlagRequired("scp")

	// Select candidate
	selectCmd := &cobra.Command{
		Use:   "select",
		Short: "Select a generated character candidate",
		RunE:  runCharacterSelect,
	}
	selectCmd.Flags().String("scp", "", "SCP entity ID (required)")
	selectCmd.Flags().Int("num", 0, "Candidate number to select (required)")
	_ = selectCmd.MarkFlagRequired("scp")
	_ = selectCmd.MarkFlagRequired("num")

	characterCmd.AddCommand(createCmd, listCmd, showCmd, updateCmd, deleteCmd, generateCmd, selectCmd)
	rootCmd.AddCommand(characterCmd)
}

func openCharacterService(cmd *cobra.Command) (*service.CharacterService, func(), error) {
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
	svc := service.NewCharacterService(db)
	return svc, func() { db.Close() }, nil
}

func runCharacterCreate(cmd *cobra.Command, args []string) error {
	cmd.SilenceUsage = true
	scpID, _ := cmd.Flags().GetString("scp-id")
	name, _ := cmd.Flags().GetString("name")
	aliasesStr, _ := cmd.Flags().GetString("aliases")
	visual, _ := cmd.Flags().GetString("visual")
	style, _ := cmd.Flags().GetString("style")
	promptBase, _ := cmd.Flags().GetString("prompt-base")

	// Parse aliases
	var aliases []string
	if aliasesStr != "" {
		for _, a := range strings.Split(aliasesStr, ",") {
			trimmed := strings.TrimSpace(a)
			if trimmed != "" {
				aliases = append(aliases, trimmed)
			}
		}
	}

	// Read visual from file if @prefixed
	visual = readTextOrFile(visual)

	svc, cleanup, err := openCharacterService(cmd)
	if err != nil {
		return fmt.Errorf("character create: %w", err)
	}
	defer cleanup()

	c, err := svc.CreateCharacter(scpID, name, aliases, visual, style, promptBase)
	if err != nil {
		return fmt.Errorf("character create: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Created character %s [%s]\n", c.CanonicalName, c.ID)
	return nil
}

func runCharacterList(cmd *cobra.Command, args []string) error {
	cmd.SilenceUsage = true
	scpID, _ := cmd.Flags().GetString("scp-id")

	svc, cleanup, err := openCharacterService(cmd)
	if err != nil {
		return fmt.Errorf("character list: %w", err)
	}
	defer cleanup()

	chars, err := svc.ListCharacters(scpID)
	if err != nil {
		return fmt.Errorf("character list: %w", err)
	}

	jsonOutput, _ := cmd.Flags().GetBool("json-output")
	if jsonOutput {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(chars)
	}

	if len(chars) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No characters found.")
		return nil
	}

	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tSCP_ID\tNAME\tALIASES")
	for _, c := range chars {
		aliasCount := fmt.Sprintf("%d aliases", len(c.Aliases))
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", c.ID, c.SCPID, c.CanonicalName, aliasCount)
	}
	return w.Flush()
}

func runCharacterShow(cmd *cobra.Command, args []string) error {
	cmd.SilenceUsage = true
	charID := args[0]

	svc, cleanup, err := openCharacterService(cmd)
	if err != nil {
		return fmt.Errorf("character show: %w", err)
	}
	defer cleanup()

	c, err := svc.GetCharacter(charID)
	if err != nil {
		return fmt.Errorf("character show: %w", err)
	}

	jsonOutput, _ := cmd.Flags().GetBool("json-output")
	if jsonOutput {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(c)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "ID:               %s\n", c.ID)
	fmt.Fprintf(cmd.OutOrStdout(), "SCP ID:           %s\n", c.SCPID)
	fmt.Fprintf(cmd.OutOrStdout(), "Canonical Name:   %s\n", c.CanonicalName)
	if len(c.Aliases) > 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "Aliases:          %s\n", strings.Join(c.Aliases, ", "))
	} else {
		fmt.Fprintln(cmd.OutOrStdout(), "Aliases:          (none)")
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Visual Descriptor: %s\n", c.VisualDescriptor)
	fmt.Fprintf(cmd.OutOrStdout(), "Style Guide:      %s\n", c.StyleGuide)
	fmt.Fprintf(cmd.OutOrStdout(), "Image Prompt Base: %s\n", c.ImagePromptBase)
	return nil
}

func runCharacterUpdate(cmd *cobra.Command, args []string) error {
	cmd.SilenceUsage = true
	charID := args[0]
	name, _ := cmd.Flags().GetString("name")
	aliasesStr, _ := cmd.Flags().GetString("aliases")
	visual, _ := cmd.Flags().GetString("visual")
	style, _ := cmd.Flags().GetString("style")
	promptBase, _ := cmd.Flags().GetString("prompt-base")

	var aliases []string
	if aliasesStr != "" {
		for _, a := range strings.Split(aliasesStr, ",") {
			trimmed := strings.TrimSpace(a)
			if trimmed != "" {
				aliases = append(aliases, trimmed)
			}
		}
	}

	visual = readTextOrFile(visual)

	svc, cleanup, err := openCharacterService(cmd)
	if err != nil {
		return fmt.Errorf("character update: %w", err)
	}
	defer cleanup()

	c, err := svc.UpdateCharacter(charID, name, aliases, visual, style, promptBase)
	if err != nil {
		return fmt.Errorf("character update: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Updated character %s [%s]\n", c.CanonicalName, c.ID)
	return nil
}

func runCharacterDelete(cmd *cobra.Command, args []string) error {
	cmd.SilenceUsage = true
	charID := args[0]

	svc, cleanup, err := openCharacterService(cmd)
	if err != nil {
		return fmt.Errorf("character delete: %w", err)
	}
	defer cleanup()

	if err := svc.DeleteCharacter(charID); err != nil {
		return fmt.Errorf("character delete: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Deleted character %s\n", charID)
	return nil
}

func runCharacterGenerate(cmd *cobra.Command, args []string) error {
	cmd.SilenceUsage = true
	scpIDRaw, _ := cmd.Flags().GetString("scp")
	count, _ := cmd.Flags().GetInt("count")
	regenerate, _ := cmd.Flags().GetBool("regenerate")

	scpID, err := sanitizeSCPID(scpIDRaw)
	if err != nil {
		return fmt.Errorf("character generate: %w", err)
	}

	svc, cleanup, err := openCharacterService(cmd)
	if err != nil {
		return fmt.Errorf("character generate: %w", err)
	}
	defer cleanup()

	// Check for existing character (reuse flow)
	if !regenerate {
		existing, err := svc.CheckExistingCharacter(scpID)
		if err == nil && existing != nil {
			fmt.Fprintf(cmd.OutOrStdout(), "Existing character found: %s\n", existing.CanonicalName)
			fmt.Fprintf(cmd.OutOrStdout(), "  Visual: %s\n", existing.VisualDescriptor)
			if existing.SelectedImagePath != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "  Image: %s\n", existing.SelectedImagePath)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\n[Y] Reuse existing  [N] Generate new candidates\n")
			fmt.Fprintf(cmd.OutOrStdout(), "To generate new: yt-pipe character generate --scp %s --regenerate\n", scpID)
			return nil
		}
	}

	// Get workspace path for saving candidates
	cfg := GetConfig()
	if cfg == nil {
		return fmt.Errorf("character generate: configuration not loaded")
	}
	workspacePath := cfg.Config.WorkspacePath
	candidateDir := fmt.Sprintf("%s/%s/characters", workspacePath, scpID)
	if err := os.MkdirAll(candidateDir, 0o755); err != nil {
		return fmt.Errorf("character generate: create candidate dir: %w", err)
	}

	// If regenerate, clean existing candidates
	if regenerate {
		entries, _ := os.ReadDir(candidateDir)
		for _, e := range entries {
			if strings.HasPrefix(e.Name(), "candidate_") {
				os.Remove(fmt.Sprintf("%s/%s", candidateDir, e.Name()))
			}
		}
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Generating %d character candidates for %s...\n", count, scpID)

	results, err := svc.GenerateCandidates(cmd.Context(), scpID, count, workspacePath)
	if err != nil {
		// Fallback: if LLM/ImageGen not configured, print manual instructions
		fmt.Fprintf(cmd.OutOrStdout(), "Auto-generation unavailable: %v\n", err)
		fmt.Fprintf(cmd.OutOrStdout(), "Create candidates manually:\n")
		for i := 1; i <= count; i++ {
			fmt.Fprintf(cmd.OutOrStdout(), "  %s/candidate_%d.png (image) + candidate_%d.txt (description)\n", candidateDir, i, i)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "\nAfter creating files, run: yt-pipe character select --scp %s --num <N>\n", scpID)
		return nil
	}

	for _, r := range results {
		fmt.Fprintf(cmd.OutOrStdout(), "  Candidate %d: %s\n", r.Index, r.ImagePath)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "\nReview images, then run: yt-pipe character select --scp %s --num <N>\n", scpID)

	return nil
}

func runCharacterSelect(cmd *cobra.Command, args []string) error {
	cmd.SilenceUsage = true
	scpIDRaw, _ := cmd.Flags().GetString("scp")
	num, _ := cmd.Flags().GetInt("num")

	scpID, err := sanitizeSCPID(scpIDRaw)
	if err != nil {
		return fmt.Errorf("character select: %w", err)
	}

	cfg := GetConfig()
	if cfg == nil {
		return fmt.Errorf("character select: configuration not loaded")
	}
	workspacePath := cfg.Config.WorkspacePath

	svc, cleanup, err := openCharacterService(cmd)
	if err != nil {
		return fmt.Errorf("character select: %w", err)
	}
	defer cleanup()

	c, err := svc.SelectCandidate(scpID, num, workspacePath)
	if err != nil {
		return fmt.Errorf("character select: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Character selected: %s [%s]\n", c.CanonicalName, c.ID)
	fmt.Fprintf(cmd.OutOrStdout(), "  Image path: %s\n", c.SelectedImagePath)
	fmt.Fprintf(cmd.OutOrStdout(), "  Ready for pipeline run.\n")

	return nil
}

// sanitizeSCPID ensures the SCP ID is safe for use in file paths.
func sanitizeSCPID(scpID string) (string, error) {
	cleaned := filepath.Base(scpID)
	if cleaned != scpID || strings.Contains(scpID, "..") || strings.ContainsAny(scpID, `/\`) {
		return "", fmt.Errorf("invalid SCP ID %q: must not contain path separators or '..'", scpID)
	}
	return cleaned, nil
}

// readTextOrFile reads text from file if prefixed with @, otherwise returns as-is.
func readTextOrFile(s string) string {
	if strings.HasPrefix(s, "@") {
		data, err := os.ReadFile(strings.TrimPrefix(s, "@"))
		if err != nil {
			return s // return original if file read fails
		}
		return strings.TrimSpace(string(data))
	}
	return s
}
