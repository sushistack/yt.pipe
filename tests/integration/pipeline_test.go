//go:build integration

package integration

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sushistack/yt.pipe/internal/glossary"
	"github.com/sushistack/yt.pipe/internal/pipeline"
	"github.com/sushistack/yt.pipe/internal/plugin/imagegen"
	"github.com/sushistack/yt.pipe/internal/plugin/llm"
	"github.com/sushistack/yt.pipe/internal/plugin/output/capcut"
	"github.com/sushistack/yt.pipe/internal/plugin/tts"
	"github.com/sushistack/yt.pipe/internal/store"
	"github.com/sushistack/yt.pipe/internal/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test fixture path - relative to project root
func testdataPath() string {
	// Integration tests are run from the project root via `go test -tags=integration ./...`
	// Check common locations
	candidates := []string{
		"testdata",
		"../../testdata",
	}
	for _, c := range candidates {
		if _, err := os.Stat(filepath.Join(c, "SCP-173")); err == nil {
			return c
		}
	}
	return "testdata"
}

func skipIfNoKey(t *testing.T, envVar string) string {
	t.Helper()
	key := os.Getenv(envVar)
	if key == "" {
		t.Skipf("%s not set", envVar)
	}
	return key
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
}

func testStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.New(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { s.Close() })
	return s
}

func testWorkspace(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	return dir
}

// TestGeminiScenarioGeneration validates the LLM provider can generate a scenario.
func TestGeminiScenarioGeneration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	apiKey := skipIfNoKey(t, "GEMINI_API_KEY")

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	// Create Gemini LLM provider
	provider, err := llm.GeminiFactory(map[string]interface{}{
		"api_key": apiKey,
		"model":   "gemini-2.0-flash",
	})
	require.NoError(t, err)

	llmProvider, ok := provider.(llm.LLM)
	require.True(t, ok)

	// Load SCP data
	scpData, err := workspace.LoadSCPData(testdataPath(), "SCP-173")
	require.NoError(t, err)

	// Generate scenario
	metadata := map[string]string{
		"title":        scpData.Meta.Title,
		"object_class": scpData.Meta.ObjectClass,
	}

	result, err := llmProvider.GenerateScenario(ctx, "SCP-173", scpData.MainText, nil, metadata)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Validate scenario structure
	assert.NotEmpty(t, result.Scenes, "scenario should have at least one scene")
	for i, scene := range result.Scenes {
		assert.NotEmpty(t, scene.Narration, "scene %d narration should not be empty", i+1)
		assert.NotEmpty(t, scene.VisualDescription, "scene %d visual description should not be empty", i+1)
	}

	t.Logf("Generated scenario with %d scenes", len(result.Scenes))
}

// TestSiliconFlowImageGeneration validates the image generation provider.
func TestSiliconFlowImageGeneration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	apiKey := skipIfNoKey(t, "SILICONFLOW_API_KEY")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Create SiliconFlow provider
	provider, err := imagegen.SiliconFlowFactory(map[string]interface{}{
		"api_key": apiKey,
		"model":   "black-forest-labs/FLUX.1-schnell",
	})
	require.NoError(t, err)

	imgProvider, ok := provider.(imagegen.ImageGen)
	require.True(t, ok)

	// Generate a test image
	result, err := imgProvider.Generate(ctx, "A concrete sculpture in a dimly lit containment room, security camera perspective, institutional setting", imagegen.GenerateOptions{
		Width:  1024,
		Height: 576,
	})
	require.NoError(t, err)
	require.NotNil(t, result)

	// Validate result
	assert.NotEmpty(t, result.ImageData, "image data should not be empty")
	assert.Greater(t, len(result.ImageData), 1000, "image data should be substantial")

	t.Logf("Generated image: %d bytes", len(result.ImageData))
}

// TestDashScopeTTSSynthesis validates the TTS provider.
func TestDashScopeTTSSynthesis(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	apiKey := skipIfNoKey(t, "DASHSCOPE_API_KEY")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create DashScope provider
	provider, err := tts.DashScopeFactory(map[string]interface{}{
		"api_key": apiKey,
		"model":   "cosyvoice-v1",
	})
	require.NoError(t, err)

	ttsProvider, ok := provider.(tts.TTS)
	require.True(t, ok)

	// Synthesize Korean text
	result, err := ttsProvider.Synthesize(ctx, "SCP-173은 콘크리트와 철근으로 만들어진 조각상입니다.", "")
	require.NoError(t, err)
	require.NotNil(t, result)

	// Validate audio result
	assert.NotEmpty(t, result.AudioData, "audio data should not be empty")
	assert.Greater(t, result.DurationSec, 0.0, "duration should be positive")

	t.Logf("Synthesized audio: %d bytes, %.1fs duration, %d word timings",
		len(result.AudioData), result.DurationSec, len(result.WordTimings))
}

