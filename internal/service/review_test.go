package service

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sushistack/yt.pipe/internal/domain"
	"github.com/sushistack/yt.pipe/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupReviewService(t *testing.T) (*ReviewService, *store.Store, string) {
	t.Helper()
	s, err := store.New(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { s.Close() })

	tmpDir := t.TempDir()
	scenesDir := filepath.Join(tmpDir, "scenes", "1")
	require.NoError(t, os.MkdirAll(scenesDir, 0o755))

	// Create project in scenario_review state (allows mutations)
	require.NoError(t, s.CreateProject(&domain.Project{
		ID: "p1", SCPID: "SCP-173", Status: domain.StageScenario,
		SceneCount: 1, WorkspacePath: tmpDir,
	}))

	// Write a minimal scenario.json
	scenario := &domain.ScenarioOutput{
		Scenes: []domain.SceneScript{
			{SceneNum: 1, Narration: "Original narration"},
		},
	}
	data, err := json.MarshalIndent(scenario, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "scenario.json"), data, 0o644))

	logger := slog.Default()
	return NewReviewService(s, logger), s, tmpDir
}

// --- UpdateNarration tests ---

func TestUpdateNarration_Success(t *testing.T) {
	svc, _, tmpDir := setupReviewService(t)

	err := svc.UpdateNarration("p1", 1, "Updated narration text")
	require.NoError(t, err)

	// Verify scenario.json was updated
	scenario, err := LoadScenarioFromFile(filepath.Join(tmpDir, "scenario.json"))
	require.NoError(t, err)
	assert.Equal(t, "Updated narration text", scenario.Scenes[0].Narration)
}

func TestUpdateNarration_CreatesBackup(t *testing.T) {
	svc, _, tmpDir := setupReviewService(t)

	err := svc.UpdateNarration("p1", 1, "New text")
	require.NoError(t, err)

	// Verify backup was created
	bakPath := filepath.Join(tmpDir, "scenario.json.bak")
	_, err = os.Stat(bakPath)
	assert.NoError(t, err, "backup file should exist")
}

func TestUpdateNarration_EmptyText(t *testing.T) {
	svc, _, _ := setupReviewService(t)

	err := svc.UpdateNarration("p1", 1, "")
	assert.Error(t, err)
	assert.IsType(t, &domain.ValidationError{}, err)
}

func TestUpdateNarration_NullBytes(t *testing.T) {
	svc, _, _ := setupReviewService(t)

	err := svc.UpdateNarration("p1", 1, "text\x00with null")
	assert.Error(t, err)
	assert.IsType(t, &domain.ValidationError{}, err)
}

func TestUpdateNarration_TooLong(t *testing.T) {
	svc, _, _ := setupReviewService(t)

	longText := strings.Repeat("a", 10001)
	err := svc.UpdateNarration("p1", 1, longText)
	assert.Error(t, err)
	assert.IsType(t, &domain.ValidationError{}, err)
}

func TestUpdateNarration_SceneNotFound(t *testing.T) {
	svc, _, _ := setupReviewService(t)

	err := svc.UpdateNarration("p1", 99, "text")
	assert.Error(t, err)
	assert.IsType(t, &domain.NotFoundError{}, err)
}

func TestUpdateNarration_WrongProjectState(t *testing.T) {
	svc, st, _ := setupReviewService(t)

	// Create project in "pending" state (not allowed for mutations)
	require.NoError(t, st.CreateProject(&domain.Project{
		ID: "p2", SCPID: "SCP-999", Status: domain.StagePending,
		SceneCount: 1, WorkspacePath: "/tmp",
	}))

	err := svc.UpdateNarration("p2", 1, "text")
	assert.Error(t, err)
	assert.IsType(t, &domain.TransitionError{}, err)
}

// --- ValidateMutationState tests ---

func TestValidateMutationState_AllowedStates(t *testing.T) {
	s, err := store.New(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { s.Close() })

	svc := NewReviewService(s, slog.Default())

	for _, status := range []string{domain.StageScenario, domain.StageImages, domain.StageTTS, domain.StageComplete} {
		pid := "p-" + status
		require.NoError(t, s.CreateProject(&domain.Project{
			ID: pid, SCPID: "SCP-173", Status: status,
			SceneCount: 1, WorkspacePath: "/w",
		}))
		project, err := svc.ValidateMutationState(pid)
		assert.NoError(t, err, "status %s should be allowed", status)
		assert.NotNil(t, project)
	}
}

