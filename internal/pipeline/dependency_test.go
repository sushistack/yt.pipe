package pipeline

import (
	"log/slog"
	"testing"

	"github.com/jay/youtube-pipeline/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDependencyChain(t *testing.T) {
	// Narration change invalidates everything downstream
	assert.Len(t, dependencyChain[AssetNarration], 5)

	// Prompt change only invalidates image
	assert.Equal(t, []AssetType{AssetImage}, dependencyChain[AssetPrompt])

	// Audio change invalidates timing and subtitle
	assert.Equal(t, []AssetType{AssetTiming, AssetSubtitle}, dependencyChain[AssetAudio])

	// Timing change invalidates subtitle
	assert.Equal(t, []AssetType{AssetSubtitle}, dependencyChain[AssetTiming])

	// Image and Subtitle are leaf nodes
	assert.Nil(t, dependencyChain[AssetImage])
	assert.Nil(t, dependencyChain[AssetSubtitle])
}

func TestDependencyTracker_InvalidateDownstream_Narration(t *testing.T) {
	db := setupTestDB(t)
	dt := NewDependencyTracker(db, slog.Default())

	proj := &domain.Project{ID: "proj-1", SCPID: "SCP-173", Status: "pending", WorkspacePath: "/tmp"}
	require.NoError(t, db.CreateProject(proj))

	require.NoError(t, db.CreateManifest(&domain.SceneManifest{
		ProjectID:    "proj-1",
		SceneNum:     1,
		ContentHash:  "hash-1",
		ImageHash:    "img-hash",
		AudioHash:    "aud-hash",
		SubtitleHash: "sub-hash",
		Status:       "complete",
	}))

	result, err := dt.InvalidateDownstream("proj-1", 1, AssetNarration)
	require.NoError(t, err)
	assert.Equal(t, 1, result.SceneNum)
	assert.Equal(t, AssetNarration, result.ChangedAsset)
	assert.Len(t, result.Invalidated, 5)

	// Verify manifest was updated
	m, err := db.GetManifest("proj-1", 1)
	require.NoError(t, err)
	assert.Equal(t, "stale", m.Status)
	assert.Empty(t, m.ImageHash)
	assert.Empty(t, m.AudioHash)
	assert.Empty(t, m.SubtitleHash)
}

func TestDependencyTracker_InvalidateDownstream_PromptOnly(t *testing.T) {
	db := setupTestDB(t)
	dt := NewDependencyTracker(db, slog.Default())

	proj := &domain.Project{ID: "proj-1", SCPID: "SCP-173", Status: "pending", WorkspacePath: "/tmp"}
	require.NoError(t, db.CreateProject(proj))

	require.NoError(t, db.CreateManifest(&domain.SceneManifest{
		ProjectID:    "proj-1",
		SceneNum:     1,
		ContentHash:  "hash-1",
		ImageHash:    "img-hash",
		AudioHash:    "aud-hash",
		SubtitleHash: "sub-hash",
		Status:       "complete",
	}))

	result, err := dt.InvalidateDownstream("proj-1", 1, AssetPrompt)
	require.NoError(t, err)
	assert.Equal(t, []AssetType{AssetImage}, result.Invalidated)

	// Audio and subtitle should be preserved
	m, err := db.GetManifest("proj-1", 1)
	require.NoError(t, err)
	assert.Empty(t, m.ImageHash)
	assert.Equal(t, "aud-hash", m.AudioHash)
	assert.Equal(t, "sub-hash", m.SubtitleHash)
}

func TestDependencyTracker_InvalidateDownstream_LeafNode(t *testing.T) {
	db := setupTestDB(t)
	dt := NewDependencyTracker(db, slog.Default())

	// Image is a leaf node, nothing downstream
	result, err := dt.InvalidateDownstream("proj-1", 1, AssetImage)
	require.NoError(t, err)
	assert.Empty(t, result.Invalidated)
}

func TestDependencyTracker_GetStaleScenes(t *testing.T) {
	db := setupTestDB(t)
	dt := NewDependencyTracker(db, slog.Default())

	proj := &domain.Project{ID: "proj-1", SCPID: "SCP-173", Status: "pending", WorkspacePath: "/tmp"}
	require.NoError(t, db.CreateProject(proj))

	require.NoError(t, db.CreateManifest(&domain.SceneManifest{
		ProjectID: "proj-1", SceneNum: 1, Status: "stale",
	}))
	require.NoError(t, db.CreateManifest(&domain.SceneManifest{
		ProjectID: "proj-1", SceneNum: 2, Status: "complete",
	}))
	require.NoError(t, db.CreateManifest(&domain.SceneManifest{
		ProjectID: "proj-1", SceneNum: 3, Status: "stale",
	}))

	stale, err := dt.GetStaleScenes("proj-1")
	require.NoError(t, err)
	assert.ElementsMatch(t, []int{1, 3}, stale)
}
