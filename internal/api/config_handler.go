package api

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jay/youtube-pipeline/internal/config"
	"github.com/jay/youtube-pipeline/internal/plugin"
)

type configResponse struct {
	SCPDataPath   string              `json:"scp_data_path"`
	WorkspacePath string              `json:"workspace_path"`
	DBPath        string              `json:"db_path"`
	API           apiConfigResponse   `json:"api"`
	LLM           llmConfigResponse   `json:"llm"`
	ImageGen      imageConfigResponse `json:"imagegen"`
	TTS           ttsConfigResponse   `json:"tts"`
	Output        outputConfigResp    `json:"output"`
	LogLevel      string              `json:"log_level"`
	LogFormat     string              `json:"log_format"`
}

type apiConfigResponse struct {
	Host   string `json:"host"`
	Port   int    `json:"port"`
	APIKey string `json:"api_key"`
}

type llmConfigResponse struct {
	Provider    string  `json:"provider"`
	APIKey      string  `json:"api_key"`
	Model       string  `json:"model"`
	Temperature float64 `json:"temperature"`
	MaxTokens   int     `json:"max_tokens"`
}

type imageConfigResponse struct {
	Provider string `json:"provider"`
	APIKey   string `json:"api_key"`
	Model    string `json:"model"`
}

type ttsConfigResponse struct {
	Provider string  `json:"provider"`
	APIKey   string  `json:"api_key"`
	Voice    string  `json:"voice"`
	Speed    float64 `json:"speed"`
}

type outputConfigResp struct {
	Provider     string `json:"provider"`
	CanvasWidth  int    `json:"canvas_width"`
	CanvasHeight int    `json:"canvas_height"`
	FPS          int    `json:"fps"`
}

// maskAPIKey masks an API key, showing only prefix and last 3 chars.
func maskAPIKey(key string) string {
	if key == "" {
		return ""
	}
	if len(key) <= 6 {
		return "***"
	}
	return key[:3] + "***..." + key[len(key)-3:]
}

func toConfigResponse(c *config.Config) configResponse {
	return configResponse{
		SCPDataPath:   c.SCPDataPath,
		WorkspacePath: c.WorkspacePath,
		DBPath:        c.DBPath,
		API: apiConfigResponse{
			Host:   c.API.Host,
			Port:   c.API.Port,
			APIKey: maskAPIKey(c.API.APIKey),
		},
		LLM: llmConfigResponse{
			Provider:    c.LLM.Provider,
			APIKey:      maskAPIKey(c.LLM.APIKey),
			Model:       c.LLM.Model,
			Temperature: c.LLM.Temperature,
			MaxTokens:   c.LLM.MaxTokens,
		},
		ImageGen: imageConfigResponse{
			Provider: c.ImageGen.Provider,
			APIKey:   maskAPIKey(c.ImageGen.APIKey),
			Model:    c.ImageGen.Model,
		},
		TTS: ttsConfigResponse{
			Provider: c.TTS.Provider,
			APIKey:   maskAPIKey(c.TTS.APIKey),
			Voice:    c.TTS.Voice,
			Speed:    c.TTS.Speed,
		},
		Output: outputConfigResp{
			Provider:     c.Output.Provider,
			CanvasWidth:  c.Output.CanvasWidth,
			CanvasHeight: c.Output.CanvasHeight,
			FPS:          c.Output.FPS,
		},
		LogLevel:  c.LogLevel,
		LogFormat: c.LogFormat,
	}
}

// handleGetConfig returns the current configuration with API keys masked.
func (s *Server) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	WriteJSON(w, r, http.StatusOK, toConfigResponse(s.cfg))
}

// handlePatchConfig applies partial configuration updates.
func (s *Server) handlePatchConfig(w http.ResponseWriter, r *http.Request) {
	var patch map[string]interface{}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&patch); err != nil {
		WriteError(w, r, http.StatusBadRequest, "INVALID_REQUEST", "invalid JSON body")
		return
	}

	// Apply known fields
	for key, val := range patch {
		if err := applyConfigPatch(s.cfg, key, val); err != nil {
			WriteError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
			return
		}
	}

	// Validate
	result := config.Validate(s.cfg)
	if !result.IsValid() {
		WriteError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", strings.Join(result.Errors, "; "))
		return
	}

	WriteJSON(w, r, http.StatusOK, toConfigResponse(s.cfg))
}

