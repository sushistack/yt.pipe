package store

import (
	"testing"

	"github.com/sushistack/yt.pipe/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateManifest_WithAllHashes(t *testing.T) {
	s := setupTestStore(t)
	require.NoError(t, s.CreateProject(&domain.Project{ID: "p1", SCPID: "SCP-1", Status: "pending", WorkspacePath: "/w"}))

	m := &domain.SceneManifest{
		ProjectID:    "p1",
		SceneNum:     1,
		ContentHash:  "content-abc",
		ImageHash:    "img-def",
		AudioHash:    "audio-ghi",
		SubtitleHash: "sub-jkl",
		Status:       "complete",
	}
	err := s.CreateManifest(m)
	require.NoError(t, err)
	assert.False(t, m.UpdatedAt.IsZero())

	got, err := s.GetManifest("p1", 1)
	require.NoError(t, err)
	assert.Equal(t, "content-abc", got.ContentHash)
	assert.Equal(t, "img-def", got.ImageHash)
	assert.Equal(t, "audio-ghi", got.AudioHash)
	assert.Equal(t, "sub-jkl", got.SubtitleHash)
	assert.Equal(t, "complete", got.Status)
}

func TestListManifestsByProject_OrderedBySceneNum(t *testing.T) {
	s := setupTestStore(t)
	require.NoError(t, s.CreateProject(&domain.Project{ID: "p1", SCPID: "SCP-1", Status: "pending", WorkspacePath: "/w"}))

	// Insert out of order
	require.NoError(t, s.CreateManifest(&domain.SceneManifest{ProjectID: "p1", SceneNum: 3, Status: "pending"}))
	require.NoError(t, s.CreateManifest(&domain.SceneManifest{ProjectID: "p1", SceneNum: 1, Status: "pending"}))
	require.NoError(t, s.CreateManifest(&domain.SceneManifest{ProjectID: "p1", SceneNum: 2, Status: "pending"}))

	manifests, err := s.ListManifestsByProject("p1")
	require.NoError(t, err)
	require.Len(t, manifests, 3)
	assert.Equal(t, 1, manifests[0].SceneNum)
	assert.Equal(t, 2, manifests[1].SceneNum)
	assert.Equal(t, 3, manifests[2].SceneNum)
}

func TestListManifestsByProject_Empty(t *testing.T) {
	s := setupTestStore(t)
	require.NoError(t, s.CreateProject(&domain.Project{ID: "p1", SCPID: "SCP-1", Status: "pending", WorkspacePath: "/w"}))

	manifests, err := s.ListManifestsByProject("p1")
	require.NoError(t, err)
	assert.Empty(t, manifests)
}

func TestListManifestsByProject_FiltersCorrectly(t *testing.T) {
	s := setupTestStore(t)
	require.NoError(t, s.CreateProject(&domain.Project{ID: "p1", SCPID: "SCP-1", Status: "pending", WorkspacePath: "/w"}))
	require.NoError(t, s.CreateProject(&domain.Project{ID: "p2", SCPID: "SCP-2", Status: "pending", WorkspacePath: "/w2"}))

	require.NoError(t, s.CreateManifest(&domain.SceneManifest{ProjectID: "p1", SceneNum: 1, Status: "pending"}))
	require.NoError(t, s.CreateManifest(&domain.SceneManifest{ProjectID: "p2", SceneNum: 1, Status: "pending"}))

	manifests, err := s.ListManifestsByProject("p1")
	require.NoError(t, err)
	assert.Len(t, manifests, 1)
	assert.Equal(t, "p1", manifests[0].ProjectID)
}

func TestUpdateManifest_PartialUpdate(t *testing.T) {
	s := setupTestStore(t)
	require.NoError(t, s.CreateProject(&domain.Project{ID: "p1", SCPID: "SCP-1", Status: "pending", WorkspacePath: "/w"}))
	m := &domain.SceneManifest{ProjectID: "p1", SceneNum: 1, ContentHash: "orig", Status: "pending"}
	require.NoError(t, s.CreateManifest(m))

	// Update only image hash
	m.ImageHash = "new-img-hash"
	m.Status = "image_done"
	err := s.UpdateManifest(m)
	require.NoError(t, err)

	got, _ := s.GetManifest("p1", 1)
	assert.Equal(t, "orig", got.ContentHash)
	assert.Equal(t, "new-img-hash", got.ImageHash)
	assert.Equal(t, "image_done", got.Status)
	assert.Empty(t, got.AudioHash)
}

func TestGetManifest_TimestampParsed(t *testing.T) {
	s := setupTestStore(t)
	require.NoError(t, s.CreateProject(&domain.Project{ID: "p1", SCPID: "SCP-1", Status: "pending", WorkspacePath: "/w"}))
	require.NoError(t, s.CreateManifest(&domain.SceneManifest{ProjectID: "p1", SceneNum: 1, Status: "pending"}))

	got, err := s.GetManifest("p1", 1)
	require.NoError(t, err)
	assert.False(t, got.UpdatedAt.IsZero())
}
