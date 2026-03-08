package cli

import (
	"fmt"
	"os"

	"github.com/jay/youtube-pipeline/internal/config"
	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Configuration management commands",
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Display merged configuration with source annotations",
	RunE:  runConfigShow,
}

var configValidateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate configuration values",
	RunE:  runConfigValidate,
}

func init() {
	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configValidateCmd)
	rootCmd.AddCommand(configCmd)
}

func runConfigShow(cmd *cobra.Command, args []string) error {
	result := GetConfig()
	if result == nil {
		return fmt.Errorf("configuration not loaded")
	}

	masked := config.MaskSecrets(result.Config)
	sources := result.Sources

	lines := formatConfigWithSources(masked, sources)
	for _, line := range lines {
		fmt.Println(line)
	}
	return nil
}

func runConfigValidate(cmd *cobra.Command, args []string) error {
	result := GetConfig()
	if result == nil {
		return fmt.Errorf("configuration not loaded")
	}

	vr := config.Validate(result.Config)

	if len(vr.Errors) > 0 {
		fmt.Fprintln(os.Stderr, "Errors:")
		for _, e := range vr.Errors {
			fmt.Fprintf(os.Stderr, "  - %s\n", e)
		}
	}

	if len(vr.Warnings) > 0 {
		fmt.Fprintln(os.Stderr, "Warnings:")
		for _, w := range vr.Warnings {
			fmt.Fprintf(os.Stderr, "  - %s\n", w)
		}
	}

	if vr.IsValid() && len(vr.Warnings) == 0 {
		fmt.Println("Configuration is valid.")
	} else if vr.IsValid() {
		fmt.Println("Configuration is valid (with warnings).")
	} else {
		return fmt.Errorf("configuration has errors")
	}

	return nil
}

// formatConfigWithSources formats the config as YAML-like output with source comments.
func formatConfigWithSources(cfg *config.Config, sources map[string]string) []string {
	var lines []string

	addLine := func(key, value string) {
		source := sources[key]
		if source == "" {
			source = "default"
		}
		lines = append(lines, fmt.Sprintf("%-30s # source: %s", fmt.Sprintf("%s: %s", key, value), source))
	}

	// Top-level fields
	addLine("scp_data_path", quote(cfg.SCPDataPath))
	addLine("workspace_path", quote(cfg.WorkspacePath))
	addLine("db_path", quote(cfg.DBPath))
	addLine("glossary_path", quote(cfg.GlossaryPath))
	addLine("log_level", quote(cfg.LogLevel))
	addLine("log_format", quote(cfg.LogFormat))

	lines = append(lines, "")

	// API section
	lines = append(lines, "api:")
	addLine("api.host", quote(cfg.API.Host))
	addLine("api.port", fmt.Sprintf("%d", cfg.API.Port))
	addLine("api.api_key", quote(cfg.API.APIKey))

	lines = append(lines, "")

	// LLM section
	lines = append(lines, "llm:")
	addLine("llm.provider", quote(cfg.LLM.Provider))
	addLine("llm.api_key", quote(cfg.LLM.APIKey))
	addLine("llm.model", quote(cfg.LLM.Model))
	addLine("llm.temperature", fmt.Sprintf("%.1f", cfg.LLM.Temperature))
	addLine("llm.max_tokens", fmt.Sprintf("%d", cfg.LLM.MaxTokens))

	lines = append(lines, "")

	// ImageGen section
	lines = append(lines, "imagegen:")
	addLine("imagegen.provider", quote(cfg.ImageGen.Provider))
	addLine("imagegen.api_key", quote(cfg.ImageGen.APIKey))
	addLine("imagegen.model", quote(cfg.ImageGen.Model))

	lines = append(lines, "")

	// TTS section
	lines = append(lines, "tts:")
	addLine("tts.provider", quote(cfg.TTS.Provider))
	addLine("tts.api_key", quote(cfg.TTS.APIKey))
	addLine("tts.voice", quote(cfg.TTS.Voice))
	addLine("tts.speed", fmt.Sprintf("%.1f", cfg.TTS.Speed))

	lines = append(lines, "")

	// Output section
	lines = append(lines, "output:")
	addLine("output.provider", quote(cfg.Output.Provider))

	return lines
}

func quote(s string) string {
	if s == "" {
		return `""`
	}
	return fmt.Sprintf("%q", s)
}
