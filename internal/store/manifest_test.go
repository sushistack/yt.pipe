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

// --- Shot Manifest Validation Score Tests ---

func TestUpdateValidationScore_RoundTrip(t *testing.T) {
	s := setupTestStore(t)
	require.NoError(t, s.CreateProject(&domain.Project{ID: "p1", SCPID: "SCP-1", Status: "pending", WorkspacePath: "/w"}))
	require.NoError(t, s.CreateShotManifest(&domain.ShotManifest{
		ProjectID: "p1", SceneNum: 1, ShotNum: 1, SentenceStart: 1, SentenceEnd: 1, CutNum: 1,
		ContentHash: "h1", ImageHash: "ih1", GenMethod: "text_to_image", Status: "generated",
	}))

	// Update validation score
	err := s.UpdateValidationScore("p1", 1, 1, 1, 85)
	require.NoError(t, err)

	// Read it back
	score, err := s.GetValidationScore("p1", 1, 1, 1)
	require.NoError(t, err)
	require.NotNil(t, score)
	assert.Equal(t, 85, *score)
}

func TestGetValidationScore_NilForUnvalidated(t *testing.T) {
	s := setupTestStore(t)
	require.NoError(t, s.CreateProject(&domain.Project{ID: "p1", SCPID: "SCP-1", Status: "pending", WorkspacePath: "/w"}))
	require.NoError(t, s.CreateShotManifest(&domain.ShotManifest{
		ProjectID: "p1", SceneNum: 1, ShotNum: 1, SentenceStart: 1, SentenceEnd: 1, CutNum: 1,
		ContentHash: "h1", ImageHash: "ih1", GenMethod: "text_to_image", Status: "generated",
	}))

	score, err := s.GetValidationScore("p1", 1, 1, 1)
	require.NoError(t, err)
	assert.Nil(t, score)
}

func TestUpdateValidationScore_NotFound(t *testing.T) {
	s := setupTestStore(t)
	require.NoError(t, s.CreateProject(&domain.Project{ID: "p1", SCPID: "SCP-1", Status: "pending", WorkspacePath: "/w"}))

	err := s.UpdateValidationScore("p1", 99, 1, 1, 85)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestGetShotManifest_IncludesValidationScore(t *testing.T) {
	s := setupTestStore(t)
	require.NoError(t, s.CreateProject(&domain.Project{ID: "p1", SCPID: "SCP-1", Status: "pending", WorkspacePath: "/w"}))
	require.NoError(t, s.CreateShotManifest(&domain.ShotManifest{
		ProjectID: "p1", SceneNum: 1, ShotNum: 1, SentenceStart: 1, SentenceEnd: 1, CutNum: 1,
		ContentHash: "h1", ImageHash: "ih1", GenMethod: "text_to_image", Status: "generated",
	}))

	// Before validation: nil
	m, err := s.GetShotManifest("p1", 1, 1, 1)
	require.NoError(t, err)
	assert.Nil(t, m.ValidationScore)

	// After validation: populated
	require.NoError(t, s.UpdateValidationScore("p1", 1, 1, 1, 72))
	m, err = s.GetShotManifest("p1", 1, 1, 1)
	require.NoError(t, err)
	require.NotNil(t, m.ValidationScore)
	assert.Equal(t, 72, *m.ValidationScore)
}

func TestListShotManifestsByScene_IncludesValidationScore(t *testing.T) {
	s := setupTestStore(t)
	require.NoError(t, s.CreateProject(&domain.Project{ID: "p1", SCPID: "SCP-1", Status: "pending", WorkspacePath: "/w"}))
	require.NoError(t, s.CreateShotManifest(&domain.ShotManifest{
		ProjectID: "p1", SceneNum: 1, ShotNum: 1, SentenceStart: 1, SentenceEnd: 1, CutNum: 1,
		ContentHash: "h1", ImageHash: "ih1", GenMethod: "text_to_image", Status: "generated",
	}))
	require.NoError(t, s.CreateShotManifest(&domain.ShotManifest{
		ProjectID: "p1", SceneNum: 1, ShotNum: 2, SentenceStart: 2, SentenceEnd: 2, CutNum: 1,
		ContentHash: "h2", ImageHash: "ih2", GenMethod: "text_to_image", Status: "generated",
	}))

	// Validate only first shot
	require.NoError(t, s.UpdateValidationScore("p1", 1, 1, 1, 90))

	manifests, err := s.ListShotManifestsByScene("p1", 1)
	require.NoError(t, err)
	require.Len(t, manifests, 2)

	require.NotNil(t, manifests[0].ValidationScore)
	assert.Equal(t, 90, *manifests[0].ValidationScore)
	assert.Nil(t, manifests[1].ValidationScore)
}

func TestGetManifest_TimestampParsed(t *testing.T) {
	s := setupTestStore(t)
	require.NoError(t, s.CreateProject(&domain.Project{ID: "p1", SCPID: "SCP-1", Status: "pending", WorkspacePath: "/w"}))
	require.NoError(t, s.CreateManifest(&domain.SceneManifest{ProjectID: "p1", SceneNum: 1, Status: "pending"}))

	got, err := s.GetManifest("p1", 1)
	require.NoError(t, err)
	assert.False(t, got.UpdatedAt.IsZero())
}
