package service

import (
	"log/slog"
	"testing"

	"github.com/sushistack/yt.pipe/internal/domain"
	"github.com/sushistack/yt.pipe/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupDashboardService(t *testing.T) (*SceneDashboardService, *store.Store) {
	t.Helper()
	s, err := store.New(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { s.Close() })

	require.NoError(t, s.CreateProject(&domain.Project{
		ID: "p1", SCPID: "SCP-173", Status: domain.StageImages,
		SceneCount: 3, WorkspacePath: "/tmp/ws",
	}))

	logger := slog.Default()
	return NewSceneDashboardService(s, logger), s
}

func TestGetDashboard_WithApprovals(t *testing.T) {
	dashSvc, s := setupDashboardService(t)

	// Create manifests for scenes
	require.NoError(t, s.CreateManifest(&domain.SceneManifest{ProjectID: "p1", SceneNum: 1, Status: "current"}))
	require.NoError(t, s.CreateManifest(&domain.SceneManifest{ProjectID: "p1", SceneNum: 2, Status: "current"}))
	require.NoError(t, s.CreateManifest(&domain.SceneManifest{ProjectID: "p1", SceneNum: 3, Status: "current"}))

	// Create image approvals
	require.NoError(t, s.InitApproval("p1", 1, domain.AssetTypeImage))
	require.NoError(t, s.InitApproval("p1", 2, domain.AssetTypeImage))
	require.NoError(t, s.InitApproval("p1", 3, domain.AssetTypeImage))
	require.NoError(t, s.MarkGenerated("p1", 1, domain.AssetTypeImage))
	require.NoError(t, s.ApproveScene("p1", 1, domain.AssetTypeImage))
	require.NoError(t, s.MarkGenerated("p1", 2, domain.AssetTypeImage))

	dashboard, err := dashSvc.GetDashboard("p1")
	require.NoError(t, err)

	assert.Equal(t, "p1", dashboard.ProjectID)
	assert.Equal(t, domain.StageImages, dashboard.ProjectStatus)
	assert.Len(t, dashboard.Scenes, 3)

	// Scene 1: approved
	assert.Equal(t, domain.ApprovalApproved, dashboard.Scenes[0].ImageStatus)
	assert.True(t, dashboard.Scenes[0].ImageApproved)
	// Scene 2: generated
	assert.Equal(t, domain.ApprovalGenerated, dashboard.Scenes[1].ImageStatus)
	assert.False(t, dashboard.Scenes[1].ImageApproved)
	// Scene 3: pending
	assert.Equal(t, domain.ApprovalPending, dashboard.Scenes[2].ImageStatus)
	assert.False(t, dashboard.Scenes[2].ImageApproved)

	// Image summary should be present since project is in image_review
	require.NotNil(t, dashboard.ImageSummary)
	assert.Equal(t, 3, dashboard.ImageSummary.Total)
	assert.Equal(t, 1, dashboard.ImageSummary.Approved)

	// n8n aggregate flags
	assert.Equal(t, 3, dashboard.TotalScenes)
	assert.Equal(t, 1, dashboard.ApprovedImageCount)
	assert.False(t, dashboard.AllImagesApproved)
	assert.False(t, dashboard.AllApproved)
}

func TestGetDashboard_NoApprovals(t *testing.T) {
	dashSvc, s := setupDashboardService(t)

	require.NoError(t, s.CreateManifest(&domain.SceneManifest{ProjectID: "p1", SceneNum: 1, Status: "current"}))

	dashboard, err := dashSvc.GetDashboard("p1")
	require.NoError(t, err)

	assert.Len(t, dashboard.Scenes, 3)
	assert.Equal(t, "none", dashboard.Scenes[0].ImageStatus)
	assert.False(t, dashboard.Scenes[0].ImageApproved)
	assert.Equal(t, 3, dashboard.TotalScenes)
	assert.Equal(t, 0, dashboard.ApprovedImageCount)
	assert.Equal(t, 0, dashboard.ApprovedTTSCount)
	assert.False(t, dashboard.AllImagesApproved)
	assert.False(t, dashboard.AllTTSApproved)
	assert.False(t, dashboard.AllApproved)
}