// TestFallbackChainActivation validates that the fallback chain works when primary fails.
func TestFallbackChainActivation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Need at least one working fallback key
	fallbackKey := os.Getenv("GEMINI_API_KEY")
	if fallbackKey == "" {
		fallbackKey = os.Getenv("QWEN_API_KEY")
	}
	if fallbackKey == "" {
		t.Skip("no fallback API key available (GEMINI_API_KEY or QWEN_API_KEY)")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	// Create primary provider with invalid key (should fail)
	primaryProvider, err := llm.GeminiFactory(map[string]interface{}{
		"api_key": "invalid-key-that-should-fail",
		"model":   "gemini-2.0-flash",
	})
	require.NoError(t, err)

	primary, ok := primaryProvider.(llm.LLM)
	require.True(t, ok)

	// Create fallback provider with valid key
	fallbackProvider, err := llm.GeminiFactory(map[string]interface{}{
		"api_key": fallbackKey,
		"model":   "gemini-2.0-flash",
	})
	require.NoError(t, err)

	fallback, ok := fallbackProvider.(llm.LLM)
	require.True(t, ok)

	// Create fallback chain
	chain, err := llm.NewFallbackChain(
		[]llm.LLM{primary, fallback},
		[]string{"primary-invalid", "fallback-valid"},
	)
	require.NoError(t, err)

	// Test completion - should fall back to working provider
	metadata := map[string]string{"title": "SCP-173"}
	result, err := chain.GenerateScenario(ctx, "SCP-173", "Test content for SCP-173 containment procedures.", nil, metadata)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.NotEmpty(t, result.Scenes)

	t.Logf("Fallback chain activated successfully, got %d scenes", len(result.Scenes))
}

// TestFullPipelineE2E runs the complete pipeline end-to-end with all real providers.
func TestFullPipelineE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	geminiKey := skipIfNoKey(t, "GEMINI_API_KEY")
	siliconflowKey := skipIfNoKey(t, "SILICONFLOW_API_KEY")
	dashscopeKey := skipIfNoKey(t, "DASHSCOPE_API_KEY")

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()

	// Create all providers
	llmRaw, err := llm.GeminiFactory(map[string]interface{}{
		"api_key": geminiKey,
		"model":   "gemini-2.0-flash",
	})
	require.NoError(t, err)
	llmProvider := llmRaw.(llm.LLM)

	imgRaw, err := imagegen.SiliconFlowFactory(map[string]interface{}{
		"api_key": siliconflowKey,
		"model":   "black-forest-labs/FLUX.1-schnell",
	})
	require.NoError(t, err)
	imgProvider := imgRaw.(imagegen.ImageGen)

	ttsRaw, err := tts.DashScopeFactory(map[string]interface{}{
		"api_key": dashscopeKey,
		"model":   "cosyvoice-v1",
	})
	require.NoError(t, err)
	ttsProvider := ttsRaw.(tts.TTS)

	assembler := capcut.New()
	db := testStore(t)
	wsPath := testWorkspace(t)
	logger := testLogger()

	runner := pipeline.NewRunner(db, llmProvider, imgProvider, ttsProvider, assembler,
		glossary.New(), logger, pipeline.RunnerConfig{
			SCPDataPath:          testdataPath(),
			WorkspacePath:        wsPath,
			Voice:                "",
			ImageOpts:            imagegen.GenerateOptions{Width: 1024, Height: 576},
			DefaultSceneDuration: 5.0,
		})

	// Run pipeline with auto-approve
	result, err := runner.RunWithOptions(ctx, "SCP-173", pipeline.RunOptions{
		AutoApprove: true,
	})
	require.NoError(t, err)
	require.NotNil(t, result)

	// Validate result
	assert.Equal(t, "complete", result.Status)
	assert.NotEmpty(t, result.ProjectID)
	assert.Equal(t, "SCP-173", result.SCPID)
	assert.Greater(t, result.SceneCount, 0)
	assert.Greater(t, result.APICalls, 0)

	// Validate stages
	completedStages := 0
	for _, s := range result.Stages {
		if s.Status == "pass" || s.Status == "auto-approved" {
			completedStages++
		}
	}
	assert.GreaterOrEqual(t, completedStages, 5, "at least 5 stages should have completed")

	t.Logf("E2E pipeline completed: %d scenes, %d stages, %d API calls, $%.4f estimated cost, %v elapsed",
		result.SceneCount, len(result.Stages), result.APICalls, result.EstimatedCost, result.TotalElapsed)
}
