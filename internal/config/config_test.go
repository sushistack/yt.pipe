package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Priority Chain Tests ---

func TestLoad_DefaultValues(t *testing.T) {
	// No config files, no env vars — only defaults
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	_ = os.Chdir(dir)
	defer func() { _ = os.Chdir(origDir) }()
	t.Setenv("HOME", dir) // Isolate from real global config

	result, err := Load("")
	require.NoError(t, err)
	cfg := result.Config

	assert.Equal(t, "/data/raw", cfg.SCPDataPath)
	assert.Equal(t, "/data/projects", cfg.WorkspacePath)
	assert.Equal(t, "/data/db/yt-pipe.db", cfg.DBPath)
	assert.Equal(t, "localhost", cfg.API.Host)
	assert.Equal(t, 8080, cfg.API.Port)
	assert.Equal(t, "gemini", cfg.LLM.Provider)
	assert.Equal(t, "https://generativelanguage.googleapis.com/v1beta/openai", cfg.LLM.Endpoint)
	assert.Equal(t, "gemini-2.0-flash", cfg.LLM.Model)
	assert.Equal(t, 0.7, cfg.LLM.Temperature)
	assert.Equal(t, 4096, cfg.LLM.MaxTokens)
	assert.Equal(t, 80.0, cfg.Scenario.FactCoverageThreshold)
	assert.Equal(t, 10, cfg.Scenario.TargetDurationMin)
	assert.Equal(t, "siliconflow", cfg.ImageGen.Provider)
	assert.Equal(t, "dashscope", cfg.TTS.Provider)
	assert.Equal(t, 1.0, cfg.TTS.Speed)
	assert.Equal(t, "capcut", cfg.Output.Provider)
	assert.Equal(t, "info", cfg.LogLevel)
	assert.Equal(t, "json", cfg.LogFormat)
}

func TestLoad_NoConfigFilesExist(t *testing.T) {
	// First-run scenario: no config files at all, must succeed with defaults
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	_ = os.Chdir(dir)
	defer func() { _ = os.Chdir(origDir) }()

	result, err := Load("")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.Config)
	assert.Equal(t, "info", result.Config.LogLevel)

	// Verify TTS clone config defaults
	assert.Equal(t, "qwen3-tts-vc-2026-01-22", result.Config.TTS.Clone.Model)
	assert.Equal(t, "narrator", result.Config.TTS.Clone.PreferredName)
	assert.Empty(t, result.Config.TTS.Clone.SamplePath)
}

func TestLoad_TTSCloneConfig(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	_ = os.Chdir(dir)
	defer func() { _ = os.Chdir(origDir) }()

	cfgContent := `
tts:
  clone:
    model: "qwen3-tts-vc-2026-01-22"
    sample_path: "/data/voice/sample.mp3"
    preferred_name: "my-narrator"
`
	cfgPath := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(cfgPath, []byte(cfgContent), 0o644))

	result, err := Load(cfgPath)
	require.NoError(t, err)
	assert.Equal(t, "qwen3-tts-vc-2026-01-22", result.Config.TTS.Clone.Model)
	assert.Equal(t, "/data/voice/sample.mp3", result.Config.TTS.Clone.SamplePath)
	assert.Equal(t, "my-narrator", result.Config.TTS.Clone.PreferredName)
}

func TestLoad_GlobalConfig(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	_ = os.Chdir(dir)
	defer func() { _ = os.Chdir(origDir) }()

	// Create a global config in a temp location
	globalDir := filepath.Join(dir, ".yt-pipe")
	require.NoError(t, os.MkdirAll(globalDir, 0o755))
	globalCfg := `
scp_data_path: "/global/raw"
log_level: "debug"
llm:
  provider: "anthropic"
  model: "claude-3"
`
	require.NoError(t, os.WriteFile(filepath.Join(globalDir, "config.yaml"), []byte(globalCfg), 0o644))

	// Override HOME to use temp dir
	t.Setenv("HOME", dir)

	result, err := Load("")
	require.NoError(t, err)
	cfg := result.Config

	assert.Equal(t, "/global/raw", cfg.SCPDataPath)
	assert.Equal(t, "debug", cfg.LogLevel)
	assert.Equal(t, "anthropic", cfg.LLM.Provider)
	assert.Equal(t, "claude-3", cfg.LLM.Model)
	// Defaults still apply for unset keys
	assert.Equal(t, 8080, cfg.API.Port)
	assert.Equal(t, "siliconflow", cfg.ImageGen.Provider)
}

