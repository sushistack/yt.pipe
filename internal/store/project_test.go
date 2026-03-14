package store

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/sushistack/yt.pipe/internal/domain"
)

func TestDeleteProject_Success(t *testing.T) {
	s := setupTestStore(t)
	require.NoError(t, s.CreateProject(&domain.Project{ID: "p1", SCPID: "SCP-1", Status: "pending", WorkspacePath: "/w"}))

	err := s.DeleteProject("p1")
	require.NoError(t, err)

	_, err = s.GetProject("p1")
	assert.IsType(t, &domain.NotFoundError{}, err)
}

func TestDeleteProject_NotFound(t *testing.T) {
	s := setupTestStore(t)
	err := s.DeleteProject("nonexistent")
	assert.IsType(t, &domain.NotFoundError{}, err)
}

func TestDeleteProject_CascadeDeletesChildren(t *testing.T) {
	s := setupTestStore(t)
	require.NoError(t, s.CreateProject(&domain.Project{ID: "p1", SCPID: "SCP-1", Status: "pending", WorkspacePath: "/w"}))
	require.NoError(t, s.CreateJob(&domain.Job{ID: "j1", ProjectID: "p1", Type: "scenario", Status: "pending"}))
	require.NoError(t, s.CreateManifest(&domain.SceneManifest{ProjectID: "p1", SceneNum: 1, Status: "pending"}))
	require.NoError(t, s.CreateExecutionLog(&domain.ExecutionLog{ProjectID: "p1", Stage: "s1", Action: "a1", Status: "ok"}))

	err := s.DeleteProject("p1")
	require.NoError(t, err)

	// Verify child records are also deleted
	jobs, err := s.ListJobsByProject("p1")
	require.NoError(t, err)
	assert.Empty(t, jobs)

	manifests, err := s.ListManifestsByProject("p1")
	require.NoError(t, err)
	assert.Empty(t, manifests)

	logs, err := s.ListExecutionLogsByProject("p1")
	require.NoError(t, err)
	assert.Empty(t, logs)
}

func TestListProjectsFiltered_NoFilters(t *testing.T) {
	s := setupTestStore(t)
	require.NoError(t, s.CreateProject(&domain.Project{ID: "p1", SCPID: "SCP-1", Status: "pending", WorkspacePath: "/w/1"}))
	require.NoError(t, s.CreateProject(&domain.Project{ID: "p2", SCPID: "SCP-2", Status: "images", WorkspacePath: "/w/2"}))
	require.NoError(t, s.CreateProject(&domain.Project{ID: "p3", SCPID: "SCP-3", Status: "complete", WorkspacePath: "/w/3"}))

	projects, total, err := s.ListProjectsFiltered("", "", 10, 0)
	require.NoError(t, err)
	assert.Equal(t, 3, total)
	assert.Len(t, projects, 3)
}

func TestListProjectsFiltered_ByStatus(t *testing.T) {
	s := setupTestStore(t)
	require.NoError(t, s.CreateProject(&domain.Project{ID: "p1", SCPID: "SCP-1", Status: "pending", WorkspacePath: "/w/1"}))
	require.NoError(t, s.CreateProject(&domain.Project{ID: "p2", SCPID: "SCP-2", Status: "images", WorkspacePath: "/w/2"}))
	require.NoError(t, s.CreateProject(&domain.Project{ID: "p3", SCPID: "SCP-3", Status: "pending", WorkspacePath: "/w/3"}))

	projects, total, err := s.ListProjectsFiltered("pending", "", 10, 0)
	require.NoError(t, err)
	assert.Equal(t, 2, total)
	assert.Len(t, projects, 2)
	for _, p := range projects {
		assert.Equal(t, "pending", p.Status)
	}
}

func TestListProjectsFiltered_BySCPID(t *testing.T) {
	s := setupTestStore(t)
	require.NoError(t, s.CreateProject(&domain.Project{ID: "p1", SCPID: "SCP-173", Status: "pending", WorkspacePath: "/w/1"}))
	require.NoError(t, s.CreateProject(&domain.Project{ID: "p2", SCPID: "SCP-999", Status: "pending", WorkspacePath: "/w/2"}))

	projects, total, err := s.ListProjectsFiltered("", "SCP-173", 10, 0)
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	assert.Len(t, projects, 1)
	assert.Equal(t, "SCP-173", projects[0].SCPID)
}

func TestListProjectsFiltered_BothFilters(t *testing.T) {
	s := setupTestStore(t)
	require.NoError(t, s.CreateProject(&domain.Project{ID: "p1", SCPID: "SCP-173", Status: "pending", WorkspacePath: "/w/1"}))
	require.NoError(t, s.CreateProject(&domain.Project{ID: "p2", SCPID: "SCP-173", Status: "complete", WorkspacePath: "/w/2"}))
	require.NoError(t, s.CreateProject(&domain.Project{ID: "p3", SCPID: "SCP-999", Status: "pending", WorkspacePath: "/w/3"}))

	projects, total, err := s.ListProjectsFiltered("pending", "SCP-173", 10, 0)
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	assert.Len(t, projects, 1)
	assert.Equal(t, "p1", projects[0].ID)
}

func TestListProjectsFiltered_Pagination(t *testing.T) {
	s := setupTestStore(t)
	for i := 0; i < 5; i++ {
		require.NoError(t, s.CreateProject(&domain.Project{
			ID: fmt.Sprintf("p%d", i), SCPID: "SCP-1", Status: "pending",
			WorkspacePath: fmt.Sprintf("/w/%d", i),
		}))
	}

	// Page 1: limit 2, offset 0
	projects, total, err := s.ListProjectsFiltered("", "", 2, 0)
	require.NoError(t, err)
	assert.Equal(t, 5, total)
	assert.Len(t, projects, 2)

	// Page 2: limit 2, offset 2
	projects2, total2, err := s.ListProjectsFiltered("", "", 2, 2)
	require.NoError(t, err)
	assert.Equal(t, 5, total2)
	assert.Len(t, projects2, 2)

	// Verify different results
	assert.NotEqual(t, projects[0].ID, projects2[0].ID)

	// Page 3: limit 2, offset 4
	projects3, _, err := s.ListProjectsFiltered("", "", 2, 4)
	require.NoError(t, err)
	assert.Len(t, projects3, 1, "last page should have 1 item")
}

func TestListProjectsFiltered_Empty(t *testing.T) {
	s := setupTestStore(t)
	projects, total, err := s.ListProjectsFiltered("", "", 10, 0)
	require.NoError(t, err)
	assert.Equal(t, 0, total)
	assert.Empty(t, projects)
}

func TestListProjectsFiltered_NoMatch(t *testing.T) {
	s := setupTestStore(t)
	require.NoError(t, s.CreateProject(&domain.Project{ID: "p1", SCPID: "SCP-1", Status: "pending", WorkspacePath: "/w"}))

	projects, total, err := s.ListProjectsFiltered("complete", "", 10, 0)
	require.NoError(t, err)
	assert.Equal(t, 0, total)
	assert.Empty(t, projects)
}

func TestCreateProject_DuplicateID(t *testing.T) {
	s := setupTestStore(t)
	require.NoError(t, s.CreateProject(&domain.Project{ID: "p1", SCPID: "SCP-1", Status: "pending", WorkspacePath: "/w/1"}))

	err := s.CreateProject(&domain.Project{ID: "p1", SCPID: "SCP-2", Status: "pending", WorkspacePath: "/w/2"})
	assert.Error(t, err, "duplicate ID should fail")
}
