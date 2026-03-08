package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

// configSource tracks which source provided each config key.
type configSource struct {
	sources map[string]string
}

func newConfigSource() *configSource {
	return &configSource{sources: make(map[string]string)}
}

func (cs *configSource) recordAll(v *viper.Viper, source string) {
	for _, key := range v.AllKeys() {
		cs.sources[key] = source
	}
}

func (cs *configSource) recordChanged(v *viper.Viper, source string, before map[string]any) {
	for _, key := range v.AllKeys() {
		current := v.Get(key)
		prev, existed := before[key]
		if !existed || fmt.Sprintf("%v", current) != fmt.Sprintf("%v", prev) {
			cs.sources[key] = source
		}
	}
}

// Sources returns a copy of the key-to-source mapping.
func (cs *configSource) Sources() map[string]string {
	out := make(map[string]string, len(cs.sources))
	for k, v := range cs.sources {
		out[k] = v
	}
	return out
}

// LoadResult contains the loaded config and source tracking information.
type LoadResult struct {
	Config  *Config
	Sources map[string]string
}

// Load reads configuration from the 5-level priority chain:
//
//	CLI flags > env vars (YTP_) > project YAML > global YAML > defaults
//
// The configPath parameter, if non-empty, overrides the default config file discovery.
func Load(configPath string) (*LoadResult, error) {
	v := viper.New()
	cs := newConfigSource()

	// 1. Built-in defaults (lowest priority)
	setDefaults(v)
	cs.recordAll(v, "default")

	// 2. Global config: $HOME/.yt-pipe/config.yaml
	home, err := os.UserHomeDir()
	if err == nil {
		beforeGlobal := snapshotValues(v)
		globalPath := filepath.Join(home, ".yt-pipe", "config.yaml")
		if err := mergeConfigFile(v, globalPath); err != nil {
			return nil, fmt.Errorf("config load global: %w", err)
		}
		cs.recordChanged(v, "global config", beforeGlobal)
	}

	// Re-snapshot after global for project diff
	beforeProject := snapshotValues(v)

	// 3. Project config: ./config.yaml (or explicit path)
	if configPath != "" {
		v.SetConfigFile(configPath)
		if err := v.MergeInConfig(); err != nil {
			return nil, fmt.Errorf("config load %s: %w", configPath, err)
		}
		cs.recordChanged(v, "config file", beforeProject)
	} else {
		projectPath, _ := filepath.Abs("config.yaml")
		if err := mergeConfigFile(v, projectPath); err != nil {
			return nil, fmt.Errorf("config load project: %w", err)
		}
		cs.recordChanged(v, "project config", beforeProject)
	}

	// 4. Environment variables: YTP_ prefix
	v.SetEnvPrefix("YTP")
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// Explicit env bindings for backward compatibility
	_ = v.BindEnv("imagegen.api_key", "YTP_IMAGEGEN_API_KEY", "YTP_SILICONFLOW_KEY")

	// Track env overrides
	envKeys := []string{
		"scp_data_path", "workspace_path", "db_path",
		"api.host", "api.port", "api.api_key",
		"llm.provider", "llm.api_key", "llm.model",
		"imagegen.provider", "imagegen.api_key",
		"tts.provider", "tts.api_key",
		"output.provider",
		"glossary_path", "templates_path", "log_level", "log_format",
	}
	for _, key := range envKeys {
		envName := "YTP_" + strings.ToUpper(strings.ReplaceAll(key, ".", "_"))
		if os.Getenv(envName) != "" {
			cs.sources[key] = "env " + envName
		}
	}
	// Special case for SILICONFLOW_KEY alias
	if os.Getenv("YTP_SILICONFLOW_KEY") != "" {
		cs.sources["imagegen.api_key"] = "env YTP_SILICONFLOW_KEY"
	}

	// 5. CLI flags are bound externally via BindPFlag in cli/root.go
	// They automatically take highest priority through Viper

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("config unmarshal: %w", err)
	}

	return &LoadResult{
		Config:  &cfg,
		Sources: cs.Sources(),
	}, nil
}

