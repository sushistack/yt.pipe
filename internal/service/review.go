package service

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strconv"
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
	domain.StatusAssembling:     true,
	domain.StatusComplete:       true,
	domain.StatusGeneratingAssets: true,
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
			Allowed:   []string{domain.StatusScenarioReview, domain.StatusApproved, domain.StatusImageReview, domain.StatusTTSReview, domain.StatusAssembling, domain.StatusComplete},
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
	if narration == "" {
		return 0, &domain.ValidationError{Field: "narration", Message: "must not be empty"}
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

// InsertScene inserts a new scene after afterSceneNum with atomic renumbering.
// afterSceneNum=0 inserts at the beginning.
func (svc *ReviewService) InsertScene(projectID string, afterSceneNum int, narration string) (int, error) {
	// Validate input (same rules as UpdateNarration)
	if strings.ContainsRune(narration, 0) {
		return 0, &domain.ValidationError{Field: "narration", Message: "must not contain null bytes"}
	}
	if len(narration) > 10000 {
		return 0, &domain.ValidationError{Field: "narration", Message: "must not exceed 10000 characters"}
	}
	if narration == "" {
		return 0, &domain.ValidationError{Field: "narration", Message: "must not be empty"}
	}

	project, err := svc.ValidateMutationState(projectID)
	if err != nil {
		return 0, err
	}

	mu := svc.getProjectLock(projectID)
	mu.Lock()
	defer mu.Unlock()

	// Check no running generation jobs
	for _, jobType := range []string{"image_generate", "tts_generate"} {
		if job, err := svc.store.GetRunningJobByProjectAndType(projectID, jobType); err == nil && job != nil {
			return 0, &domain.ConflictError{Message: fmt.Sprintf("cannot insert scene while %s job is running", jobType)}
		}
	}

	// Validate afterSceneNum range
	maxNum, _ := svc.store.MaxSceneNum(projectID)
	if project.SceneCount > maxNum {
		maxNum = project.SceneCount
	}
	if afterSceneNum < 0 || afterSceneNum > maxNum {
		return 0, &domain.ValidationError{Field: "after", Message: fmt.Sprintf("must be between 0 and %d", maxNum)}
	}

	newSceneNum := afterSceneNum + 1
	scenesDir := filepath.Join(project.WorkspacePath, "scenes")

	// Phase 1: Filesystem renumbering (temp-rename pattern)
	// Find actually existing scene dirs > afterSceneNum
	entries, err := os.ReadDir(scenesDir)
	if err != nil && !os.IsNotExist(err) {
		return 0, fmt.Errorf("service: read scenes dir: %w", err)
	}

	var toShift []shiftEntry
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		num, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}
		if num > afterSceneNum {
			toShift = append(toShift, shiftEntry{num: num, name: e.Name()})
		}
	}

	// Sort descending to prevent collisions
	sort.Slice(toShift, func(i, j int) bool { return toShift[i].num > toShift[j].num })

	// Phase 1a: rename to temp names
	var renamedToTemp []shiftEntry
	for _, d := range toShift {
		src := filepath.Join(scenesDir, d.name)
		tmp := filepath.Join(scenesDir, fmt.Sprintf("%d_shift_tmp", d.num))
		if err := os.Rename(src, tmp); err != nil {
			// Rollback temp renames
			for _, rd := range renamedToTemp {
				tmpPath := filepath.Join(scenesDir, fmt.Sprintf("%d_shift_tmp", rd.num))
				origPath := filepath.Join(scenesDir, rd.name)
				_ = os.Rename(tmpPath, origPath)
			}
			return 0, fmt.Errorf("service: rename scene %d to temp: %w", d.num, err)
		}
		renamedToTemp = append(renamedToTemp, d)
	}

	// Phase 1b: rename temp to final (N+1)
	var renamedToFinal []shiftEntry
	for _, d := range toShift {
		tmp := filepath.Join(scenesDir, fmt.Sprintf("%d_shift_tmp", d.num))
		dst := filepath.Join(scenesDir, fmt.Sprintf("%d", d.num+1))
		if err := os.Rename(tmp, dst); err != nil {
			// Rollback: final→temp→original
			for _, rd := range renamedToFinal {
				dstPath := filepath.Join(scenesDir, fmt.Sprintf("%d", rd.num+1))
				tmpPath := filepath.Join(scenesDir, fmt.Sprintf("%d_shift_tmp", rd.num))
				_ = os.Rename(dstPath, tmpPath)
			}
			// Then temp→original
			for _, rd := range renamedToTemp {
				tmpPath := filepath.Join(scenesDir, fmt.Sprintf("%d_shift_tmp", rd.num))
				origPath := filepath.Join(scenesDir, rd.name)
				_ = os.Rename(tmpPath, origPath)
			}
			return 0, fmt.Errorf("service: rename scene temp to %d: %w", d.num+1, err)
		}
		renamedToFinal = append(renamedToFinal, d)
	}

	// Phase 2: DB renumbering (single transaction)
	tx, err := svc.store.DB().Begin()
	if err != nil {
		svc.rollbackFilesystemRenames(scenesDir, toShift)
		return 0, fmt.Errorf("service: begin tx: %w", err)
	}

	if err := store.RenumberSceneApprovalsTx(tx, projectID, afterSceneNum, 1); err != nil {
		tx.Rollback()
		svc.rollbackFilesystemRenames(scenesDir, toShift)
		return 0, fmt.Errorf("service: renumber approvals: %w", err)
	}
	if err := store.RenumberSceneManifestsTx(tx, projectID, afterSceneNum, 1); err != nil {
		tx.Rollback()
		svc.rollbackFilesystemRenames(scenesDir, toShift)
		return 0, fmt.Errorf("service: renumber manifests: %w", err)
	}
	if err := store.RenumberSceneBGMTx(tx, projectID, afterSceneNum, 1); err != nil {
		tx.Rollback()
		svc.rollbackFilesystemRenames(scenesDir, toShift)
		return 0, fmt.Errorf("service: renumber bgm: %w", err)
	}
	if err := store.RenumberSceneMoodsTx(tx, projectID, afterSceneNum, 1); err != nil {
		tx.Rollback()
		svc.rollbackFilesystemRenames(scenesDir, toShift)
		return 0, fmt.Errorf("service: renumber moods: %w", err)
	}

	if err := tx.Commit(); err != nil {
		svc.rollbackFilesystemRenames(scenesDir, toShift)
		return 0, fmt.Errorf("service: commit tx: %w", err)
	}

	// Phase 3: Update scenario.json
	scenarioPath := filepath.Join(project.WorkspacePath, "scenario.json")
	scenario, err := LoadScenarioFromFile(scenarioPath)
	if err != nil {
		scenario = &domain.ScenarioOutput{Scenes: []domain.SceneScript{}}
	}
	// Shift existing scene numbers
	for i := range scenario.Scenes {
		if scenario.Scenes[i].SceneNum > afterSceneNum {
			scenario.Scenes[i].SceneNum++
		}
	}
	// Append new scene
	scenario.Scenes = append(scenario.Scenes, domain.SceneScript{
		SceneNum:  newSceneNum,
		Narration: narration,
	})
	// Sort by scene number
	sort.Slice(scenario.Scenes, func(i, j int) bool {
		return scenario.Scenes[i].SceneNum < scenario.Scenes[j].SceneNum
	})
	data, err := json.MarshalIndent(scenario, "", "  ")
	if err != nil {
		return 0, fmt.Errorf("service: marshal scenario after insert: %w", err)
	}
	if err := workspace.WriteFileAtomic(scenarioPath, data); err != nil {
		return 0, fmt.Errorf("service: write scenario after insert: %w", err)
	}

	// Create new scene directory
	if _, err := workspace.InitSceneDir(project.WorkspacePath, newSceneNum); err != nil {
		return 0, fmt.Errorf("service: create scene dir: %w", err)
	}

	// Init approval records for the new scene
	for _, assetType := range []string{domain.AssetTypeImage, domain.AssetTypeTTS} {
		if err := svc.store.InitApproval(projectID, newSceneNum, assetType); err != nil {
			return 0, fmt.Errorf("service: init approval: %w", err)
		}
	}

	// Update project scene count
	remaining := 0
	if entries, err := os.ReadDir(scenesDir); err == nil {
		for _, e := range entries {
			if e.IsDir() {
				if _, err := strconv.Atoi(e.Name()); err == nil {
					remaining++
				}
			}
		}
	}
	project.SceneCount = remaining
	if err := svc.store.UpdateProject(project); err != nil {
		return 0, fmt.Errorf("service: update project scene count: %w", err)
	}

	svc.logger.Info("scene inserted",
		"project_id", projectID,
		"after_scene_num", afterSceneNum,
		"new_scene_num", newSceneNum)
	return newSceneNum, nil
}

