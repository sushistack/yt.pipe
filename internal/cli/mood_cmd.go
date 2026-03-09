package cli

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/sushistack/yt.pipe/internal/service"
	"github.com/sushistack/yt.pipe/internal/store"
)

var moodCmd = &cobra.Command{
	Use:   "mood",
	Short: "Manage TTS mood presets",
}

func init() {
	// List
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List all mood presets",
		RunE:  runMoodList,
	}

	// Create
	createCmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new mood preset",
		RunE:  runMoodCreate,
	}
	createCmd.Flags().String("name", "", "preset name (required)")
	createCmd.Flags().Float64("speed", 1.0, "speech speed multiplier")
	createCmd.Flags().String("emotion", "neutral", "emotion type (required)")
	createCmd.Flags().Float64("pitch", 1.0, "pitch multiplier")
	createCmd.Flags().String("description", "", "preset description")
	_ = createCmd.MarkFlagRequired("name")
	_ = createCmd.MarkFlagRequired("emotion")

	// Update
	updateCmd := &cobra.Command{
		Use:   "update <preset-id>",
		Short: "Update a mood preset",
		Args:  cobra.ExactArgs(1),
		RunE:  runMoodUpdate,
	}
	updateCmd.Flags().String("name", "", "new preset name")
	updateCmd.Flags().Float64("speed", 0, "new speech speed")
	updateCmd.Flags().String("emotion", "", "new emotion type")
	updateCmd.Flags().Float64("pitch", 0, "new pitch multiplier")
	updateCmd.Flags().String("description", "", "new description")

	// Delete
	deleteCmd := &cobra.Command{
		Use:   "delete <preset-id>",
		Short: "Delete a mood preset",
		Args:  cobra.ExactArgs(1),
		RunE:  runMoodDelete,
	}

	// Show
	showCmd := &cobra.Command{
		Use:   "show <preset-id>",
		Short: "Show mood preset detail",
		Args:  cobra.ExactArgs(1),
		RunE:  runMoodShow,
	}

	// Review
	reviewCmd := &cobra.Command{
		Use:   "review <project-id>",
		Short: "Review auto-mapped mood assignments",
		Args:  cobra.ExactArgs(1),
		RunE:  runMoodReview,
	}
	reviewCmd.Flags().Bool("confirm-all", false, "confirm all pending assignments")
	reviewCmd.Flags().Int("confirm", 0, "confirm a specific scene number")
	reviewCmd.Flags().Int("reassign", 0, "reassign a specific scene number")
	reviewCmd.Flags().String("preset", "", "preset ID for reassignment")

	moodCmd.AddCommand(listCmd, createCmd, updateCmd, deleteCmd, showCmd, reviewCmd)
	rootCmd.AddCommand(moodCmd)
}

func openMoodService(cmd *cobra.Command) (*service.MoodService, func(), error) {
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
	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	svc := service.NewMoodService(db, nil, logger) // LLM nil for CLI CRUD operations
	return svc, func() { db.Close() }, nil
}

func runMoodList(cmd *cobra.Command, args []string) error {
	cmd.SilenceUsage = true

	svc, cleanup, err := openMoodService(cmd)
	if err != nil {
		return fmt.Errorf("mood list: %w", err)
	}
	defer cleanup()

	presets, err := svc.ListPresets()
	if err != nil {
		return fmt.Errorf("mood list: %w", err)
	}

	jsonOutput, _ := cmd.Flags().GetBool("json-output")
	if jsonOutput {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(presets)
	}

	if len(presets) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No mood presets found.")
		return nil
	}

	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tNAME\tDESCRIPTION\tSPEED\tEMOTION\tPITCH")
	for _, p := range presets {
		desc := p.Description
		if len(desc) > 30 {
			desc = desc[:27] + "..."
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%.1f\t%s\t%.1f\n", p.ID, p.Name, desc, p.Speed, p.Emotion, p.Pitch)
	}
	return w.Flush()
}