func TestValidateMutationState_DisallowedStates(t *testing.T) {
	s, err := store.New(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { s.Close() })

	svc := NewReviewService(s, slog.Default())

	for _, status := range []string{domain.StagePending} {
		pid := "p-" + status
		require.NoError(t, s.CreateProject(&domain.Project{
			ID: pid, SCPID: "SCP-173", Status: status,
			SceneCount: 1, WorkspacePath: "/w",
		}))
		_, err := svc.ValidateMutationState(pid)
		assert.Error(t, err, "status %s should be disallowed", status)
		assert.IsType(t, &domain.TransitionError{}, err)
	}
}

func TestValidateMutationState_ProjectNotFound(t *testing.T) {
	s, err := store.New(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { s.Close() })

	svc := NewReviewService(s, slog.Default())
	_, err = svc.ValidateMutationState("nonexistent")
	assert.Error(t, err)
	assert.IsType(t, &domain.NotFoundError{}, err)
}

// --- AddScene tests ---

func TestAddScene_Success(t *testing.T) {
	svc, st, tmpDir := setupReviewService(t)

	newNum, err := svc.AddScene("p1", "New scene narration")
	require.NoError(t, err)
	assert.Equal(t, 2, newNum)

	// Verify scenario.json has 2 scenes
	scenario, err := LoadScenarioFromFile(filepath.Join(tmpDir, "scenario.json"))
	require.NoError(t, err)
	assert.Len(t, scenario.Scenes, 2)
	assert.Equal(t, "New scene narration", scenario.Scenes[1].Narration)

	// Verify scene directory was created
	_, err = os.Stat(filepath.Join(tmpDir, "scenes", "2"))
	assert.NoError(t, err)

	// Verify approvals were initialized
	imgApprovals, err := st.ListApprovalsByProject("p1", domain.AssetTypeImage)
	require.NoError(t, err)
	found := false
	for _, a := range imgApprovals {
		if a.SceneNum == 2 {
			found = true
			assert.Equal(t, domain.ApprovalPending, a.Status)
		}
	}
	assert.True(t, found, "approval for new scene should exist")

	// Verify project scene count updated
	project, err := st.GetProject("p1")
	require.NoError(t, err)
	assert.Equal(t, 2, project.SceneCount)
}

func TestAddScene_NullBytes(t *testing.T) {
	svc, _, _ := setupReviewService(t)

	_, err := svc.AddScene("p1", "text\x00null")
	assert.Error(t, err)
	assert.IsType(t, &domain.ValidationError{}, err)
}

// --- DeleteScene tests ---

func TestDeleteScene_Success(t *testing.T) {
	svc, st, tmpDir := setupReviewService(t)

	// Init approvals for scene 1
	require.NoError(t, st.InitApproval("p1", 1, domain.AssetTypeImage))
	require.NoError(t, st.InitApproval("p1", 1, domain.AssetTypeTTS))

	err := svc.DeleteScene("p1", 1)
	require.NoError(t, err)

	// Verify scene directory was removed
	_, err = os.Stat(filepath.Join(tmpDir, "scenes", "1"))
	assert.True(t, os.IsNotExist(err))

	// Verify scenario.json has no scenes
	scenario, err := LoadScenarioFromFile(filepath.Join(tmpDir, "scenario.json"))
	require.NoError(t, err)
	assert.Len(t, scenario.Scenes, 0)
}

func TestDeleteScene_WrongState(t *testing.T) {
	svc, st, _ := setupReviewService(t)

	require.NoError(t, st.CreateProject(&domain.Project{
		ID: "p2", SCPID: "SCP-999", Status: domain.StagePending,
		SceneCount: 1, WorkspacePath: "/tmp",
	}))

	err := svc.DeleteScene("p2", 1)
	assert.Error(t, err)
	assert.IsType(t, &domain.TransitionError{}, err)
}
