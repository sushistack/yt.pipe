package service

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/sushistack/yt.pipe/internal/domain"
	"github.com/sushistack/yt.pipe/internal/plugin/llm"
	"github.com/sushistack/yt.pipe/internal/store"
	"github.com/sushistack/yt.pipe/internal/workspace"
)

// ScenarioService handles scenario generation and management.
type ScenarioService struct {
	store      *store.Store
	llm        llm.LLM
	projectSvc *ProjectService
}

// NewScenarioService creates a new ScenarioService.
func NewScenarioService(s *store.Store, l llm.LLM, ps *ProjectService) *ScenarioService {
	return &ScenarioService{store: s, llm: l, projectSvc: ps}
}

// GenerateScenario generates a scenario from SCP data and saves it to the project workspace.
// It creates a project, generates the scenario via LLM, saves output files, and transitions to scenario_review.
func (ss *ScenarioService) GenerateScenario(ctx context.Context, scpData *workspace.SCPData, workspacePath string) (*domain.ScenarioOutput, *domain.Project, error) {
	// Create project
	project, err := ss.projectSvc.CreateProject(ctx, scpData.SCPID, workspacePath)
	if err != nil {
		return nil, nil, fmt.Errorf("scenario: create project: %w", err)
	}

	// Convert facts to FactTag slice for LLM
	var factTags []domain.FactTag
	for k, v := range scpData.Facts.Facts {
		factTags = append(factTags, domain.FactTag{Key: k, Content: v})
	}

	// Build metadata from meta.json
	metadata := map[string]string{
		"title":        scpData.Meta.Title,
		"object_class": scpData.Meta.ObjectClass,
		"series":       scpData.Meta.Series,
	}

	// Generate scenario via LLM plugin
	scenario, err := ss.llm.GenerateScenario(ctx, scpData.SCPID, scpData.MainText, factTags, metadata)
	if err != nil {
		return nil, project, fmt.Errorf("scenario: llm generation: %w", err)
	}

	// Save scenario JSON
	scenarioJSON, err := json.MarshalIndent(scenario, "", "  ")
	if err != nil {
		return nil, project, fmt.Errorf("scenario: marshal output: %w", err)
	}
	scenarioPath := filepath.Join(workspacePath, "scenario.json")
	if err := workspace.WriteFileAtomic(scenarioPath, scenarioJSON); err != nil {
		return nil, project, fmt.Errorf("scenario: save json: %w", err)
	}

	// Save scenario markdown for human review
	md := renderScenarioMarkdown(scenario)
	mdPath := filepath.Join(workspacePath, "scenario.md")
	if err := workspace.WriteFileAtomic(mdPath, []byte(md)); err != nil {
		return nil, project, fmt.Errorf("scenario: save markdown: %w", err)
	}

	// Update scene count
	project.SceneCount = len(scenario.Scenes)
	if err := ss.store.UpdateProject(project); err != nil {
		return nil, project, fmt.Errorf("scenario: update scene count: %w", err)
	}

	// Transition to scenario_review
	project, err = ss.projectSvc.TransitionProject(ctx, project.ID, domain.StatusScenarioReview)
	if err != nil {
		return nil, project, fmt.Errorf("scenario: transition to review: %w", err)
	}

	return scenario, project, nil
}

// RegenerateSection regenerates a single scene using the LLM plugin.
func (ss *ScenarioService) RegenerateSection(ctx context.Context, projectID string, sceneNum int, instruction string) (*domain.SceneScript, error) {
	project, err := ss.projectSvc.GetProject(ctx, projectID)
	if err != nil {
		return nil, err
	}

	if project.Status != domain.StatusScenarioReview {
		return nil, &domain.TransitionError{
			Current:   project.Status,
			Requested: "regenerate_section",
			Allowed:   []string{domain.StatusScenarioReview},
		}
	}

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
	md := renderScenarioMarkdown(scenario)
	mdPath := filepath.Join(project.WorkspacePath, "scenario.md")
	if err := workspace.WriteFileAtomic(mdPath, []byte(md)); err != nil {
		return nil, fmt.Errorf("scenario: save updated markdown: %w", err)
	}

	return newScene, nil
}

// ApproveScenario transitions a project from scenario_review to approved.
func (ss *ScenarioService) ApproveScenario(ctx context.Context, projectID string) (*domain.Project, error) {
	return ss.projectSvc.TransitionProject(ctx, projectID, domain.StatusApproved)
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

func renderScenarioMarkdown(s *domain.ScenarioOutput) string {
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
