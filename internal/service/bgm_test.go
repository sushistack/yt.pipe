package service

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/sushistack/yt.pipe/internal/domain"
	"github.com/sushistack/yt.pipe/internal/plugin/llm"
	"github.com/sushistack/yt.pipe/internal/store"
)

// mockLLMForBGM implements llm.LLM for BGM service testing.
type mockLLMForBGM struct {
	mock.Mock
}

func (m *mockLLMForBGM) Complete(ctx context.Context, messages []llm.Message, opts llm.CompletionOptions) (*llm.CompletionResult, error) {
	args := m.Called(ctx, messages, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*llm.CompletionResult), args.Error(1)
}

func (m *mockLLMForBGM) GenerateScenario(ctx context.Context, scpID string, mainText string, facts []domain.FactTag, metadata map[string]string) (*domain.ScenarioOutput, error) {
	return nil, nil
}

func (m *mockLLMForBGM) RegenerateSection(ctx context.Context, scenario *domain.ScenarioOutput, sceneNum int, instruction string) (*domain.SceneScript, error) {
	return nil, nil
}

func setupBGMService(t *testing.T) (*BGMService, *store.Store, *mockLLMForBGM) {
	t.Helper()
	s, err := store.New(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { s.Close() })

	mockLLM := &mockLLMForBGM{}
	svc := NewBGMService(s, mockLLM)
	return svc, s, mockLLM
}

func createTempFile(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.mp3")
	require.NoError(t, os.WriteFile(path, []byte("fake-audio"), 0644))
	return path
}

func TestBGMService_CreateBGM_Success(t *testing.T) {
	svc, _, _ := setupBGMService(t)
	path := createTempFile(t)

	bgm, err := svc.CreateBGM("Test BGM", path, []string{"epic", "dark"}, 120000, domain.LicenseRoyaltyFree, "https://example.com", "Test Artist")
	require.NoError(t, err)
	assert.NotEmpty(t, bgm.ID)
	assert.Equal(t, "Test BGM", bgm.Name)
	assert.Equal(t, []string{"epic", "dark"}, bgm.MoodTags)
}

func TestBGMService_CreateBGM_EmptyName(t *testing.T) {
	svc, _, _ := setupBGMService(t)
	_, err := svc.CreateBGM("", "/path", nil, 0, domain.LicenseCustom, "", "")
	assert.Error(t, err)
	assert.IsType(t, &domain.ValidationError{}, err)
}

func TestBGMService_CreateBGM_FileNotFound(t *testing.T) {
	svc, _, _ := setupBGMService(t)
	_, err := svc.CreateBGM("Test", "/nonexistent/file.mp3", nil, 0, domain.LicenseCustom, "", "")
	assert.Error(t, err)
	assert.IsType(t, &domain.ValidationError{}, err)
}

func TestBGMService_CreateBGM_InvalidLicense(t *testing.T) {
	svc, _, _ := setupBGMService(t)
	path := createTempFile(t)
	_, err := svc.CreateBGM("Test", path, nil, 0, "invalid", "", "")
	assert.Error(t, err)
	assert.IsType(t, &domain.ValidationError{}, err)
}

func TestBGMService_GetBGM(t *testing.T) {
	svc, _, _ := setupBGMService(t)
	path := createTempFile(t)
	created, _ := svc.CreateBGM("Test", path, []string{"calm"}, 60000, domain.LicenseCCBY, "", "")

	got, err := svc.GetBGM(created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, got.ID)
}

func TestBGMService_ListBGMs(t *testing.T) {
	svc, _, _ := setupBGMService(t)
	path := createTempFile(t)
	svc.CreateBGM("BGM A", path, nil, 0, domain.LicenseCustom, "", "")
	svc.CreateBGM("BGM B", path, nil, 0, domain.LicenseCustom, "", "")

	bgms, err := svc.ListBGMs()
	require.NoError(t, err)
	assert.Len(t, bgms, 2)
}

func TestBGMService_UpdateBGM(t *testing.T) {
	svc, _, _ := setupBGMService(t)
	path := createTempFile(t)
	created, _ := svc.CreateBGM("Original", path, []string{"epic"}, 0, domain.LicenseCustom, "", "")

	updated, err := svc.UpdateBGM(created.ID, "Updated", []string{"calm"}, domain.LicenseCCBYSA, "New credit")
	require.NoError(t, err)
	assert.Equal(t, "Updated", updated.Name)
	assert.Equal(t, []string{"calm"}, updated.MoodTags)
}

