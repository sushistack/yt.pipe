package pipeline

import (
	"log/slog"
	"testing"

	"github.com/sushistack/yt.pipe/internal/domain"
	"github.com/sushistack/yt.pipe/internal/service"
	"github.com/sushistack/yt.pipe/internal/store"
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

func TestSceneSkipChecker_FilterShotsForImageGen_NoManifests(t *testing.T) {
	db := setupTestDB(t)
	checker := NewSceneSkipChecker(db, slog.Default())

	scenePrompts := []*service.ScenePromptOutput{
		{SceneNum: 1, Shots: []service.ShotOutput{
			{ShotNum: 1, SentenceText: "sentence 1"},
			{ShotNum: 2, SentenceText: "sentence 2"},
		}},
	}

	toGen, toSkip := checker.FilterShotsForImageGen("proj-1", scenePrompts)
	assert.Len(t, toGen, 2)
	assert.Empty(t, toSkip)
}

func TestSceneSkipChecker_FilterShotsForImageGen_WithMatching(t *testing.T) {
	db := setupTestDB(t)

	proj := &domain.Project{ID: "proj-1", SCPID: "SCP-173", Status: "pending", WorkspacePath: "/tmp"}
	require.NoError(t, db.CreateProject(proj))

	// Pre-populate shot manifests with sentence_start/cut_num for new schema
	sentenceHash := service.ContentHash([]byte("sentence 1"))
	require.NoError(t, db.CreateShotManifest(&domain.ShotManifest{
		ProjectID:     "proj-1",
		SceneNum:      1,
		ShotNum:       1,
		SentenceStart: 1,
		SentenceEnd:   1,
		CutNum:        1,
		ContentHash:   sentenceHash,
		ImageHash:     "some-image-hash",
		GenMethod:     "text_to_image",
		Status:        "generated",
	}))
	require.NoError(t, db.CreateShotManifest(&domain.ShotManifest{
		ProjectID:     "proj-1",
		SceneNum:      1,
		ShotNum:       2,
		SentenceStart: 2,
		SentenceEnd:   2,
		CutNum:        1,
		ContentHash:   "old-hash",
		ImageHash:     "",
		GenMethod:     "text_to_image",
		Status:        "pending",
	}))

	checker := NewSceneSkipChecker(db, slog.Default())

	scenePrompts := []*service.ScenePromptOutput{
		{SceneNum: 1, Shots: []service.ShotOutput{
			{ShotNum: 1, SentenceText: "sentence 1"}, // Same hash + image exists → skip
			{ShotNum: 2, SentenceText: "sentence 2"}, // No image hash → generate
		}},
	}

	toGen, toSkip := checker.FilterShotsForImageGen("proj-1", scenePrompts)
	assert.Len(t, toGen, 1)
	assert.Equal(t, domain.ShotKey{SceneNum: 1, ShotNum: 2}, toGen[0])
	assert.Len(t, toSkip, 1)
	assert.Equal(t, domain.ShotKey{SceneNum: 1, ShotNum: 1}, toSkip[0])
}

func TestSceneSkipChecker_FilterShotsForImageGen_SceneExpanded(t *testing.T) {
	db := setupTestDB(t)

	proj := &domain.Project{ID: "proj-1", SCPID: "SCP-173", Status: "pending", WorkspacePath: "/tmp"}
	require.NoError(t, db.CreateProject(proj))

	// Scene previously had 2 shots
	require.NoError(t, db.CreateShotManifest(&domain.ShotManifest{
		ProjectID: "proj-1", SceneNum: 1, ShotNum: 1,
		SentenceStart: 1, SentenceEnd: 1, CutNum: 1,
		ContentHash: "hash1", ImageHash: "img1", GenMethod: "text_to_image", Status: "generated",
	}))
	require.NoError(t, db.CreateShotManifest(&domain.ShotManifest{
		ProjectID: "proj-1", SceneNum: 1, ShotNum: 2,
		SentenceStart: 2, SentenceEnd: 2, CutNum: 1,
		ContentHash: "hash2", ImageHash: "img2", GenMethod: "text_to_image", Status: "generated",
	}))

	checker := NewSceneSkipChecker(db, slog.Default())

	// Now scene has 3 shots → all should regenerate
	scenePrompts := []*service.ScenePromptOutput{
		{SceneNum: 1, Shots: []service.ShotOutput{
			{ShotNum: 1, SentenceText: "s1"},
			{ShotNum: 2, SentenceText: "s2"},
			{ShotNum: 3, SentenceText: "s3"},
		}},
	}

	toGen, toSkip := checker.FilterShotsForImageGen("proj-1", scenePrompts)
	assert.Len(t, toGen, 3)
	assert.Empty(t, toSkip)
}

func TestSceneSkipChecker_FilterCutsForImageGen(t *testing.T) {
	db := setupTestDB(t)

	proj := &domain.Project{ID: "proj-1", SCPID: "SCP-173", Status: "pending", WorkspacePath: "/tmp"}
	require.NoError(t, db.CreateProject(proj))

	// Pre-populate with cut manifests
	cutHash := service.ContentHash([]byte("prompt for cut 1"))
	require.NoError(t, db.CreateShotManifest(&domain.ShotManifest{
		ProjectID: "proj-1", SceneNum: 1,
		SentenceStart: 1, SentenceEnd: 1, CutNum: 1,
		ContentHash: cutHash, ImageHash: "img1", GenMethod: "text_to_image", Status: "generated",
	}))

	checker := NewSceneSkipChecker(db, slog.Default())

	sceneCuts := []*service.SceneCutOutput{
		{SceneNum: 1, Cuts: []service.CutOutput{
			{SentenceStart: 1, SentenceEnd: 1, CutNum: 1, FinalPrompt: "prompt for cut 1"},
			{SentenceStart: 2, SentenceEnd: 3, CutNum: 1, FinalPrompt: "new merged cut"},
		}},
	}

	toGen, toSkip := checker.FilterCutsForImageGen("proj-1", sceneCuts)
	// Cut count changed (1 stored → 2 new), so all should regenerate
	assert.Len(t, toGen, 2)
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