func TestGetDashboard_NotFound(t *testing.T) {
	dashSvc, _ := setupDashboardService(t)
	_, err := dashSvc.GetDashboard("nonexistent")
	assert.Error(t, err)
}

func TestGetDashboard_AllImagesApprovedTTSPending(t *testing.T) {
	dashSvc, s := setupDashboardService(t)

	// Create manifests
	for i := 1; i <= 3; i++ {
		require.NoError(t, s.CreateManifest(&domain.SceneManifest{ProjectID: "p1", SceneNum: i, Status: "current"}))
	}

	// Approve all images
	for i := 1; i <= 3; i++ {
		require.NoError(t, s.InitApproval("p1", i, domain.AssetTypeImage))
		require.NoError(t, s.MarkGenerated("p1", i, domain.AssetTypeImage))
		require.NoError(t, s.ApproveScene("p1", i, domain.AssetTypeImage))
	}

	// TTS: only scene 1 approved, scene 2 generated, scene 3 pending
	require.NoError(t, s.InitApproval("p1", 1, domain.AssetTypeTTS))
	require.NoError(t, s.InitApproval("p1", 2, domain.AssetTypeTTS))
	require.NoError(t, s.InitApproval("p1", 3, domain.AssetTypeTTS))
	require.NoError(t, s.MarkGenerated("p1", 1, domain.AssetTypeTTS))
	require.NoError(t, s.ApproveScene("p1", 1, domain.AssetTypeTTS))
	require.NoError(t, s.MarkGenerated("p1", 2, domain.AssetTypeTTS))

	dashboard, err := dashSvc.GetDashboard("p1")
	require.NoError(t, err)

	// All images approved
	assert.True(t, dashboard.AllImagesApproved)
	assert.Equal(t, 3, dashboard.ApprovedImageCount)

	// TTS not all approved
	assert.False(t, dashboard.AllTTSApproved)
	assert.Equal(t, 1, dashboard.ApprovedTTSCount)

	// Overall not all approved
	assert.False(t, dashboard.AllApproved)

	// Per-scene checks
	for _, scene := range dashboard.Scenes {
		assert.True(t, scene.ImageApproved)
	}
	assert.True(t, dashboard.Scenes[0].TTSApproved)
	assert.False(t, dashboard.Scenes[1].TTSApproved)
	assert.False(t, dashboard.Scenes[2].TTSApproved)
}

func TestGetDashboard_AllApproved(t *testing.T) {
	dashSvc, s := setupDashboardService(t)

	for i := 1; i <= 3; i++ {
		require.NoError(t, s.CreateManifest(&domain.SceneManifest{ProjectID: "p1", SceneNum: i, Status: "current"}))
	}

	// Approve all images and TTS
	for i := 1; i <= 3; i++ {
		require.NoError(t, s.InitApproval("p1", i, domain.AssetTypeImage))
		require.NoError(t, s.MarkGenerated("p1", i, domain.AssetTypeImage))
		require.NoError(t, s.ApproveScene("p1", i, domain.AssetTypeImage))

		require.NoError(t, s.InitApproval("p1", i, domain.AssetTypeTTS))
		require.NoError(t, s.MarkGenerated("p1", i, domain.AssetTypeTTS))
		require.NoError(t, s.ApproveScene("p1", i, domain.AssetTypeTTS))
	}

	dashboard, err := dashSvc.GetDashboard("p1")
	require.NoError(t, err)

	assert.True(t, dashboard.AllImagesApproved)
	assert.True(t, dashboard.AllTTSApproved)
	assert.True(t, dashboard.AllApproved)
	assert.Equal(t, 3, dashboard.TotalScenes)
	assert.Equal(t, 3, dashboard.ApprovedImageCount)
	assert.Equal(t, 3, dashboard.ApprovedTTSCount)
}

