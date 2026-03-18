package store

import (
	"testing"

	"github.com/sushistack/yt.pipe/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupApprovalTestStore(t *testing.T) *Store {
	t.Helper()
	s := setupTestStore(t)
	require.NoError(t, s.CreateProject(&domain.Project{
		ID: "p1", SCPID: "SCP-173", Status: domain.StagePending, WorkspacePath: "/w",
	}))
	return s
}

func TestInitApproval_Success(t *testing.T) {
	s := setupApprovalTestStore(t)

	err := s.InitApproval("p1", 1, domain.AssetTypeImage)
	require.NoError(t, err)

	got, err := s.GetApproval("p1", 1, domain.AssetTypeImage)
	require.NoError(t, err)
	assert.Equal(t, "p1", got.ProjectID)
	assert.Equal(t, 1, got.SceneNum)
	assert.Equal(t, domain.AssetTypeImage, got.AssetType)
	assert.Equal(t, domain.ApprovalPending, got.Status)
	assert.Equal(t, 0, got.Attempts)
}

func TestInitApproval_InvalidAssetType(t *testing.T) {
	s := setupApprovalTestStore(t)
	err := s.InitApproval("p1", 1, "video")
	assert.Error(t, err)
}

func TestMarkGenerated_Success(t *testing.T) {
	s := setupApprovalTestStore(t)
	require.NoError(t, s.InitApproval("p1", 1, domain.AssetTypeImage))

	err := s.MarkGenerated("p1", 1, domain.AssetTypeImage)
	require.NoError(t, err)

	got, _ := s.GetApproval("p1", 1, domain.AssetTypeImage)
	assert.Equal(t, domain.ApprovalGenerated, got.Status)
	assert.Equal(t, 1, got.Attempts)
}

func TestMarkGenerated_NotFound(t *testing.T) {
	s := setupApprovalTestStore(t)
	err := s.MarkGenerated("p1", 99, domain.AssetTypeImage)
	assert.IsType(t, &domain.NotFoundError{}, err)
}

func TestApproveScene_Success(t *testing.T) {
	s := setupApprovalTestStore(t)
	require.NoError(t, s.InitApproval("p1", 1, domain.AssetTypeImage))
	require.NoError(t, s.MarkGenerated("p1", 1, domain.AssetTypeImage))

	err := s.ApproveScene("p1", 1, domain.AssetTypeImage)
	require.NoError(t, err)

	got, _ := s.GetApproval("p1", 1, domain.AssetTypeImage)
	assert.Equal(t, domain.ApprovalApproved, got.Status)
}

func TestRejectScene_Success(t *testing.T) {
	s := setupApprovalTestStore(t)
	require.NoError(t, s.InitApproval("p1", 1, domain.AssetTypeTTS))
	require.NoError(t, s.MarkGenerated("p1", 1, domain.AssetTypeTTS))

	err := s.RejectScene("p1", 1, domain.AssetTypeTTS)
	require.NoError(t, err)

	got, _ := s.GetApproval("p1", 1, domain.AssetTypeTTS)
	assert.Equal(t, domain.ApprovalRejected, got.Status)
}

func TestRejectAndRegenerate_Cycle(t *testing.T) {
	s := setupApprovalTestStore(t)
	require.NoError(t, s.InitApproval("p1", 1, domain.AssetTypeImage))

	// First attempt: generate → reject
	require.NoError(t, s.MarkGenerated("p1", 1, domain.AssetTypeImage))
	require.NoError(t, s.RejectScene("p1", 1, domain.AssetTypeImage))

	// Second attempt: regenerate → approve
	require.NoError(t, s.MarkGenerated("p1", 1, domain.AssetTypeImage))
	require.NoError(t, s.ApproveScene("p1", 1, domain.AssetTypeImage))

	got, _ := s.GetApproval("p1", 1, domain.AssetTypeImage)
	assert.Equal(t, domain.ApprovalApproved, got.Status)
	assert.Equal(t, 2, got.Attempts)
}

