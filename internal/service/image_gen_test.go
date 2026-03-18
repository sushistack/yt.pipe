package service

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/sushistack/yt.pipe/internal/domain"
	"github.com/sushistack/yt.pipe/internal/mocks"
	"github.com/sushistack/yt.pipe/internal/plugin/imagegen"
	"github.com/sushistack/yt.pipe/internal/plugin/llm"
	"github.com/sushistack/yt.pipe/internal/store"
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

func TestSetValidator_And_SetValidationConfig(t *testing.T) {
	mockIG := mocks.NewMockImageGen(t)
	svc, _ := newTestImageGenService(t, mockIG)

	assert.Nil(t, svc.validator)
	assert.Nil(t, svc.validationConfig)

	mockLLM := mocks.NewMockLLM(t)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	validator := NewImageValidatorService(mockLLM, nil, logger)

	svc.SetValidator(validator)
	assert.NotNil(t, svc.validator)

	cfg := &ValidationConfig{Threshold: 70, MaxAttempts: 3}
	svc.SetValidationConfig(cfg)
	assert.NotNil(t, svc.validationConfig)
	assert.Equal(t, 70, svc.validationConfig.Threshold)
	assert.Equal(t, 3, svc.validationConfig.MaxAttempts)
}

func TestGenerateShotImage_NoValidator_BackwardCompat(t *testing.T) {
	// When validator is nil, GenerateShotImage should work identically to before
	mockIG := mocks.NewMockImageGen(t)
	svc, st := newTestImageGenService(t, mockIG)
	ctx := context.Background()
	projectPath := t.TempDir()
	projectID := "test-no-validator"
	createTestProject(t, st, projectID)

	mockIG.On("Generate", mock.Anything, "test prompt", mock.Anything).
		Return(&imagegen.ImageResult{ImageData: []byte("fake-data"), Format: "png"}, nil)

	// Ensure validator is nil (default)
	assert.Nil(t, svc.validator)

	shot, err := svc.GenerateShotImage(ctx, projectID, projectPath,
		1, 1, 1, 1, 1, "test prompt", "", false, "SCP-TEST", imagegen.GenerateOptions{})
	require.NoError(t, err)
	assert.NotNil(t, shot)
	assert.Contains(t, shot.ImagePath, "cut_1_1.png")
}

func TestGenerateShotImage_WithValidator_PassFirstAttempt(t *testing.T) {
	mockIG := mocks.NewMockImageGen(t)
	svc, st := newTestImageGenService(t, mockIG)
	ctx := context.Background()
	projectPath := t.TempDir()
	projectID := "test-validate-pass"
	createTestProject(t, st, projectID)

	mockIG.On("Generate", mock.Anything, "test prompt", mock.Anything).
		Return(&imagegen.ImageResult{ImageData: []byte("fake-data"), Format: "png"}, nil)

	// Setup validator with mock LLM that returns high score
	mockLLM := mocks.NewMockLLM(t)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	validator := NewImageValidatorService(mockLLM, st, logger)

	mockLLM.On("CompleteWithVision", mock.Anything, mock.Anything, mock.Anything).
		Return(&llm.CompletionResult{
			Content: `{"prompt_match": 90, "character_match": -1, "technical_score": 85, "reasons": ["good"]}`,
		}, nil)

	svc.SetValidator(validator)
	svc.SetValidationConfig(&ValidationConfig{Threshold: 70, MaxAttempts: 3})

	shot, err := svc.GenerateShotImage(ctx, projectID, projectPath,
		1, 1, 1, 1, 1, "test prompt", "", false, "SCP-TEST", imagegen.GenerateOptions{})
	require.NoError(t, err)
	assert.NotNil(t, shot)
	assert.Contains(t, shot.ImagePath, "cut_1_1.png")

	// Verify validation score was persisted
	manifest, err := st.GetShotManifest(projectID, 1, 1, 1)
	require.NoError(t, err)
	assert.NotNil(t, manifest.ValidationScore)
}

