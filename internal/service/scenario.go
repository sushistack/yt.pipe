package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/sushistack/yt.pipe/internal/domain"
	"github.com/sushistack/yt.pipe/internal/glossary"
	"github.com/sushistack/yt.pipe/internal/plugin/llm"
	"github.com/sushistack/yt.pipe/internal/store"
	"github.com/sushistack/yt.pipe/internal/workspace"
)

// ScenarioService handles scenario generation and management.
type ScenarioService struct {
	store        *store.Store
	llm          llm.LLM
	projectSvc   *ProjectService
	templatesDir string
	glossary     *glossary.Glossary
}

// NewScenarioService creates a new ScenarioService.
func NewScenarioService(s *store.Store, l llm.LLM, ps *ProjectService) *ScenarioService {
	return &ScenarioService{store: s, llm: l, projectSvc: ps}
}

// SetTemplatesDir enables 4-stage pipeline scenario generation.
func (ss *ScenarioService) SetTemplatesDir(dir string) {
	ss.templatesDir = dir
}

// SetGlossary sets the glossary for the 4-stage pipeline.
func (ss *ScenarioService) SetGlossary(g *glossary.Glossary) {
	ss.glossary = g
}

// GenerateScenario generates a scenario from SCP data and saves it to the project workspace.
// It creates a project, generates the scenario via LLM, saves output files, and transitions to scenario_review.
func (ss *ScenarioService) GenerateScenario(ctx context.Context, scpData *workspace.SCPData, workspacePath string) (*domain.ScenarioOutput, *domain.Project, error) {
	// Create project
	project, err := ss.projectSvc.CreateProject(ctx, scpData.SCPID, workspacePath)
	if err != nil {
		return nil, nil, fmt.Errorf("scenario: create project: %w", err)
	}

	scenario, err := ss.generateScenarioInternal(ctx, scpData, workspacePath)
	if err != nil {
		return nil, project, err
	}

	// Update scene count
	project.SceneCount = len(scenario.Scenes)
	if err := ss.store.UpdateProject(project); err != nil {
		return nil, project, fmt.Errorf("scenario: update scene count: %w", err)
	}

	// Set stage to scenario
	project, err = ss.projectSvc.SetProjectStage(ctx, project.ID, domain.StageScenario)
	if err != nil {
		return nil, project, fmt.Errorf("scenario: set stage to scenario: %w", err)
	}

	return scenario, project, nil
}

// RegenerateSection regenerates a single scene using the LLM plugin.
func (ss *ScenarioService) RegenerateSection(ctx context.Context, projectID string, sceneNum int, instruction string) (*domain.SceneScript, error) {
	project, err := ss.projectSvc.GetProject(ctx, projectID)
	if err != nil {
		return nil, err
	}

	// Regeneration is allowed whenever scenario exists (no state gate)

	// Load existing scenario
	scenarioPath := filepath.Join(project.WorkspacePath, "scenario.json")
	scenario, err := LoadScenarioFromFile(scenarioPath)
	if err != nil {
		return nil, fmt.Errorf("scenario: load for regeneration: %w", err)
	}

	// Regenerate section via LLM
	newScene, err := ss.llm.RegenerateSection(ctx, scenario, sceneNum, instruction)
	if err != nil {
		return nil, fmt.Errorf("scenario: regenerate section: %w", err)
	}

	// Replace scene in scenario
	for i, s := range scenario.Scenes {
		if s.SceneNum == sceneNum {
			scenario.Scenes[i] = *newScene
			break
		}
	}

	// Save updated scenario
	scenarioJSON, err := json.MarshalIndent(scenario, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("scenario: marshal updated output: %w", err)
	}
	if err := workspace.WriteFileAtomic(scenarioPath, scenarioJSON); err != nil {
		return nil, fmt.Errorf("scenario: save updated json: %w", err)
	}

	// Update markdown
	md := RenderScenarioMarkdown(scenario)
	mdPath := filepath.Join(project.WorkspacePath, "scenario.md")
	if err := workspace.WriteFileAtomic(mdPath, []byte(md)); err != nil {
		return nil, fmt.Errorf("scenario: save updated markdown: %w", err)
	}

	return newScene, nil
}