type shiftEntry struct {
	num  int
	name string
}

// rollbackFilesystemRenames reverses filesystem renaming: N+1 → N for all shifted dirs.
func (svc *ReviewService) rollbackFilesystemRenames(scenesDir string, shifted []shiftEntry) {
	// shifted is sorted descending; reverse back: N+1 → N
	for i := len(shifted) - 1; i >= 0; i-- {
		d := shifted[i]
		src := filepath.Join(scenesDir, fmt.Sprintf("%d", d.num+1))
		dst := filepath.Join(scenesDir, d.name)
		_ = os.Rename(src, dst)
	}
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

	// Delete BGM assignment record (cleanup orphans)
	if err := svc.store.DeleteSceneBGM(projectID, sceneNum); err != nil {
		return fmt.Errorf("service: delete scene bgm: %w", err)
	}

	// Delete mood assignment record (cleanup orphans)
	if err := svc.store.DeleteSceneMoodAssignment(projectID, sceneNum); err != nil {
		// Ignore not-found errors — mood assignment may not exist
		if _, ok := err.(*domain.NotFoundError); !ok {
			return fmt.Errorf("service: delete scene mood: %w", err)
		}
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

	// Update project scene count (count remaining numeric scene dirs only)
	remaining := 0
	scenesDir := filepath.Join(project.WorkspacePath, "scenes")
	if entries, err := os.ReadDir(scenesDir); err == nil {
		for _, e := range entries {
			if e.IsDir() {
				if _, err := strconv.Atoi(e.Name()); err == nil {
					remaining++
				}
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
