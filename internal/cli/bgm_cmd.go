package cli

import (
	"fmt"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"github.com/sushistack/yt.pipe/internal/domain"
	"github.com/sushistack/yt.pipe/internal/service"
	"github.com/sushistack/yt.pipe/internal/store"
)

var bgmCmd = &cobra.Command{
	Use:   "bgm",
	Short: "Manage BGM preset library",
}

func init() {
	// Add
	addCmd := &cobra.Command{
		Use:   "add",
		Short: "Register a new BGM file",
		RunE:  runBGMAdd,
	}
	addCmd.Flags().String("name", "", "BGM display name (required)")
	addCmd.Flags().String("file", "", "path to audio file (required)")
	addCmd.Flags().String("moods", "", "comma-separated mood tags")
	addCmd.Flags().String("license-type", "royalty_free", "license type: royalty_free, cc_by, cc_by_sa, cc_by_nc, custom")
	addCmd.Flags().String("credit", "", "credit text for attribution")
	addCmd.Flags().String("source", "", "license source URL")
	addCmd.Flags().Int64("duration", 0, "duration in milliseconds")
	_ = addCmd.MarkFlagRequired("name")
	_ = addCmd.MarkFlagRequired("file")

	// List
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List all BGMs",
		RunE:  runBGMList,
	}
	listCmd.Flags().String("mood", "", "filter by mood tag")

	// Show
	showCmd := &cobra.Command{
		Use:   "show <bgm-id>",
		Short: "Show BGM details",
		Args:  cobra.ExactArgs(1),
		RunE:  runBGMShow,
	}

	// Update
	updateCmd := &cobra.Command{
		Use:   "update <bgm-id>",
		Short: "Update a BGM",
		Args:  cobra.ExactArgs(1),
		RunE:  runBGMUpdate,
	}
	updateCmd.Flags().String("name", "", "new name")
	updateCmd.Flags().String("moods", "", "new comma-separated mood tags")
	updateCmd.Flags().String("license-type", "", "new license type")
	updateCmd.Flags().String("credit", "", "new credit text")

	// Delete
	deleteCmd := &cobra.Command{
		Use:   "delete <bgm-id>",
		Short: "Delete a BGM",
		Args:  cobra.ExactArgs(1),
		RunE:  runBGMDelete,
	}

	// Review
	reviewCmd := &cobra.Command{
		Use:   "review <project-id>",
		Short: "Review pending BGM recommendations for a project",
		Args:  cobra.ExactArgs(1),
		RunE:  runBGMReview,
	}
	reviewCmd.Flags().Bool("confirm-all", false, "confirm all pending recommendations")
	reviewCmd.Flags().Int("confirm", 0, "confirm specific scene number")
	reviewCmd.Flags().Int("reassign", 0, "reassign BGM for scene number")
	reviewCmd.Flags().String("bgm", "", "BGM ID for reassignment")
	reviewCmd.Flags().Int("adjust", 0, "adjust params for scene number")
	reviewCmd.Flags().Float64("volume", 0, "volume in dB")
	reviewCmd.Flags().Int("fade-in", 2000, "fade-in in ms")
	reviewCmd.Flags().Int("fade-out", 2000, "fade-out in ms")
	reviewCmd.Flags().Float64("ducking", -12, "ducking in dB")

	bgmCmd.AddCommand(addCmd, listCmd, showCmd, updateCmd, deleteCmd, reviewCmd)
	rootCmd.AddCommand(bgmCmd)
}

func openBGMService(cmd *cobra.Command) (*service.BGMService, *store.Store, func(), error) {
	cfg := GetConfig()
	if cfg == nil {
		return nil, nil, nil, fmt.Errorf("configuration not loaded")
	}
	c := cfg.Config
	dbPath := c.DBPath
	if dbPath == "" {
		dbPath = c.WorkspacePath + "/yt-pipe.db"
	}
	db, err := store.New(dbPath)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("open database: %w", err)
	}
	svc := service.NewBGMService(db, nil) // LLM not needed for CLI operations
	return svc, db, func() { db.Close() }, nil
}

