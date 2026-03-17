package service

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/sushistack/yt.pipe/internal/domain"
	"github.com/sushistack/yt.pipe/internal/mocks"
	"github.com/sushistack/yt.pipe/internal/plugin/imagegen"
	"github.com/sushistack/yt.pipe/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func newTestImageGenService(t *testing.T, mockIG *mocks.MockImageGen) (*ImageGenService, *store.Store) {
	t.Helper()
	s, err := store.New(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { s.Close() })
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	return NewImageGenService(mockIG, s, logger), s
}

func createTestProject(t *testing.T, s *store.Store, projectID string) {
	t.Helper()
	err := s.CreateProject(&domain.Project{
		ID:            projectID,
		SCPID:         "SCP-TEST",
		Status:        domain.StageImages,
		WorkspacePath: "/tmp/test",
	})
	require.NoError(t, err)
}

func TestGenerateShotImage_Success(t *testing.T) {
	mockIG := mocks.NewMockImageGen(t)
	svc, st := newTestImageGenService(t, mockIG)
	ctx := context.Background()
	projectPath := t.TempDir()
	projectID := "test-project"
	createTestProject(t, st, projectID)

	mockIG.On("Generate", mock.Anything, "shot prompt, anime illustration", mock.Anything).
		Return(&imagegen.ImageResult{
			ImageData: []byte("fake-png-data"),
			Format:    "png",
			Width:     1024,
			Height:    1024,
		}, nil)

	shot, err := svc.GenerateShotImage(ctx, projectID, projectPath,
		1, 1, 1, 1, 1, "shot prompt, anime illustration", "", false, "SCP-TEST", imagegen.GenerateOptions{})
	require.NoError(t, err)

	assert.Equal(t, 1, shot.SentenceStart)
	assert.Contains(t, shot.ImagePath, "cut_1_1.png")
	assert.FileExists(t, shot.ImagePath)
	assert.FileExists(t, filepath.Join(projectPath, "scenes", "1", "cut_1_1_prompt.txt"))

	// Verify shot manifest updated
	manifest, err := st.GetShotManifest(projectID, 1, 1, 1)
	require.NoError(t, err)
	assert.NotEmpty(t, manifest.ImageHash)
	assert.Equal(t, "generated", manifest.Status)
}

func TestGenerateAllShotImages_Success(t *testing.T) {
	mockIG := mocks.NewMockImageGen(t)
	svc, st := newTestImageGenService(t, mockIG)
	ctx := context.Background()
	projectPath := t.TempDir()
	createTestProject(t, st, "proj")

	mockIG.On("Generate", mock.Anything, mock.Anything, mock.Anything).
		Return(&imagegen.ImageResult{
			ImageData: []byte("fake-data"),
			Format:    "png",
		}, nil)

	scenePrompts := []*ScenePromptOutput{
		{SceneNum: 1, Shots: []ShotOutput{
			{ShotNum: 1, FinalPrompt: "prompt 1", SentenceText: "s1", ShotDesc: &ShotDescription{EntityVisible: false}},
			{ShotNum: 2, FinalPrompt: "prompt 2", SentenceText: "s2", ShotDesc: &ShotDescription{EntityVisible: false}},
		}},
		{SceneNum: 2, Shots: []ShotOutput{
			{ShotNum: 1, FinalPrompt: "prompt 3", SentenceText: "s3", ShotDesc: &ShotDescription{EntityVisible: false}},
		}},
	}

	scenes, err := svc.GenerateAllShotImages(ctx, scenePrompts, "proj", projectPath, "SCP-TEST", imagegen.GenerateOptions{}, nil)
	require.NoError(t, err)
	assert.Len(t, scenes, 2)
	assert.Len(t, scenes[0].Shots, 2)
	assert.Len(t, scenes[1].Shots, 1)
}