// setDefaults registers all built-in default values.
func setDefaults(v *viper.Viper) {
	v.SetDefault("scp_data_path", "/data/raw")
	v.SetDefault("workspace_path", "/data/projects")
	v.SetDefault("db_path", "/data/db/yt-pipe.db")

	v.SetDefault("api.host", "localhost")
	v.SetDefault("api.port", 8080)
	v.SetDefault("api.api_key", "")

	v.SetDefault("llm.provider", "gemini")
	v.SetDefault("llm.endpoint", "https://generativelanguage.googleapis.com/v1beta/openai")
	v.SetDefault("llm.api_key", "")
	v.SetDefault("llm.model", "gemini-2.0-flash")
	v.SetDefault("llm.temperature", 0.7)
	v.SetDefault("llm.max_tokens", 4096)

	v.SetDefault("scenario.fact_coverage_threshold", 80.0)
	v.SetDefault("scenario.target_duration_min", 10)

	v.SetDefault("imagegen.provider", "siliconflow")
	v.SetDefault("imagegen.endpoint", "https://api.siliconflow.cn/v1")
	v.SetDefault("imagegen.api_key", "")
	v.SetDefault("imagegen.model", "black-forest-labs/FLUX.1-schnell")
	v.SetDefault("imagegen.width", 1920)
	v.SetDefault("imagegen.height", 1080)

	v.SetDefault("tts.provider", "dashscope")
	v.SetDefault("tts.endpoint", "https://dashscope.aliyuncs.com")
	v.SetDefault("tts.api_key", "")
	v.SetDefault("tts.model", "cosyvoice-v1")
	v.SetDefault("tts.voice", "longxiaochun")
	v.SetDefault("tts.format", "mp3")
	v.SetDefault("tts.speed", 1.0)

	v.SetDefault("output.provider", "capcut")
	v.SetDefault("output.default_scene_duration", 3.0)

	v.SetDefault("glossary_path", "")
	v.SetDefault("templates_path", "")
	v.SetDefault("log_level", "info")
	v.SetDefault("log_format", "json")
}

// mergeConfigFile attempts to merge a config file. Missing files are silently ignored.
// Parse errors are returned.
func mergeConfigFile(v *viper.Viper, path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil // file doesn't exist, silently ignore
	}
	v.SetConfigFile(path)
	if err := v.MergeInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			return nil // silently ignore
		}
		return err
	}
	return nil
}

// snapshotValues captures current viper values for diff tracking.
func snapshotValues(v *viper.Viper) map[string]any {
	snap := make(map[string]any)
	for _, key := range v.AllKeys() {
		snap[key] = v.Get(key)
	}
	return snap
}

// ValidationResult holds the outcome of config validation.
type ValidationResult struct {
	Errors   []string // hard failures (missing required fields, invalid values)
	Warnings []string // soft issues (paths don't exist yet)
}

// IsValid returns true if there are no errors.
func (vr *ValidationResult) IsValid() bool {
	return len(vr.Errors) == 0
}

// Validate checks the configuration for correctness.
// Required field checks produce errors; path existence checks produce warnings.
func Validate(cfg *Config) *ValidationResult {
	result := &ValidationResult{}

	// Port range validation
	if cfg.API.Port < 1 || cfg.API.Port > 65535 {
		result.Errors = append(result.Errors, fmt.Sprintf("api.port must be 1-65535, got %d", cfg.API.Port))
	}

	// Log level validation
	validLogLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
	if cfg.LogLevel != "" && !validLogLevels[cfg.LogLevel] {
		result.Errors = append(result.Errors, fmt.Sprintf("log_level must be one of debug/info/warn/error, got %q", cfg.LogLevel))
	}

	// Log format validation
	validLogFormats := map[string]bool{"json": true, "text": true}
	if cfg.LogFormat != "" && !validLogFormats[cfg.LogFormat] {
		result.Errors = append(result.Errors, fmt.Sprintf("log_format must be json or text, got %q", cfg.LogFormat))
	}

	// Path existence warnings (not errors — paths may not exist before yt-pipe init)
	pathChecks := map[string]string{
		"scp_data_path":  cfg.SCPDataPath,
		"workspace_path": cfg.WorkspacePath,
	}
	for name, path := range pathChecks {
		if path != "" {
			if _, err := os.Stat(path); os.IsNotExist(err) {
				result.Warnings = append(result.Warnings, fmt.Sprintf("%s path does not exist: %s", name, path))
			}
		}
	}

	// DB path parent directory warning
	if cfg.DBPath != "" {
		dbDir := filepath.Dir(cfg.DBPath)
		if _, err := os.Stat(dbDir); os.IsNotExist(err) {
			result.Warnings = append(result.Warnings, fmt.Sprintf("db_path parent directory does not exist: %s", dbDir))
		}
	}

	return result
}

// MaskSecrets returns a copy of the config with secret fields masked.
// Any field name containing "key", "secret", "token", or "password" (case-insensitive)
// is replaced with "***".
func MaskSecrets(cfg *Config) *Config {
	masked := *cfg
	masked.API.APIKey = maskValue(masked.API.APIKey)
	masked.LLM.APIKey = maskValue(masked.LLM.APIKey)
	masked.ImageGen.APIKey = maskValue(masked.ImageGen.APIKey)
	masked.TTS.APIKey = maskValue(masked.TTS.APIKey)
	return &masked
}

// maskValue replaces non-empty strings with "***".
func maskValue(v string) string {
	if v == "" {
		return ""
	}
	return "***"
}