func TestGenerateShotImage_WithValidator_RegenerationTriggered(t *testing.T) {
	mockIG := mocks.NewMockImageGen(t)
	svc, st := newTestImageGenService(t, mockIG)
	ctx := context.Background()
	projectPath := t.TempDir()
	projectID := "test-validate-regen"
	createTestProject(t, st, projectID)

	// First Generate call (initial), second Generate call (regeneration)
	mockIG.On("Generate", mock.Anything, "test prompt", mock.Anything).
		Return(&imagegen.ImageResult{ImageData: []byte("fake-data"), Format: "png"}, nil)

	// Setup validator
	mockLLM := mocks.NewMockLLM(t)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	validator := NewImageValidatorService(mockLLM, st, logger)

	// First validation: low score (triggers regen), second: high score
	callCount := 0
	mockLLM.On("CompleteWithVision", mock.Anything, mock.Anything, mock.Anything).
		Return(func(ctx context.Context, msgs []llm.VisionMessage, opts llm.CompletionOptions) (*llm.CompletionResult, error) {
			callCount++
			if callCount == 1 {
				return &llm.CompletionResult{
					Content: `{"prompt_match": 30, "character_match": -1, "technical_score": 40, "reasons": ["poor quality"]}`,
				}, nil
			}
			return &llm.CompletionResult{
				Content: `{"prompt_match": 90, "character_match": -1, "technical_score": 85, "reasons": ["good"]}`,
			}, nil
		})

	svc.SetValidator(validator)
	svc.SetValidationConfig(&ValidationConfig{Threshold: 70, MaxAttempts: 3})

	shot, err := svc.GenerateShotImage(ctx, projectID, projectPath,
		1, 1, 1, 1, 1, "test prompt", "", false, "SCP-TEST", imagegen.GenerateOptions{})
	require.NoError(t, err)
	assert.NotNil(t, shot)

	// Should have called Generate at least twice (initial + regeneration)
	assert.GreaterOrEqual(t, len(mockIG.Calls), 2)
}

func TestGenerateShotImage_WithValidator_ValidationError_StillSucceeds(t *testing.T) {
	mockIG := mocks.NewMockImageGen(t)
	svc, st := newTestImageGenService(t, mockIG)
	ctx := context.Background()
	projectPath := t.TempDir()
	projectID := "test-validate-err"
	createTestProject(t, st, projectID)

	mockIG.On("Generate", mock.Anything, "test prompt", mock.Anything).
		Return(&imagegen.ImageResult{ImageData: []byte("fake-data"), Format: "png"}, nil)

	// Setup validator with mock LLM that returns error
	mockLLM := mocks.NewMockLLM(t)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	validator := NewImageValidatorService(mockLLM, st, logger)

	mockLLM.On("CompleteWithVision", mock.Anything, mock.Anything, mock.Anything).
		Return(nil, assert.AnError)

	svc.SetValidator(validator)
	svc.SetValidationConfig(&ValidationConfig{Threshold: 70, MaxAttempts: 3})

	// Image generation should still succeed even if validation fails
	shot, err := svc.GenerateShotImage(ctx, projectID, projectPath,
		1, 1, 1, 1, 1, "test prompt", "", false, "SCP-TEST", imagegen.GenerateOptions{})
	require.NoError(t, err)
	assert.NotNil(t, shot)
	assert.Contains(t, shot.ImagePath, "cut_1_1.png")
}

func TestGenerateShotImage_WithValidator_VisionNotSupported(t *testing.T) {
	mockIG := mocks.NewMockImageGen(t)
	svc, st := newTestImageGenService(t, mockIG)
	ctx := context.Background()
	projectPath := t.TempDir()
	projectID := "test-validate-no-vision"
	createTestProject(t, st, projectID)

	mockIG.On("Generate", mock.Anything, "test prompt", mock.Anything).
		Return(&imagegen.ImageResult{ImageData: []byte("fake-data"), Format: "png"}, nil)

	// Setup validator with mock LLM that returns ErrNotSupported
	mockLLM := mocks.NewMockLLM(t)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	validator := NewImageValidatorService(mockLLM, st, logger)

	mockLLM.On("CompleteWithVision", mock.Anything, mock.Anything, mock.Anything).
		Return(nil, llm.ErrNotSupported)

	svc.SetValidator(validator)
	svc.SetValidationConfig(&ValidationConfig{Threshold: 70, MaxAttempts: 3})

	// Image generation should succeed, validation skipped
	shot, err := svc.GenerateShotImage(ctx, projectID, projectPath,
		1, 1, 1, 1, 1, "test prompt", "", false, "SCP-TEST", imagegen.GenerateOptions{})
	require.NoError(t, err)
	assert.NotNil(t, shot)
	assert.Contains(t, shot.ImagePath, "cut_1_1.png")
}
