package cli

import (
	"fmt"
	"log/slog"

	"github.com/sushistack/yt.pipe/internal/config"
	"github.com/sushistack/yt.pipe/internal/plugin"
	"github.com/sushistack/yt.pipe/internal/plugin/imagegen"
	"github.com/sushistack/yt.pipe/internal/plugin/llm"
	"github.com/sushistack/yt.pipe/internal/plugin/output"
	"github.com/sushistack/yt.pipe/internal/plugin/output/capcut"
	"github.com/sushistack/yt.pipe/internal/plugin/tts"
)

// pluginRegistry is the global plugin registry for the CLI.
var pluginRegistry = plugin.NewRegistry()

func init() {
	// Register LLM providers
	_ = pluginRegistry.Register(plugin.PluginTypeLLM, "gemini", llm.GeminiFactory)
	_ = pluginRegistry.Register(plugin.PluginTypeLLM, "qwen", llm.QwenFactory)
	_ = pluginRegistry.Register(plugin.PluginTypeLLM, "deepseek", llm.DeepSeekFactory)

	// Register ImageGen providers
	_ = pluginRegistry.Register(plugin.PluginTypeImageGen, "siliconflow", imagegen.SiliconFlowFactory)

	// Register TTS providers
	_ = pluginRegistry.Register(plugin.PluginTypeTTS, "dashscope", tts.DashScopeFactory)
}

// PluginRegistry returns the global plugin registry for registering providers.
func PluginRegistry() *plugin.Registry {
	return pluginRegistry
}

// createPlugins creates plugin instances from configuration using the plugin registry.
func createPlugins(cfg *config.LoadResult) (llm.LLM, imagegen.ImageGen, tts.TTS, error) {
	c := cfg.Config

	// Create LLM plugin
	llmCfg := map[string]interface{}{
		"api_key":     c.LLM.APIKey,
		"model":       c.LLM.Model,
		"temperature": c.LLM.Temperature,
		"max_tokens":  c.LLM.MaxTokens,
	}
	llmRaw, err := pluginRegistry.Create(plugin.PluginTypeLLM, c.LLM.Provider, llmCfg)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("create LLM plugin (%s): %w", c.LLM.Provider, err)
	}
	llmPlugin, ok := llmRaw.(llm.LLM)
	if !ok {
		return nil, nil, nil, fmt.Errorf("LLM plugin %q does not implement llm.LLM interface", c.LLM.Provider)
	}

	// Create ImageGen plugin
	imgCfg := map[string]interface{}{
		"endpoint": c.ImageGen.Endpoint,
		"api_key":  c.ImageGen.APIKey,
		"model":    c.ImageGen.Model,
	}
	imgRaw, err := pluginRegistry.Create(plugin.PluginTypeImageGen, c.ImageGen.Provider, imgCfg)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("create ImageGen plugin (%s): %w", c.ImageGen.Provider, err)
	}
	imgPlugin, ok := imgRaw.(imagegen.ImageGen)
	if !ok {
		return nil, nil, nil, fmt.Errorf("ImageGen plugin %q does not implement imagegen.ImageGen interface", c.ImageGen.Provider)
	}

	// Create TTS plugin
	ttsCfg := map[string]interface{}{
		"endpoint": c.TTS.Endpoint,
		"api_key":  c.TTS.APIKey,
		"model":    c.TTS.Model,
		"voice":    c.TTS.Voice,
		"format":   c.TTS.Format,
	}
	ttsRaw, err := pluginRegistry.Create(plugin.PluginTypeTTS, c.TTS.Provider, ttsCfg)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("create TTS plugin (%s): %w", c.TTS.Provider, err)
	}
	ttsPlugin, ok := ttsRaw.(tts.TTS)
	if !ok {
		return nil, nil, nil, fmt.Errorf("TTS plugin %q does not implement tts.TTS interface", c.TTS.Provider)
	}

	return llmPlugin, imgPlugin, ttsPlugin, nil
}

// PluginSet holds all plugin instances along with their availability status.
type PluginSet struct {
	LLM      llm.LLM
	ImageGen imagegen.ImageGen
	TTS      tts.TTS
	Output   output.Assembler
	Status   map[string]bool // plugin type -> available
}

// createPluginsGraceful creates plugin instances with graceful degradation.
// Individual plugin failures are logged as warnings; the server can still start.
func createPluginsGraceful(cfg *config.LoadResult) *PluginSet {
	c := cfg.Config
	ps := &PluginSet{
		Status: map[string]bool{
			"llm":      false,
			"imagegen": false,
			"tts":      false,
			"output":   false,
		},
	}

	// Create LLM plugin
	if c.LLM.Provider != "" {
		llmCfg := map[string]interface{}{
			"api_key":     c.LLM.APIKey,
			"model":       c.LLM.Model,
			"temperature": c.LLM.Temperature,
			"max_tokens":  c.LLM.MaxTokens,
		}
		raw, err := pluginRegistry.Create(plugin.PluginTypeLLM, c.LLM.Provider, llmCfg)
		if err != nil {
			slog.Warn("failed to initialize LLM plugin", "provider", c.LLM.Provider, "error", err)
		} else if p, ok := raw.(llm.LLM); ok {
			ps.LLM = p
			ps.Status["llm"] = true
		} else {
			slog.Warn("LLM plugin does not implement llm.LLM interface", "provider", c.LLM.Provider)
		}
	} else {
		slog.Warn("LLM provider not configured")
	}

	// Create ImageGen plugin
	if c.ImageGen.Provider != "" {
		imgCfg := map[string]interface{}{
			"endpoint": c.ImageGen.Endpoint,
			"api_key":  c.ImageGen.APIKey,
			"model":    c.ImageGen.Model,
		}
		raw, err := pluginRegistry.Create(plugin.PluginTypeImageGen, c.ImageGen.Provider, imgCfg)
		if err != nil {
			slog.Warn("failed to initialize ImageGen plugin", "provider", c.ImageGen.Provider, "error", err)
		} else if p, ok := raw.(imagegen.ImageGen); ok {
			ps.ImageGen = p
			ps.Status["imagegen"] = true
		} else {
			slog.Warn("ImageGen plugin does not implement imagegen.ImageGen interface", "provider", c.ImageGen.Provider)
		}
	} else {
		slog.Warn("ImageGen provider not configured")
	}

	// Create TTS plugin
	if c.TTS.Provider != "" {
		ttsCfg := map[string]interface{}{
			"endpoint": c.TTS.Endpoint,
			"api_key":  c.TTS.APIKey,
			"model":    c.TTS.Model,
			"voice":    c.TTS.Voice,
			"format":   c.TTS.Format,
		}
		raw, err := pluginRegistry.Create(plugin.PluginTypeTTS, c.TTS.Provider, ttsCfg)
		if err != nil {
			slog.Warn("failed to initialize TTS plugin", "provider", c.TTS.Provider, "error", err)
		} else if p, ok := raw.(tts.TTS); ok {
			ps.TTS = p
			ps.Status["tts"] = true
		} else {
			slog.Warn("TTS plugin does not implement tts.TTS interface", "provider", c.TTS.Provider)
		}
	} else {
		slog.Warn("TTS provider not configured")
	}

	// Create Output (CapCut assembler) plugin — always available as built-in
	ps.Output = capcut.New()
	ps.Status["output"] = true

	return ps
}
