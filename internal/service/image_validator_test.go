package service

import (
	"context"
	"fmt"
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

func setupValidator(t *testing.T) (*ImageValidatorService, *mocks.MockLLM) {
	t.Helper()
	mockLLM := mocks.NewMockLLM(t)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	svc := NewImageValidatorService(mockLLM, nil, logger)
	return svc, mockLLM
}

func setupValidatorWithStore(t *testing.T) (*ImageValidatorService, *mocks.MockLLM, *store.Store) {
	t.Helper()
	mockLLM := mocks.NewMockLLM(t)
	s, err := store.New(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { s.Close() })
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	svc := NewImageValidatorService(mockLLM, s, logger)
	return svc, mockLLM, s
}

func createTestImage(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "test.png")
	require.NoError(t, os.WriteFile(p, []byte("fake-png-data"), 0644))
	return p
}

// validationJSON returns a JSON string for a given score triple.
func validationJSON(prompt, character, technical int) string {
	return fmt.Sprintf(
		`{"prompt_match": %d, "character_match": %d, "technical_score": %d, "reasons": ["test reason"]}`,
		prompt, character, technical,
	)
}

func TestValidateImage_Success_WithCharacter(t *testing.T) {
	svc, mockLLM := setupValidator(t)
	imgPath := createTestImage(t)

	mockLLM.On("CompleteWithVision", mock.Anything, mock.Anything, mock.Anything).
		Return(&llm.CompletionResult{
			Content: `{"prompt_match": 85, "character_match": 70, "technical_score": 90, "reasons": ["good match", "minor distortion"]}`,
		}, nil)

	result, err := svc.ValidateImage(context.Background(), imgPath, "a dark corridor", []imagegen.CharacterRef{
		{Name: "SCP-173", VisualDescriptor: "concrete statue"},
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 85, result.PromptMatch)
	assert.Equal(t, 70, result.CharacterMatch)
	assert.Equal(t, 90, result.TechnicalScore)
	assert.Len(t, result.Reasons, 2)
}

func TestValidateImage_Success_NoCharacter(t *testing.T) {
	svc, mockLLM := setupValidator(t)
	imgPath := createTestImage(t)

	mockLLM.On("CompleteWithVision", mock.Anything, mock.Anything, mock.Anything).
		Return(&llm.CompletionResult{
			Content: `{"prompt_match": 80, "character_match": 50, "technical_score": 95, "reasons": ["good"]}`,
		}, nil)

	result, err := svc.ValidateImage(context.Background(), imgPath, "empty hallway", nil)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, -1, result.CharacterMatch, "should be -1 for no character refs")
	assert.Equal(t, 80, result.PromptMatch)
	assert.Equal(t, 95, result.TechnicalScore)
}

func TestValidateImage_ErrNotSupported(t *testing.T) {
	svc, mockLLM := setupValidator(t)
	imgPath := createTestImage(t)

	mockLLM.On("CompleteWithVision", mock.Anything, mock.Anything, mock.Anything).
		Return(nil, llm.ErrNotSupported)

	result, err := svc.ValidateImage(context.Background(), imgPath, "test prompt", nil)

	assert.Nil(t, result)
	assert.Nil(t, err)
}

func TestValidateImage_MalformedJSON(t *testing.T) {
	svc, mockLLM := setupValidator(t)
	imgPath := createTestImage(t)

	mockLLM.On("CompleteWithVision", mock.Anything, mock.Anything, mock.Anything).
		Return(&llm.CompletionResult{
			Content: "This is not valid JSON at all",
		}, nil)

	result, err := svc.ValidateImage(context.Background(), imgPath, "test prompt", nil)

	assert.Nil(t, result)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse validation response")
	assert.Contains(t, err.Error(), "This is not valid JSON at all")
}

func TestValidateImage_MissingFile(t *testing.T) {
	svc, _ := setupValidator(t)

	result, err := svc.ValidateImage(context.Background(), "/nonexistent/image.png", "test prompt", nil)

	assert.Nil(t, result)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "image file not found")
}

func TestValidateImage_VisionMessageStructure(t *testing.T) {
	svc, mockLLM := setupValidator(t)
	imgPath := createTestImage(t)

	var capturedMsgs []llm.VisionMessage
	mockLLM.On("CompleteWithVision", mock.Anything, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			capturedMsgs = args.Get(1).([]llm.VisionMessage)
		}).
		Return(&llm.CompletionResult{
			Content: `{"prompt_match": 80, "character_match": -1, "technical_score": 90, "reasons": []}`,
		}, nil)

	_, err := svc.ValidateImage(context.Background(), imgPath, "test prompt", nil)
	require.NoError(t, err)

	// Verify message structure
	require.Len(t, capturedMsgs, 2)

	// System message
	assert.Equal(t, "system", capturedMsgs[0].Role)
	require.Len(t, capturedMsgs[0].Content, 1)
	assert.Equal(t, "text", capturedMsgs[0].Content[0].Type)
	assert.Contains(t, capturedMsgs[0].Content[0].Text, "image quality evaluator")

	// User message: text + image
	assert.Equal(t, "user", capturedMsgs[1].Role)
	require.Len(t, capturedMsgs[1].Content, 2)
	assert.Equal(t, "text", capturedMsgs[1].Content[0].Type)
	assert.Contains(t, capturedMsgs[1].Content[0].Text, "test prompt")
	assert.Equal(t, "image_url", capturedMsgs[1].Content[1].Type)
	assert.Contains(t, capturedMsgs[1].Content[1].ImageURL, "data:image/png;base64,")
}