func TestBGMService_DeleteBGM(t *testing.T) {
	svc, _, _ := setupBGMService(t)
	path := createTempFile(t)
	created, _ := svc.CreateBGM("Test", path, nil, 0, domain.LicenseCustom, "", "")

	err := svc.DeleteBGM(created.ID)
	require.NoError(t, err)

	_, err = svc.GetBGM(created.ID)
	assert.IsType(t, &domain.NotFoundError{}, err)
}

func TestBGMService_AutoRecommendBGMs(t *testing.T) {
	svc, s, mockLLM := setupBGMService(t)

	// Create BGMs directly in store (bypass file check)
	require.NoError(t, s.CreateBGM(&domain.BGM{ID: "bgm-epic", Name: "Epic BGM", FilePath: "/music/epic.mp3", MoodTags: []string{"epic", "intense"}, LicenseType: domain.LicenseRoyaltyFree}))
	require.NoError(t, s.CreateBGM(&domain.BGM{ID: "bgm-calm", Name: "Calm BGM", FilePath: "/music/calm.mp3", MoodTags: []string{"calm", "peaceful"}, LicenseType: domain.LicenseRoyaltyFree}))

	scenes := []domain.Scene{
		{SceneNum: 1, Narration: "A terrifying creature emerges from the darkness"},
		{SceneNum: 2, Narration: "Peace returns to the facility"},
	}

	mockLLM.On("Complete", mock.Anything, mock.Anything, mock.Anything).Return(
		&llm.CompletionResult{
			Content: `[{"scene_num":1,"mood_tags":["epic","intense"]},{"scene_num":2,"mood_tags":["calm","peaceful"]}]`,
		}, nil,
	)

	err := svc.AutoRecommendBGMs(context.Background(), "proj-1", scenes)
	require.NoError(t, err)

	// Verify assignments were created
	assignments, err := s.ListSceneBGMAssignments("proj-1")
	require.NoError(t, err)
	assert.Len(t, assignments, 2)
	assert.Equal(t, "bgm-epic", assignments[0].BGMID)
	assert.Equal(t, "bgm-calm", assignments[1].BGMID)
	assert.True(t, assignments[0].AutoRecommended)
	assert.False(t, assignments[0].Confirmed)
}

func TestBGMService_AutoRecommendBGMs_NoBGMs(t *testing.T) {
	svc, _, _ := setupBGMService(t)

	scenes := []domain.Scene{{SceneNum: 1, Narration: "Test"}}
	err := svc.AutoRecommendBGMs(context.Background(), "proj-1", scenes)
	require.NoError(t, err) // Should silently skip
}

func TestBGMService_AutoRecommendBGMs_EmptyScenes(t *testing.T) {
	svc, _, _ := setupBGMService(t)
	err := svc.AutoRecommendBGMs(context.Background(), "proj-1", nil)
	require.NoError(t, err)
}

func TestBGMService_GetPendingConfirmations(t *testing.T) {
	svc, s, _ := setupBGMService(t)
	require.NoError(t, s.CreateBGM(&domain.BGM{ID: "bgm-1", Name: "BGM1", FilePath: "/m/1", MoodTags: []string{}, LicenseType: domain.LicenseCustom}))

	require.NoError(t, s.AssignBGMToScene(&domain.SceneBGMAssignment{ProjectID: "p1", SceneNum: 1, BGMID: "bgm-1", Confirmed: false}))
	require.NoError(t, s.AssignBGMToScene(&domain.SceneBGMAssignment{ProjectID: "p1", SceneNum: 2, BGMID: "bgm-1", Confirmed: true}))

	pending, err := svc.GetPendingConfirmations("p1")
	require.NoError(t, err)
	assert.Len(t, pending, 1)
	assert.Equal(t, 1, pending[0].SceneNum)
}

func TestBGMService_ConfirmBGM(t *testing.T) {
	svc, s, _ := setupBGMService(t)
	require.NoError(t, s.CreateBGM(&domain.BGM{ID: "bgm-1", Name: "BGM1", FilePath: "/m/1", MoodTags: []string{}, LicenseType: domain.LicenseCustom}))
	require.NoError(t, s.AssignBGMToScene(&domain.SceneBGMAssignment{ProjectID: "p1", SceneNum: 1, BGMID: "bgm-1"}))

	require.NoError(t, svc.ConfirmBGM("p1", 1))

	got, _ := s.GetSceneBGMAssignment("p1", 1)
	assert.True(t, got.Confirmed)
}