func runBGMAdd(cmd *cobra.Command, args []string) error {
	cmd.SilenceUsage = true
	name, _ := cmd.Flags().GetString("name")
	file, _ := cmd.Flags().GetString("file")
	moodsStr, _ := cmd.Flags().GetString("moods")
	licenseType, _ := cmd.Flags().GetString("license-type")
	credit, _ := cmd.Flags().GetString("credit")
	source, _ := cmd.Flags().GetString("source")
	duration, _ := cmd.Flags().GetInt64("duration")

	var moods []string
	if moodsStr != "" {
		for _, m := range strings.Split(moodsStr, ",") {
			trimmed := strings.TrimSpace(m)
			if trimmed != "" {
				moods = append(moods, trimmed)
			}
		}
	}

	svc, _, cleanup, err := openBGMService(cmd)
	if err != nil {
		return fmt.Errorf("bgm add: %w", err)
	}
	defer cleanup()

	bgm, err := svc.CreateBGM(name, file, moods, duration, domain.LicenseType(licenseType), source, credit)
	if err != nil {
		return fmt.Errorf("bgm add: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Registered BGM %s [%s]\n", bgm.Name, bgm.ID)
	return nil
}

func runBGMList(cmd *cobra.Command, args []string) error {
	cmd.SilenceUsage = true
	moodFilter, _ := cmd.Flags().GetString("mood")

	svc, db, cleanup, err := openBGMService(cmd)
	if err != nil {
		return fmt.Errorf("bgm list: %w", err)
	}
	defer cleanup()
	_ = svc // use store directly for filtered search

	var bgms []*domain.BGM
	if moodFilter != "" {
		bgms, err = db.SearchByMoodTags([]string{moodFilter})
	} else {
		bgms, err = db.ListBGMs()
	}
	if err != nil {
		return fmt.Errorf("bgm list: %w", err)
	}

	if len(bgms) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No BGMs found.")
		return nil
	}

	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tNAME\tMOOD_TAGS\tLICENSE\tDURATION")
	for _, b := range bgms {
		tags := strings.Join(b.MoodTags, ",")
		dur := formatDuration(b.DurationMs)
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", b.ID, b.Name, tags, b.LicenseType, dur)
	}
	return w.Flush()
}

