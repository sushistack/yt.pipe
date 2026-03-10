package service

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/sushistack/yt.pipe/internal/domain"
	"github.com/sushistack/yt.pipe/internal/store"
	"github.com/sushistack/yt.pipe/internal/workspace"
)

// ReviewService handles review dashboard mutations: narration editing,
// scene add/delete, and reject-with-regenerate.
type ReviewService struct {
	store        *store.Store
	logger       *slog.Logger
	projectLocks sync.Map // projectID → *sync.Mutex
}

// NewReviewService creates a new ReviewService.
func NewReviewService(s *store.Store, logger *slog.Logger) *ReviewService {
	return &ReviewService{store: s, logger: logger}
}

func (svc *ReviewService) getProjectLock(projectID string) *sync.Mutex {
	val, _ := svc.projectLocks.LoadOrStore(projectID, &sync.Mutex{})
	return val.(*sync.Mutex)
}

// allowedMutationStates defines project states that allow review mutations.
var allowedMutationStates = map[string]bool{
	domain.StatusScenarioReview: true,
	domain.StatusApproved:       true,
	domain.StatusImageReview:    true,
	domain.StatusTTSReview:      true,
}

// ValidateMutationState checks that the project is in a state that allows mutations.
func (svc *ReviewService) ValidateMutationState(projectID string) (*domain.Project, error) {
	project, err := svc.store.GetProject(projectID)
	if err != nil {
		return nil, err
	}
	if !allowedMutationStates[project.Status] {
		return nil, &domain.TransitionError{
			Current:   project.Status,
			Requested: "mutation",
			Allowed:   []string{domain.StatusScenarioReview, domain.StatusApproved, domain.StatusImageReview, domain.StatusTTSReview},
		}
	}
	return project, nil
}

// UpdateNarration updates the narration text for a specific scene in scenario.json.
func (svc *ReviewService) UpdateNarration(projectID string, sceneNum int, text string) error {
	// Validate input
	if strings.ContainsRune(text, 0) {
		return &domain.ValidationError{Field: "narration", Message: "must not contain null bytes"}
	}
	if len(text) > 10000 {
		return &domain.ValidationError{Field: "narration", Message: "must not exceed 10000 characters"}
	}
	if text == "" {
		return &domain.ValidationError{Field: "narration", Message: "must not be empty"}
	}

	project, err := svc.ValidateMutationState(projectID)
	if err != nil {
		return err
	}

	mu := svc.getProjectLock(projectID)
	mu.Lock()
	defer mu.Unlock()

	scenarioPath := filepath.Join(project.WorkspacePath, "scenario.json")

	// Backup scenario.json before modification
	if data, err := os.ReadFile(scenarioPath); err == nil {
		bakPath := scenarioPath + ".bak"
		_ = os.WriteFile(bakPath, data, 0o644)
	}

	scenario, err := LoadScenarioFromFile(scenarioPath)
	if err != nil {
		return fmt.Errorf("service: load scenario: %w", err)
	}

	found := false
	for i := range scenario.Scenes {
		if scenario.Scenes[i].SceneNum == sceneNum {
			scenario.Scenes[i].Narration = text
			found = true
			break
		}
	}
	if !found {
		return &domain.NotFoundError{Resource: "scene", ID: fmt.Sprintf("%d", sceneNum)}
	}

	data, err := json.MarshalIndent(scenario, "", "  ")
	if err != nil {
		return fmt.Errorf("service: marshal scenario: %w", err)
	}

	if err := workspace.WriteFileAtomic(scenarioPath, data); err != nil {
		return fmt.Errorf("service: write scenario: %w", err)
	}

	svc.logger.Info("narration updated",
		"project_id", projectID,
		"scene_num", sceneNum)
	return nil
}

