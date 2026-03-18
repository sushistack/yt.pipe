package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/sushistack/yt.pipe/internal/domain"
	"github.com/sushistack/yt.pipe/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupApprovalService(t *testing.T) (*ApprovalService, *store.Store) {
	t.Helper()
	s, err := store.New(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { s.Close() })

	require.NoError(t, s.CreateProject(&domain.Project{
		ID: "p1", SCPID: "SCP-173", Status: domain.StagePending, WorkspacePath: "/w",
	}))

	logger := slog.Default()
	return NewApprovalService(s, logger), s
}

func TestInitApprovals_Success(t *testing.T) {
	svc, s := setupApprovalService(t)

	err := svc.InitApprovals("p1", 3, domain.AssetTypeImage)
	require.NoError(t, err)

	approvals, err := s.ListApprovalsByProject("p1", domain.AssetTypeImage)
	require.NoError(t, err)
	assert.Len(t, approvals, 3)
	for i, a := range approvals {
		assert.Equal(t, i+1, a.SceneNum)
		assert.Equal(t, domain.ApprovalPending, a.Status)
	}
}

func TestMarkGenerated_ValidTransition(t *testing.T) {
	svc, _ := setupApprovalService(t)
	require.NoError(t, svc.InitApprovals("p1", 1, domain.AssetTypeImage))

	err := svc.MarkGenerated("p1", 1, domain.AssetTypeImage)
	require.NoError(t, err)
}

func TestMarkGenerated_InvalidTransition(t *testing.T) {
	svc, s := setupApprovalService(t)
	require.NoError(t, svc.InitApprovals("p1", 1, domain.AssetTypeImage))

	// Mark generated then approved
	require.NoError(t, s.MarkGenerated("p1", 1, domain.AssetTypeImage))
	require.NoError(t, s.ApproveScene("p1", 1, domain.AssetTypeImage))

	// Cannot mark generated after approved
	err := svc.MarkGenerated("p1", 1, domain.AssetTypeImage)
	assert.Error(t, err)
	assert.IsType(t, &domain.TransitionError{}, err)
}

func TestApproveScene_ValidTransition(t *testing.T) {
	svc, _ := setupApprovalService(t)
	require.NoError(t, svc.InitApprovals("p1", 1, domain.AssetTypeTTS))
	require.NoError(t, svc.MarkGenerated("p1", 1, domain.AssetTypeTTS))

	err := svc.ApproveScene("p1", 1, domain.AssetTypeTTS)
	require.NoError(t, err)
}

func TestApproveScene_InvalidTransition(t *testing.T) {
	svc, _ := setupApprovalService(t)
	require.NoError(t, svc.InitApprovals("p1", 1, domain.AssetTypeImage))

	// Cannot approve from pending (must be generated first)
	err := svc.ApproveScene("p1", 1, domain.AssetTypeImage)
	assert.Error(t, err)
	assert.IsType(t, &domain.TransitionError{}, err)
}

func TestRejectScene_ValidTransition(t *testing.T) {
	svc, _ := setupApprovalService(t)
	require.NoError(t, svc.InitApprovals("p1", 1, domain.AssetTypeImage))
	require.NoError(t, svc.MarkGenerated("p1", 1, domain.AssetTypeImage))

	err := svc.RejectScene("p1", 1, domain.AssetTypeImage)
	require.NoError(t, err)
}

func TestRejectScene_InvalidTransition(t *testing.T) {
	svc, _ := setupApprovalService(t)
	require.NoError(t, svc.InitApprovals("p1", 1, domain.AssetTypeImage))

	// Cannot reject from pending
	err := svc.RejectScene("p1", 1, domain.AssetTypeImage)
	assert.Error(t, err)
	assert.IsType(t, &domain.TransitionError{}, err)
}

func TestRejectAndRegenerate_FullCycle(t *testing.T) {
	svc, _ := setupApprovalService(t)
	require.NoError(t, svc.InitApprovals("p1", 1, domain.AssetTypeImage))

	// Generate → reject → regenerate → approve
	require.NoError(t, svc.MarkGenerated("p1", 1, domain.AssetTypeImage))
	require.NoError(t, svc.RejectScene("p1", 1, domain.AssetTypeImage))
	require.NoError(t, svc.MarkGenerated("p1", 1, domain.AssetTypeImage))
	require.NoError(t, svc.ApproveScene("p1", 1, domain.AssetTypeImage))
}

