package domain

import (
	"fmt"
	"time"
)

// Scene approval asset type constants
const (
	AssetTypeImage = "image"
	AssetTypeTTS   = "tts"
)

// Scene approval status constants
const (
	ApprovalPending   = "pending"
	ApprovalGenerated = "generated"
	ApprovalApproved  = "approved"
	ApprovalRejected  = "rejected"
)

// SceneApproval tracks per-scene approval status for image and TTS assets.
type SceneApproval struct {
	ProjectID string    `json:"project_id"`
	SceneNum  int       `json:"scene_num"`
	AssetType string    `json:"asset_type"` // "image" or "tts"
	Status    string    `json:"status"`     // "pending", "generated", "approved", "rejected"
	Attempts  int       `json:"attempts"`
	UpdatedAt time.Time `json:"updated_at"`
}

// allowedApprovalTransitions defines valid status transitions for scene approvals.
// pending → generated → approved
// generated → rejected → generated (retry cycle)
var allowedApprovalTransitions = map[string][]string{
	ApprovalPending:   {ApprovalGenerated},
	ApprovalGenerated: {ApprovalApproved, ApprovalRejected},
	ApprovalRejected:  {ApprovalGenerated},
	ApprovalApproved:  {},
}

// CanApprovalTransition checks if a scene approval status transition is valid.
func CanApprovalTransition(current, requested string) bool {
	allowed, ok := allowedApprovalTransitions[current]
	if !ok {
		return false
	}
	for _, s := range allowed {
		if s == requested {
			return true
		}
	}
	return false
}

// ValidateSceneApproval validates the fields of a SceneApproval.
func ValidateSceneApproval(projectID string, sceneNum int, assetType string) error {
	if projectID == "" {
		return &ValidationError{Field: "project_id", Message: "must not be empty"}
	}
	if sceneNum < 1 {
		return &ValidationError{Field: "scene_num", Message: fmt.Sprintf("must be positive, got %d", sceneNum)}
	}
	if assetType != AssetTypeImage && assetType != AssetTypeTTS {
		return &ValidationError{Field: "asset_type", Message: fmt.Sprintf("must be %q or %q, got %q", AssetTypeImage, AssetTypeTTS, assetType)}
	}
	return nil
}