// AddScene appends a new scene to the project.
func (svc *ReviewService) AddScene(projectID string, narration string) (int, error) {
	// Validate input
	if strings.ContainsRune(narration, 0) {
		return 0, &domain.ValidationError{Field: "narration", Message: "must not contain null bytes"}
	}
	if len(narration) > 10000 {
		return 0, &domain.ValidationError{Field: "narration", Message: "must not exceed 10000 characters"}
	}

	project, err := svc.ValidateMutationState(projectID)
	if err != nil {
		return 0, err
	}

	mu := svc.getProjectLock(projectID)
	mu.Lock()
	defer mu.Unlock()

	// Determine next scene number from both approvals and scenario
	maxNum, _ := svc.store.MaxSceneNum(projectID)
	if project.SceneCount > maxNum {
		maxNum = project.SceneCount
	}
	newSceneNum := maxNum + 1

	// Create scene directory
	if _, err := workspace.InitSceneDir(project.WorkspacePath, newSceneNum); err != nil {
		return 0, fmt.Errorf("service: create scene dir: %w", err)
	}

	// Update scenario.json
	scenarioPath := filepath.Join(project.WorkspacePath, "scenario.json")
	scenario, err := LoadScenarioFromFile(scenarioPath)
	if err != nil {
		// If scenario doesn't exist yet, create a minimal one
		scenario = &domain.ScenarioOutput{Scenes: []domain.SceneScript{}}
	}
	scenario.Scenes = append(scenario.Scenes, domain.SceneScript{
		SceneNum:  newSceneNum,
		Narration: narration,
	})
	data, err := json.MarshalIndent(scenario, "", "  ")
	if err != nil {
		return 0, fmt.Errorf("service: marshal scenario: %w", err)
	}
	if err := workspace.WriteFileAtomic(scenarioPath, data); err != nil {
		return 0, fmt.Errorf("service: write scenario: %w", err)
	}

	// Init approval records for the new scene
	for _, assetType := range []string{domain.AssetTypeImage, domain.AssetTypeTTS} {
		if err := svc.store.InitApproval(projectID, newSceneNum, assetType); err != nil {
			return 0, fmt.Errorf("service: init approval: %w", err)
		}
	}

	// Update project scene count
	project.SceneCount = newSceneNum
	if err := svc.store.UpdateProject(project); err != nil {
		return 0, fmt.Errorf("service: update project scene count: %w", err)
	}

	svc.logger.Info("scene added",
		"project_id", projectID,
		"scene_num", newSceneNum)
	return newSceneNum, nil
}

// DeleteScene removes a scene from the project without reindexing.
func (svc *ReviewService) DeleteScene(projectID string, sceneNum int) error {
	project, err := svc.ValidateMutationState(projectID)
	if err != nil {
		return err
	}

	mu := svc.getProjectLock(projectID)
	mu.Lock()
	defer mu.Unlock()

	// Delete approval records
	if err := svc.store.DeleteSceneApprovals(projectID, sceneNum); err != nil {
		return fmt.Errorf("service: delete approvals: %w", err)
	}

	// Delete manifest record
	if err := svc.store.DeleteSceneManifest(projectID, sceneNum); err != nil {
		return fmt.Errorf("service: delete manifest: %w", err)
	}

	// Remove scene from scenario.json
	scenarioPath := filepath.Join(project.WorkspacePath, "scenario.json")
	if scenario, err := LoadScenarioFromFile(scenarioPath); err == nil {
		filtered := make([]domain.SceneScript, 0, len(scenario.Scenes))
		for _, sc := range scenario.Scenes {
			if sc.SceneNum != sceneNum {
				filtered = append(filtered, sc)
			}
		}
		scenario.Scenes = filtered
		if data, err := json.MarshalIndent(scenario, "", "  "); err == nil {
			_ = workspace.WriteFileAtomic(scenarioPath, data)
		}
	}

	// Remove scene directory
	sceneDir := filepath.Join(project.WorkspacePath, "scenes", fmt.Sprintf("%d", sceneNum))
	_ = os.RemoveAll(sceneDir)

	// Update project scene count (count remaining scenes)
	remaining := 0
	scenesDir := filepath.Join(project.WorkspacePath, "scenes")
	if entries, err := os.ReadDir(scenesDir); err == nil {
		for _, e := range entries {
			if e.IsDir() {
				remaining++
			}
		}
	}
	project.SceneCount = remaining
	_ = svc.store.UpdateProject(project)

	svc.logger.Info("scene deleted",
		"project_id", projectID,
		"scene_num", sceneNum)
	return nil
}
