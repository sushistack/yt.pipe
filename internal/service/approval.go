package service

import (
	"fmt"
	"log/slog"

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
