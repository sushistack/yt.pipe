package service

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sushistack/yt.pipe/internal/domain"
	"github.com/sushistack/yt.pipe/internal/mocks"
	"github.com/sushistack/yt.pipe/internal/plugin/llm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func setupShotBreakdownTemplates(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	imgDir := filepath.Join(tmpDir, "image")
	require.NoError(t, os.MkdirAll(imgDir, 0o755))

	// Minimal templates for testing — use new per-sentence variables
	tmpl1 := "Break down scene {scene_number} shot {shot_number}: {sentence} with descriptor: {frozen_descriptor}"
	tmpl2 := "Convert shot to prompt: {shot_json} with descriptor: {frozen_descriptor}"

	require.NoError(t, os.WriteFile(filepath.Join(imgDir, "01_shot_breakdown.md"), []byte(tmpl1), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(imgDir, "02_shot_to_prompt.md"), []byte(tmpl2), 0o644))

	return tmpDir
}

func TestNewShotBreakdownPipeline_Success(t *testing.T) {
	tmpDir := setupShotBreakdownTemplates(t)
	mockLLM := mocks.NewMockLLM(t)

	sp, err := NewShotBreakdownPipeline(mockLLM, ShotBreakdownConfig{TemplatesDir: tmpDir})
	require.NoError(t, err)
	assert.NotNil(t, sp)
	assert.Len(t, sp.templates, 2)
}