func TestLoad_ProjectOverridesGlobal(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	_ = os.Chdir(dir)
	defer func() { _ = os.Chdir(origDir) }()

	// Global config
	globalDir := filepath.Join(dir, ".yt-pipe")
	require.NoError(t, os.MkdirAll(globalDir, 0o755))
	globalCfg := `
scp_data_path: "/global/raw"
log_level: "debug"
llm:
  provider: "anthropic"
  model: "claude-3"
  temperature: 0.5
`
	require.NoError(t, os.WriteFile(filepath.Join(globalDir, "config.yaml"), []byte(globalCfg), 0o644))
	t.Setenv("HOME", dir)

	// Project config overrides only llm.provider
	projectCfg := `
llm:
  provider: "openai"
log_level: "warn"
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(projectCfg), 0o644))

	result, err := Load("")
	require.NoError(t, err)
	cfg := result.Config

	// Project overrides
	assert.Equal(t, "openai", cfg.LLM.Provider)
	assert.Equal(t, "warn", cfg.LogLevel)
	// Global values preserved for non-overridden keys
	assert.Equal(t, "/global/raw", cfg.SCPDataPath)
}

func TestLoad_EnvOverridesFile(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	_ = os.Chdir(dir)
	defer func() { _ = os.Chdir(origDir) }()

	// Project config
	projectCfg := `
log_level: "debug"
api:
  port: 9090
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(projectCfg), 0o644))
	t.Setenv("HOME", dir) // no global config

	// Env vars override file
	t.Setenv("YTP_LOG_LEVEL", "error")
	t.Setenv("YTP_API_PORT", "3000")

	result, err := Load("")
	require.NoError(t, err)
	cfg := result.Config

	assert.Equal(t, "error", cfg.LogLevel)
	assert.Equal(t, 3000, cfg.API.Port)
}

func TestLoad_ExplicitConfigPath(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	_ = os.Chdir(dir)
	defer func() { _ = os.Chdir(origDir) }()

	customCfg := `
scp_data_path: "/custom/data"
log_level: "error"
`
	customPath := filepath.Join(dir, "custom.yaml")
	require.NoError(t, os.WriteFile(customPath, []byte(customCfg), 0o644))
	t.Setenv("HOME", dir)

	result, err := Load(customPath)
	require.NoError(t, err)
	cfg := result.Config

	assert.Equal(t, "/custom/data", cfg.SCPDataPath)
	assert.Equal(t, "error", cfg.LogLevel)
	// Defaults for unset
	assert.Equal(t, 8080, cfg.API.Port)
}

// --- Environment Variable Mapping Tests ---

func TestLoad_EnvMapping_LLMApiKey(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	_ = os.Chdir(dir)
	defer func() { _ = os.Chdir(origDir) }()
	t.Setenv("HOME", dir)

	t.Setenv("YTP_LLM_API_KEY", "sk-test-123")

	result, err := Load("")
	require.NoError(t, err)
	assert.Equal(t, "sk-test-123", result.Config.LLM.APIKey)
}

func TestLoad_EnvMapping_SiliconFlowKey(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	_ = os.Chdir(dir)
	defer func() { _ = os.Chdir(origDir) }()
	t.Setenv("HOME", dir)

	t.Setenv("YTP_SILICONFLOW_KEY", "sf-key-456")

	result, err := Load("")
	require.NoError(t, err)
	assert.Equal(t, "sf-key-456", result.Config.ImageGen.APIKey)
}

func TestLoad_EnvMapping_NestedKeys(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	_ = os.Chdir(dir)
	defer func() { _ = os.Chdir(origDir) }()
	t.Setenv("HOME", dir)

	t.Setenv("YTP_API_PORT", "9999")
	t.Setenv("YTP_API_HOST", "0.0.0.0")

	result, err := Load("")
	require.NoError(t, err)
	assert.Equal(t, 9999, result.Config.API.Port)
	assert.Equal(t, "0.0.0.0", result.Config.API.Host)
}

// --- Config Merging Tests ---

