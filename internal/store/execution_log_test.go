package store

import (
	"testing"

	"github.com/sushistack/yt.pipe/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateExecutionLog_WithJobID(t *testing.T) {
	s := setupTestStore(t)
	require.NoError(t, s.CreateProject(&domain.Project{ID: "p1", SCPID: "SCP-1", Status: "pending", WorkspacePath: "/w"}))
	require.NoError(t, s.CreateJob(&domain.Job{ID: "j1", ProjectID: "p1", Type: "scenario", Status: "running"}))

	dur := 2000
	cost := 0.10
	log := &domain.ExecutionLog{
		ProjectID:        "p1",
		JobID:            "j1",
		Stage:            "scenario",
		Action:           "generate",
		Status:           "success",
		DurationMs:       &dur,
		EstimatedCostUSD: &cost,
		Details:          "completed ok",
	}
	err := s.CreateExecutionLog(log)
	require.NoError(t, err)
	assert.NotZero(t, log.ID)
	assert.False(t, log.CreatedAt.IsZero())
}

func TestCreateExecutionLog_WithoutOptionalFields(t *testing.T) {
	s := setupTestStore(t)
	require.NoError(t, s.CreateProject(&domain.Project{ID: "p1", SCPID: "SCP-1", Status: "pending", WorkspacePath: "/w"}))

	log := &domain.ExecutionLog{
		ProjectID: "p1",
		Stage:     "image",
		Action:    "generate",
		Status:    "failed",
	}
	err := s.CreateExecutionLog(log)
	require.NoError(t, err)
	assert.NotZero(t, log.ID)
}

func TestListExecutionLogsByProject_Empty(t *testing.T) {
	s := setupTestStore(t)
	require.NoError(t, s.CreateProject(&domain.Project{ID: "p1", SCPID: "SCP-1", Status: "pending", WorkspacePath: "/w"}))

	logs, err := s.ListExecutionLogsByProject("p1")
	require.NoError(t, err)
	assert.Empty(t, logs)
}

func TestListExecutionLogsByProject_FiltersCorrectly(t *testing.T) {
	s := setupTestStore(t)
	require.NoError(t, s.CreateProject(&domain.Project{ID: "p1", SCPID: "SCP-1", Status: "pending", WorkspacePath: "/w"}))
	require.NoError(t, s.CreateProject(&domain.Project{ID: "p2", SCPID: "SCP-2", Status: "pending", WorkspacePath: "/w2"}))

	require.NoError(t, s.CreateExecutionLog(&domain.ExecutionLog{ProjectID: "p1", Stage: "s1", Action: "a1", Status: "ok"}))
	require.NoError(t, s.CreateExecutionLog(&domain.ExecutionLog{ProjectID: "p2", Stage: "s2", Action: "a2", Status: "ok"}))

	logs, err := s.ListExecutionLogsByProject("p1")
	require.NoError(t, err)
	assert.Len(t, logs, 1)
	assert.Equal(t, "p1", logs[0].ProjectID)
}

func TestListAllExecutionLogs_Success(t *testing.T) {
	s := setupTestStore(t)
	require.NoError(t, s.CreateProject(&domain.Project{ID: "p1", SCPID: "SCP-1", Status: "pending", WorkspacePath: "/w"}))
	require.NoError(t, s.CreateProject(&domain.Project{ID: "p2", SCPID: "SCP-2", Status: "pending", WorkspacePath: "/w2"}))

	require.NoError(t, s.CreateExecutionLog(&domain.ExecutionLog{ProjectID: "p1", Stage: "s1", Action: "a1", Status: "ok"}))
	require.NoError(t, s.CreateExecutionLog(&domain.ExecutionLog{ProjectID: "p2", Stage: "s2", Action: "a2", Status: "ok"}))
	require.NoError(t, s.CreateExecutionLog(&domain.ExecutionLog{ProjectID: "p1", Stage: "s3", Action: "a3", Status: "ok"}))

	logs, err := s.ListAllExecutionLogs()
	require.NoError(t, err)
	assert.Len(t, logs, 3)
}

func TestListAllExecutionLogs_Empty(t *testing.T) {
	s := setupTestStore(t)
	logs, err := s.ListAllExecutionLogs()
	require.NoError(t, err)
	assert.Empty(t, logs)
}

func TestListExecutionLogsByProject_PreservesJobID(t *testing.T) {
	s := setupTestStore(t)
	require.NoError(t, s.CreateProject(&domain.Project{ID: "p1", SCPID: "SCP-1", Status: "pending", WorkspacePath: "/w"}))
	require.NoError(t, s.CreateJob(&domain.Job{ID: "j1", ProjectID: "p1", Type: "scenario", Status: "running"}))

	require.NoError(t, s.CreateExecutionLog(&domain.ExecutionLog{ProjectID: "p1", JobID: "j1", Stage: "s1", Action: "a1", Status: "ok"}))

	logs, err := s.ListExecutionLogsByProject("p1")
	require.NoError(t, err)
	require.Len(t, logs, 1)
	assert.Equal(t, "j1", logs[0].JobID)
}

func TestListExecutionLogsByProject_NullJobID(t *testing.T) {
	s := setupTestStore(t)
	require.NoError(t, s.CreateProject(&domain.Project{ID: "p1", SCPID: "SCP-1", Status: "pending", WorkspacePath: "/w"}))

	// Log without a job ID (empty string → NULL in DB)
	require.NoError(t, s.CreateExecutionLog(&domain.ExecutionLog{ProjectID: "p1", Stage: "s1", Action: "a1", Status: "ok"}))

	logs, err := s.ListExecutionLogsByProject("p1")
	require.NoError(t, err)
	require.Len(t, logs, 1)
	assert.Empty(t, logs[0].JobID)
}
