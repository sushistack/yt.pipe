package service

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"

	"github.com/sushistack/yt.pipe/internal/domain"
	"github.com/sushistack/yt.pipe/internal/store"
)

// ApprovalStatus summarizes the approval state for a project's asset type.
type ApprovalStatus struct {
	AssetType    string `json:"asset_type"`
	Total        int    `json:"total"`
	Pending      int    `json:"pending"`
	Generated    int    `json:"generated"`
	Approved     int    `json:"approved"`
	Rejected     int    `json:"rejected"`
	AllApproved  bool   `json:"all_approved"`
}

// ApprovalService orchestrates per-scene approval workflows.
type ApprovalService struct {
	store  *store.Store
	logger *slog.Logger
}

// NewApprovalService creates a new ApprovalService.
func NewApprovalService(s *store.Store, logger *slog.Logger) *ApprovalService {
	return &ApprovalService{store: s, logger: logger}
}

// InitApprovals initializes approval records for all scenes of a given asset type.
func (svc *ApprovalService) InitApprovals(projectID string, sceneCount int, assetType string) error {
	for i := 1; i <= sceneCount; i++ {
		if err := svc.store.InitApproval(projectID, i, assetType); err != nil {
			return fmt.Errorf("init approvals: scene %d: %w", i, err)
		}
	}
	svc.logger.Info("initialized approvals",
		"project_id", projectID,
		"asset_type", assetType,
		"scene_count", sceneCount)
	return nil
}

// MarkGenerated marks a scene as "generated" after asset generation.
// Validates that current status is "pending" or "rejected" (retry).
func (svc *ApprovalService) MarkGenerated(projectID string, sceneNum int, assetType string) error {
	current, err := svc.store.GetApproval(projectID, sceneNum, assetType)
	if err != nil {
		return err
	}
	if !domain.CanApprovalTransition(current.Status, domain.ApprovalGenerated) {
		return &domain.TransitionError{
			Current:   current.Status,
			Requested: domain.ApprovalGenerated,
			Allowed:   []string{domain.ApprovalPending, domain.ApprovalRejected},
		}
	}
	return svc.store.MarkGenerated(projectID, sceneNum, assetType)
}

// ApproveScene approves a scene's asset. Validates current status is "generated".
func (svc *ApprovalService) ApproveScene(projectID string, sceneNum int, assetType string) error {
	current, err := svc.store.GetApproval(projectID, sceneNum, assetType)
	if err != nil {
		return err
	}
	if !domain.CanApprovalTransition(current.Status, domain.ApprovalApproved) {
		return &domain.TransitionError{
			Current:   current.Status,
			Requested: domain.ApprovalApproved,
			Allowed:   []string{domain.ApprovalGenerated},
		}
	}
	return svc.store.ApproveScene(projectID, sceneNum, assetType)
}

// RejectScene rejects a scene's asset. Validates current status is "generated".
func (svc *ApprovalService) RejectScene(projectID string, sceneNum int, assetType string) error {
	current, err := svc.store.GetApproval(projectID, sceneNum, assetType)
	if err != nil {
		return err
	}
	if !domain.CanApprovalTransition(current.Status, domain.ApprovalRejected) {
		return &domain.TransitionError{
			Current:   current.Status,
			Requested: domain.ApprovalRejected,
			Allowed:   []string{domain.ApprovalGenerated},
		}
	}
	return svc.store.RejectScene(projectID, sceneNum, assetType)
}

// AutoApproveAll bulk-approves all scenes for an asset type.
// Used with --skip-approval flag.
func (svc *ApprovalService) AutoApproveAll(projectID, assetType string) error {
	affected, err := svc.store.BulkApproveAll(projectID, assetType)
	if err != nil {
		return err
	}
	svc.logger.Warn("skip-approval enabled: all scenes auto-approved",
		"project_id", projectID,
		"asset_type", assetType,
		"scenes_approved", affected)
	return nil
}

// AutoApproveByScore auto-approves scenes whose minimum validation score meets the threshold.
// Scenes with score >= threshold are approved; scenes with score < threshold or NULL score
// remain in "generated" status for manual review.
func (svc *ApprovalService) AutoApproveByScore(ctx context.Context, projectID, assetType string, threshold int) (autoApproved []int, reviewRequired []int, err error) {
	scores, err := svc.store.ListSceneValidationScores(projectID, assetType)
	if err != nil {
		return nil, nil, fmt.Errorf("auto approve by score: %w", err)
	}

	for _, svs := range scores {
		if svs.ValidationScore != nil && *svs.ValidationScore >= threshold {
			if err := svc.ApproveScene(projectID, svs.SceneNum, assetType); err != nil {
				return nil, nil, fmt.Errorf("auto approve scene %d: %w", svs.SceneNum, err)
			}
			svc.logger.Info("scene auto-approved by validation score",
				"project_id", projectID,
				"asset_type", assetType,
				"scene_num", svs.SceneNum,
				"validation_score", *svs.ValidationScore,
				"threshold", threshold)
			autoApproved = append(autoApproved, svs.SceneNum)
		} else {
			reviewRequired = append(reviewRequired, svs.SceneNum)
		}
	}

	svc.logger.Info("auto-approval complete",
		"project_id", projectID,
		"asset_type", assetType,
		"auto_approved", len(autoApproved),
		"review_required", len(reviewRequired),
		"threshold", threshold)
	return autoApproved, reviewRequired, nil
}

// AllApproved checks if all scenes for a given asset type are approved.
func (svc *ApprovalService) AllApproved(projectID, assetType string) (bool, error) {
	return svc.store.AllApproved(projectID, assetType)
}