func applyConfigPatch(cfg *config.Config, key string, val interface{}) error {
	switch key {
	case "log_level":
		s, ok := val.(string)
		if !ok {
			return &validationErr{field: key, msg: "must be a string"}
		}
		valid := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
		if !valid[s] {
			return &validationErr{field: key, msg: "must be debug/info/warn/error"}
		}
		cfg.LogLevel = s
	case "log_format":
		s, ok := val.(string)
		if !ok {
			return &validationErr{field: key, msg: "must be a string"}
		}
		if s != "json" && s != "text" {
			return &validationErr{field: key, msg: "must be json or text"}
		}
		cfg.LogFormat = s
	case "llm.model":
		s, ok := val.(string)
		if !ok {
			return &validationErr{field: key, msg: "must be a string"}
		}
		cfg.LLM.Model = s
	case "llm.temperature":
		f, ok := val.(float64)
		if !ok {
			return &validationErr{field: key, msg: "must be a number"}
		}
		cfg.LLM.Temperature = f
	case "llm.max_tokens":
		f, ok := val.(float64)
		if !ok {
			return &validationErr{field: key, msg: "must be a number"}
		}
		cfg.LLM.MaxTokens = int(f)
	case "tts.voice":
		s, ok := val.(string)
		if !ok {
			return &validationErr{field: key, msg: "must be a string"}
		}
		cfg.TTS.Voice = s
	case "tts.speed":
		f, ok := val.(float64)
		if !ok {
			return &validationErr{field: key, msg: "must be a number"}
		}
		cfg.TTS.Speed = f
	default:
		return &validationErr{field: key, msg: "unknown or immutable config key"}
	}
	return nil
}

type validationErr struct {
	field string
	msg   string
}

func (e *validationErr) Error() string {
	return e.field + ": " + e.msg
}

// Plugin management

type pluginInfo struct {
	Type      string   `json:"type"`
	Active    string   `json:"active"`
	Available []string `json:"available"`
}

// handleListPlugins returns registered plugins grouped by type.
func (s *Server) handleListPlugins(w http.ResponseWriter, r *http.Request) {
	types := []plugin.PluginType{
		plugin.PluginTypeLLM,
		plugin.PluginTypeImageGen,
		plugin.PluginTypeTTS,
		plugin.PluginTypeOutput,
	}

	activeMap := map[plugin.PluginType]string{
		plugin.PluginTypeLLM:      s.cfg.LLM.Provider,
		plugin.PluginTypeImageGen: s.cfg.ImageGen.Provider,
		plugin.PluginTypeTTS:      s.cfg.TTS.Provider,
		plugin.PluginTypeOutput:   s.cfg.Output.Provider,
	}

	var plugins []pluginInfo
	for _, pt := range types {
		providers := s.registry.Providers(pt)
		plugins = append(plugins, pluginInfo{
			Type:      string(pt),
			Active:    activeMap[pt],
			Available: providers,
		})
	}

	WriteJSON(w, r, http.StatusOK, map[string]interface{}{
		"plugins": plugins,
	})
}

type setActiveRequest struct {
	Provider string `json:"provider"`
}

// handleSetActivePlugin switches the active plugin for a given type.
func (s *Server) handleSetActivePlugin(w http.ResponseWriter, r *http.Request) {
	pluginType := chi.URLParam(r, "type")

	var req setActiveRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		WriteError(w, r, http.StatusBadRequest, "INVALID_REQUEST", "invalid JSON body")
		return
	}

	if req.Provider == "" {
		WriteError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "provider is required")
		return
	}

	switch pluginType {
	case "llm":
		s.cfg.LLM.Provider = req.Provider
	case "imagegen":
		s.cfg.ImageGen.Provider = req.Provider
	case "tts":
		s.cfg.TTS.Provider = req.Provider
	case "output":
		s.cfg.Output.Provider = req.Provider
	default:
		WriteError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "unknown plugin type: "+pluginType)
		return
	}

	WriteJSON(w, r, http.StatusOK, map[string]string{
		"type":     pluginType,
		"provider": req.Provider,
	})
}