func TestValidateImage_JSONInCodeFence(t *testing.T) {
	svc, mockLLM := setupValidator(t)
	imgPath := createTestImage(t)

	mockLLM.On("CompleteWithVision", mock.Anything, mock.Anything, mock.Anything).
		Return(&llm.CompletionResult{
			Content: "```json\n{\"prompt_match\": 75, \"character_match\": -1, \"technical_score\": 85, \"reasons\": [\"ok\"]}\n```",
		}, nil)

	result, err := svc.ValidateImage(context.Background(), imgPath, "test", nil)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 75, result.PromptMatch)
	assert.Equal(t, 85, result.TechnicalScore)
}

func TestDetectMIME(t *testing.T) {
	assert.Equal(t, "image/png", detectMIME(".png"))
	assert.Equal(t, "image/jpeg", detectMIME(".jpg"))
	assert.Equal(t, "image/jpeg", detectMIME(".jpeg"))
	assert.Equal(t, "image/webp", detectMIME(".webp"))
	assert.Equal(t, "image/png", detectMIME(".bmp")) // default
}

// ==================== ValidateAndRegenerate Tests ====================

func TestValidateAndRegenerate_PassOnFirstAttempt(t *testing.T) {
	svc, mockLLM := setupValidator(t)
	imgPath := createTestImage(t)

	// Score: 0.7*80 + 0.3*90 = 56+27 = 83 >= 70 → pass
	mockLLM.On("CompleteWithVision", mock.Anything, mock.Anything, mock.Anything).
		Return(&llm.CompletionResult{Content: validationJSON(80, -1, 90)}, nil).Once()

	regenerated := false
	result, err := svc.ValidateAndRegenerate(
		context.Background(), imgPath, "test prompt", nil,
		70, 3, "p1", 1, 1, 1,
		func(ctx context.Context) error { regenerated = true; return nil },
	)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.GreaterOrEqual(t, result.Score, 70)
	assert.False(t, result.ShouldRegenerate)
	assert.False(t, regenerated, "should not regenerate when first attempt passes")
}

func TestValidateAndRegenerate_PassOnSecondAttempt(t *testing.T) {
	svc, mockLLM := setupValidator(t)
	imgPath := createTestImage(t)

	// First attempt: score = 0.7*40 + 0.3*50 = 28+15 = 43 < 70 → fail
	mockLLM.On("CompleteWithVision", mock.Anything, mock.Anything, mock.Anything).
		Return(&llm.CompletionResult{Content: validationJSON(40, -1, 50)}, nil).Once()
	// Second attempt: score = 0.7*80 + 0.3*90 = 56+27 = 83 >= 70 → pass
	mockLLM.On("CompleteWithVision", mock.Anything, mock.Anything, mock.Anything).
		Return(&llm.CompletionResult{Content: validationJSON(80, -1, 90)}, nil).Once()

	regenCount := 0
	result, err := svc.ValidateAndRegenerate(
		context.Background(), imgPath, "test prompt", nil,
		70, 3, "p1", 1, 1, 1,
		func(ctx context.Context) error { regenCount++; return nil },
	)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.GreaterOrEqual(t, result.Score, 70)
	assert.False(t, result.ShouldRegenerate)
	assert.Equal(t, 1, regenCount, "should regenerate exactly once")
}

func TestValidateAndRegenerate_AllAttemptsExhausted_KeepsBest(t *testing.T) {
	svc, mockLLM := setupValidator(t)
	imgPath := createTestImage(t)

	// Attempt 1: score = 0.7*30 + 0.3*40 = 21+12 = 33
	mockLLM.On("CompleteWithVision", mock.Anything, mock.Anything, mock.Anything).
		Return(&llm.CompletionResult{Content: validationJSON(30, -1, 40)}, nil).Once()
	// Attempt 2: score = 0.7*50 + 0.3*60 = 35+18 = 53 (best)
	mockLLM.On("CompleteWithVision", mock.Anything, mock.Anything, mock.Anything).
		Return(&llm.CompletionResult{Content: validationJSON(50, -1, 60)}, nil).Once()
	// Attempt 3: score = 0.7*40 + 0.3*50 = 28+15 = 43
	mockLLM.On("CompleteWithVision", mock.Anything, mock.Anything, mock.Anything).
		Return(&llm.CompletionResult{Content: validationJSON(40, -1, 50)}, nil).Once()

	regenCount := 0
	result, err := svc.ValidateAndRegenerate(
		context.Background(), imgPath, "test prompt", nil,
		70, 3, "p1", 1, 1, 1,
		func(ctx context.Context) error { regenCount++; return nil },
	)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 53, result.Score, "should keep best scoring result")
	assert.False(t, result.ShouldRegenerate, "should be false when exhausted")
	assert.Equal(t, 2, regenCount, "should regenerate twice (between 3 attempts)")
}