// GenerateScenarioForProject generates a scenario for an existing project.
// Unlike GenerateScenario, this does not create a new project — it uses the provided one.
func (ss *ScenarioService) GenerateScenarioForProject(ctx context.Context, project *domain.Project, scpData *workspace.SCPData) (*domain.ScenarioOutput, error) {
	scenario, err := ss.generateScenarioInternal(ctx, scpData, project.WorkspacePath)
	if err != nil {
		return nil, err
	}

	// Save scenario JSON
	scenarioJSON, err := json.MarshalIndent(scenario, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("scenario: marshal output: %w", err)
	}
	scenarioPath := filepath.Join(project.WorkspacePath, "scenario.json")
	if err := workspace.WriteFileAtomic(scenarioPath, scenarioJSON); err != nil {
		return nil, fmt.Errorf("scenario: save json: %w", err)
	}

	// Save scenario markdown for human review
	md := RenderScenarioMarkdown(scenario)
	mdPath := filepath.Join(project.WorkspacePath, "scenario.md")
	if err := workspace.WriteFileAtomic(mdPath, []byte(md)); err != nil {
		return nil, fmt.Errorf("scenario: save markdown: %w", err)
	}

	// Update scene count
	project.SceneCount = len(scenario.Scenes)
	if err := ss.store.UpdateProject(project); err != nil {
		return nil, fmt.Errorf("scenario: update scene count: %w", err)
	}

	// Set stage to scenario
	if _, err := ss.projectSvc.SetProjectStage(ctx, project.ID, domain.StageScenario); err != nil {
		return nil, fmt.Errorf("scenario: set stage to scenario: %w", err)
	}

	return scenario, nil
}

// ApproveScenario is a no-op in the new stage model — scenario approval
// no longer changes the project stage. Returns the current project.
func (ss *ScenarioService) ApproveScenario(ctx context.Context, projectID string) (*domain.Project, error) {
	return ss.projectSvc.GetProject(ctx, projectID)
}

// LoadScenarioFromFile loads a ScenarioOutput from a JSON file.
func LoadScenarioFromFile(path string) (*domain.ScenarioOutput, error) {
	data, err := workspace.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("scenario: read file: %w", err)
	}
	var s domain.ScenarioOutput
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("scenario: unmarshal: %w", err)
	}
	return &s, nil
}

// generateScenarioInternal tries 4-stage pipeline first, falls back to legacy one-shot.
func (ss *ScenarioService) generateScenarioInternal(ctx context.Context, scpData *workspace.SCPData, workspacePath string) (*domain.ScenarioOutput, error) {
	// Clear previous stage checkpoints so templates changes take effect.
	// Without this, cached stages from previous runs would be reused even after prompt updates.
	stagesDir := filepath.Join(workspacePath, "stages")
	_ = os.RemoveAll(stagesDir)

	// 4-stage pipeline is the ONLY scenario generation path
	if ss.templatesDir == "" {
		return nil, fmt.Errorf("scenario: templates_path is not configured — cannot generate scenario. Set templates_path in config or YTP_TEMPLATES_PATH env var")
	}

	pipe, pipeErr := NewScenarioPipeline(ss.llm, ss.glossary, ScenarioPipelineConfig{
		TemplatesDir: ss.templatesDir,
	})
	if pipeErr != nil {
		return nil, fmt.Errorf("scenario: init 4-stage pipeline: %w", pipeErr)
	}

	slog.Info("using 4-stage scenario pipeline", "templates_dir", ss.templatesDir)
	pipeResult, runErr := pipe.Run(ctx, scpData, workspacePath)
	if runErr != nil {
		return nil, fmt.Errorf("scenario: 4-stage pipeline: %w", runErr)
	}
	scenario := pipeResult.Scenario

	// Record pipeline mode in metadata
	if scenario.Metadata == nil {
		scenario.Metadata = make(map[string]any)
	}
	scenario.Metadata["pipeline_mode"] = "4-stage"
	scenario.Metadata["templates_dir"] = ss.templatesDir
	scenario.Metadata["format_guide"] = "applied"
	scenario.Metadata["attempts"] = fmt.Sprintf("%d", pipeResult.Attempts)

	// Save output files
	scenarioJSON, _ := json.MarshalIndent(scenario, "", "  ")
	_ = workspace.WriteFileAtomic(filepath.Join(workspacePath, "scenario.json"), scenarioJSON)
	md := RenderScenarioMarkdown(scenario)
	_ = workspace.WriteFileAtomic(filepath.Join(workspacePath, "scenario.md"), []byte(md))

	return scenario, nil
}

// RenderScenarioMarkdown renders a scenario to human-readable markdown.
func RenderScenarioMarkdown(s *domain.ScenarioOutput) string {
	md := fmt.Sprintf("# %s\n\n**SCP ID:** %s\n\n", s.Title, s.SCPID)

	for _, scene := range s.Scenes {
		md += fmt.Sprintf("## Scene %d\n\n", scene.SceneNum)
		md += fmt.Sprintf("**Mood:** %s\n\n", scene.Mood)
		md += fmt.Sprintf("### Narration\n\n%s\n\n", scene.Narration)
		md += fmt.Sprintf("### Visual Description\n\n%s\n\n", scene.VisualDescription)

		if len(scene.FactTags) > 0 {
			md += "### Fact References\n\n"
			for _, ft := range scene.FactTags {
				md += fmt.Sprintf("- **[%s]** %s\n", ft.Key, ft.Content)
			}
			md += "\n"
		}
	}

	return md
}