func TestBGMService_ReassignBGM(t *testing.T) {
	svc, s, _ := setupBGMService(t)
	require.NoError(t, s.CreateBGM(&domain.BGM{ID: "bgm-1", Name: "BGM1", FilePath: "/m/1", MoodTags: []string{}, LicenseType: domain.LicenseCustom}))
	require.NoError(t, s.CreateBGM(&domain.BGM{ID: "bgm-2", Name: "BGM2", FilePath: "/m/2", MoodTags: []string{}, LicenseType: domain.LicenseCustom}))
	require.NoError(t, s.AssignBGMToScene(&domain.SceneBGMAssignment{ProjectID: "p1", SceneNum: 1, BGMID: "bgm-1"}))

	require.NoError(t, svc.ReassignBGM("p1", 1, "bgm-2"))

	got, _ := s.GetSceneBGMAssignment("p1", 1)
	assert.Equal(t, "bgm-2", got.BGMID)
	assert.True(t, got.Confirmed)
}

func TestBGMService_AdjustBGMParams(t *testing.T) {
	svc, s, _ := setupBGMService(t)
	require.NoError(t, s.CreateBGM(&domain.BGM{ID: "bgm-1", Name: "BGM1", FilePath: "/m/1", MoodTags: []string{}, LicenseType: domain.LicenseCustom}))
	require.NoError(t, s.AssignBGMToScene(&domain.SceneBGMAssignment{ProjectID: "p1", SceneNum: 1, BGMID: "bgm-1"}))

	require.NoError(t, svc.AdjustBGMParams("p1", 1, -6.0, 500, 1500, -18.0))

	got, _ := s.GetSceneBGMAssignment("p1", 1)
	assert.Equal(t, -6.0, got.VolumeDB)
	assert.Equal(t, 500, got.FadeInMs)
	assert.Equal(t, 1500, got.FadeOutMs)
	assert.Equal(t, -18.0, got.DuckingDB)
}

func TestBGMService_GetCredits(t *testing.T) {
	svc, s, _ := setupBGMService(t)
	require.NoError(t, s.CreateBGM(&domain.BGM{ID: "bgm-1", Name: "BGM1", FilePath: "/m/1", MoodTags: []string{}, LicenseType: domain.LicenseCustom, CreditText: "Artist A - Song X"}))
	require.NoError(t, s.CreateBGM(&domain.BGM{ID: "bgm-2", Name: "BGM2", FilePath: "/m/2", MoodTags: []string{}, LicenseType: domain.LicenseCustom, CreditText: "Artist B - Song Y"}))

	// One confirmed, one not
	require.NoError(t, s.AssignBGMToScene(&domain.SceneBGMAssignment{ProjectID: "p1", SceneNum: 1, BGMID: "bgm-1", Confirmed: true}))
	require.NoError(t, s.AssignBGMToScene(&domain.SceneBGMAssignment{ProjectID: "p1", SceneNum: 2, BGMID: "bgm-2", Confirmed: false}))

	credits, err := svc.GetCredits("p1")
	require.NoError(t, err)
	assert.Len(t, credits, 1)
	assert.Equal(t, "bgm", credits[0].Type)
	assert.Equal(t, "Artist A - Song X", credits[0].Text)
}

func TestBGMService_GetCredits_DeduplicatesSameBGM(t *testing.T) {
	svc, s, _ := setupBGMService(t)
	require.NoError(t, s.CreateBGM(&domain.BGM{ID: "bgm-1", Name: "BGM1", FilePath: "/m/1", MoodTags: []string{}, LicenseType: domain.LicenseCustom, CreditText: "Artist A"}))

	// Same BGM on two scenes
	require.NoError(t, s.AssignBGMToScene(&domain.SceneBGMAssignment{ProjectID: "p1", SceneNum: 1, BGMID: "bgm-1", Confirmed: true}))
	// scene_bgm_assignments uses (project_id, scene_num) as PK, so different scene_num
	require.NoError(t, s.AssignBGMToScene(&domain.SceneBGMAssignment{ProjectID: "p1", SceneNum: 2, BGMID: "bgm-1", Confirmed: true}))

	credits, err := svc.GetCredits("p1")
	require.NoError(t, err)
	assert.Len(t, credits, 1) // Deduplicated
}