func TestLoad_MergePreservesUnsetKeys(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	_ = os.Chdir(dir)
	defer func() { _ = os.Chdir(origDir) }()

	globalDir := filepath.Join(dir, ".yt-pipe")
	require.NoError(t, os.MkdirAll(globalDir, 0o755))
	globalCfg := `
llm:
  provider: "anthropic"
  model: "claude-3"
  temperature: 0.5
tts:
  provider: "google"
  voice: "ko-KR-Standard-A"
`
	require.NoError(t, os.WriteFile(filepath.Join(globalDir, "config.yaml"), []byte(globalCfg), 0o644))
	t.Setenv("HOME", dir)

	// Project overrides only llm.provider
	projectCfg := `
llm:
  provider: "openai"
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(projectCfg), 0o644))

	result, err := Load("")
	require.NoError(t, err)
	cfg := result.Config

	// Overridden
	assert.Equal(t, "openai", cfg.LLM.Provider)
	// Preserved from global
	assert.Equal(t, "google", cfg.TTS.Provider)
	assert.Equal(t, "ko-KR-Standard-A", cfg.TTS.Voice)
}

// --- Resilience Tests ---

func TestLoad_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	_ = os.Chdir(dir)
	defer func() { _ = os.Chdir(origDir) }()
	t.Setenv("HOME", dir)

	// Write invalid YAML as project config
	require.NoError(t, os.WriteFile(filepath.Join(dir, "config.yaml"), []byte("{{invalid yaml:::"), 0o644))

	_, err := Load("")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "config load")
}

func TestLoad_GlobalConfigMissing(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	_ = os.Chdir(dir)
	defer func() { _ = os.Chdir(origDir) }()

	// HOME points to dir with no .yt-pipe/ — should silently ignore
	t.Setenv("HOME", dir)

	result, err := Load("")
	require.NoError(t, err)
	require.NotNil(t, result)
	// Defaults applied
	assert.Equal(t, "info", result.Config.LogLevel)
}

// --- Validation Tests ---

func TestValidate_MissingRequiredFields(t *testing.T) {
	cfg := &Config{
		API: APIConfig{Port: 8080},
	}
	result := Validate(cfg)
	// No errors for empty strings — they're just not configured yet
	assert.True(t, result.IsValid())
}

func TestValidate_InvalidPort(t *testing.T) {
	tests := []struct {
		name string
		port int
	}{
		{"zero", 0},
		{"negative", -1},
		{"too_high", 70000},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{API: APIConfig{Port: tt.port}}
			result := Validate(cfg)
			assert.False(t, result.IsValid())
			assert.Len(t, result.Errors, 1)
			assert.Contains(t, result.Errors[0], "api.port")
		})
	}
}

func TestValidate_PathsWarnOnly(t *testing.T) {
	cfg := &Config{
		SCPDataPath:   "/nonexistent/scp/data",
		WorkspacePath: "/nonexistent/workspace",
		DBPath:        "/nonexistent/db/yt-pipe.db",
		API:           APIConfig{Port: 8080},
		LogLevel:      "info",
		LogFormat:     "json",
	}
	result := Validate(cfg)
	// Should be valid (paths are warnings, not errors)
	assert.True(t, result.IsValid())
	assert.NotEmpty(t, result.Warnings)
}

func TestValidate_ValidConfig(t *testing.T) {
	dir := t.TempDir()
	cfg := &Config{
		SCPDataPath:   dir,
		WorkspacePath: dir,
		DBPath:        filepath.Join(dir, "test.db"),
		API:           APIConfig{Host: "localhost", Port: 8080},
		LogLevel:      "info",
		LogFormat:     "json",
	}
	result := Validate(cfg)
	assert.True(t, result.IsValid())
	assert.Empty(t, result.Errors)
	assert.Empty(t, result.Warnings)
}

func TestValidate_InvalidLogLevel(t *testing.T) {
	cfg := &Config{
		API:      APIConfig{Port: 8080},
		LogLevel: "verbose",
	}
	result := Validate(cfg)
	assert.False(t, result.IsValid())
	assert.Contains(t, result.Errors[0], "log_level")
}

func TestValidate_InvalidLogFormat(t *testing.T) {
	cfg := &Config{
		API:       APIConfig{Port: 8080},
		LogFormat: "xml",
	}
	result := Validate(cfg)
	assert.False(t, result.IsValid())
	assert.Contains(t, result.Errors[0], "log_format")
}

// --- Secret Masking Tests ---

func TestMaskSecrets_ApiKeyFields(t *testing.T) {
	cfg := &Config{
		API:      APIConfig{APIKey: "my-api-key", Host: "localhost", Port: 8080},
		LLM:     LLMConfig{APIKey: "sk-123", Provider: "openai"},
		ImageGen: ImageGenConfig{APIKey: "sf-456", Provider: "siliconflow"},
		TTS:     TTSConfig{APIKey: "tts-789", Provider: "openai"},
	}
	masked := MaskSecrets(cfg)

	assert.Equal(t, "***", masked.API.APIKey)
	assert.Equal(t, "***", masked.LLM.APIKey)
	assert.Equal(t, "***", masked.ImageGen.APIKey)
	assert.Equal(t, "***", masked.TTS.APIKey)
}

func TestMaskSecrets_NonSecretFieldsUnchanged(t *testing.T) {
	cfg := &Config{
		SCPDataPath: "/data/raw",
		API:         APIConfig{Host: "localhost", Port: 8080},
		LLM:        LLMConfig{Provider: "openai", Model: "gpt-4"},
		LogLevel:    "info",
	}
	masked := MaskSecrets(cfg)

	assert.Equal(t, "/data/raw", masked.SCPDataPath)
	assert.Equal(t, "localhost", masked.API.Host)
	assert.Equal(t, 8080, masked.API.Port)
	assert.Equal(t, "openai", masked.LLM.Provider)
	assert.Equal(t, "gpt-4", masked.LLM.Model)
	assert.Equal(t, "info", masked.LogLevel)
}

func TestMaskSecrets_EmptyKeysStayEmpty(t *testing.T) {
	cfg := &Config{
		API: APIConfig{APIKey: ""},
		LLM: LLMConfig{APIKey: ""},
	}
	masked := MaskSecrets(cfg)
	assert.Equal(t, "", masked.API.APIKey)
	assert.Equal(t, "", masked.LLM.APIKey)
}

// --- Source Tracking Tests ---

func TestLoad_SourceTracking_DefaultsOnly(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	_ = os.Chdir(dir)
	defer func() { _ = os.Chdir(origDir) }()
	t.Setenv("HOME", dir)

	result, err := Load("")
	require.NoError(t, err)

	// All keys should have "default" source when no config files exist
	assert.Equal(t, "default", result.Sources["log_level"])
	assert.Equal(t, "default", result.Sources["api.port"])
}

func TestLoad_SourceTracking_GlobalConfig(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	_ = os.Chdir(dir)
	defer func() { _ = os.Chdir(origDir) }()

	// Create global config that overrides log_level
	globalDir := filepath.Join(dir, ".yt-pipe")
	require.NoError(t, os.MkdirAll(globalDir, 0o755))
	globalCfg := `log_level: "debug"`
	require.NoError(t, os.WriteFile(filepath.Join(globalDir, "config.yaml"), []byte(globalCfg), 0o644))
	t.Setenv("HOME", dir)

	result, err := Load("")
	require.NoError(t, err)

	assert.Equal(t, "global config", result.Sources["log_level"])
	// Unset keys remain "default"
	assert.Equal(t, "default", result.Sources["api.port"])
}

func TestLoad_SourceTracking_ProjectConfig(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	_ = os.Chdir(dir)
	defer func() { _ = os.Chdir(origDir) }()
	t.Setenv("HOME", dir)

	// Project config overrides api.port
	projectCfg := `
api:
  port: 9090
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(projectCfg), 0o644))

	result, err := Load("")
	require.NoError(t, err)

	assert.Equal(t, "project config", result.Sources["api.port"])
	assert.Equal(t, "default", result.Sources["log_level"])
}

