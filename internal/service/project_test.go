package service

import (
	"context"
	"testing"

	"github.com/sushistack/yt.pipe/internal/domain"
	"github.com/sushistack/yt.pipe/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestService(t *testing.T) *ProjectService {
	t.Helper()
	s, err := store.New(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { s.Close() })
	return NewProjectService(s)
}

func TestCreateProject_Success(t *testing.T) {
	svc := setupTestService(t)
	ctx := context.Background()

	p, err := svc.CreateProject(ctx, "SCP-173", "/tmp/workspace/scp-173")
	require.NoError(t, err)
	assert.NotEmpty(t, p.ID)
	assert.Equal(t, "SCP-173", p.SCPID)
	assert.Equal(t, domain.StatusPending, p.Status)
	assert.Equal(t, "/tmp/workspace/scp-173", p.WorkspacePath)
	assert.False(t, p.CreatedAt.IsZero())
}

func TestCreateProject_EmptySCPID(t *testing.T) {
	svc := setupTestService(t)
	ctx := context.Background()

	_, err := svc.CreateProject(ctx, "", "/tmp/workspace")
	require.Error(t, err)
	var ve *domain.ValidationError
	assert.ErrorAs(t, err, &ve)
	assert.Equal(t, "scp_id", ve.Field)
}

func TestCreateProject_EmptyWorkspacePath(t *testing.T) {
	svc := setupTestService(t)
	ctx := context.Background()

	_, err := svc.CreateProject(ctx, "SCP-173", "")
	require.Error(t, err)
	var ve *domain.ValidationError
	assert.ErrorAs(t, err, &ve)
	assert.Equal(t, "workspace_path", ve.Field)
}

func TestGetProject_Success(t *testing.T) {
	svc := setupTestService(t)
	ctx := context.Background()

	created, err := svc.CreateProject(ctx, "SCP-173", "/tmp/ws")
	require.NoError(t, err)

	got, err := svc.GetProject(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, got.ID)
	assert.Equal(t, "SCP-173", got.SCPID)
}

func TestGetProject_NotFound(t *testing.T) {
	svc := setupTestService(t)
	ctx := context.Background()

	_, err := svc.GetProject(ctx, "nonexistent")
	require.Error(t, err)
	var nfe *domain.NotFoundError
	assert.ErrorAs(t, err, &nfe)
}

func TestListProjects(t *testing.T) {
	svc := setupTestService(t)
	ctx := context.Background()

	_, err := svc.CreateProject(ctx, "SCP-173", "/tmp/ws1")
	require.NoError(t, err)
	_, err = svc.CreateProject(ctx, "SCP-096", "/tmp/ws2")
	require.NoError(t, err)

	list, err := svc.ListProjects(ctx)
	require.NoError(t, err)
	assert.Len(t, list, 2)
}

func TestTransitionProject_ValidFullLifecycle(t *testing.T) {
	svc := setupTestService(t)
	ctx := context.Background()

	p, err := svc.CreateProject(ctx, "SCP-173", "/tmp/ws")
	require.NoError(t, err)
	assert.Equal(t, domain.StatusPending, p.Status)

	transitions := []string{
		domain.StatusScenarioReview,
		domain.StatusApproved,
		domain.StatusGeneratingAssets,
		domain.StatusAssembling,
		domain.StatusComplete,
	}

	for _, next := range transitions {
		p, err = svc.TransitionProject(ctx, p.ID, next)
		require.NoError(t, err, "transition to %s failed", next)
		assert.Equal(t, next, p.Status)
	}
}

func TestTransitionProject_InvalidTransition(t *testing.T) {
	svc := setupTestService(t)
	ctx := context.Background()

	p, err := svc.CreateProject(ctx, "SCP-173", "/tmp/ws")
	require.NoError(t, err)

	// pending → complete is not allowed
	_, err = svc.TransitionProject(ctx, p.ID, domain.StatusComplete)
	require.Error(t, err)
	var te *domain.TransitionError
	assert.ErrorAs(t, err, &te)
	assert.Equal(t, domain.StatusPending, te.Current)
	assert.Equal(t, domain.StatusComplete, te.Requested)
}

func TestTransitionProject_NotFound(t *testing.T) {
	svc := setupTestService(t)
	ctx := context.Background()

	_, err := svc.TransitionProject(ctx, "nonexistent", domain.StatusScenarioReview)
	require.Error(t, err)
	var nfe *domain.NotFoundError
	assert.ErrorAs(t, err, &nfe)
}

func TestTransitionProject_RecordsExecutionLog(t *testing.T) {
	svc := setupTestService(t)
	ctx := context.Background()

	p, err := svc.CreateProject(ctx, "SCP-173", "/tmp/ws")
	require.NoError(t, err)

	_, err = svc.TransitionProject(ctx, p.ID, domain.StatusScenarioReview)
	require.NoError(t, err)

	// Verify execution log was recorded
	logs, err := svc.store.ListExecutionLogsByProject(p.ID)
	require.NoError(t, err)
	require.Len(t, logs, 1)
	assert.Equal(t, "state_machine", logs[0].Stage)
	assert.Equal(t, "transition", logs[0].Action)
	assert.Contains(t, logs[0].Details, "pending -> scenario_review")
}

func TestTransitionProject_ScenarioReviewBackToPending(t *testing.T) {
	svc := setupTestService(t)
	ctx := context.Background()

	p, err := svc.CreateProject(ctx, "SCP-173", "/tmp/ws")
	require.NoError(t, err)

	p, err = svc.TransitionProject(ctx, p.ID, domain.StatusScenarioReview)
	require.NoError(t, err)

	// scenario_review → pending is allowed (rejection flow)
	p, err = svc.TransitionProject(ctx, p.ID, domain.StatusPending)
	require.NoError(t, err)
	assert.Equal(t, domain.StatusPending, p.Status)
}

func TestTransitionProject_ApprovalFlowLifecycle(t *testing.T) {
	svc := setupTestService(t)
	ctx := context.Background()

	p, err := svc.CreateProject(ctx, "SCP-173", "/tmp/ws")
	require.NoError(t, err)

	// New approval flow: pending → scenario_review → approved → image_review → tts_review → assembling → complete
	transitions := []string{
		domain.StatusScenarioReview,
		domain.StatusApproved,
		domain.StatusImageReview,
		domain.StatusTTSReview,
		domain.StatusAssembling,
		domain.StatusComplete,
	}

	for _, next := range transitions {
		p, err = svc.TransitionProject(ctx, p.ID, next)
		require.NoError(t, err, "transition to %s failed", next)
		assert.Equal(t, next, p.Status)
	}
}

func TestTransitionProject_CompleteHasNoTransitions(t *testing.T) {
	svc := setupTestService(t)
	ctx := context.Background()

	p, err := svc.CreateProject(ctx, "SCP-173", "/tmp/ws")
	require.NoError(t, err)

	// Walk through all states to complete
	for _, s := range []string{domain.StatusScenarioReview, domain.StatusApproved, domain.StatusGeneratingAssets, domain.StatusAssembling, domain.StatusComplete} {
		p, err = svc.TransitionProject(ctx, p.ID, s)
		require.NoError(t, err)
	}

	// complete → anything is not allowed
	_, err = svc.TransitionProject(ctx, p.ID, domain.StatusPending)
	require.Error(t, err)
	var te *domain.TransitionError
	assert.ErrorAs(t, err, &te)
}
