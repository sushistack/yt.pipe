package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/sushistack/yt.pipe/internal/domain"
	"github.com/sushistack/yt.pipe/internal/service"
	"github.com/sushistack/yt.pipe/internal/store"
)

var templateCmd = &cobra.Command{
	Use:     "prompt",
	Aliases: []string{"template"},
	Short:   "Manage prompt templates",
}

func init() {
	// List
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List prompt templates",
		RunE:  runTemplateList,
	}
	listCmd.Flags().String("category", "", "filter by category: scenario, image, tts, caption")

	// Show
	showCmd := &cobra.Command{
		Use:   "show <template-id>",
		Short: "Show template content",
		Args:  cobra.ExactArgs(1),
		RunE:  runTemplateShow,
	}
	showCmd.Flags().Int("version", 0, "show specific version content")

	// Create
	createCmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new template",
		RunE:  runTemplateCreate,
	}
	createCmd.Flags().String("category", "", "template category: scenario, image, tts, caption (required)")
	createCmd.Flags().String("name", "", "template name (required)")
	createCmd.Flags().String("file", "", "path to template content file (required)")
	_ = createCmd.MarkFlagRequired("category")
	_ = createCmd.MarkFlagRequired("name")
	_ = createCmd.MarkFlagRequired("file")

	// Update
	updateCmd := &cobra.Command{
		Use:   "update <template-id>",
		Short: "Update template content",
		Args:  cobra.ExactArgs(1),
		RunE:  runTemplateUpdate,
	}
	updateCmd.Flags().String("file", "", "path to new template content file (required)")
	_ = updateCmd.MarkFlagRequired("file")

	// Rollback
	rollbackCmd := &cobra.Command{
		Use:   "rollback <template-id>",
		Short: "Rollback template to a specific version",
		Args:  cobra.ExactArgs(1),
		RunE:  runTemplateRollback,
	}
	rollbackCmd.Flags().Int("version", 0, "version number to rollback to (required)")
	_ = rollbackCmd.MarkFlagRequired("version")

	// Delete
	deleteCmd := &cobra.Command{
		Use:   "delete <template-id>",
		Short: "Delete a template",
		Args:  cobra.ExactArgs(1),
		RunE:  runTemplateDelete,
	}

	// Override
	overrideCmd := &cobra.Command{
		Use:   "override <template-id>",
		Short: "Manage per-project template overrides",
		Args:  cobra.ExactArgs(1),
		RunE:  runTemplateOverride,
	}
	overrideCmd.Flags().String("project", "", "project ID (required)")
	overrideCmd.Flags().String("file", "", "path to override content file")
	overrideCmd.Flags().Bool("delete", false, "delete the override")
	_ = overrideCmd.MarkFlagRequired("project")

	templateCmd.AddCommand(listCmd, showCmd, createCmd, updateCmd, rollbackCmd, deleteCmd, overrideCmd)
	rootCmd.AddCommand(templateCmd)
}

func openTemplateService(cmd *cobra.Command) (*service.TemplateService, func(), error) {
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
	svc := service.NewTemplateService(db)
	return svc, func() { db.Close() }, nil
}

func runTemplateList(cmd *cobra.Command, args []string) error {
	cmd.SilenceUsage = true
	category, _ := cmd.Flags().GetString("category")

	svc, cleanup, err := openTemplateService(cmd)
	if err != nil {
		return fmt.Errorf("prompt list: %w", err)
	}
	defer cleanup()

	templates, err := svc.ListTemplates(category)
	if err != nil {
		return fmt.Errorf("prompt list: %w", err)
	}

	jsonOutput, _ := cmd.Flags().GetBool("json-output")
	if jsonOutput {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(templates)
	}

	if len(templates) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No templates found.")
		return nil
	}

	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tCATEGORY\tNAME\tVERSION\tDEFAULT")
	for _, t := range templates {
		def := ""
		if t.IsDefault {
			def = "yes"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\tv%d\t%s\n", t.ID, t.Category, t.Name, t.Version, def)
	}
	return w.Flush()
}