func TestLoad_AutoApprovalDefaults(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	_ = os.Chdir(dir)
	defer func() { _ = os.Chdir(origDir) }()
	t.Setenv("HOME", dir)

	result, err := Load("")
	require.NoError(t, err)
	assert.False(t, result.Config.AutoApproval.Enabled)
	assert.Equal(t, 80, result.Config.AutoApproval.Threshold)
}

func TestValidate_AutoApprovalWithoutImageValidation(t *testing.T) {
	cfg := &Config{
		API:             APIConfig{Port: 8080},
		AutoApproval:    AutoApproval{Enabled: true, Threshold: 80},
		ImageValidation: ImageValidation{Enabled: false},
	}
	result := Validate(cfg)
	assert.True(t, result.IsValid()) // warning, not error
	require.NotEmpty(t, result.Warnings)
	assert.Contains(t, result.Warnings[0], "auto_approval requires image_validation")
}

func TestValidate_AutoApprovalWithImageValidation(t *testing.T) {
	dir := t.TempDir()
	cfg := &Config{
		SCPDataPath:     dir,
		WorkspacePath:   dir,
		DBPath:          dir + "/test.db",
		API:             APIConfig{Port: 8080},
		AutoApproval:    AutoApproval{Enabled: true, Threshold: 80},
		ImageValidation: ImageValidation{Enabled: true, Threshold: 70, MaxAttempts: 3},
	}
	result := Validate(cfg)
	assert.True(t, result.IsValid())
	// No auto_approval warning when image_validation is enabled
	for _, w := range result.Warnings {
		assert.NotContains(t, w, "auto_approval")
	}
}

func TestLoad_SourceTracking_EnvOverride(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	_ = os.Chdir(dir)
	defer func() { _ = os.Chdir(origDir) }()
	t.Setenv("HOME", dir)

	t.Setenv("YTP_LOG_LEVEL", "error")

	result, err := Load("")
	require.NoError(t, err)

	assert.Equal(t, "env YTP_LOG_LEVEL", result.Sources["log_level"])
}
