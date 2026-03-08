package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/sushistack/yt.pipe/internal/domain"
	"github.com/sushistack/yt.pipe/internal/plugin/output"
	"github.com/sushistack/yt.pipe/internal/workspace"
)

// AssemblerService handles output assembly and copyright generation.
type AssemblerService struct {
	assembler    output.Assembler
	projectSvc   *ProjectService
	templatePath string
	metaPath     string
	canvas       output.CanvasConfig
}

// NewAssemblerService creates a new AssemblerService.
func NewAssemblerService(a output.Assembler, ps *ProjectService) *AssemblerService {
	return &AssemblerService{
		assembler:  a,
		projectSvc: ps,
		canvas:     output.DefaultCanvasConfig(),
	}
}

// WithConfig sets the CapCut template and canvas configuration from OutputConfig.
func (s *AssemblerService) WithConfig(templatePath, metaPath string, canvas output.CanvasConfig) {
	s.templatePath = templatePath
	s.metaPath = metaPath
	s.canvas = canvas
}

// Assemble creates the final output project from all scene assets.
func (s *AssemblerService) Assemble(ctx context.Context, projectID string, scenes []domain.Scene) (*output.AssembleResult, error) {
	if len(scenes) == 0 {
		return nil, &domain.ValidationError{Field: "scenes", Message: "no scenes provided"}
	}

	project, err := s.projectSvc.GetProject(ctx, projectID)
	if err != nil {
		return nil, err
	}

	// Validate all scenes have required assets (image, audio, subtitle)
	if err := ValidateSceneAssets(scenes, project.SCPID); err != nil {
		return nil, err
	}

	// Ensure output directory exists
	outputDir := filepath.Join(project.WorkspacePath, "output")
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return nil, fmt.Errorf("assembler: create output directory: %w", err)
	}

	input := output.AssembleInput{
		Project:      *project,
		Scenes:       scenes,
		OutputDir:    outputDir,
		TemplatePath: s.templatePath,
		MetaPath:     s.metaPath,
		Canvas:       s.canvas,
	}

	result, err := s.assembler.Assemble(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("assembler: %w", err)
	}

	// Validate assembled output
	if err := s.assembler.Validate(ctx, result.OutputPath); err != nil {
		return nil, fmt.Errorf("assembler validation: %w", err)
	}

	// Transition to assembling then complete
	if _, err := s.projectSvc.TransitionProject(ctx, projectID, domain.StatusAssembling); err != nil {
		return nil, fmt.Errorf("assembler: transition to assembling: %w", err)
	}
	if _, err := s.projectSvc.TransitionProject(ctx, projectID, domain.StatusComplete); err != nil {
		return nil, fmt.Errorf("assembler: transition to complete: %w", err)
	}

	slog.Info("assembly complete",
		"project_id", projectID,
		"output", result.OutputPath,
		"scenes", result.SceneCount,
		"total_duration_sec", result.TotalDuration,
		"images", result.ImageCount,
		"subtitles", result.SubtitleCount)

	return result, nil
}

// ccBySA3Template is the standard CC-BY-SA 3.0 attribution text format.
const ccBySA3Template = `Content based on the SCP Foundation (https://scp-wiki.wikidot.com/)
SCP Foundation content is licensed under CC-BY-SA 3.0
https://creativecommons.org/licenses/by-sa/3.0/

Original Author(s): %s
SCP Entry: %s
Source: https://scp-wiki.wikidot.com/%s

This video contains AI-generated content (images, narration, scenario).
`

// GenerateCopyrightNotice creates a description.txt with CC-BY-SA 3.0 attribution
// in the project output directory. Called automatically during assembly (FR18).
func (s *AssemblerService) GenerateCopyrightNotice(projectPath, scpID, author string) error {
	if author == "" {
		author = "Unknown"
	}
	notice := fmt.Sprintf(ccBySA3Template, author, scpID, scpID)
	descPath := filepath.Join(projectPath, "output", "description.txt")
	if err := workspace.WriteFileAtomic(descPath, []byte(notice)); err != nil {
		slog.Error("copyright: failed to write description.txt",
			"project_path", projectPath,
			"scp_id", scpID,
			"error", err)
		return fmt.Errorf("copyright: write description.txt: %w", err)
	}
	slog.Info("copyright notice generated",
		"scp_id", scpID,
		"author", author,
		"path", descPath)
	return nil
}

