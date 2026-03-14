package service

import (
	"log/slog"
	"testing"

	"github.com/sushistack/yt.pipe/internal/domain"
	"github.com/sushistack/yt.pipe/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupApprovalService(t *testing.T) (*ApprovalService, *store.Store) {
	t.Helper()
	s, err := store.New(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { s.Close() })

	require.NoError(t, s.CreateProject(&domain.Project{
		ID: "p1", SCPID: "SCP-173", Status: domain.StagePending, WorkspacePath: "/w",
	}))

	logger := slog.Default()
	return NewApprovalService(s, logger), s
}

func TestInitApprovals_Success(t *testing.T) {
	svc, s := setupApprovalService(t)

	err := svc.InitApprovals("p1", 3, domain.AssetTypeImage)
	require.NoError(t, err)

	approvals, err := s.ListApprovalsByProject("p1", domain.AssetTypeImage)
	require.NoError(t, err)
	assert.Len(t, approvals, 3)
	for i, a := range approvals {
		assert.Equal(t, i+1, a.SceneNum)
		assert.Equal(t, domain.ApprovalPending, a.Status)
	}
}

func TestMarkGenerated_ValidTransition(t *testing.T) {
	svc, _ := setupApprovalService(t)
	require.NoError(t, svc.InitApprovals("p1", 1, domain.AssetTypeImage))

	err := svc.MarkGenerated("p1", 1, domain.AssetTypeImage)
	require.NoError(t, err)
}

func TestMarkGenerated_InvalidTransition(t *testing.T) {
	svc, s := setupApprovalService(t)
	require.NoError(t, svc.InitApprovals("p1", 1, domain.AssetTypeImage))

	// Mark generated then approved
	require.NoError(t, s.MarkGenerated("p1", 1, domain.AssetTypeImage))
	require.NoError(t, s.ApproveScene("p1", 1, domain.AssetTypeImage))

	// Cannot mark generated after approved
	err := svc.MarkGenerated("p1", 1, domain.AssetTypeImage)
	assert.Error(t, err)
	assert.IsType(t, &domain.TransitionError{}, err)
}

func TestApproveScene_ValidTransition(t *testing.T) {
	svc, _ := setupApprovalService(t)
	require.NoError(t, svc.InitApprovals("p1", 1, domain.AssetTypeTTS))
	require.NoError(t, svc.MarkGenerated("p1", 1, domain.AssetTypeTTS))

	err := svc.ApproveScene("p1", 1, domain.AssetTypeTTS)
	require.NoError(t, err)
}

func TestApproveScene_InvalidTransition(t *testing.T) {
	svc, _ := setupApprovalService(t)
	require.NoError(t, svc.InitApprovals("p1", 1, domain.AssetTypeImage))

	// Cannot approve from pending (must be generated first)
	err := svc.ApproveScene("p1", 1, domain.AssetTypeImage)
	assert.Error(t, err)
	assert.IsType(t, &domain.TransitionError{}, err)
}

func TestRejectScene_ValidTransition(t *testing.T) {
	svc, _ := setupApprovalService(t)
	require.NoError(t, svc.InitApprovals("p1", 1, domain.AssetTypeImage))
	require.NoError(t, svc.MarkGenerated("p1", 1, domain.AssetTypeImage))

	err := svc.RejectScene("p1", 1, domain.AssetTypeImage)
	require.NoError(t, err)
}

func TestRejectScene_InvalidTransition(t *testing.T) {
	svc, _ := setupApprovalService(t)
	require.NoError(t, svc.InitApprovals("p1", 1, domain.AssetTypeImage))

	// Cannot reject from pending
	err := svc.RejectScene("p1", 1, domain.AssetTypeImage)
	assert.Error(t, err)
	assert.IsType(t, &domain.TransitionError{}, err)
}

func TestRejectAndRegenerate_FullCycle(t *testing.T) {
	svc, _ := setupApprovalService(t)
	require.NoError(t, svc.InitApprovals("p1", 1, domain.AssetTypeImage))

	// Generate → reject → regenerate → approve
	require.NoError(t, svc.MarkGenerated("p1", 1, domain.AssetTypeImage))
	require.NoError(t, svc.RejectScene("p1", 1, domain.AssetTypeImage))
	require.NoError(t, svc.MarkGenerated("p1", 1, domain.AssetTypeImage))
	require.NoError(t, svc.ApproveScene("p1", 1, domain.AssetTypeImage))
}

func TestAutoApproveAll_Success(t *testing.T) {
	svc, _ := setupApprovalService(t)
	require.NoError(t, svc.InitApprovals("p1", 3, domain.AssetTypeImage))

	err := svc.AutoApproveAll("p1", domain.AssetTypeImage)
	require.NoError(t, err)

	ok, err := svc.AllApproved("p1", domain.AssetTypeImage)
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestAllApproved_PartialApproval(t *testing.T) {
	svc, _ := setupApprovalService(t)
	require.NoError(t, svc.InitApprovals("p1", 2, domain.AssetTypeImage))

	// Only approve scene 1
	require.NoError(t, svc.MarkGenerated("p1", 1, domain.AssetTypeImage))
	require.NoError(t, svc.ApproveScene("p1", 1, domain.AssetTypeImage))

	ok, err := svc.AllApproved("p1", domain.AssetTypeImage)
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestGetApprovalStatus_Summary(t *testing.T) {
	svc, _ := setupApprovalService(t)
	require.NoError(t, svc.InitApprovals("p1", 4, domain.AssetTypeImage))

	// Scene 1: approved
	require.NoError(t, svc.MarkGenerated("p1", 1, domain.AssetTypeImage))
	require.NoError(t, svc.ApproveScene("p1", 1, domain.AssetTypeImage))
	// Scene 2: generated (awaiting review)
	require.NoError(t, svc.MarkGenerated("p1", 2, domain.AssetTypeImage))
	// Scene 3: rejected
	require.NoError(t, svc.MarkGenerated("p1", 3, domain.AssetTypeImage))
	require.NoError(t, svc.RejectScene("p1", 3, domain.AssetTypeImage))
	// Scene 4: pending

	status, err := svc.GetApprovalStatus("p1", domain.AssetTypeImage)
	require.NoError(t, err)
	assert.Equal(t, 4, status.Total)
	assert.Equal(t, 1, status.Approved)
	assert.Equal(t, 1, status.Generated)
	assert.Equal(t, 1, status.Rejected)
	assert.Equal(t, 1, status.Pending)
	assert.False(t, status.AllApproved)
}
