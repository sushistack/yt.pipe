package pipeline

import (
	"context"
	"testing"

	"github.com/jay/youtube-pipeline/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validConfig(t *testing.T) *config.Config {
	t.Helper()
	scpDir := t.TempDir()
	wsDir := t.TempDir()
	t.Setenv("YTP_LLM_API_KEY", "test-llm-key")
	t.Setenv("YTP_IMAGEGEN_API_KEY", "test-imagegen-key")
	t.Setenv("YTP_TTS_API_KEY", "test-tts-key")
	return &config.Config{
		SCPDataPath:   scpDir,
		WorkspacePath: wsDir,
		LLM:           config.LLMConfig{Provider: "openai", APIKey: "test-llm-key", Model: "gpt-4"},
		ImageGen:      config.ImageGenConfig{Provider: "siliconflow", APIKey: "test-imagegen-key", Model: "flux"},
		TTS:           config.TTSConfig{Provider: "openai", APIKey: "test-tts-key", Voice: "alloy", Speed: 1.0},
		Output:        config.OutputConfig{Provider: "capcut"},
	}
}

func TestRunDryRun_ValidConfig(t *testing.T) {
	cfg := validConfig(t)
	result, err := RunDryRun(context.Background(), cfg, "SCP-173")
	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Equal(t, "SCP-173", result.SCPID)
	assert.Empty(t, result.Errors)
	assert.Len(t, result.Stages, 7)
	for _, s := range result.Stages {
		assert.Equal(t, "pass", s.Status, "stage %s should pass", s.Name)
	}
}

func TestRunDryRun_NilConfig(t *testing.T) {
	_, err := RunDryRun(context.Background(), nil, "SCP-173")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "config is nil")
}

func TestRunDryRun_MissingSCPDataPath(t *testing.T) {
	cfg := validConfig(t)
	cfg.SCPDataPath = "/nonexistent/path"

	result, err := RunDryRun(context.Background(), cfg, "SCP-173")
	require.NoError(t, err)
	assert.False(t, result.Success)
	assert.Equal(t, "fail", result.Stages[0].Status)
	assert.Contains(t, result.Stages[0].Error, "does not exist")
}

func TestRunDryRun_EmptySCPDataPath(t *testing.T) {
	cfg := validConfig(t)
	cfg.SCPDataPath = ""

	result, err := RunDryRun(context.Background(), cfg, "SCP-173")
	require.NoError(t, err)
	assert.False(t, result.Success)
	assert.Equal(t, "fail", result.Stages[0].Status)
	assert.Contains(t, result.Stages[0].Error, "not configured")
}

func TestRunDryRun_MissingLLMProvider(t *testing.T) {
	cfg := validConfig(t)
	cfg.LLM.Provider = ""

	result, err := RunDryRun(context.Background(), cfg, "SCP-173")
	require.NoError(t, err)
	assert.False(t, result.Success)
	// Find scenario_generate stage
	for _, s := range result.Stages {
		if s.Name == "scenario_generate" {
			assert.Equal(t, "fail", s.Status)
			assert.Contains(t, s.Error, "llm.provider")
			return
		}
	}
	t.Fatal("scenario_generate stage not found")
}

func TestRunDryRun_MissingLLMAPIKey(t *testing.T) {
	cfg := validConfig(t)
	cfg.LLM.APIKey = ""

	result, err := RunDryRun(context.Background(), cfg, "SCP-173")
	require.NoError(t, err)
	assert.False(t, result.Success)
	for _, s := range result.Stages {
		if s.Name == "scenario_generate" {
			assert.Equal(t, "fail", s.Status)
			assert.Contains(t, s.Error, "YTP_LLM_API_KEY")
			return
		}
	}
	t.Fatal("scenario_generate stage not found")
}

func TestRunDryRun_MissingImageGenAPIKey(t *testing.T) {
	cfg := validConfig(t)
	cfg.ImageGen.APIKey = ""

	result, err := RunDryRun(context.Background(), cfg, "SCP-173")
	require.NoError(t, err)
	assert.False(t, result.Success)
	for _, s := range result.Stages {
		if s.Name == "image_generate" {
			assert.Equal(t, "fail", s.Status)
			assert.Contains(t, s.Error, "YTP_IMAGEGEN_API_KEY")
			return
		}
	}
	t.Fatal("image_generate stage not found")
}

func TestRunDryRun_TTSEdgeNoAPIKey(t *testing.T) {
	cfg := validConfig(t)
	cfg.TTS.Provider = "edge"
	cfg.TTS.APIKey = ""

	result, err := RunDryRun(context.Background(), cfg, "SCP-173")
	require.NoError(t, err)
	for _, s := range result.Stages {
		if s.Name == "tts_synthesize" {
			assert.Equal(t, "pass", s.Status, "edge TTS should pass without API key")
			return
		}
	}
	t.Fatal("tts_synthesize stage not found")
}

