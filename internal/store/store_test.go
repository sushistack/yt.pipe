package store

import (
	"testing"

	"github.com/sushistack/yt.pipe/internal/domain"
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
	assert.Equal(t, 11, version)
}

func TestNew_SceneApprovalsTableCreated(t *testing.T) {
	s := setupTestStore(t)

	// Setup project for FK
	require.NoError(t, s.CreateProject(&domain.Project{ID: "p1", SCPID: "SCP-1", Status: "pending", WorkspacePath: "/w"}))

	// Insert valid scene approval
	_, err := s.db.Exec(`INSERT INTO scene_approvals (project_id, scene_num, asset_type, status, attempts, updated_at)
		VALUES ('p1', 1, 'image', 'pending', 0, '2026-01-01T00:00:00Z')`)
	assert.NoError(t, err)

	_, err = s.db.Exec(`INSERT INTO scene_approvals (project_id, scene_num, asset_type, status, attempts, updated_at)
		VALUES ('p1', 1, 'tts', 'generated', 1, '2026-01-01T00:00:00Z')`)
	assert.NoError(t, err)

	// Invalid asset_type should fail
	_, err = s.db.Exec(`INSERT INTO scene_approvals (project_id, scene_num, asset_type, status, attempts, updated_at)
		VALUES ('p1', 2, 'video', 'pending', 0, '2026-01-01T00:00:00Z')`)
	assert.Error(t, err, "invalid asset_type should be rejected by CHECK constraint")

	// Invalid status should fail
	_, err = s.db.Exec(`INSERT INTO scene_approvals (project_id, scene_num, asset_type, status, attempts, updated_at)
		VALUES ('p1', 2, 'image', 'invalid', 0, '2026-01-01T00:00:00Z')`)
	assert.Error(t, err, "invalid status should be rejected by CHECK constraint")

	// Composite PK constraint: duplicate (project_id, scene_num, asset_type) should fail
	_, err = s.db.Exec(`INSERT INTO scene_approvals (project_id, scene_num, asset_type, status, attempts, updated_at)
		VALUES ('p1', 1, 'image', 'pending', 0, '2026-01-01T00:00:00Z')`)
	assert.Error(t, err, "duplicate composite PK should fail")
}

func TestNew_TemplateTablesCreated(t *testing.T) {
	s := setupTestStore(t)

	// Verify prompt_templates table exists
	_, err := s.db.Exec(`INSERT INTO prompt_templates (id, category, name, content, version, is_default, created_at, updated_at)
		VALUES ('t1', 'scenario', 'test', 'content', 1, 0, '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`)
	assert.NoError(t, err)

	// Verify prompt_template_versions table exists
	_, err = s.db.Exec(`INSERT INTO prompt_template_versions (id, template_id, version, content, created_at)
		VALUES ('v1', 't1', 1, 'content', '2026-01-01T00:00:00Z')`)
	assert.NoError(t, err)

	// Verify project_template_overrides table exists
	_, err = s.db.Exec(`INSERT INTO project_template_overrides (project_id, template_id, content, created_at)
		VALUES ('p1', 't1', 'override', '2026-01-01T00:00:00Z')`)
	assert.NoError(t, err)
}

func TestNew_TemplateCategoryConstraint(t *testing.T) {
	s := setupTestStore(t)

	// Valid categories should succeed
	for _, cat := range []string{"scenario", "image", "tts", "caption"} {
		_, err := s.db.Exec(`INSERT INTO prompt_templates (id, category, name, content, version, is_default, created_at, updated_at)
			VALUES (?, ?, 'test', 'content', 1, 0, '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`,
			"t-"+cat, cat)
		assert.NoError(t, err, "category %s should be valid", cat)
	}

	// Invalid category should fail due to CHECK constraint
	_, err := s.db.Exec(`INSERT INTO prompt_templates (id, category, name, content, version, is_default, created_at, updated_at)
		VALUES ('t-bad', 'invalid', 'test', 'content', 1, 0, '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`)
	assert.Error(t, err, "invalid category should be rejected by CHECK constraint")
}

func TestNew_BGMTablesCreated(t *testing.T) {
	s := setupTestStore(t)

	// Verify bgms table exists with CHECK constraint
	_, err := s.db.Exec(`INSERT INTO bgms (id, name, file_path, mood_tags, duration_ms, license_type, created_at)
		VALUES ('bgm1', 'Test BGM', '/path/bgm.mp3', '["epic","dark"]', 120000, 'royalty_free', '2026-01-01T00:00:00Z')`)
	assert.NoError(t, err)

	// Invalid license_type should fail due to CHECK constraint
	_, err = s.db.Exec(`INSERT INTO bgms (id, name, file_path, license_type, created_at)
		VALUES ('bgm2', 'Bad', '/path/bad.mp3', 'invalid', '2026-01-01T00:00:00Z')`)
	assert.Error(t, err, "invalid license_type should be rejected by CHECK constraint")

	// Verify scene_bgm_assignments table exists
	_, err = s.db.Exec(`INSERT INTO scene_bgm_assignments (project_id, scene_num, bgm_id)
		VALUES ('p1', 1, 'bgm1')`)
	assert.NoError(t, err)
}

// Project CRUD tests

func TestCreateProject_Success(t *testing.T) {
	s := setupTestStore(t)
	p := &domain.Project{
		ID: "proj-1", SCPID: "SCP-173", Status: domain.StagePending,
		SceneCount: 5, WorkspacePath: "/data/projects/proj-1",
	}
	err := s.CreateProject(p)
	require.NoError(t, err)
	assert.False(t, p.CreatedAt.IsZero())
}

func TestGetProject_Success(t *testing.T) {
	s := setupTestStore(t)
	p := &domain.Project{
		ID: "proj-1", SCPID: "SCP-173", Status: domain.StagePending,
		SceneCount: 5, WorkspacePath: "/data/projects/proj-1",
	}
	require.NoError(t, s.CreateProject(p))

	got, err := s.GetProject("proj-1")
	require.NoError(t, err)
	assert.Equal(t, "proj-1", got.ID)
	assert.Equal(t, "SCP-173", got.SCPID)
	assert.Equal(t, domain.StagePending, got.Status)
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
	p := &domain.Project{ID: "proj-1", SCPID: "SCP-173", Status: domain.StagePending, WorkspacePath: "/w/1"}
	require.NoError(t, s.CreateProject(p))

	p.Status = domain.StageScenario
	err := s.UpdateProject(p)
	require.NoError(t, err)

	got, _ := s.GetProject("proj-1")
	assert.Equal(t, domain.StageScenario, got.Status)
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
