package store

import (
	"testing"

	"github.com/jay/youtube-pipeline/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := New(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { s.Close() })
	return s
}

func TestNew_MigrationApplied(t *testing.T) {
	s := setupTestStore(t)
	version, err := s.SchemaVersion()
	require.NoError(t, err)
	assert.Equal(t, 1, version)
}

// Project CRUD tests

func TestCreateProject_Success(t *testing.T) {
	s := setupTestStore(t)
	p := &domain.Project{
		ID: "proj-1", SCPID: "SCP-173", Status: domain.StatusPending,
		SceneCount: 5, WorkspacePath: "/data/projects/proj-1",
	}
	err := s.CreateProject(p)
	require.NoError(t, err)
	assert.False(t, p.CreatedAt.IsZero())
}

func TestGetProject_Success(t *testing.T) {
	s := setupTestStore(t)
	p := &domain.Project{
		ID: "proj-1", SCPID: "SCP-173", Status: domain.StatusPending,
		SceneCount: 5, WorkspacePath: "/data/projects/proj-1",
	}
	require.NoError(t, s.CreateProject(p))

	got, err := s.GetProject("proj-1")
	require.NoError(t, err)
	assert.Equal(t, "proj-1", got.ID)
	assert.Equal(t, "SCP-173", got.SCPID)
	assert.Equal(t, domain.StatusPending, got.Status)
}

func TestGetProject_NotFound(t *testing.T) {
	s := setupTestStore(t)
	_, err := s.GetProject("nonexistent")
	assert.Error(t, err)
	assert.IsType(t, &domain.NotFoundError{}, err)
}

func TestListProjects_Empty(t *testing.T) {
	s := setupTestStore(t)
	projects, err := s.ListProjects()
	require.NoError(t, err)
	assert.Empty(t, projects)
}

func TestListProjects_Multiple(t *testing.T) {
	s := setupTestStore(t)
	require.NoError(t, s.CreateProject(&domain.Project{ID: "p1", SCPID: "SCP-1", Status: "pending", WorkspacePath: "/w/1"}))
	require.NoError(t, s.CreateProject(&domain.Project{ID: "p2", SCPID: "SCP-2", Status: "pending", WorkspacePath: "/w/2"}))

	projects, err := s.ListProjects()
	require.NoError(t, err)
	assert.Len(t, projects, 2)
}

func TestUpdateProject_Success(t *testing.T) {
	s := setupTestStore(t)
	p := &domain.Project{ID: "proj-1", SCPID: "SCP-173", Status: domain.StatusPending, WorkspacePath: "/w/1"}
	require.NoError(t, s.CreateProject(p))

	p.Status = domain.StatusScenarioReview
	err := s.UpdateProject(p)
	require.NoError(t, err)

	got, _ := s.GetProject("proj-1")
	assert.Equal(t, domain.StatusScenarioReview, got.Status)
}

func TestUpdateProject_NotFound(t *testing.T) {
	s := setupTestStore(t)
	p := &domain.Project{ID: "nonexistent", SCPID: "SCP-1", Status: "pending", WorkspacePath: "/w"}
	err := s.UpdateProject(p)
	assert.IsType(t, &domain.NotFoundError{}, err)
}

// Job CRUD tests

func TestCreateJob_Success(t *testing.T) {
	s := setupTestStore(t)
	require.NoError(t, s.CreateProject(&domain.Project{ID: "p1", SCPID: "SCP-1", Status: "pending", WorkspacePath: "/w"}))

	j := &domain.Job{ID: "job-1", ProjectID: "p1", Type: "scenario", Status: "pending"}
	err := s.CreateJob(j)
	require.NoError(t, err)
	assert.False(t, j.CreatedAt.IsZero())
}

func TestGetJob_Success(t *testing.T) {
	s := setupTestStore(t)
	require.NoError(t, s.CreateProject(&domain.Project{ID: "p1", SCPID: "SCP-1", Status: "pending", WorkspacePath: "/w"}))
	require.NoError(t, s.CreateJob(&domain.Job{ID: "job-1", ProjectID: "p1", Type: "scenario", Status: "pending"}))

	got, err := s.GetJob("job-1")
	require.NoError(t, err)
	assert.Equal(t, "job-1", got.ID)
	assert.Equal(t, "scenario", got.Type)
}

func TestGetJob_NotFound(t *testing.T) {
	s := setupTestStore(t)
	_, err := s.GetJob("nonexistent")
	assert.IsType(t, &domain.NotFoundError{}, err)
}

func TestListJobsByProject_Success(t *testing.T) {
	s := setupTestStore(t)
	require.NoError(t, s.CreateProject(&domain.Project{ID: "p1", SCPID: "SCP-1", Status: "pending", WorkspacePath: "/w"}))
	require.NoError(t, s.CreateJob(&domain.Job{ID: "j1", ProjectID: "p1", Type: "scenario", Status: "pending"}))
	require.NoError(t, s.CreateJob(&domain.Job{ID: "j2", ProjectID: "p1", Type: "image", Status: "running"}))

	jobs, err := s.ListJobsByProject("p1")
	require.NoError(t, err)
	assert.Len(t, jobs, 2)
}