func TestRunDryRun_MissingTTSAPIKey(t *testing.T) {
	cfg := validConfig(t)
	cfg.TTS.Provider = "openai"
	cfg.TTS.APIKey = ""

	result, err := RunDryRun(context.Background(), cfg, "SCP-173")
	require.NoError(t, err)
	assert.False(t, result.Success)
	for _, s := range result.Stages {
		if s.Name == "tts_synthesize" {
			assert.Equal(t, "fail", s.Status)
			assert.Contains(t, s.Error, "YTP_TTS_API_KEY")
			return
		}
	}
	t.Fatal("tts_synthesize stage not found")
}

func TestRunDryRun_MissingOutputProvider(t *testing.T) {
	cfg := validConfig(t)
	cfg.Output.Provider = ""

	result, err := RunDryRun(context.Background(), cfg, "SCP-173")
	require.NoError(t, err)
	assert.False(t, result.Success)
	for _, s := range result.Stages {
		if s.Name == "output_assemble" {
			assert.Equal(t, "fail", s.Status)
			assert.Contains(t, s.Error, "output.provider")
			return
		}
	}
	t.Fatal("output_assemble stage not found")
}

func TestRunDryRun_MissingWorkspacePath(t *testing.T) {
	cfg := validConfig(t)
	cfg.WorkspacePath = "/nonexistent/workspace"

	result, err := RunDryRun(context.Background(), cfg, "SCP-173")
	require.NoError(t, err)
	assert.False(t, result.Success)
	for _, s := range result.Stages {
		if s.Name == "output_assemble" {
			assert.Equal(t, "fail", s.Status)
			assert.Contains(t, s.Error, "does not exist")
			return
		}
	}
	t.Fatal("output_assemble stage not found")
}

func TestRunDryRun_AllStagesExecuted(t *testing.T) {
	cfg := validConfig(t)
	result, err := RunDryRun(context.Background(), cfg, "SCP-999")
	require.NoError(t, err)

	expectedStages := []string{
		"scp_load", "scenario_generate", "image_generate",
		"tts_synthesize", "timing_resolve", "subtitle_generate", "output_assemble",
	}
	require.Len(t, result.Stages, len(expectedStages))
	for i, name := range expectedStages {
		assert.Equal(t, name, result.Stages[i].Name)
	}
}

func TestRunDryRun_ConfigMasked(t *testing.T) {
	cfg := validConfig(t)
	result, err := RunDryRun(context.Background(), cfg, "SCP-173")
	require.NoError(t, err)
	// API keys should be masked in result
	assert.Equal(t, "***", result.Config.LLMAPIKey)
	assert.Equal(t, "***", result.Config.ImageGenAPIKey)
	assert.Equal(t, "***", result.Config.TTSAPIKey)
}

func TestRunDryRun_ContextCancelled(t *testing.T) {
	cfg := validConfig(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	result, err := RunDryRun(ctx, cfg, "SCP-173")
	require.NoError(t, err)
	assert.False(t, result.Success)
	// All stages should be skipped
	for _, s := range result.Stages {
		assert.Equal(t, "skip", s.Status)
	}
}

func TestRunDryRun_DeterministicOutput(t *testing.T) {
	cfg := validConfig(t)
	r1, _ := RunDryRun(context.Background(), cfg, "SCP-173")
	r2, _ := RunDryRun(context.Background(), cfg, "SCP-173")

	assert.Equal(t, r1.Success, r2.Success)
	assert.Equal(t, r1.SCPID, r2.SCPID)
	require.Len(t, r1.Stages, len(r2.Stages))
	for i := range r1.Stages {
		assert.Equal(t, r1.Stages[i].Name, r2.Stages[i].Name)
		assert.Equal(t, r1.Stages[i].Status, r2.Stages[i].Status)
		assert.Equal(t, r1.Stages[i].OutputSummary, r2.Stages[i].OutputSummary)
	}
}

func TestRunDryRun_ContinuesAfterFailure(t *testing.T) {
	cfg := validConfig(t)
	cfg.LLM.APIKey = "" // scenario_generate will fail

	result, err := RunDryRun(context.Background(), cfg, "SCP-173")
	require.NoError(t, err)
	assert.False(t, result.Success)
	// Even though scenario_generate fails, later stages should still run
	assert.Len(t, result.Stages, 7)
	// scp_load should pass
	assert.Equal(t, "pass", result.Stages[0].Status)
	// scenario_generate should fail
	assert.Equal(t, "fail", result.Stages[1].Status)
	// Later stages should still execute (not skipped)
	for _, s := range result.Stages[2:] {
		assert.NotEqual(t, "skip", s.Status, "stage %s should not be skipped", s.Name)
	}
}
