// Package config handles application configuration loading and validation.
package config

// Config represents the complete application configuration.
type Config struct {
	SCPDataPath   string         `mapstructure:"scp_data_path"`
	WorkspacePath string         `mapstructure:"workspace_path"`
	DBPath        string         `mapstructure:"db_path"`
	API           APIConfig      `mapstructure:"api"`
	LLM           LLMConfig      `mapstructure:"llm"`
	ImageGen      ImageGenConfig `mapstructure:"imagegen"`
	TTS           TTSConfig      `mapstructure:"tts"`
	Output        OutputConfig   `mapstructure:"output"`
	Webhooks      WebhookConfig  `mapstructure:"webhooks"`
	GlossaryPath  string         `mapstructure:"glossary_path"`
	TemplatesPath string         `mapstructure:"templates_path"`
	LogLevel      string         `mapstructure:"log_level"`
	LogFormat     string         `mapstructure:"log_format"`
}

// WebhookConfig holds webhook notification settings.
type WebhookConfig struct {
	URLs            []string `mapstructure:"urls"`
	TimeoutSeconds  int      `mapstructure:"timeout_seconds"`
	RetryMaxAttempts int     `mapstructure:"retry_max_attempts"`
}

// AuthConfig holds API authentication settings.
type AuthConfig struct {
	Enabled bool   `mapstructure:"enabled"`
	Key     string `mapstructure:"key"`
}

// APIConfig holds HTTP API server settings.
type APIConfig struct {
	Host   string     `mapstructure:"host"`
	Port   int        `mapstructure:"port"`
	APIKey string     `mapstructure:"api_key"`
	Auth   AuthConfig `mapstructure:"auth"`
}

// LLMConfig holds LLM plugin settings.
type LLMConfig struct {
	Provider    string  `mapstructure:"provider"`
	APIKey      string  `mapstructure:"api_key"`
	Model       string  `mapstructure:"model"`
	Temperature float64 `mapstructure:"temperature"`
	MaxTokens   int     `mapstructure:"max_tokens"`
}

// ImageGenConfig holds image generation plugin settings.
type ImageGenConfig struct {
	Provider string `mapstructure:"provider"`
	APIKey   string `mapstructure:"api_key"`
	Model    string `mapstructure:"model"`
}

// TTSConfig holds text-to-speech plugin settings.
type TTSConfig struct {
	Provider string  `mapstructure:"provider"`
	APIKey   string  `mapstructure:"api_key"`
	Voice    string  `mapstructure:"voice"`
	Speed    float64 `mapstructure:"speed"`
}

// OutputConfig holds output assembler plugin settings.
type OutputConfig struct {
	Provider     string `mapstructure:"provider"`
	TemplatePath string `mapstructure:"template_path"` // Path to CapCut draft template JSON
	MetaPath     string `mapstructure:"meta_path"`     // Path to CapCut draft meta info JSON
	CanvasWidth  int    `mapstructure:"canvas_width"`  // Output canvas width (default: 1920)
	CanvasHeight int    `mapstructure:"canvas_height"` // Output canvas height (default: 1080)
	FPS          int    `mapstructure:"fps"`            // Frames per second (default: 30)
}