func runMoodCreate(cmd *cobra.Command, args []string) error {
	cmd.SilenceUsage = true
	name, _ := cmd.Flags().GetString("name")
	speed, _ := cmd.Flags().GetFloat64("speed")
	emotion, _ := cmd.Flags().GetString("emotion")
	pitch, _ := cmd.Flags().GetFloat64("pitch")
	description, _ := cmd.Flags().GetString("description")

	svc, cleanup, err := openMoodService(cmd)
	if err != nil {
		return fmt.Errorf("mood create: %w", err)
	}
	defer cleanup()

	p, err := svc.CreatePreset(name, description, speed, emotion, pitch, nil)
	if err != nil {
		return fmt.Errorf("mood create: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Created mood preset %s [%s]\n", p.Name, p.ID)
	return nil
}

func runMoodUpdate(cmd *cobra.Command, args []string) error {
	cmd.SilenceUsage = true
	presetID := args[0]

	svc, cleanup, err := openMoodService(cmd)
	if err != nil {
		return fmt.Errorf("mood update: %w", err)
	}
	defer cleanup()

	var name, emotion, description *string
	var speed, pitch *float64

	if cmd.Flags().Changed("name") {
		v, _ := cmd.Flags().GetString("name")
		name = &v
	}
	if cmd.Flags().Changed("emotion") {
		v, _ := cmd.Flags().GetString("emotion")
		emotion = &v
	}
	if cmd.Flags().Changed("description") {
		v, _ := cmd.Flags().GetString("description")
		description = &v
	}
	if cmd.Flags().Changed("speed") {
		v, _ := cmd.Flags().GetFloat64("speed")
		speed = &v
	}
	if cmd.Flags().Changed("pitch") {
		v, _ := cmd.Flags().GetFloat64("pitch")
		pitch = &v
	}

	p, err := svc.UpdatePreset(presetID, name, description, speed, emotion, pitch)
	if err != nil {
		return fmt.Errorf("mood update: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Updated mood preset %s [%s]\n", p.Name, p.ID)
	return nil
}

func runMoodDelete(cmd *cobra.Command, args []string) error {
	cmd.SilenceUsage = true
	presetID := args[0]

	svc, cleanup, err := openMoodService(cmd)
	if err != nil {
		return fmt.Errorf("mood delete: %w", err)
	}
	defer cleanup()

	if err := svc.DeletePreset(presetID); err != nil {
		return fmt.Errorf("mood delete: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Deleted mood preset %s\n", presetID)
	return nil
}

func runMoodShow(cmd *cobra.Command, args []string) error {
	cmd.SilenceUsage = true
	presetID := args[0]

	svc, cleanup, err := openMoodService(cmd)
	if err != nil {
		return fmt.Errorf("mood show: %w", err)
	}
	defer cleanup()

	p, err := svc.GetPreset(presetID)
	if err != nil {
		return fmt.Errorf("mood show: %w", err)
	}

	jsonOutput, _ := cmd.Flags().GetBool("json-output")
	if jsonOutput {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(p)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "ID:          %s\n", p.ID)
	fmt.Fprintf(cmd.OutOrStdout(), "Name:        %s\n", p.Name)
	fmt.Fprintf(cmd.OutOrStdout(), "Description: %s\n", p.Description)
	fmt.Fprintf(cmd.OutOrStdout(), "Speed:       %.2f\n", p.Speed)
	fmt.Fprintf(cmd.OutOrStdout(), "Emotion:     %s\n", p.Emotion)
	fmt.Fprintf(cmd.OutOrStdout(), "Pitch:       %.2f\n", p.Pitch)
	if len(p.ParamsJSON) > 0 {
		paramsBytes, _ := json.MarshalIndent(p.ParamsJSON, "             ", "  ")
		fmt.Fprintf(cmd.OutOrStdout(), "Params:      %s\n", string(paramsBytes))
	}
	return nil
}

func runMoodReview(cmd *cobra.Command, args []string) error {
	cmd.SilenceUsage = true
	projectID := args[0]

	svc, cleanup, err := openMoodService(cmd)
	if err != nil {
		return fmt.Errorf("mood review: %w", err)
	}
	defer cleanup()

	confirmAll, _ := cmd.Flags().GetBool("confirm-all")
	confirmScene, _ := cmd.Flags().GetInt("confirm")
	reassignScene, _ := cmd.Flags().GetInt("reassign")
	presetID, _ := cmd.Flags().GetString("preset")

	// Confirm all
	if confirmAll {
		count, err := svc.ConfirmAll(projectID)
		if err != nil {
			return fmt.Errorf("mood review: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Confirmed %d scene mood assignments\n", count)
		return nil
	}

	// Confirm specific scene
	if confirmScene > 0 {
		if err := svc.ConfirmScene(projectID, confirmScene); err != nil {
			return fmt.Errorf("mood review: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Confirmed mood for scene %d\n", confirmScene)
		return nil
	}

	// Reassign specific scene
	if reassignScene > 0 {
		if presetID == "" {
			return fmt.Errorf("mood review: --preset is required with --reassign")
		}
		if err := svc.ReassignScene(projectID, reassignScene, presetID); err != nil {
			return fmt.Errorf("mood review: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Reassigned scene %d to preset %s\n", reassignScene, presetID)
		return nil
	}

	// Default: show pending confirmations
	pending, err := svc.GetPendingConfirmations(projectID)
	if err != nil {
		return fmt.Errorf("mood review: %w", err)
	}

	if len(pending) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No pending mood assignments.")
		return nil
	}

	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "SCENE\tPRESET_ID\tAUTO_MAPPED\tCONFIRMED")
	for _, a := range pending {
		fmt.Fprintf(w, "%d\t%s\t%v\t%v\n", a.SceneNum, a.PresetID, a.AutoMapped, a.Confirmed)
	}
	return w.Flush()
}