func runTemplateShow(cmd *cobra.Command, args []string) error {
	cmd.SilenceUsage = true
	templateID := args[0]
	version, _ := cmd.Flags().GetInt("version")

	svc, cleanup, err := openTemplateService(cmd)
	if err != nil {
		return fmt.Errorf("prompt show: %w", err)
	}
	defer cleanup()

	if version > 0 {
		v, err := svc.GetTemplateVersion(templateID, version)
		if err != nil {
			return fmt.Errorf("prompt show: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "# Template: %s (version %d)\n\n%s\n", v.TemplateID, v.Version, v.Content)
		return nil
	}

	t, err := svc.GetTemplate(templateID)
	if err != nil {
		return fmt.Errorf("prompt show: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "# %s (%s) v%d\n\n%s\n", t.Name, t.Category, t.Version, t.Content)
	return nil
}

func runTemplateCreate(cmd *cobra.Command, args []string) error {
	cmd.SilenceUsage = true
	category, _ := cmd.Flags().GetString("category")
	name, _ := cmd.Flags().GetString("name")
	filePath, _ := cmd.Flags().GetString("file")

	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("prompt create: read file: %w", err)
	}

	svc, cleanup, err := openTemplateService(cmd)
	if err != nil {
		return fmt.Errorf("prompt create: %w", err)
	}
	defer cleanup()

	t, err := svc.CreateTemplate(templateCategoryFromString(category), name, string(content), false)
	if err != nil {
		return fmt.Errorf("prompt create: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Created template %s (%s) [%s]\n", t.Name, t.Category, t.ID)
	return nil
}

func runTemplateUpdate(cmd *cobra.Command, args []string) error {
	cmd.SilenceUsage = true
	templateID := args[0]
	filePath, _ := cmd.Flags().GetString("file")

	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("prompt update: read file: %w", err)
	}

	svc, cleanup, err := openTemplateService(cmd)
	if err != nil {
		return fmt.Errorf("prompt update: %w", err)
	}
	defer cleanup()

	t, err := svc.UpdateTemplate(templateID, string(content))
	if err != nil {
		return fmt.Errorf("prompt update: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Updated template %s to v%d\n", t.Name, t.Version)
	return nil
}

func runTemplateRollback(cmd *cobra.Command, args []string) error {
	cmd.SilenceUsage = true
	templateID := args[0]
	version, _ := cmd.Flags().GetInt("version")

	svc, cleanup, err := openTemplateService(cmd)
	if err != nil {
		return fmt.Errorf("prompt rollback: %w", err)
	}
	defer cleanup()

	t, err := svc.RollbackTemplate(templateID, version)
	if err != nil {
		return fmt.Errorf("prompt rollback: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Rolled back %s to version %d content (now v%d)\n", t.Name, version, t.Version)
	return nil
}

func runTemplateDelete(cmd *cobra.Command, args []string) error {
	cmd.SilenceUsage = true
	templateID := args[0]

	svc, cleanup, err := openTemplateService(cmd)
	if err != nil {
		return fmt.Errorf("prompt delete: %w", err)
	}
	defer cleanup()

	if err := svc.DeleteTemplate(templateID); err != nil {
		return fmt.Errorf("prompt delete: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Deleted template %s\n", templateID)
	return nil
}

func runTemplateOverride(cmd *cobra.Command, args []string) error {
	cmd.SilenceUsage = true
	templateID := args[0]
	projectID, _ := cmd.Flags().GetString("project")
	filePath, _ := cmd.Flags().GetString("file")
	deleteFlag, _ := cmd.Flags().GetBool("delete")

	svc, cleanup, err := openTemplateService(cmd)
	if err != nil {
		return fmt.Errorf("prompt override: %w", err)
	}
	defer cleanup()

	if deleteFlag {
		if err := svc.DeleteOverride(projectID, templateID); err != nil {
			return fmt.Errorf("prompt override: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Deleted override for template %s in project %s\n", templateID, projectID)
		return nil
	}

	if filePath == "" {
		return fmt.Errorf("prompt override: --file is required (use --delete to remove)")
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("prompt override: read file: %w", err)
	}

	if err := svc.SetOverride(projectID, templateID, string(content)); err != nil {
		return fmt.Errorf("prompt override: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Set override for template %s in project %s\n", templateID, projectID)
	return nil
}

func templateCategoryFromString(s string) domain.TemplateCategory {
	return domain.TemplateCategory(s)
}