func runBGMShow(cmd *cobra.Command, args []string) error {
	cmd.SilenceUsage = true
	bgmID := args[0]

	svc, _, cleanup, err := openBGMService(cmd)
	if err != nil {
		return fmt.Errorf("bgm show: %w", err)
	}
	defer cleanup()

	b, err := svc.GetBGM(bgmID)
	if err != nil {
		return fmt.Errorf("bgm show: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "ID:             %s\n", b.ID)
	fmt.Fprintf(cmd.OutOrStdout(), "Name:           %s\n", b.Name)
	fmt.Fprintf(cmd.OutOrStdout(), "File:           %s\n", b.FilePath)
	fmt.Fprintf(cmd.OutOrStdout(), "Mood Tags:      %s\n", strings.Join(b.MoodTags, ", "))
	fmt.Fprintf(cmd.OutOrStdout(), "Duration:       %s\n", formatDuration(b.DurationMs))
	fmt.Fprintf(cmd.OutOrStdout(), "License Type:   %s\n", b.LicenseType)
	fmt.Fprintf(cmd.OutOrStdout(), "License Source: %s\n", b.LicenseSource)
	fmt.Fprintf(cmd.OutOrStdout(), "Credit Text:    %s\n", b.CreditText)
	return nil
}

func runBGMUpdate(cmd *cobra.Command, args []string) error {
	cmd.SilenceUsage = true
	bgmID := args[0]
	name, _ := cmd.Flags().GetString("name")
	moodsStr, _ := cmd.Flags().GetString("moods")
	licenseType, _ := cmd.Flags().GetString("license-type")
	credit, _ := cmd.Flags().GetString("credit")

	var moods []string
	if moodsStr != "" {
		for _, m := range strings.Split(moodsStr, ",") {
			trimmed := strings.TrimSpace(m)
			if trimmed != "" {
				moods = append(moods, trimmed)
			}
		}
	}

	svc, _, cleanup, err := openBGMService(cmd)
	if err != nil {
		return fmt.Errorf("bgm update: %w", err)
	}
	defer cleanup()

	b, err := svc.UpdateBGM(bgmID, name, moods, domain.LicenseType(licenseType), credit)
	if err != nil {
		return fmt.Errorf("bgm update: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Updated BGM %s [%s]\n", b.Name, b.ID)
	return nil
}

func runBGMDelete(cmd *cobra.Command, args []string) error {
	cmd.SilenceUsage = true
	bgmID := args[0]

	svc, _, cleanup, err := openBGMService(cmd)
	if err != nil {
		return fmt.Errorf("bgm delete: %w", err)
	}
	defer cleanup()

	if err := svc.DeleteBGM(bgmID); err != nil {
		return fmt.Errorf("bgm delete: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Deleted BGM %s\n", bgmID)
	return nil
}

func runBGMReview(cmd *cobra.Command, args []string) error {
	cmd.SilenceUsage = true
	projectID := args[0]

	svc, db, cleanup, err := openBGMService(cmd)
	if err != nil {
		return fmt.Errorf("bgm review: %w", err)
	}
	defer cleanup()

	confirmAll, _ := cmd.Flags().GetBool("confirm-all")
	confirmScene, _ := cmd.Flags().GetInt("confirm")
	reassignScene, _ := cmd.Flags().GetInt("reassign")
	adjustScene, _ := cmd.Flags().GetInt("adjust")

	// Handle confirm-all
	if confirmAll {
		pending, err := svc.GetPendingConfirmations(projectID)
		if err != nil {
			return fmt.Errorf("bgm review: %w", err)
		}
		for _, a := range pending {
			if err := svc.ConfirmBGM(projectID, a.SceneNum); err != nil {
				return fmt.Errorf("bgm review: confirm scene %d: %w", a.SceneNum, err)
			}
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Confirmed %d BGM assignments\n", len(pending))
		return nil
	}

	// Handle confirm specific scene
	if confirmScene > 0 {
		if err := svc.ConfirmBGM(projectID, confirmScene); err != nil {
			return fmt.Errorf("bgm review: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Confirmed BGM for scene %d\n", confirmScene)
		return nil
	}

	// Handle reassign
	if reassignScene > 0 {
		newBGMID, _ := cmd.Flags().GetString("bgm")
		if newBGMID == "" {
			return fmt.Errorf("--bgm is required with --reassign")
		}
		if err := svc.ReassignBGM(projectID, reassignScene, newBGMID); err != nil {
			return fmt.Errorf("bgm review: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Reassigned scene %d to BGM %s\n", reassignScene, newBGMID)
		return nil
	}

	// Handle adjust
	if adjustScene > 0 {
		vol, _ := cmd.Flags().GetFloat64("volume")
		fadeIn, _ := cmd.Flags().GetInt("fade-in")
		fadeOut, _ := cmd.Flags().GetInt("fade-out")
		ducking, _ := cmd.Flags().GetFloat64("ducking")
		if err := svc.AdjustBGMParams(projectID, adjustScene, vol, fadeIn, fadeOut, ducking); err != nil {
			return fmt.Errorf("bgm review: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Adjusted BGM params for scene %d\n", adjustScene)
		return nil
	}

	// Default: show pending recommendations
	assignments, err := db.ListSceneBGMAssignments(projectID)
	if err != nil {
		return fmt.Errorf("bgm review: %w", err)
	}

	if len(assignments) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No BGM assignments for this project.")
		return nil
	}

	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "SCENE\tBGM\tVOLUME\tFADE_IN\tFADE_OUT\tDUCKING\tSTATUS")
	for _, a := range assignments {
		bgm, err := db.GetBGM(a.BGMID)
		bgmName := a.BGMID
		if err == nil {
			bgmName = bgm.Name
		}
		status := "pending"
		if a.Confirmed {
			status = "confirmed"
		}
		fmt.Fprintf(w, "%d\t%s\t%.1fdB\t%dms\t%dms\t%.1fdB\t%s\n",
			a.SceneNum, bgmName, a.VolumeDB, a.FadeInMs, a.FadeOutMs, a.DuckingDB, status)
	}
	return w.Flush()
}

func formatDuration(ms int64) string {
	d := time.Duration(ms) * time.Millisecond
	m := int(d.Minutes())
	s := int(d.Seconds()) % 60
	return fmt.Sprintf("%d:%02d", m, s)
}