func TestUpdateJob_Success(t *testing.T) {
	s := setupTestStore(t)
	require.NoError(t, s.CreateProject(&domain.Project{ID: "p1", SCPID: "SCP-1", Status: "pending", WorkspacePath: "/w"}))
	j := &domain.Job{ID: "j1", ProjectID: "p1", Type: "scenario", Status: "pending"}
	require.NoError(t, s.CreateJob(j))

	j.Status = "completed"
	j.Progress = 100
	err := s.UpdateJob(j)
	require.NoError(t, err)

	got, _ := s.GetJob("j1")
	assert.Equal(t, "completed", got.Status)
	assert.Equal(t, 100, got.Progress)
}

// Manifest CRUD tests

func TestCreateManifest_Success(t *testing.T) {
	s := setupTestStore(t)
	require.NoError(t, s.CreateProject(&domain.Project{ID: "p1", SCPID: "SCP-1", Status: "pending", WorkspacePath: "/w"}))

	m := &domain.SceneManifest{ProjectID: "p1", SceneNum: 1, Status: "pending"}
	err := s.CreateManifest(m)
	require.NoError(t, err)
	assert.False(t, m.UpdatedAt.IsZero())
}

func TestGetManifest_Success(t *testing.T) {
	s := setupTestStore(t)
	require.NoError(t, s.CreateProject(&domain.Project{ID: "p1", SCPID: "SCP-1", Status: "pending", WorkspacePath: "/w"}))
	require.NoError(t, s.CreateManifest(&domain.SceneManifest{ProjectID: "p1", SceneNum: 1, ContentHash: "abc", Status: "pending"}))

	got, err := s.GetManifest("p1", 1)
	require.NoError(t, err)
	assert.Equal(t, "abc", got.ContentHash)
}

func TestGetManifest_NotFound(t *testing.T) {
	s := setupTestStore(t)
	_, err := s.GetManifest("p1", 99)
	assert.IsType(t, &domain.NotFoundError{}, err)
}

func TestListManifestsByProject_Success(t *testing.T) {
	s := setupTestStore(t)
	require.NoError(t, s.CreateProject(&domain.Project{ID: "p1", SCPID: "SCP-1", Status: "pending", WorkspacePath: "/w"}))
	require.NoError(t, s.CreateManifest(&domain.SceneManifest{ProjectID: "p1", SceneNum: 1, Status: "pending"}))
	require.NoError(t, s.CreateManifest(&domain.SceneManifest{ProjectID: "p1", SceneNum: 2, Status: "pending"}))

	manifests, err := s.ListManifestsByProject("p1")
	require.NoError(t, err)
	assert.Len(t, manifests, 2)
	assert.Equal(t, 1, manifests[0].SceneNum)
	assert.Equal(t, 2, manifests[1].SceneNum)
}

func TestUpdateManifest_Success(t *testing.T) {
	s := setupTestStore(t)
	require.NoError(t, s.CreateProject(&domain.Project{ID: "p1", SCPID: "SCP-1", Status: "pending", WorkspacePath: "/w"}))
	m := &domain.SceneManifest{ProjectID: "p1", SceneNum: 1, Status: "pending"}
	require.NoError(t, s.CreateManifest(m))

	m.ImageHash = "img-hash"
	m.Status = "image_done"
	err := s.UpdateManifest(m)
	require.NoError(t, err)

	got, _ := s.GetManifest("p1", 1)
	assert.Equal(t, "img-hash", got.ImageHash)
	assert.Equal(t, "image_done", got.Status)
}

func TestUpdateManifest_NotFound(t *testing.T) {
	s := setupTestStore(t)
	m := &domain.SceneManifest{ProjectID: "nope", SceneNum: 1, Status: "pending"}
	err := s.UpdateManifest(m)
	assert.IsType(t, &domain.NotFoundError{}, err)
}

// Execution log tests

func TestCreateExecutionLog_Success(t *testing.T) {
	s := setupTestStore(t)
	require.NoError(t, s.CreateProject(&domain.Project{ID: "p1", SCPID: "SCP-1", Status: "pending", WorkspacePath: "/w"}))

	dur := 1500
	cost := 0.05
	log := &domain.ExecutionLog{
		ProjectID: "p1", Stage: "scenario", Action: "generate",
		Status: "success", DurationMs: &dur, EstimatedCostUSD: &cost, Details: "ok",
	}
	err := s.CreateExecutionLog(log)
	require.NoError(t, err)
	assert.NotZero(t, log.ID)
}

func TestListExecutionLogsByProject_Success(t *testing.T) {
	s := setupTestStore(t)
	require.NoError(t, s.CreateProject(&domain.Project{ID: "p1", SCPID: "SCP-1", Status: "pending", WorkspacePath: "/w"}))
	require.NoError(t, s.CreateExecutionLog(&domain.ExecutionLog{ProjectID: "p1", Stage: "s1", Action: "a1", Status: "ok"}))
	require.NoError(t, s.CreateExecutionLog(&domain.ExecutionLog{ProjectID: "p1", Stage: "s2", Action: "a2", Status: "ok"}))

	logs, err := s.ListExecutionLogsByProject("p1")
	require.NoError(t, err)
	assert.Len(t, logs, 2)
}