func TestGetDashboard_RejectResetsApprovalFlag(t *testing.T) {
	dashSvc, s := setupDashboardService(t)

	for i := 1; i <= 3; i++ {
		require.NoError(t, s.CreateManifest(&domain.SceneManifest{ProjectID: "p1", SceneNum: i, Status: "current"}))
	}

	// Approve all images
	for i := 1; i <= 3; i++ {
		require.NoError(t, s.InitApproval("p1", i, domain.AssetTypeImage))
		require.NoError(t, s.MarkGenerated("p1", i, domain.AssetTypeImage))
		require.NoError(t, s.ApproveScene("p1", i, domain.AssetTypeImage))
	}

	// Verify all_images_approved is true
	dashboard, err := dashSvc.GetDashboard("p1")
	require.NoError(t, err)
	assert.True(t, dashboard.AllImagesApproved)

	// Now reject scene 2's image (need to transition: approved cannot go to rejected directly)
	// The rejection flow uses the ApprovalService which checks transitions.
	// In reality, a "reject" after approve would require a special reset.
	// For this test, we simulate by directly setting the DB status.
	// But since the state machine says approved -> nothing is allowed,
	// we test the scenario where a scene is generated but not yet approved,
	// then rejected.

	// Reset: create a new project with a fresh approval set
	require.NoError(t, s.CreateProject(&domain.Project{
		ID: "p2", SCPID: "SCP-999", Status: domain.StageImages,
		SceneCount: 2, WorkspacePath: "/tmp/ws2",
	}))
	for i := 1; i <= 2; i++ {
		require.NoError(t, s.CreateManifest(&domain.SceneManifest{ProjectID: "p2", SceneNum: i, Status: "current"}))
		require.NoError(t, s.InitApproval("p2", i, domain.AssetTypeImage))
		require.NoError(t, s.MarkGenerated("p2", i, domain.AssetTypeImage))
	}
	// Approve scene 1, reject scene 2
	require.NoError(t, s.ApproveScene("p2", 1, domain.AssetTypeImage))
	require.NoError(t, s.RejectScene("p2", 2, domain.AssetTypeImage))

	dashSvc2 := NewSceneDashboardService(s, slog.Default())
	dashboard2, err := dashSvc2.GetDashboard("p2")
	require.NoError(t, err)

	assert.True(t, dashboard2.Scenes[0].ImageApproved)
	assert.False(t, dashboard2.Scenes[1].ImageApproved)
	assert.Equal(t, 1, dashboard2.ApprovedImageCount)
	assert.False(t, dashboard2.AllImagesApproved)
	assert.False(t, dashboard2.AllApproved)
}