func TestGetApproval_NotFound(t *testing.T) {
	s := setupApprovalTestStore(t)
	_, err := s.GetApproval("p1", 99, domain.AssetTypeImage)
	assert.IsType(t, &domain.NotFoundError{}, err)
}

func TestListApprovalsByProject_Success(t *testing.T) {
	s := setupApprovalTestStore(t)
	require.NoError(t, s.InitApproval("p1", 1, domain.AssetTypeImage))
	require.NoError(t, s.InitApproval("p1", 2, domain.AssetTypeImage))
	require.NoError(t, s.InitApproval("p1", 1, domain.AssetTypeTTS))

	// List only image approvals
	images, err := s.ListApprovalsByProject("p1", domain.AssetTypeImage)
	require.NoError(t, err)
	assert.Len(t, images, 2)
	assert.Equal(t, 1, images[0].SceneNum)
	assert.Equal(t, 2, images[1].SceneNum)

	// List only TTS approvals
	ttsApprovals, err := s.ListApprovalsByProject("p1", domain.AssetTypeTTS)
	require.NoError(t, err)
	assert.Len(t, ttsApprovals, 1)
}

func TestAllApproved_AllApproved(t *testing.T) {
	s := setupApprovalTestStore(t)
	require.NoError(t, s.InitApproval("p1", 1, domain.AssetTypeImage))
	require.NoError(t, s.InitApproval("p1", 2, domain.AssetTypeImage))

	// Mark all as generated then approved
	require.NoError(t, s.MarkGenerated("p1", 1, domain.AssetTypeImage))
	require.NoError(t, s.ApproveScene("p1", 1, domain.AssetTypeImage))
	require.NoError(t, s.MarkGenerated("p1", 2, domain.AssetTypeImage))
	require.NoError(t, s.ApproveScene("p1", 2, domain.AssetTypeImage))

	ok, err := s.AllApproved("p1", domain.AssetTypeImage)
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestAllApproved_NotAllApproved(t *testing.T) {
	s := setupApprovalTestStore(t)
	require.NoError(t, s.InitApproval("p1", 1, domain.AssetTypeImage))
	require.NoError(t, s.InitApproval("p1", 2, domain.AssetTypeImage))

	// Only approve scene 1
	require.NoError(t, s.MarkGenerated("p1", 1, domain.AssetTypeImage))
	require.NoError(t, s.ApproveScene("p1", 1, domain.AssetTypeImage))

	ok, err := s.AllApproved("p1", domain.AssetTypeImage)
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestAllApproved_NoRecords(t *testing.T) {
	s := setupApprovalTestStore(t)
	ok, err := s.AllApproved("p1", domain.AssetTypeImage)
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestListSceneValidationScores_WithScores(t *testing.T) {
	s := setupApprovalTestStore(t)
	// Scene 1: one shot with score 90
	require.NoError(t, s.InitApproval("p1", 1, domain.AssetTypeImage))
	require.NoError(t, s.MarkGenerated("p1", 1, domain.AssetTypeImage))
	require.NoError(t, s.CreateShotManifest(&domain.ShotManifest{
		ProjectID: "p1", SceneNum: 1, ShotNum: 1, SentenceStart: 1, SentenceEnd: 1, CutNum: 1, Status: "generated",
	}))
	require.NoError(t, s.UpdateValidationScore("p1", 1, 1, 1, 90))

	// Scene 2: two shots with scores 85, 70 → min=70
	require.NoError(t, s.InitApproval("p1", 2, domain.AssetTypeImage))
	require.NoError(t, s.MarkGenerated("p1", 2, domain.AssetTypeImage))
	require.NoError(t, s.CreateShotManifest(&domain.ShotManifest{
		ProjectID: "p1", SceneNum: 2, ShotNum: 1, SentenceStart: 1, SentenceEnd: 1, CutNum: 1, Status: "generated",
	}))
	require.NoError(t, s.UpdateValidationScore("p1", 2, 1, 1, 85))
	require.NoError(t, s.CreateShotManifest(&domain.ShotManifest{
		ProjectID: "p1", SceneNum: 2, ShotNum: 2, SentenceStart: 2, SentenceEnd: 2, CutNum: 1, Status: "generated",
	}))
	require.NoError(t, s.UpdateValidationScore("p1", 2, 2, 1, 70))

	results, err := s.ListSceneValidationScores("p1", domain.AssetTypeImage)
	require.NoError(t, err)
	require.Len(t, results, 2)

	assert.Equal(t, 1, results[0].SceneNum)
	require.NotNil(t, results[0].ValidationScore)
	assert.Equal(t, 90, *results[0].ValidationScore)

	assert.Equal(t, 2, results[1].SceneNum)
	require.NotNil(t, results[1].ValidationScore)
	assert.Equal(t, 70, *results[1].ValidationScore)
}

func TestListSceneValidationScores_NullScore(t *testing.T) {
	s := setupApprovalTestStore(t)
	// Scene with shot but no validation score
	require.NoError(t, s.InitApproval("p1", 1, domain.AssetTypeImage))
	require.NoError(t, s.MarkGenerated("p1", 1, domain.AssetTypeImage))
	require.NoError(t, s.CreateShotManifest(&domain.ShotManifest{
		ProjectID: "p1", SceneNum: 1, ShotNum: 1, SentenceStart: 1, SentenceEnd: 1, CutNum: 1, Status: "generated",
	}))

	results, err := s.ListSceneValidationScores("p1", domain.AssetTypeImage)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Nil(t, results[0].ValidationScore)
}

func TestListSceneValidationScores_NoShots(t *testing.T) {
	s := setupApprovalTestStore(t)
	// Scene with approval but no shot manifests at all
	require.NoError(t, s.InitApproval("p1", 1, domain.AssetTypeImage))
	require.NoError(t, s.MarkGenerated("p1", 1, domain.AssetTypeImage))

	results, err := s.ListSceneValidationScores("p1", domain.AssetTypeImage)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Nil(t, results[0].ValidationScore) // No shots → NULL
}

func TestListSceneValidationScores_OnlyGeneratedScenes(t *testing.T) {
	s := setupApprovalTestStore(t)
	// Scene 1: approved (should be excluded)
	require.NoError(t, s.InitApproval("p1", 1, domain.AssetTypeImage))
	require.NoError(t, s.MarkGenerated("p1", 1, domain.AssetTypeImage))
	require.NoError(t, s.ApproveScene("p1", 1, domain.AssetTypeImage))

	// Scene 2: generated (should be included)
	require.NoError(t, s.InitApproval("p1", 2, domain.AssetTypeImage))
	require.NoError(t, s.MarkGenerated("p1", 2, domain.AssetTypeImage))

	results, err := s.ListSceneValidationScores("p1", domain.AssetTypeImage)
	require.NoError(t, err)
	require.Len(t, results, 1) // Only scene 2
	assert.Equal(t, 2, results[0].SceneNum)
}

func TestBulkApproveAll_Success(t *testing.T) {
	s := setupApprovalTestStore(t)
	require.NoError(t, s.InitApproval("p1", 1, domain.AssetTypeImage))
	require.NoError(t, s.InitApproval("p1", 2, domain.AssetTypeImage))
	require.NoError(t, s.InitApproval("p1", 3, domain.AssetTypeImage))

	affected, err := s.BulkApproveAll("p1", domain.AssetTypeImage)
	require.NoError(t, err)
	assert.Equal(t, int64(3), affected)

	ok, err := s.AllApproved("p1", domain.AssetTypeImage)
	require.NoError(t, err)
	assert.True(t, ok)
}
