package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateSceneApproval_Valid(t *testing.T) {
	assert.NoError(t, ValidateSceneApproval("proj-1", 1, AssetTypeImage))
	assert.NoError(t, ValidateSceneApproval("proj-1", 5, AssetTypeTTS))
}

func TestValidateSceneApproval_EmptyProjectID(t *testing.T) {
	err := ValidateSceneApproval("", 1, AssetTypeImage)
	assert.Error(t, err)
	assert.IsType(t, &ValidationError{}, err)
}

func TestValidateSceneApproval_InvalidSceneNum(t *testing.T) {
	err := ValidateSceneApproval("proj-1", 0, AssetTypeImage)
	assert.Error(t, err)
	assert.IsType(t, &ValidationError{}, err)
}

func TestValidateSceneApproval_InvalidAssetType(t *testing.T) {
	err := ValidateSceneApproval("proj-1", 1, "video")
	assert.Error(t, err)
	assert.IsType(t, &ValidationError{}, err)
}

func TestCanApprovalTransition(t *testing.T) {
	tests := []struct {
		name     string
		current  string
		next     string
		expected bool
	}{
		{"pending to generated", ApprovalPending, ApprovalGenerated, true},
		{"generated to approved", ApprovalGenerated, ApprovalApproved, true},
		{"generated to rejected", ApprovalGenerated, ApprovalRejected, true},
		{"rejected to generated", ApprovalRejected, ApprovalGenerated, true},
		// Invalid transitions
		{"pending to approved", ApprovalPending, ApprovalApproved, false},
		{"pending to rejected", ApprovalPending, ApprovalRejected, false},
		{"approved to anything", ApprovalApproved, ApprovalGenerated, false},
		{"rejected to approved", ApprovalRejected, ApprovalApproved, false},
		{"unknown status", "unknown", ApprovalGenerated, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, CanApprovalTransition(tt.current, tt.next))
		})
	}
}
