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
	assert.Equal(t, domain.StagePending, p.Status)
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

func TestSetProjectStage_FullLifecycle(t *testing.T) {
	svc := setupTestService(t)
	ctx := context.Background()

	p, err := svc.CreateProject(ctx, "SCP-173", "/tmp/ws")
	require.NoError(t, err)
	assert.Equal(t, domain.StagePending, p.Status)

	stages := []string{
		domain.StageScenario,
		domain.StageImages,
		domain.StageTTS,
		domain.StageComplete,
	}

	for _, next := range stages {
		p, err = svc.SetProjectStage(ctx, p.ID, next)
		require.NoError(t, err, "set stage to %s failed", next)
		assert.Equal(t, next, p.Status)
	}
}

func TestSetProjectStage_NotFound(t *testing.T) {
	svc := setupTestService(t)
	ctx := context.Background()

	_, err := svc.SetProjectStage(ctx, "nonexistent", domain.StageScenario)
	require.Error(t, err)
	var nfe *domain.NotFoundError
	assert.ErrorAs(t, err, &nfe)
}

func TestSetProjectStage_RecordsExecutionLog(t *testing.T) {
	svc := setupTestService(t)
	ctx := context.Background()

	p, err := svc.CreateProject(ctx, "SCP-173", "/tmp/ws")
	require.NoError(t, err)

	_, err = svc.SetProjectStage(ctx, p.ID, domain.StageScenario)
	require.NoError(t, err)

	// Verify execution log was recorded
	logs, err := svc.store.ListExecutionLogsByProject(p.ID)
	require.NoError(t, err)
	require.Len(t, logs, 1)
	assert.Equal(t, "stage_change", logs[0].Stage)
	assert.Equal(t, "set_stage", logs[0].Action)
	assert.Contains(t, logs[0].Details, "pending -> scenario")
}

func TestSetProjectStage_BackToPending(t *testing.T) {
	svc := setupTestService(t)
	ctx := context.Background()

	p, err := svc.CreateProject(ctx, "SCP-173", "/tmp/ws")
	require.NoError(t, err)

	p, err = svc.SetProjectStage(ctx, p.ID, domain.StageScenario)
	require.NoError(t, err)

	// scenario → pending is allowed (reset/rejection flow)
	p, err = svc.SetProjectStage(ctx, p.ID, domain.StagePending)
	require.NoError(t, err)
	assert.Equal(t, domain.StagePending, p.Status)
}
