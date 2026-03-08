package pipeline

import (
	"log/slog"
	"testing"

	"github.com/jay/youtube-pipeline/internal/domain"
	"github.com/jay/youtube-pipeline/internal/service"
	"github.com/jay/youtube-pipeline/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestDB(t *testing.T) *store.Store {
	t.Helper()
	db, err := store.New(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	return db
}

func TestSceneSkipChecker_FilterScenesForImageGen_NoManifests(t *testing.T) {
	db := setupTestDB(t)
	checker := NewSceneSkipChecker(db, slog.Default())

	prompts := []service.ImagePromptResult{
		{SceneNum: 1, SanitizedPrompt: "prompt 1"},
		{SceneNum: 2, SanitizedPrompt: "prompt 2"},
	}

	toGen, toSkip := checker.FilterScenesForImageGen("proj-1", prompts)
	assert.Len(t, toGen, 2)
	assert.Empty(t, toSkip)
}

func TestSceneSkipChecker_FilterScenesForImageGen_WithMatching(t *testing.T) {
	db := setupTestDB(t)

	// Create a project first
	proj := &domain.Project{ID: "proj-1", SCPID: "SCP-173", Status: "pending", WorkspacePath: "/tmp"}
	require.NoError(t, db.CreateProject(proj))

	// Pre-populate manifest with matching hash
	prompt1Hash := service.ContentHash([]byte("prompt 1"))
	require.NoError(t, db.CreateManifest(&domain.SceneManifest{
		ProjectID:   "proj-1",
		SceneNum:    1,
		ContentHash: prompt1Hash,
		ImageHash:   "some-image-hash", // Has been generated
		Status:      "image_generated",
	}))

	checker := NewSceneSkipChecker(db, slog.Default())

	prompts := []service.ImagePromptResult{
		{SceneNum: 1, SanitizedPrompt: "prompt 1"}, // Same hash → skip
		{SceneNum: 2, SanitizedPrompt: "prompt 2"}, // No manifest → generate
	}

	toGen, toSkip := checker.FilterScenesForImageGen("proj-1", prompts)
	assert.Equal(t, []int{2}, toGen)
	assert.Equal(t, []int{1}, toSkip)
}

func TestSceneSkipChecker_FilterScenesForImageGen_ChangedPrompt(t *testing.T) {
	db := setupTestDB(t)

	proj := &domain.Project{ID: "proj-1", SCPID: "SCP-173", Status: "pending", WorkspacePath: "/tmp"}
	require.NoError(t, db.CreateProject(proj))

	oldHash := service.ContentHash([]byte("old prompt"))
	require.NoError(t, db.CreateManifest(&domain.SceneManifest{
		ProjectID:   "proj-1",
		SceneNum:    1,
		ContentHash: oldHash,
		ImageHash:   "img-hash",
		Status:      "image_generated",
	}))

	checker := NewSceneSkipChecker(db, slog.Default())

	prompts := []service.ImagePromptResult{
		{SceneNum: 1, SanitizedPrompt: "new prompt"}, // Changed → regenerate
	}

	toGen, toSkip := checker.FilterScenesForImageGen("proj-1", prompts)
	assert.Equal(t, []int{1}, toGen)
	assert.Empty(t, toSkip)
}

func TestIncrementalResult_Summary(t *testing.T) {
	ir := IncrementalResult{TotalScenes: 10, Regenerated: 2, Skipped: 8}
	assert.Equal(t, "2 scenes regenerated, 8 skipped", ir.Summary())
}

func TestIncrementalResult_AllSkipped(t *testing.T) {
	ir := IncrementalResult{TotalScenes: 5, Regenerated: 0, Skipped: 5}
	assert.Equal(t, "0 scenes regenerated, 5 skipped", ir.Summary())
}