func TestGenerateAllShotImages_SkipMap(t *testing.T) {
	mockIG := mocks.NewMockImageGen(t)
	svc, st := newTestImageGenService(t, mockIG)
	ctx := context.Background()
	projectPath := t.TempDir()
	createTestProject(t, st, "proj")

	// Only shot 2 should generate (shot 1 skipped)
	mockIG.On("Generate", mock.Anything, "prompt 2", mock.Anything).
		Return(&imagegen.ImageResult{ImageData: []byte("data"), Format: "png"}, nil)

	scenePrompts := []*ScenePromptOutput{
		{SceneNum: 1, Shots: []ShotOutput{
			{ShotNum: 1, FinalPrompt: "prompt 1", SentenceText: "s1", ShotDesc: &ShotDescription{EntityVisible: false}},
			{ShotNum: 2, FinalPrompt: "prompt 2", SentenceText: "s2", ShotDesc: &ShotDescription{EntityVisible: false}},
		}},
	}

	skipMap := map[domain.ShotKey]bool{
		{SceneNum: 1, ShotNum: 1}: true,
	}

	scenes, err := svc.GenerateAllShotImages(ctx, scenePrompts, "proj", projectPath, "SCP-TEST", imagegen.GenerateOptions{}, skipMap)
	require.NoError(t, err)
	assert.Len(t, scenes, 1)
	assert.Len(t, scenes[0].Shots, 2)
	// Shot 1 skipped — no ImagePath
	assert.Empty(t, scenes[0].Shots[0].ImagePath)
	// Shot 2 generated
	assert.NotEmpty(t, scenes[0].Shots[1].ImagePath)
}

func TestGenerateAllShotImages_PartialFailure(t *testing.T) {
	mockIG := mocks.NewMockImageGen(t)
	svc, st := newTestImageGenService(t, mockIG)
	ctx := context.Background()
	projectPath := t.TempDir()
	projectID := "proj-partial"
	createTestProject(t, st, projectID)

	mockIG.On("Generate", mock.Anything, "prompt 1", mock.Anything).
		Return(&imagegen.ImageResult{ImageData: []byte("data"), Format: "png"}, nil)
	mockIG.On("Generate", mock.Anything, "prompt 2", mock.Anything).
		Return(nil, assert.AnError)

	scenePrompts := []*ScenePromptOutput{
		{SceneNum: 1, Shots: []ShotOutput{
			{ShotNum: 1, FinalPrompt: "prompt 1", SentenceText: "s1", ShotDesc: &ShotDescription{EntityVisible: false}},
			{ShotNum: 2, FinalPrompt: "prompt 2", SentenceText: "s2", ShotDesc: &ShotDescription{EntityVisible: false}},
		}},
	}

	scenes, err := svc.GenerateAllShotImages(ctx, scenePrompts, projectID, projectPath, "SCP-TEST", imagegen.GenerateOptions{}, nil)
	require.Error(t, err)
	assert.Len(t, scenes, 1)
	// Shot 1 succeeded, shot 2 failed but still in array
	assert.Len(t, scenes[0].Shots, 2)
}

func TestReadManualPrompt_Exists(t *testing.T) {
	mockIG := mocks.NewMockImageGen(t)
	svc, _ := newTestImageGenService(t, mockIG)
	projectPath := t.TempDir()

	sceneDir := filepath.Join(projectPath, "scenes", "3")
	require.NoError(t, os.MkdirAll(sceneDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(sceneDir, "prompt.txt"), []byte("edited prompt"), 0o644))

	prompt, exists, err := svc.ReadManualPrompt(projectPath, 3)
	require.NoError(t, err)
	assert.True(t, exists)
	assert.Equal(t, "edited prompt", prompt)
}

func TestReadManualPrompt_NotExists(t *testing.T) {
	mockIG := mocks.NewMockImageGen(t)
	svc, _ := newTestImageGenService(t, mockIG)
	projectPath := t.TempDir()

	prompt, exists, err := svc.ReadManualPrompt(projectPath, 99)
	require.NoError(t, err)
	assert.False(t, exists)
	assert.Empty(t, prompt)
}
