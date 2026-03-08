package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConfig_ZeroValues(t *testing.T) {
	var cfg Config
	assert.Empty(t, cfg.SCPDataPath)
	assert.Empty(t, cfg.WorkspacePath)
	assert.Empty(t, cfg.DBPath)
	assert.Empty(t, cfg.GlossaryPath)
	assert.Empty(t, cfg.LogLevel)
	assert.Empty(t, cfg.LogFormat)
}

func TestAPIConfig_ZeroValues(t *testing.T) {
	var cfg APIConfig
	assert.Empty(t, cfg.Host)
	assert.Equal(t, 0, cfg.Port)
	assert.Empty(t, cfg.APIKey)
}

func TestLLMConfig_ZeroValues(t *testing.T) {
	var cfg LLMConfig
	assert.Empty(t, cfg.Provider)
	assert.Empty(t, cfg.APIKey)
	assert.Empty(t, cfg.Model)
	assert.Equal(t, float64(0), cfg.Temperature)
	assert.Equal(t, 0, cfg.MaxTokens)
}

func TestImageGenConfig_ZeroValues(t *testing.T) {
	var cfg ImageGenConfig
	assert.Empty(t, cfg.Provider)
	assert.Empty(t, cfg.APIKey)
	assert.Empty(t, cfg.Model)
}

func TestTTSConfig_ZeroValues(t *testing.T) {
	var cfg TTSConfig
	assert.Empty(t, cfg.Provider)
	assert.Empty(t, cfg.APIKey)
	assert.Empty(t, cfg.Voice)
	assert.Equal(t, float64(0), cfg.Speed)
}

func TestOutputConfig_ZeroValues(t *testing.T) {
	var cfg OutputConfig
	assert.Empty(t, cfg.Provider)
	assert.Empty(t, cfg.TemplatePath)
	assert.Empty(t, cfg.MetaPath)
	assert.Equal(t, 0, cfg.CanvasWidth)
	assert.Equal(t, 0, cfg.CanvasHeight)
	assert.Equal(t, 0, cfg.FPS)
}
