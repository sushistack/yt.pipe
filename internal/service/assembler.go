package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/jay/youtube-pipeline/internal/domain"
	"github.com/jay/youtube-pipeline/internal/plugin/output"
	"github.com/jay/youtube-pipeline/internal/workspace"
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
	var missing []string
	for _, scene := range scenes {
		if scene.ImagePath == "" {
			missing = append(missing, fmt.Sprintf("scene %d: missing image", scene.SceneNum))
		}
		if scene.AudioPath == "" {
			missing = append(missing, fmt.Sprintf("scene %d: missing audio", scene.SceneNum))
		}
		if scene.SubtitlePath == "" {
			missing = append(missing, fmt.Sprintf("scene %d: missing subtitle", scene.SceneNum))
		}
	}
	if len(missing) > 0 {
		return nil, &domain.ValidationError{Field: "scenes", Message: fmt.Sprintf("incomplete assets: %v", missing)}
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

// LogSpecialCopyright logs a prominent warning for special copyright conditions
// and writes the warning to the project metadata file (FR19).
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
	return workspace.WriteFileAtomic(warningPath, data)
}