func TestGetDashboard_RejectRegenerateApproveCycle(t *testing.T) {
	s, err := store.New(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { s.Close() })

	require.NoError(t, s.CreateProject(&domain.Project{
		ID: "p1", SCPID: "SCP-173", Status: domain.StageImages,
		SceneCount: 2, WorkspacePath: "/tmp/ws",
	}))

	for i := 1; i <= 2; i++ {
		require.NoError(t, s.CreateManifest(&domain.SceneManifest{ProjectID: "p1", SceneNum: i, Status: "current"}))
		require.NoError(t, s.InitApproval("p1", i, domain.AssetTypeImage))
		require.NoError(t, s.InitApproval("p1", i, domain.AssetTypeTTS))
	}

	dashSvc := NewSceneDashboardService(s, slog.Default())
	approvalSvc := NewApprovalService(s, slog.Default())

	// Step 1: Generate all
	for i := 1; i <= 2; i++ {
		require.NoError(t, approvalSvc.MarkGenerated("p1", i, domain.AssetTypeImage))
		require.NoError(t, approvalSvc.MarkGenerated("p1", i, domain.AssetTypeTTS))
	}

	// Step 2: Approve scene 1, reject scene 2 image
	require.NoError(t, approvalSvc.ApproveScene("p1", 1, domain.AssetTypeImage))
	require.NoError(t, approvalSvc.ApproveScene("p1", 1, domain.AssetTypeTTS))
	require.NoError(t, approvalSvc.RejectScene("p1", 2, domain.AssetTypeImage))
	require.NoError(t, approvalSvc.ApproveScene("p1", 2, domain.AssetTypeTTS))

	dashboard, err := dashSvc.GetDashboard("p1")
	require.NoError(t, err)
	assert.False(t, dashboard.AllImagesApproved, "scene 2 image rejected")
	assert.True(t, dashboard.AllTTSApproved, "all TTS approved")
	assert.False(t, dashboard.AllApproved)
	assert.Equal(t, 1, dashboard.ApprovedImageCount)

	// Step 3: Regenerate scene 2 image
	require.NoError(t, approvalSvc.MarkGenerated("p1", 2, domain.AssetTypeImage))

	dashboard, err = dashSvc.GetDashboard("p1")
	require.NoError(t, err)
	assert.False(t, dashboard.Scenes[1].ImageApproved, "scene 2 regenerated but not re-approved")
	assert.False(t, dashboard.AllImagesApproved)

	// Step 4: Re-approve scene 2 image
	require.NoError(t, approvalSvc.ApproveScene("p1", 2, domain.AssetTypeImage))

	dashboard, err = dashSvc.GetDashboard("p1")
	require.NoError(t, err)
	assert.True(t, dashboard.Scenes[1].ImageApproved, "scene 2 re-approved")
	assert.True(t, dashboard.AllImagesApproved)
	assert.True(t, dashboard.AllTTSApproved)
	assert.True(t, dashboard.AllApproved)
	assert.Equal(t, 2, dashboard.ApprovedImageCount)
	assert.Equal(t, 2, dashboard.ApprovedTTSCount)
	assert.Equal(t, 2, dashboard.TotalScenes)
}

func TestGetDashboard_SceneAssets(t *testing.T) {
	dashSvc, s := setupDashboardService(t)

	require.NoError(t, s.CreateManifest(&domain.SceneManifest{ProjectID: "p1", SceneNum: 1, Status: "current"}))

	dashboard, err := dashSvc.GetDashboard("p1")
	require.NoError(t, err)

	scene := dashboard.Scenes[0]
	require.NotNil(t, scene.Assets)
	assert.Equal(t, "/tmp/ws/scenes/1/image.png", scene.Assets.ImagePath)
	assert.Equal(t, "/tmp/ws/scenes/1/audio.wav", scene.Assets.AudioPath)
	assert.Equal(t, "/tmp/ws/scenes/1/subtitle.srt", scene.Assets.SubtitlePath)
}

func TestGetDashboard_SummariesAlwaysPresent(t *testing.T) {
	// Test that summaries are computed even when project is NOT in review state
	s, err := store.New(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { s.Close() })

	// Project in "approved" status (not image_review or tts_review)
	require.NoError(t, s.CreateProject(&domain.Project{
		ID: "p1", SCPID: "SCP-173", Status: domain.StageScenario,
		SceneCount: 1, WorkspacePath: "/tmp/ws",
	}))
	require.NoError(t, s.CreateManifest(&domain.SceneManifest{ProjectID: "p1", SceneNum: 1, Status: "current"}))
	require.NoError(t, s.InitApproval("p1", 1, domain.AssetTypeImage))

	dashSvc := NewSceneDashboardService(s, slog.Default())
	dashboard, err := dashSvc.GetDashboard("p1")
	require.NoError(t, err)

	// Summaries should still be present for n8n polling
	require.NotNil(t, dashboard.ImageSummary)
	assert.Equal(t, 1, dashboard.ImageSummary.Total)
}
