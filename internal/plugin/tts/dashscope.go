package tts

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/sushistack/yt.pipe/internal/domain"
	"github.com/sushistack/yt.pipe/internal/plugin"
	"github.com/sushistack/yt.pipe/internal/retry"
)

const (
	defaultDashScopeEndpoint = "https://dashscope.aliyuncs.com"
	defaultDashScopeModel    = "cosyvoice-v1"
	defaultDashScopeFormat   = "mp3"
	defaultDashScopeVoice    = "longxiaochun"

	voiceClonePrefix = "cosyvoice-clone-"
)

// Compile-time interface check.
var _ TTS = (*DashScopeProvider)(nil)

// DashScopeProvider implements the TTS interface for DashScope CosyVoice API.
type DashScopeProvider struct {
	endpoint   string
	apiKey     string
	model      string
	format     string
	voice      string
	httpClient *http.Client
	pluginCfg  plugin.PluginConfig
}

// DashScopeConfig holds configuration for creating a DashScope provider.
type DashScopeConfig struct {
	Endpoint string
	APIKey   string
	Model    string
	Format   string
	Voice    string
}

// NewDashScopeProvider creates a new DashScope CosyVoice TTS provider.
func NewDashScopeProvider(cfg DashScopeConfig) (*DashScopeProvider, error) {
	if cfg.APIKey == "" {
		return nil, &APIError{
			Provider:   "dashscope",
			StatusCode: 401,
			Message:    "DashScope API authentication failed: check tts.api_key config",
		}
	}

	endpoint := cfg.Endpoint
	if endpoint == "" {
		endpoint = defaultDashScopeEndpoint
	}
	model := cfg.Model
	if model == "" {
		model = defaultDashScopeModel
	}
	format := cfg.Format
	if format == "" {
		format = defaultDashScopeFormat
	}
	voice := cfg.Voice
	if voice == "" {
		voice = defaultDashScopeVoice
	}

	pluginCfg := plugin.DefaultPluginConfig("dashscope")

	return &DashScopeProvider{
		endpoint: strings.TrimRight(endpoint, "/"),
		apiKey:   cfg.APIKey,
		model:    model,
		format:   format,
		voice:    voice,
		httpClient: &http.Client{
			Timeout: pluginCfg.Timeout,
		},
		pluginCfg: pluginCfg,
	}, nil
}

// DashScope API request/response types.

type dsRequest struct {
	Model      string      `json:"model"`
	Input      dsInput     `json:"input"`
	Parameters dsParams    `json:"parameters"`
}

type dsInput struct {
	Text string `json:"text"`
}

type dsParams struct {
	Voice      string `json:"voice"`
	Format     string `json:"format"`
	VoiceClone bool   `json:"voice_clone,omitempty"`
}

type dsResponse struct {
	RequestID string   `json:"request_id"`
	Output    dsOutput `json:"output"`
	Usage     dsUsage  `json:"usage"`
}

type dsOutput struct {
	Audio       string         `json:"audio"`
	WordTimings []dsWordTiming `json:"word_timings,omitempty"`
	DurationMs  int            `json:"duration_ms"`
}

type dsWordTiming struct {
	Word    string `json:"word"`
	StartMs int    `json:"start_ms"`
	EndMs   int    `json:"end_ms"`
}

type dsUsage struct {
	Characters int `json:"characters"`
}

type dsErrorResponse struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id"`
}

// Synthesize converts text to speech using DashScope CosyVoice API.
func (p *DashScopeProvider) Synthesize(ctx context.Context, text string, voice string) (*SynthesisResult, error) {
	return p.synthesize(ctx, text, voice, nil)
}

// SynthesizeWithOverrides applies pronunciation overrides before synthesis.
func (p *DashScopeProvider) SynthesizeWithOverrides(ctx context.Context, text string, voice string, overrides map[string]string) (*SynthesisResult, error) {
	return p.synthesize(ctx, text, voice, overrides)
}

