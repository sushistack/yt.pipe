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

// createTestProject inserts a project record to satisfy FK constraints on scene_manifests.
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

func TestGenerateSceneImage_Success(t *testing.T) {
	mockIG := mocks.NewMockImageGen(t)
	svc, st := newTestImageGenService(t, mockIG)
	ctx := context.Background()
	projectPath := t.TempDir()
	projectID := "test-project"
	createTestProject(t, st, projectID)

	mockIG.On("Generate", mock.Anything, "sanitized prompt, digital illustration", mock.Anything).
		Return(&imagegen.ImageResult{
			ImageData: []byte("fake-png-data"),
			Format:    "png",
			Width:     1024,
			Height:    1024,
		}, nil)

	prompt := ImagePromptResult{
		SceneNum:        1,
		OriginalPrompt:  "original prompt",
		SanitizedPrompt: "sanitized prompt, digital illustration",
	}

	scene, err := svc.GenerateSceneImage(ctx, prompt, projectID, projectPath, imagegen.GenerateOptions{})
	require.NoError(t, err)

	assert.Equal(t, 1, scene.SceneNum)
	assert.Contains(t, scene.ImagePath, "image.png")
	assert.FileExists(t, scene.ImagePath)
	assert.FileExists(t, filepath.Join(projectPath, "scenes", "1", "prompt.txt"))

	// AC1: Verify manifest updated with image hash
	manifest, err := st.GetManifest(projectID, 1)
	require.NoError(t, err)
	assert.NotEmpty(t, manifest.ImageHash)
	assert.Equal(t, "image_generated", manifest.Status)
}

func TestGenerateAllImages_Success(t *testing.T) {
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

	prompts := []ImagePromptResult{
		{SceneNum: 1, SanitizedPrompt: "prompt 1"},
		{SceneNum: 2, SanitizedPrompt: "prompt 2"},
	}

	scenes, err := svc.GenerateAllImages(ctx, prompts, "proj", projectPath, imagegen.GenerateOptions{}, nil)
	require.NoError(t, err)
	assert.Len(t, scenes, 2)
}

func TestGenerateAllImages_PartialFailure(t *testing.T) {
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

	prompts := []ImagePromptResult{
		{SceneNum: 1, SanitizedPrompt: "prompt 1"},
		{SceneNum: 2, SanitizedPrompt: "prompt 2"},
	}

	scenes, err := svc.GenerateAllImages(ctx, prompts, projectID, projectPath, imagegen.GenerateOptions{}, nil)
	require.Error(t, err)
	assert.Len(t, scenes, 1) // partial results

	// AC4: Verify failed scene marked in manifest
	manifest, getErr := st.GetManifest(projectID, 2)
	require.NoError(t, getErr)
	assert.Equal(t, "image_failed", manifest.Status)
}

func TestGenerateAllImages_SceneFilter(t *testing.T) {
	mockIG := mocks.NewMockImageGen(t)
	svc, st := newTestImageGenService(t, mockIG)
	ctx := context.Background()
	projectPath := t.TempDir()
	createTestProject(t, st, "proj")

	// Only scene 2 should be called
	mockIG.On("Generate", mock.Anything, "prompt 2", mock.Anything).
		Return(&imagegen.ImageResult{ImageData: []byte("data"), Format: "png"}, nil)

	prompts := []ImagePromptResult{
		{SceneNum: 1, SanitizedPrompt: "prompt 1"},
		{SceneNum: 2, SanitizedPrompt: "prompt 2"},
		{SceneNum: 3, SanitizedPrompt: "prompt 3"},
	}

	// AC2: Only regenerate scene 2
	scenes, err := svc.GenerateAllImages(ctx, prompts, "proj", projectPath, imagegen.GenerateOptions{}, []int{2})
	require.NoError(t, err)
	assert.Len(t, scenes, 1)
	assert.Equal(t, 2, scenes[0].SceneNum)
}

func TestGenerateAllImages_ContextCancellation(t *testing.T) {
	mockIG := mocks.NewMockImageGen(t)
	svc, _ := newTestImageGenService(t, mockIG)
	projectPath := t.TempDir()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	prompts := []ImagePromptResult{
		{SceneNum: 1, SanitizedPrompt: "prompt 1"},
	}

	scenes, err := svc.GenerateAllImages(ctx, prompts, "proj", projectPath, imagegen.GenerateOptions{}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cancelled")
	assert.Empty(t, scenes)
}

func TestReadManualPrompt_Exists(t *testing.T) {
	mockIG := mocks.NewMockImageGen(t)
	svc, _ := newTestImageGenService(t, mockIG)
	projectPath := t.TempDir()

	// Create scene dir and prompt file
	sceneDir := filepath.Join(projectPath, "scenes", "3")
	require.NoError(t, os.MkdirAll(sceneDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(sceneDir, "prompt.txt"), []byte("edited prompt"), 0o644))

	// AC3: Read manually edited prompt
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

func TestFilterPrompts(t *testing.T) {
	prompts := []ImagePromptResult{
		{SceneNum: 1}, {SceneNum: 2}, {SceneNum: 3}, {SceneNum: 4}, {SceneNum: 5},
	}

	// No filter - all returned
	assert.Len(t, filterPrompts(prompts, nil), 5)

	// Filter specific scenes
	filtered := filterPrompts(prompts, []int{2, 4})
	assert.Len(t, filtered, 2)
	assert.Equal(t, 2, filtered[0].SceneNum)
	assert.Equal(t, 4, filtered[1].SceneNum)

	// Filter non-existent scene
	assert.Len(t, filterPrompts(prompts, []int{99}), 0)
}
