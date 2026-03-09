package store

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/sushistack/yt.pipe/internal/domain"
)

func createTestJob(t *testing.T, s *Store, id, projectID, status string) *domain.Job {
	t.Helper()
	j := &domain.Job{
		ID:        id,
		ProjectID: projectID,
		Type:      "pipeline_run",
		Status:    status,
	}
	require.NoError(t, s.CreateJob(j))
	return j
}

func TestGetLatestJobByProject(t *testing.T) {
	s := setupTestStore(t)

	// Setup project
	require.NoError(t, s.CreateProject(&domain.Project{ID: "p1", SCPID: "SCP-1", Status: "pending", WorkspacePath: "/w"}))

	// No jobs yet
	j, err := s.GetLatestJobByProject("p1")
	require.NoError(t, err)
	assert.Nil(t, j)

	// Create two jobs with explicit timestamps to ensure ordering
	oldTime := time.Now().UTC().Add(-1 * time.Hour).Format(time.RFC3339)
	newTime := time.Now().UTC().Format(time.RFC3339)

	_, err = s.db.Exec(
		`INSERT INTO jobs (id, project_id, type, status, progress, result, error, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"j1", "p1", "pipeline_run", "complete", 100, "", "", oldTime, oldTime,
	)
	require.NoError(t, err)

	_, err = s.db.Exec(
		`INSERT INTO jobs (id, project_id, type, status, progress, result, error, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"j2", "p1", "pipeline_run", "running", 50, "", "", newTime, newTime,
	)
	require.NoError(t, err)

	// Should return the most recent
	j, err = s.GetLatestJobByProject("p1")
	require.NoError(t, err)
	require.NotNil(t, j)
	assert.Equal(t, "j2", j.ID)
}

func TestMarkStaleJobsFailed(t *testing.T) {
	s := setupTestStore(t)

	require.NoError(t, s.CreateProject(&domain.Project{ID: "p1", SCPID: "SCP-1", Status: "pending", WorkspacePath: "/w"}))

	createTestJob(t, s, "j1", "p1", "running")
	createTestJob(t, s, "j2", "p1", "running")
	createTestJob(t, s, "j3", "p1", "complete")

	count, err := s.MarkStaleJobsFailed("server restarted")
	require.NoError(t, err)
	assert.Equal(t, int64(2), count)

	// Verify j1 and j2 are failed
	j1, err := s.GetJob("j1")
	require.NoError(t, err)
	assert.Equal(t, "failed", j1.Status)
	assert.Equal(t, "server restarted", j1.Error)

	j2, err := s.GetJob("j2")
	require.NoError(t, err)
	assert.Equal(t, "failed", j2.Status)

	// j3 should remain complete
	j3, err := s.GetJob("j3")
	require.NoError(t, err)
	assert.Equal(t, "complete", j3.Status)
}

func TestMarkStaleJobsFailed_NoRunningJobs(t *testing.T) {
	s := setupTestStore(t)

	require.NoError(t, s.CreateProject(&domain.Project{ID: "p1", SCPID: "SCP-1", Status: "pending", WorkspacePath: "/w"}))
	createTestJob(t, s, "j1", "p1", "complete")

	count, err := s.MarkStaleJobsFailed("server restarted")
	require.NoError(t, err)
	assert.Equal(t, int64(0), count)
}

func TestPurgeOldJobs(t *testing.T) {
	s := setupTestStore(t)

	require.NoError(t, s.CreateProject(&domain.Project{ID: "p1", SCPID: "SCP-1", Status: "pending", WorkspacePath: "/w"}))

	// Create an old completed job with created_at 10 days ago
	oldTime := time.Now().UTC().Add(-10 * 24 * time.Hour).Format(time.RFC3339)
	_, err := s.db.Exec(
		`INSERT INTO jobs (id, project_id, type, status, progress, result, error, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"old-job", "p1", "pipeline_run", "complete", 100, "", "", oldTime, oldTime,
	)
	require.NoError(t, err)

	// Create an old running job (should NOT be purged)
	_, err = s.db.Exec(
		`INSERT INTO jobs (id, project_id, type, status, progress, result, error, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"old-running", "p1", "pipeline_run", "running", 50, "", "", oldTime, oldTime,
	)
	require.NoError(t, err)

	// Create a recent completed job (should NOT be purged)
	createTestJob(t, s, "recent-job", "p1", "complete")

	purged, err := s.PurgeOldJobs(7)
	require.NoError(t, err)
	assert.Equal(t, int64(1), purged)

	// old-job should be gone
	_, err = s.GetJob("old-job")
	assert.Error(t, err)

	// old-running should still exist
	j, err := s.GetJob("old-running")
	require.NoError(t, err)
	assert.Equal(t, "running", j.Status)

	// recent-job should still exist
	j, err = s.GetJob("recent-job")
	require.NoError(t, err)
	assert.Equal(t, "complete", j.Status)
}

func TestPurgeOldJobs_PurgesFailedAndCancelled(t *testing.T) {
	s := setupTestStore(t)

	require.NoError(t, s.CreateProject(&domain.Project{ID: "p1", SCPID: "SCP-1", Status: "pending", WorkspacePath: "/w"}))

	oldTime := time.Now().UTC().Add(-10 * 24 * time.Hour).Format(time.RFC3339)
	for _, status := range []string{"failed", "cancelled"} {
		_, err := s.db.Exec(
			`INSERT INTO jobs (id, project_id, type, status, progress, result, error, created_at, updated_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			"old-"+status, "p1", "pipeline_run", status, 0, "", "", oldTime, oldTime,
		)
		require.NoError(t, err)
	}

	purged, err := s.PurgeOldJobs(7)
	require.NoError(t, err)
	assert.Equal(t, int64(2), purged)
}