func TestValidateAndRegenerate_VisionNotSupported(t *testing.T) {
	svc, mockLLM := setupValidator(t)
	imgPath := createTestImage(t)

	mockLLM.On("CompleteWithVision", mock.Anything, mock.Anything, mock.Anything).
		Return(nil, llm.ErrNotSupported)

	result, err := svc.ValidateAndRegenerate(
		context.Background(), imgPath, "test prompt", nil,
		70, 3, "p1", 1, 1, 1,
		func(ctx context.Context) error { return nil },
	)

	assert.Nil(t, result)
	assert.Nil(t, err)
}

func TestValidateAndRegenerate_RegenerateFnError(t *testing.T) {
	svc, mockLLM := setupValidator(t)
	imgPath := createTestImage(t)

	// First attempt fails threshold
	mockLLM.On("CompleteWithVision", mock.Anything, mock.Anything, mock.Anything).
		Return(&llm.CompletionResult{Content: validationJSON(30, -1, 40)}, nil).Once()

	result, err := svc.ValidateAndRegenerate(
		context.Background(), imgPath, "test prompt", nil,
		70, 3, "p1", 1, 1, 1,
		func(ctx context.Context) error { return fmt.Errorf("imagegen API error") },
	)

	assert.Nil(t, result)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "regenerate image")
	assert.Contains(t, err.Error(), "imagegen API error")
}

func TestValidateAndRegenerate_ValidateImageError(t *testing.T) {
	svc, mockLLM := setupValidator(t)
	imgPath := createTestImage(t)

	mockLLM.On("CompleteWithVision", mock.Anything, mock.Anything, mock.Anything).
		Return(nil, fmt.Errorf("LLM connection failed"))

	result, err := svc.ValidateAndRegenerate(
		context.Background(), imgPath, "test prompt", nil,
		70, 3, "p1", 1, 1, 1,
		func(ctx context.Context) error { return nil },
	)

	assert.Nil(t, result)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "validation attempt 1")
}

func TestValidateAndRegenerate_PersistsScoreToStore(t *testing.T) {
	svc, mockLLM, s := setupValidatorWithStore(t)
	imgPath := createTestImage(t)

	// Create project and shot manifest for the store to update
	require.NoError(t, s.CreateProject(&domain.Project{
		ID: "p1", SCPID: "SCP-173", Status: domain.StagePending, WorkspacePath: "/w",
	}))
	require.NoError(t, s.CreateShotManifest(&domain.ShotManifest{
		ProjectID: "p1", SceneNum: 1, SentenceStart: 1, CutNum: 1,
		ContentHash: "abc", GenMethod: "text_to_image", Status: "generated",
	}))

	// Score: 0.7*80 + 0.3*90 = 83
	mockLLM.On("CompleteWithVision", mock.Anything, mock.Anything, mock.Anything).
		Return(&llm.CompletionResult{Content: validationJSON(80, -1, 90)}, nil).Once()

	result, err := svc.ValidateAndRegenerate(
		context.Background(), imgPath, "test prompt", nil,
		70, 3, "p1", 1, 1, 1,
		func(ctx context.Context) error { return nil },
	)

	require.NoError(t, err)
	require.NotNil(t, result)

	// Verify score was persisted
	score, err := s.GetValidationScore("p1", 1, 1, 1)
	require.NoError(t, err)
	require.NotNil(t, score)
	assert.Equal(t, result.Score, *score)
}

func TestValidateAndRegenerate_MaxAttemptsDefault(t *testing.T) {
	svc, mockLLM := setupValidator(t)
	imgPath := createTestImage(t)

	// With maxAttempts=0, should default to 1 attempt
	mockLLM.On("CompleteWithVision", mock.Anything, mock.Anything, mock.Anything).
		Return(&llm.CompletionResult{Content: validationJSON(80, -1, 90)}, nil).Once()

	result, err := svc.ValidateAndRegenerate(
		context.Background(), imgPath, "test prompt", nil,
		70, 0, "p1", 1, 1, 1,
		func(ctx context.Context) error { return nil },
	)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.ShouldRegenerate)
}