// CheckSpecialCopyright checks if an SCP entry has additional copyright conditions.
// Returns the copyright notes and true if special conditions exist (FR19).
func CheckSpecialCopyright(meta *workspace.MetaFile) (string, bool) {
	if meta.CopyrightNotes != "" {
		return meta.CopyrightNotes, true
	}
	return "", false
}

// ValidateSceneAssets checks all scenes have required assets (image, audio, subtitle).
// Returns a detailed error listing per-scene missing files with recovery command.
func ValidateSceneAssets(scenes []domain.Scene, scpID string) error {
	var missing []string
	for _, scene := range scenes {
		var sceneMissing []string
		if scene.ImagePath == "" {
			sceneMissing = append(sceneMissing, "image")
		}
		if scene.AudioPath == "" {
			sceneMissing = append(sceneMissing, "audio")
		}
		if scene.SubtitlePath == "" {
			sceneMissing = append(sceneMissing, "subtitle")
		}
		if len(sceneMissing) > 0 {
			missing = append(missing, fmt.Sprintf("scene %d: missing %s", scene.SceneNum, joinStrings(sceneMissing)))
		}
	}
	if len(missing) > 0 {
		msg := fmt.Sprintf("Cannot assemble: %d scene(s) have missing assets:\n", len(missing))
		for _, m := range missing {
			msg += fmt.Sprintf("  - %s\n", m)
		}
		msg += fmt.Sprintf("Run `yt-pipe status %s --scenes` to see details.", scpID)
		return &domain.ValidationError{Field: "scenes", Message: msg}
	}
	return nil
}

func joinStrings(ss []string) string {
	result := ""
	for i, s := range ss {
		if i > 0 {
			result += ", "
		}
		result += s
	}
	return result
}

// LogSpecialCopyright logs a prominent warning for special copyright conditions
// and writes the warning to the project metadata file (FR19).
// If special conditions exist, they are also appended to description.txt.
func LogSpecialCopyright(projectPath, scpID string, meta *workspace.MetaFile) error {
	notes, hasSpecial := CheckSpecialCopyright(meta)
	if !hasSpecial {
		return nil
	}

	// Prominent structured log warning (FR19)
	slog.Warn("SPECIAL COPYRIGHT CONDITIONS",
		"scp_id", scpID,
		"conditions", notes,
		"action_required", "Review additional licensing before publication")

	// CLI warning
	fmt.Fprintf(os.Stderr, "⚠ %s has additional copyright conditions: %s\n", scpID, notes)

	// Write warning to project metadata file
	warning := map[string]string{
		"scp_id":     scpID,
		"conditions": notes,
		"warning":    fmt.Sprintf("%s has additional copyright conditions: %s", scpID, notes),
	}
	data, err := json.MarshalIndent(warning, "", "  ")
	if err != nil {
		return fmt.Errorf("copyright: marshal warning: %w", err)
	}
	warningPath := filepath.Join(projectPath, "output", "copyright_warning.json")
	if err := workspace.WriteFileAtomic(warningPath, data); err != nil {
		return err
	}

	// Append special conditions to description.txt
	descPath := filepath.Join(projectPath, "output", "description.txt")
	appendText := fmt.Sprintf("\n--- Additional Copyright Conditions ---\n%s\n", notes)
	f, err := os.OpenFile(descPath, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		slog.Warn("copyright: could not append to description.txt", "error", err)
		return nil // non-fatal: description.txt may not exist yet
	}
	defer f.Close()
	_, err = f.WriteString(appendText)
	return err
}

// LicenseCheckResult holds the result of a license field validation.
type LicenseCheckResult struct {
	Valid    bool     `json:"valid"`
	Warnings []string `json:"warnings,omitempty"`
}

// CheckLicenseFields validates that all required attribution fields are present in meta.json.
// Missing fields are reported as warnings (does not block assembly).
func CheckLicenseFields(meta *workspace.MetaFile) *LicenseCheckResult {
	result := &LicenseCheckResult{Valid: true}

	if meta.Author == "" {
		result.Warnings = append(result.Warnings, "meta.json: missing 'author' field — attribution may be incomplete")
	}
	if meta.URL == "" {
		result.Warnings = append(result.Warnings, "meta.json: missing 'url' field — source link unavailable")
	}
	if meta.Title == "" {
		result.Warnings = append(result.Warnings, "meta.json: missing 'title' field — entry title unavailable")
	}

	if len(result.Warnings) > 0 {
		result.Valid = false
	}
	return result
}