// GetApprovalStatus returns a summary of approval states for a project's asset type.
func (svc *ApprovalService) GetApprovalStatus(projectID, assetType string) (*ApprovalStatus, error) {
	approvals, err := svc.store.ListApprovalsByProject(projectID, assetType)
	if err != nil {
		return nil, err
	}

	status := &ApprovalStatus{AssetType: assetType, Total: len(approvals)}
	for _, a := range approvals {
		switch a.Status {
		case domain.ApprovalPending:
			status.Pending++
		case domain.ApprovalGenerated:
			status.Generated++
		case domain.ApprovalApproved:
			status.Approved++
		case domain.ApprovalRejected:
			status.Rejected++
		}
	}
	status.AllApproved = status.Total > 0 && status.Approved == status.Total
	return status, nil
}

// BatchPreviewItem contains preview data for a single scene in batch review.
type BatchPreviewItem struct {
	SceneNum        int    `json:"scene_num"`
	ImagePath       string `json:"image_path"`
	NarrationFirst  string `json:"narration_first"`
	Mood            string `json:"mood"`
	ValidationScore *int   `json:"validation_score"`
	Status          string `json:"status"`
}

// GetBatchPreview returns preview data for all scenes in a project, ordered by scene number.
// Combines approval status, scenario data (narration/mood), image paths, and validation scores.
func (svc *ApprovalService) GetBatchPreview(ctx context.Context, projectID, assetType string) ([]BatchPreviewItem, error) {
	// 1. Get project for workspace path
	project, err := svc.store.GetProject(projectID)
	if err != nil {
		return nil, fmt.Errorf("batch preview: get project: %w", err)
	}

	// 2. Load approvals
	approvals, err := svc.store.ListApprovalsByProject(projectID, assetType)
	if err != nil {
		return nil, fmt.Errorf("batch preview: list approvals: %w", err)
	}

	// 3. Load scenario for narration + mood (gracefully handle missing)
	scenarioPath := filepath.Join(project.WorkspacePath, "scenario.json")
	scenario, _ := LoadScenarioFromFile(scenarioPath)
	sceneMeta := make(map[int]*domain.SceneScript)
	if scenario != nil {
		for i := range scenario.Scenes {
			sceneMeta[scenario.Scenes[i].SceneNum] = &scenario.Scenes[i]
		}
	}

	// 4. Load validation scores for all scenes
	scoreMap := make(map[int]*int)
	scores, _ := svc.store.ListAllSceneValidationScores(projectID, assetType)
	for _, svs := range scores {
		svs := svs
		scoreMap[svs.SceneNum] = svs.ValidationScore
	}

	// 5. Build preview items
	items := make([]BatchPreviewItem, 0, len(approvals))
	for _, a := range approvals {
		item := BatchPreviewItem{
			SceneNum:  a.SceneNum,
			ImagePath: filepath.Join(project.WorkspacePath, "scenes", fmt.Sprintf("%d", a.SceneNum), "image.png"),
			Status:    a.Status,
		}

		// Narration + Mood from scenario
		if sc, ok := sceneMeta[a.SceneNum]; ok {
			item.Mood = sc.Mood
			sentences := domain.SplitNarrationSentences(sc.Narration)
			if len(sentences) > 0 {
				item.NarrationFirst = sentences[0]
			}
		}

		// Validation score
		if score, ok := scoreMap[a.SceneNum]; ok {
			item.ValidationScore = score
		}

		items = append(items, item)
	}

	return items, nil
}

// BatchApprovalResult summarizes the outcome of a batch approval operation.
type BatchApprovalResult struct {
	TotalScenes   int   `json:"total_scenes"`
	ApprovedCount int   `json:"approved_count"`
	FlaggedCount  int   `json:"flagged_count"`
	FlaggedScenes []int `json:"flagged_scenes"`
}

// BatchApprove approves all generated scenes except those in flaggedScenes.
// Flagged scenes remain in their current status for rework.
// Only scenes with "generated" status are eligible for approval.
func (svc *ApprovalService) BatchApprove(ctx context.Context, projectID, assetType string, flaggedScenes []int) (*BatchApprovalResult, error) {
	approvals, err := svc.store.ListApprovalsByProject(projectID, assetType)
	if err != nil {
		return nil, fmt.Errorf("batch approve: list approvals: %w", err)
	}

	// Build set of valid scene numbers
	validScenes := make(map[int]bool, len(approvals))
	for _, a := range approvals {
		validScenes[a.SceneNum] = true
	}

	// Validate flagged scenes
	for _, num := range flaggedScenes {
		if !validScenes[num] {
			return nil, fmt.Errorf("batch approve: scene %d does not exist (valid scenes: 1-%d)", num, len(approvals))
		}
	}

	flaggedSet := make(map[int]bool, len(flaggedScenes))
	for _, num := range flaggedScenes {
		flaggedSet[num] = true
	}

	result := &BatchApprovalResult{
		TotalScenes:   len(approvals),
		FlaggedScenes: flaggedScenes,
		FlaggedCount:  len(flaggedScenes),
	}

	for _, a := range approvals {
		if flaggedSet[a.SceneNum] {
			continue
		}
		if a.Status != domain.ApprovalGenerated {
			continue
		}
		if err := svc.ApproveScene(projectID, a.SceneNum, assetType); err != nil {
			return nil, fmt.Errorf("batch approve scene %d: %w", a.SceneNum, err)
		}
		result.ApprovedCount++
	}

	svc.logger.Info("batch approval complete",
		"project_id", projectID,
		"asset_type", assetType,
		"total_scenes", result.TotalScenes,
		"approved_count", result.ApprovedCount,
		"flagged_count", result.FlaggedCount)

	return result, nil
}