func (p *DashScopeProvider) synthesize(ctx context.Context, text string, voice string, overrides map[string]string) (*SynthesisResult, error) {
	if strings.TrimSpace(text) == "" {
		return nil, &APIError{
			Provider:   "dashscope",
			StatusCode: 400,
			Message:    "empty text: nothing to synthesize",
		}
	}
	if voice == "" {
		voice = p.voice
	}

	// Apply pronunciation overrides to text
	processedText := applyOverrides(text, overrides)

	isClone := isCloneVoice(voice)

	reqBody := dsRequest{
		Model: p.model,
		Input: dsInput{Text: processedText},
		Parameters: dsParams{
			Voice:      voice,
			Format:     p.format,
			VoiceClone: isClone,
		},
	}

	var result *SynthesisResult
	start := time.Now()

	err := retry.Do(ctx, p.pluginCfg.MaxRetries, p.pluginCfg.BaseDelay, func() error {
		var synthErr error
		result, synthErr = p.doSynthesize(ctx, reqBody)
		return synthErr
	})

	elapsed := time.Since(start)
	if err != nil {
		slog.Error("dashscope tts synthesis failed",
			"model", p.model,
			"voice", voice,
			"text_len", len(text),
			"duration_ms", elapsed.Milliseconds(),
			"err", err,
		)
		return nil, err
	}

	slog.Info("dashscope tts synthesized",
		"model", p.model,
		"voice", voice,
		"text_len", len(text),
		"audio_bytes", len(result.AudioData),
		"duration_sec", result.DurationSec,
		"word_count", len(result.WordTimings),
		"elapsed_ms", elapsed.Milliseconds(),
	)

	return result, nil
}

func (p *DashScopeProvider) doSynthesize(ctx context.Context, reqBody dsRequest) (*SynthesisResult, error) {
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	url := p.endpoint + "/api/v1/services/aigc/text2audio/generation"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, &APIError{
			Provider:   "dashscope",
			StatusCode: 0,
			Message:    "network error",
			Err:        err,
		}
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errResp dsErrorResponse
		_ = json.Unmarshal(respBody, &errResp)
		msg := errResp.Message
		if msg == "" {
			msg = string(respBody)
		}
		return nil, &APIError{
			Provider:   "dashscope",
			StatusCode: resp.StatusCode,
			Message:    msg,
		}
	}

	var dsResp dsResponse
	if err := json.Unmarshal(respBody, &dsResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	if dsResp.Output.Audio == "" {
		return nil, &APIError{
			Provider:   "dashscope",
			StatusCode: 200,
			Message:    "empty response: no audio data returned",
		}
	}

	audioData, err := base64.StdEncoding.DecodeString(dsResp.Output.Audio)
	if err != nil {
		return nil, fmt.Errorf("decode audio base64: %w", err)
	}

	wordTimings := make([]domain.WordTiming, len(dsResp.Output.WordTimings))
	for i, wt := range dsResp.Output.WordTimings {
		wordTimings[i] = domain.WordTiming{
			Word:     wt.Word,
			StartSec: float64(wt.StartMs) / 1000.0,
			EndSec:   float64(wt.EndMs) / 1000.0,
		}
	}

	durationSec := float64(dsResp.Output.DurationMs) / 1000.0

	return &SynthesisResult{
		AudioData:   audioData,
		WordTimings: wordTimings,
		DurationSec: durationSec,
	}, nil
}

// isCloneVoice checks if the voice ID indicates a cloned voice.
func isCloneVoice(voice string) bool {
	return strings.HasPrefix(voice, voiceClonePrefix)
}

// applyOverrides replaces terms in text with their pronunciation overrides.
func applyOverrides(text string, overrides map[string]string) string {
	if len(overrides) == 0 {
		return text
	}
	result := text
	for term, pronunciation := range overrides {
		result = strings.ReplaceAll(result, term, pronunciation)
	}
	return result
}

// DashScopeFactory creates a DashScope provider via the plugin registry.
func DashScopeFactory(cfg map[string]interface{}) (interface{}, error) {
	return NewDashScopeProvider(DashScopeConfig{
		Endpoint: stringFromCfg(cfg, "endpoint", defaultDashScopeEndpoint),
		APIKey:   stringFromCfg(cfg, "api_key", ""),
		Model:    stringFromCfg(cfg, "model", defaultDashScopeModel),
		Format:   stringFromCfg(cfg, "format", defaultDashScopeFormat),
		Voice:    stringFromCfg(cfg, "voice", defaultDashScopeVoice),
	})
}

func stringFromCfg(cfg map[string]interface{}, key, defaultVal string) string {
	if v, ok := cfg[key]; ok {
		if s, ok := v.(string); ok && s != "" {
			return s
		}
	}
	return defaultVal
}