func TestNewShotBreakdownPipeline_MissingTemplate(t *testing.T) {
	mockLLM := mocks.NewMockLLM(t)

	_, err := NewShotBreakdownPipeline(mockLLM, ShotBreakdownConfig{TemplatesDir: "/nonexistent"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "load template")
}

func TestGenerateScenePrompt_Success(t *testing.T) {
	tmpDir := setupShotBreakdownTemplates(t)
	mockLLM := mocks.NewMockLLM(t)

	// Stage 1: Shot breakdown response
	shotDesc := ShotDescription{
		ShotNumber:    1,
		Role:          "establishing",
		CameraType:    "wide",
		EntityVisible: true,
		Subject:       "A tall humanoid figure with pale skin, standing in a dark hallway",
		Lighting:      "dim overhead fluorescent",
		Mood:          "ominous",
		Motion:        "slow dolly forward",
	}
	shotJSON, _ := json.Marshal(shotDesc)

	mockLLM.On("Complete", mock.Anything, mock.MatchedBy(func(msgs []llm.Message) bool {
		return len(msgs) > 0 && containsStr(msgs[0].Content, "Break down scene")
	}), mock.Anything).Return(&llm.CompletionResult{
		Content: string(shotJSON),
	}, nil).Once()

	// Stage 2: Shot-to-prompt response
	promptResult := ShotPromptResult{
		Prompt:         "A tall humanoid figure with pale skin, wide shot, dim overhead fluorescent lighting, ominous atmosphere",
		NegativePrompt: "blurry, low quality, watermark",
		EntityVisible:  true,
	}
	promptJSON, _ := json.Marshal(promptResult)

	mockLLM.On("Complete", mock.Anything, mock.MatchedBy(func(msgs []llm.Message) bool {
		return len(msgs) > 0 && containsStr(msgs[0].Content, "Convert shot to prompt")
	}), mock.Anything).Return(&llm.CompletionResult{
		Content: string(promptJSON),
	}, nil).Once()

	sp, err := NewShotBreakdownPipeline(mockLLM, ShotBreakdownConfig{TemplatesDir: tmpDir})
	require.NoError(t, err)

	// Synopsis is a single sentence → generates 1 shot
	output, err := sp.GenerateScenePrompt(context.Background(), ScenePromptInput{
		SceneNum:             1,
		Synopsis:             "A dark hallway in the facility",
		EmotionalBeat:        "ominous",
		EntityVisualIdentity: "Visual identity profile text",
		FrozenDescriptor:     "A tall humanoid figure with pale skin",
		PreviousLastShotCtx:  "(first scene)",
	})
	require.NoError(t, err)
	assert.Equal(t, 1, output.SceneNum)
	require.Len(t, output.Shots, 1)
	assert.Equal(t, "wide", output.Shots[0].ShotDesc.CameraType)
	assert.True(t, output.Shots[0].ShotDesc.EntityVisible)
	assert.NotEmpty(t, output.Shots[0].FinalPrompt)
	assert.Contains(t, output.Shots[0].FinalPrompt, "anime")
	assert.NotEmpty(t, output.Shots[0].NegativePrompt)
}

func TestGenerateAllScenePrompts_Continuity(t *testing.T) {
	tmpDir := setupShotBreakdownTemplates(t)
	mockLLM := mocks.NewMockLLM(t)

	// Return valid shot breakdown for both calls
	shot := ShotDescription{
		ShotNumber: 1, Role: "establishing", CameraType: "wide",
		EntityVisible: false, Subject: "empty room", Lighting: "bright", Mood: "calm", Motion: "static",
	}
	shotJSON, _ := json.Marshal(shot)

	prompt := ShotPromptResult{
		Prompt: "empty room, wide shot, bright lighting", NegativePrompt: "blur", EntityVisible: false,
	}
	promptJSON, _ := json.Marshal(prompt)

	mockLLM.On("Complete", mock.Anything, mock.MatchedBy(func(msgs []llm.Message) bool {
		return containsStr(msgs[0].Content, "Break down scene")
	}), mock.Anything).Return(&llm.CompletionResult{Content: string(shotJSON)}, nil)

	mockLLM.On("Complete", mock.Anything, mock.MatchedBy(func(msgs []llm.Message) bool {
		return containsStr(msgs[0].Content, "Convert shot to prompt")
	}), mock.Anything).Return(&llm.CompletionResult{Content: string(promptJSON)}, nil)

	sp, _ := NewShotBreakdownPipeline(mockLLM, ShotBreakdownConfig{TemplatesDir: tmpDir})

	scenario := &domain.ScenarioOutput{
		Scenes: []domain.SceneScript{
			{SceneNum: 1, Narration: "장면 하나다.", Mood: "calm"},
			{SceneNum: 2, Narration: "장면 둘이다.", Mood: "tense"},
		},
	}

	results, err := sp.GenerateAllScenePrompts(context.Background(), scenario, "frozen desc", "visual identity")
	require.NoError(t, err)
	assert.Len(t, results, 2)
	assert.NotNil(t, results[0])
	assert.NotNil(t, results[1])
	// Each scene has 1 sentence → 1 shot
	assert.Len(t, results[0].Shots, 1)
	assert.Len(t, results[1].Shots, 1)
}

func TestSanitizeImagePrompt(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		contains string
		excludes string
	}{
		{
			name:     "adds anime suffix",
			input:    "a dark hallway",
			contains: "anime illustration",
		},
		{
			name:     "does not duplicate suffix",
			input:    "a dark hallway, anime illustration, dark horror anime style",
			contains: "anime",
		},
		{
			name:     "removes dangerous terms",
			input:    "a gore scene with blood",
			excludes: "gore",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeImagePrompt(tt.input)
			if tt.contains != "" {
				assert.Contains(t, result, tt.contains)
			}
			if tt.excludes != "" {
				assert.NotContains(t, result, tt.excludes)
			}
		})
	}
}

func TestFormatShotContext(t *testing.T) {
	shot := &ShotDescription{
		CameraType: "wide",
		Subject:    "dark hallway",
		Lighting:   "dim",
		Mood:       "ominous",
	}
	ctx := formatShotContext(shot)
	assert.Contains(t, ctx, "wide")
	assert.Contains(t, ctx, "dark hallway")
	assert.Contains(t, ctx, "dim")

	// Nil shot
	assert.Contains(t, formatShotContext(nil), "no previous shot")
}

func containsStr(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && strings.Contains(s, substr)
}