func TestAutoApproveAll_Success(t *testing.T) {
	svc, _ := setupApprovalService(t)
	require.NoError(t, svc.InitApprovals("p1", 3, domain.AssetTypeImage))

	err := svc.AutoApproveAll("p1", domain.AssetTypeImage)
	require.NoError(t, err)

	ok, err := svc.AllApproved("p1", domain.AssetTypeImage)
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestAllApproved_PartialApproval(t *testing.T) {
	svc, _ := setupApprovalService(t)
	require.NoError(t, svc.InitApprovals("p1", 2, domain.AssetTypeImage))

	// Only approve scene 1
	require.NoError(t, svc.MarkGenerated("p1", 1, domain.AssetTypeImage))
	require.NoError(t, svc.ApproveScene("p1", 1, domain.AssetTypeImage))

	ok, err := svc.AllApproved("p1", domain.AssetTypeImage)
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestGetApprovalStatus_Summary(t *testing.T) {
	svc, _ := setupApprovalService(t)
	require.NoError(t, svc.InitApprovals("p1", 4, domain.AssetTypeImage))

	// Scene 1: approved
	require.NoError(t, svc.MarkGenerated("p1", 1, domain.AssetTypeImage))
	require.NoError(t, svc.ApproveScene("p1", 1, domain.AssetTypeImage))
	// Scene 2: generated (awaiting review)
	require.NoError(t, svc.MarkGenerated("p1", 2, domain.AssetTypeImage))
	// Scene 3: rejected
	require.NoError(t, svc.MarkGenerated("p1", 3, domain.AssetTypeImage))
	require.NoError(t, svc.RejectScene("p1", 3, domain.AssetTypeImage))
	// Scene 4: pending

	status, err := svc.GetApprovalStatus("p1", domain.AssetTypeImage)
	require.NoError(t, err)
	assert.Equal(t, 4, status.Total)
	assert.Equal(t, 1, status.Approved)
	assert.Equal(t, 1, status.Generated)
	assert.Equal(t, 1, status.Rejected)
	assert.Equal(t, 1, status.Pending)
	assert.False(t, status.AllApproved)
}

// --- AutoApproveByScore tests ---

// setupAutoApproveScenario creates a project with N scenes, each with shot manifests and
// validation scores, and scene approvals in "generated" status.
func setupAutoApproveScenario(t *testing.T, scores [][]int) (*ApprovalService, *store.Store) {
	t.Helper()
	s, err := store.New(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { s.Close() })

	require.NoError(t, s.CreateProject(&domain.Project{
		ID: "p1", SCPID: "SCP-173", Status: domain.StagePending, WorkspacePath: "/w",
	}))

	sceneCount := len(scores)
	svc := NewApprovalService(s, slog.Default())
	require.NoError(t, svc.InitApprovals("p1", sceneCount, domain.AssetTypeImage))

	for sceneNum := 1; sceneNum <= sceneCount; sceneNum++ {
		// Mark generated so auto-approve can work
		require.NoError(t, svc.MarkGenerated("p1", sceneNum, domain.AssetTypeImage))

		// Create shot manifests with validation scores
		for cutIdx, score := range scores[sceneNum-1] {
			shot := &domain.ShotManifest{
				ProjectID:     "p1",
				SceneNum:      sceneNum,
				ShotNum:       cutIdx + 1,
				SentenceStart: cutIdx + 1,
				SentenceEnd:   cutIdx + 1,
				CutNum:        cutIdx + 1,
				Status:        "generated",
			}
			require.NoError(t, s.CreateShotManifest(shot))
			if score >= 0 { // -1 means NULL (no score)
				require.NoError(t, s.UpdateValidationScore("p1", sceneNum, cutIdx+1, cutIdx+1, score))
			}
		}
	}

	return svc, s
}

func TestAutoApproveByScore_AllAboveThreshold(t *testing.T) {
	// 3 scenes, each with 1 shot scoring above 80
	svc, _ := setupAutoApproveScenario(t, [][]int{{90}, {85}, {80}})

	autoApproved, reviewRequired, err := svc.AutoApproveByScore(context.Background(), "p1", domain.AssetTypeImage, 80)
	require.NoError(t, err)
	assert.Equal(t, []int{1, 2, 3}, autoApproved)
	assert.Empty(t, reviewRequired)

	ok, err := svc.AllApproved("p1", domain.AssetTypeImage)
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestAutoApproveByScore_ThresholdBoundary(t *testing.T) {
	// Scene 1: score 79 (below), Scene 2: score 80 (at threshold), Scene 3: score 81 (above)
	svc, _ := setupAutoApproveScenario(t, [][]int{{79}, {80}, {81}})

	autoApproved, reviewRequired, err := svc.AutoApproveByScore(context.Background(), "p1", domain.AssetTypeImage, 80)
	require.NoError(t, err)
	assert.Equal(t, []int{2, 3}, autoApproved)
	assert.Equal(t, []int{1}, reviewRequired)
}

func TestAutoApproveByScore_NullScoreRemains(t *testing.T) {
	// Scene 1: score 90, Scene 2: NULL score (-1 means no validation)
	svc, _ := setupAutoApproveScenario(t, [][]int{{90}, {-1}})

	autoApproved, reviewRequired, err := svc.AutoApproveByScore(context.Background(), "p1", domain.AssetTypeImage, 80)
	require.NoError(t, err)
	assert.Equal(t, []int{1}, autoApproved)
	assert.Equal(t, []int{2}, reviewRequired)
}

func TestAutoApproveByScore_MixedScores(t *testing.T) {
	// Scene 1: shots with scores [90, 70] → min=70 (below 80)
	// Scene 2: shots with scores [85, 90] → min=85 (above 80)
	svc, _ := setupAutoApproveScenario(t, [][]int{{90, 70}, {85, 90}})

	autoApproved, reviewRequired, err := svc.AutoApproveByScore(context.Background(), "p1", domain.AssetTypeImage, 80)
	require.NoError(t, err)
	assert.Equal(t, []int{2}, autoApproved)
	assert.Equal(t, []int{1}, reviewRequired)
}

func TestAutoApproveByScore_NoGeneratedScenes(t *testing.T) {
	// All scenes already approved — no "generated" scenes to process
	s, err := store.New(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { s.Close() })

	require.NoError(t, s.CreateProject(&domain.Project{
		ID: "p1", SCPID: "SCP-173", Status: domain.StagePending, WorkspacePath: "/w",
	}))

	svc := NewApprovalService(s, slog.Default())
	require.NoError(t, svc.InitApprovals("p1", 2, domain.AssetTypeImage))
	// Approve all directly
	require.NoError(t, svc.AutoApproveAll("p1", domain.AssetTypeImage))

	autoApproved, reviewRequired, err := svc.AutoApproveByScore(context.Background(), "p1", domain.AssetTypeImage, 80)
	require.NoError(t, err)
	assert.Empty(t, autoApproved)
	assert.Empty(t, reviewRequired)
}

// --- GetBatchPreview tests ---

// setupBatchPreviewProject creates a project with workspace, scenario file, approvals, and shot manifests.
func setupBatchPreviewProject(t *testing.T, sceneCount int, scenarioScenes []domain.SceneScript, scores map[int]int) (*ApprovalService, *store.Store) {
	t.Helper()
	s, err := store.New(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { s.Close() })

	wsDir := t.TempDir()

	require.NoError(t, s.CreateProject(&domain.Project{
		ID: "p1", SCPID: "SCP-173", Status: domain.StageImages, WorkspacePath: wsDir, SceneCount: sceneCount,
	}))

	// Write scenario.json if scenes provided
	if scenarioScenes != nil {
		scenario := &domain.ScenarioOutput{
			SCPID:  "SCP-173",
			Title:  "Test",
			Scenes: scenarioScenes,
		}
		data, _ := json.Marshal(scenario)
		require.NoError(t, os.WriteFile(filepath.Join(wsDir, "scenario.json"), data, 0o644))
	}

	svc := NewApprovalService(s, slog.Default())

	// Create scene dirs and image placeholders
	for i := 1; i <= sceneCount; i++ {
		sceneDir := filepath.Join(wsDir, "scenes", fmt.Sprintf("%d", i))
		require.NoError(t, os.MkdirAll(sceneDir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(sceneDir, "image.png"), []byte("fake"), 0o644))
	}

	return svc, s
}

func TestGetBatchPreview_FullAssembly(t *testing.T) {
	scenes := []domain.SceneScript{
		{SceneNum: 1, Narration: "첫 번째 문장이다. 두 번째 문장.", Mood: "tense"},
		{SceneNum: 2, Narration: "SCP가 나타났다. 위험하다.", Mood: "horror"},
	}
	svc, s := setupBatchPreviewProject(t, 2, scenes, nil)

	require.NoError(t, svc.InitApprovals("p1", 2, domain.AssetTypeImage))
	require.NoError(t, svc.MarkGenerated("p1", 1, domain.AssetTypeImage))
	require.NoError(t, svc.MarkGenerated("p1", 2, domain.AssetTypeImage))

	// Add shot manifests with validation scores
	require.NoError(t, s.CreateShotManifest(&domain.ShotManifest{
		ProjectID: "p1", SceneNum: 1, ShotNum: 1, SentenceStart: 1, SentenceEnd: 1, CutNum: 1, Status: "generated",
	}))
	require.NoError(t, s.UpdateValidationScore("p1", 1, 1, 1, 90))
	require.NoError(t, s.CreateShotManifest(&domain.ShotManifest{
		ProjectID: "p1", SceneNum: 2, ShotNum: 1, SentenceStart: 1, SentenceEnd: 1, CutNum: 1, Status: "generated",
	}))
	require.NoError(t, s.UpdateValidationScore("p1", 2, 1, 1, 75))

	items, err := svc.GetBatchPreview(context.Background(), "p1", domain.AssetTypeImage)
	require.NoError(t, err)
	require.Len(t, items, 2)

	// Scene 1
	assert.Equal(t, 1, items[0].SceneNum)
	assert.Equal(t, "첫 번째 문장이다.", items[0].NarrationFirst)
	assert.Equal(t, "tense", items[0].Mood)
	assert.Equal(t, domain.ApprovalGenerated, items[0].Status)
	require.NotNil(t, items[0].ValidationScore)
	assert.Equal(t, 90, *items[0].ValidationScore)
	assert.Contains(t, items[0].ImagePath, "scenes/1/image.png")

	// Scene 2
	assert.Equal(t, 2, items[1].SceneNum)
	assert.Equal(t, "SCP가 나타났다.", items[1].NarrationFirst)
	assert.Equal(t, "horror", items[1].Mood)
	require.NotNil(t, items[1].ValidationScore)
	assert.Equal(t, 75, *items[1].ValidationScore)
}

func TestGetBatchPreview_NilValidationScore(t *testing.T) {
	scenes := []domain.SceneScript{
		{SceneNum: 1, Narration: "문장이다.", Mood: "calm"},
	}
	svc, _ := setupBatchPreviewProject(t, 1, scenes, nil)

	require.NoError(t, svc.InitApprovals("p1", 1, domain.AssetTypeImage))
	require.NoError(t, svc.MarkGenerated("p1", 1, domain.AssetTypeImage))
	// No shot manifests → no validation scores

	items, err := svc.GetBatchPreview(context.Background(), "p1", domain.AssetTypeImage)
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Nil(t, items[0].ValidationScore)
	assert.Equal(t, "calm", items[0].Mood)
}

func TestGetBatchPreview_MixedStatuses(t *testing.T) {
	scenes := []domain.SceneScript{
		{SceneNum: 1, Narration: "승인된 씬.", Mood: "calm"},
		{SceneNum: 2, Narration: "생성된 씬.", Mood: "tense"},
		{SceneNum: 3, Narration: "거부된 씬.", Mood: "horror"},
	}
	svc, _ := setupBatchPreviewProject(t, 3, scenes, nil)

	require.NoError(t, svc.InitApprovals("p1", 3, domain.AssetTypeImage))
	require.NoError(t, svc.MarkGenerated("p1", 1, domain.AssetTypeImage))
	require.NoError(t, svc.ApproveScene("p1", 1, domain.AssetTypeImage))
	require.NoError(t, svc.MarkGenerated("p1", 2, domain.AssetTypeImage))
	require.NoError(t, svc.MarkGenerated("p1", 3, domain.AssetTypeImage))
	require.NoError(t, svc.RejectScene("p1", 3, domain.AssetTypeImage))

	items, err := svc.GetBatchPreview(context.Background(), "p1", domain.AssetTypeImage)
	require.NoError(t, err)
	require.Len(t, items, 3)

	assert.Equal(t, domain.ApprovalApproved, items[0].Status)
	assert.Equal(t, domain.ApprovalGenerated, items[1].Status)
	assert.Equal(t, domain.ApprovalRejected, items[2].Status)

	// Verify ordering
	assert.Equal(t, 1, items[0].SceneNum)
	assert.Equal(t, 2, items[1].SceneNum)
	assert.Equal(t, 3, items[2].SceneNum)
}

func TestGetBatchPreview_MissingScenario(t *testing.T) {
	// No scenario file — narration and mood should be empty
	svc, _ := setupBatchPreviewProject(t, 1, nil, nil)

	require.NoError(t, svc.InitApprovals("p1", 1, domain.AssetTypeImage))
	require.NoError(t, svc.MarkGenerated("p1", 1, domain.AssetTypeImage))

	items, err := svc.GetBatchPreview(context.Background(), "p1", domain.AssetTypeImage)
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Empty(t, items[0].NarrationFirst)
	assert.Empty(t, items[0].Mood)
	assert.Equal(t, domain.ApprovalGenerated, items[0].Status)
}

// --- BatchApprove tests ---

func TestBatchApprove_PartialFlagging(t *testing.T) {
	svc, _ := setupApprovalService(t)
	require.NoError(t, svc.InitApprovals("p1", 4, domain.AssetTypeImage))
	for i := 1; i <= 4; i++ {
		require.NoError(t, svc.MarkGenerated("p1", i, domain.AssetTypeImage))
	}

	result, err := svc.BatchApprove(context.Background(), "p1", domain.AssetTypeImage, []int{2, 4})
	require.NoError(t, err)
	assert.Equal(t, 4, result.TotalScenes)
	assert.Equal(t, 2, result.ApprovedCount)
	assert.Equal(t, 2, result.FlaggedCount)
	assert.Equal(t, []int{2, 4}, result.FlaggedScenes)

	// Verify scene 1 and 3 are approved
	ok1, _ := checkSceneStatus(t, svc, "p1", 1, domain.AssetTypeImage, domain.ApprovalApproved)
	assert.True(t, ok1)
	ok3, _ := checkSceneStatus(t, svc, "p1", 3, domain.AssetTypeImage, domain.ApprovalApproved)
	assert.True(t, ok3)

	// Verify scene 2 and 4 remain generated
	ok2, _ := checkSceneStatus(t, svc, "p1", 2, domain.AssetTypeImage, domain.ApprovalGenerated)
	assert.True(t, ok2)
	ok4, _ := checkSceneStatus(t, svc, "p1", 4, domain.AssetTypeImage, domain.ApprovalGenerated)
	assert.True(t, ok4)
}

func TestBatchApprove_NoFlags(t *testing.T) {
	svc, _ := setupApprovalService(t)
	require.NoError(t, svc.InitApprovals("p1", 3, domain.AssetTypeImage))
	for i := 1; i <= 3; i++ {
		require.NoError(t, svc.MarkGenerated("p1", i, domain.AssetTypeImage))
	}

	result, err := svc.BatchApprove(context.Background(), "p1", domain.AssetTypeImage, nil)
	require.NoError(t, err)
	assert.Equal(t, 3, result.ApprovedCount)
	assert.Equal(t, 0, result.FlaggedCount)

	ok, err := svc.AllApproved("p1", domain.AssetTypeImage)
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestBatchApprove_InvalidSceneNumber(t *testing.T) {
	svc, _ := setupApprovalService(t)
	require.NoError(t, svc.InitApprovals("p1", 3, domain.AssetTypeImage))
	for i := 1; i <= 3; i++ {
		require.NoError(t, svc.MarkGenerated("p1", i, domain.AssetTypeImage))
	}

	_, err := svc.BatchApprove(context.Background(), "p1", domain.AssetTypeImage, []int{5})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "scene 5 does not exist")
}

func TestBatchApprove_SkipsNonGenerated(t *testing.T) {
	svc, _ := setupApprovalService(t)
	require.NoError(t, svc.InitApprovals("p1", 3, domain.AssetTypeImage))

	// Scene 1: already approved
	require.NoError(t, svc.MarkGenerated("p1", 1, domain.AssetTypeImage))
	require.NoError(t, svc.ApproveScene("p1", 1, domain.AssetTypeImage))
	// Scene 2: generated (eligible)
	require.NoError(t, svc.MarkGenerated("p1", 2, domain.AssetTypeImage))
	// Scene 3: pending (not eligible)

	result, err := svc.BatchApprove(context.Background(), "p1", domain.AssetTypeImage, nil)
	require.NoError(t, err)
	assert.Equal(t, 1, result.ApprovedCount) // Only scene 2
}

// checkSceneStatus is a test helper to verify a scene's approval status.
func checkSceneStatus(t *testing.T, svc *ApprovalService, projectID string, sceneNum int, assetType, expectedStatus string) (bool, error) {
	t.Helper()
	status, err := svc.GetApprovalStatus(projectID, assetType)
	if err != nil {
		return false, err
	}
	_ = status // Used for debugging
	// Direct check via store
	approvals, err := svc.store.ListApprovalsByProject(projectID, assetType)
	if err != nil {
		return false, err
	}
	for _, a := range approvals {
		if a.SceneNum == sceneNum {
			return a.Status == expectedStatus, nil
		}
	}
	return false, fmt.Errorf("scene %d not found", sceneNum)
}
