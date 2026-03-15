package tts

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sushistack/yt.pipe/internal/plugin"
	"github.com/sushistack/yt.pipe/internal/retry"
)

const (
	defaultDashScopeEndpoint = "https://dashscope-intl.aliyuncs.com"
	defaultDashScopeModel    = "qwen3-tts-flash"
	defaultDashScopeFormat   = "wav"
	defaultDashScopeVoice    = "Cherry"

	voiceClonePrefix = "cosyvoice-clone-"

	// Qwen3 TTS API path (multimodal-generation)
	qwenTTSAPIPath = "/api/v1/services/aigc/multimodal-generation/generation"
)

// Compile-time interface checks.
var _ TTS = (*DashScopeProvider)(nil)
var _ VoiceCloner = (*DashScopeProvider)(nil)

// DashScopeProvider implements the TTS interface for DashScope Qwen3 TTS API.
type DashScopeProvider struct {
	endpoint   string
	apiKey     string
	model      string
	cloneModel string
	format     string
	voice      string
	language   string
	httpClient *http.Client
	pluginCfg  plugin.PluginConfig
}

// DashScopeConfig holds configuration for creating a DashScope provider.
type DashScopeConfig struct {
	Endpoint   string
	APIKey     string
	Model      string
	CloneModel string
	Format     string
	Voice      string
	Language   string
}

// NewDashScopeProvider creates a new DashScope Qwen3 TTS provider.
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

	cloneModel := cfg.CloneModel
	if cloneModel == "" {
		cloneModel = "qwen3-tts-vc-2026-01-22"
	}

	return &DashScopeProvider{
		endpoint:   strings.TrimRight(endpoint, "/"),
		apiKey:     cfg.APIKey,
		model:      model,
		cloneModel: cloneModel,
		format:     format,
		voice:      voice,
		language:   cfg.Language,
		httpClient: &http.Client{
			Timeout: pluginCfg.Timeout,
		},
		pluginCfg: pluginCfg,
	}, nil
}

// Qwen3 TTS API request/response types.

type qwenRequest struct {
	Model string    `json:"model"`
	Input qwenInput `json:"input"`
}

type qwenInput struct {
	Text     string `json:"text"`
	Voice    string `json:"voice"`
	Language string `json:"language_type,omitempty"`
}

type qwenResponse struct {
	RequestID string     `json:"request_id"`
	Output    qwenOutput `json:"output"`
	Usage     qwenUsage  `json:"usage,omitempty"`
}

type qwenOutput struct {
	Audio        *qwenAudio `json:"audio,omitempty"`
	FinishReason string     `json:"finish_reason,omitempty"`
}

type qwenAudio struct {
	URL       string      `json:"url,omitempty"`
	ExpiresAt json.Number `json:"expires_at,omitempty"`
}

type qwenUsage struct {
	InputTokens  int `json:"input_tokens,omitempty"`
	OutputTokens int `json:"output_tokens,omitempty"`
}

type qwenErrorResponse struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id"`
}

// Synthesize converts text to speech using DashScope Qwen3 TTS API.
func (p *DashScopeProvider) Synthesize(ctx context.Context, text string, voice string, opts *TTSOptions) (*SynthesisResult, error) {
	return p.synthesize(ctx, text, voice, nil, opts)
}

// SynthesizeWithOverrides applies pronunciation overrides before synthesis.
func (p *DashScopeProvider) SynthesizeWithOverrides(ctx context.Context, text string, voice string, overrides map[string]string, opts *TTSOptions) (*SynthesisResult, error) {
	return p.synthesize(ctx, text, voice, overrides, opts)
}

func (p *DashScopeProvider) synthesize(ctx context.Context, text string, voice string, overrides map[string]string, opts *TTSOptions) (*SynthesisResult, error) {
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

	// Use clone model when voice is a cloned voice ID
	model := p.model
	if isCloneVoice(voice) {
		model = p.cloneModel
	}

	reqBody := qwenRequest{
		Model: model,
		Input: qwenInput{
			Text:     processedText,
			Voice:    voice,
			Language: p.language,
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
		"elapsed_ms", elapsed.Milliseconds(),
	)

	return result, nil
}

func (p *DashScopeProvider) doSynthesize(ctx context.Context, reqBody qwenRequest) (*SynthesisResult, error) {
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	url := p.endpoint + qwenTTSAPIPath
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
		var errResp qwenErrorResponse
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

	var qwenResp qwenResponse
	if err := json.Unmarshal(respBody, &qwenResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	if qwenResp.Output.Audio == nil || qwenResp.Output.Audio.URL == "" {
		return nil, &APIError{
			Provider:   "dashscope",
			StatusCode: 200,
			Message:    "empty response: no audio URL returned",
		}
	}

	// Download the audio file from the returned URL
	audioData, err := p.downloadAudio(ctx, qwenResp.Output.Audio.URL)
	if err != nil {
		return nil, fmt.Errorf("download audio: %w", err)
	}

	// Calculate duration from WAV header (24kHz, 16-bit, mono)
	durationSec := wavDuration(audioData)

	return &SynthesisResult{
		AudioData:   audioData,
		WordTimings: nil, // Qwen3 TTS does not support word-level timestamps
		DurationSec: durationSec,
	}, nil
}

// downloadAudio fetches the audio file from the URL returned by the API.
func (p *DashScopeProvider) downloadAudio(ctx context.Context, audioURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, audioURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create download request: %w", err)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, &APIError{
			Provider:   "dashscope",
			StatusCode: 0,
			Message:    "audio download network error",
			Err:        err,
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, &APIError{
			Provider:   "dashscope",
			StatusCode: resp.StatusCode,
			Message:    fmt.Sprintf("audio download failed: HTTP %d", resp.StatusCode),
		}
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read audio data: %w", err)
	}

	if len(data) == 0 {
		return nil, &APIError{
			Provider:   "dashscope",
			StatusCode: 200,
			Message:    "downloaded audio is empty",
		}
	}

	return data, nil
}

// wavDuration calculates the duration from a WAV file's header.
// Returns 0 if the data is not a valid WAV.
func wavDuration(data []byte) float64 {
	// WAV header: 44 bytes minimum
	// Bytes 24-27: sample rate (uint32 LE)
	// Bytes 32-33: block align (uint16 LE) = channels * bitsPerSample / 8
	// Bytes 40-43: data chunk size (uint32 LE)
	if len(data) < 44 {
		return 0
	}
	// Verify RIFF header
	if string(data[0:4]) != "RIFF" || string(data[8:12]) != "WAVE" {
		return 0
	}

	sampleRate := binary.LittleEndian.Uint32(data[24:28])
	blockAlign := binary.LittleEndian.Uint16(data[32:34])

	if sampleRate == 0 || blockAlign == 0 {
		return 0
	}

	// Find "data" subchunk — it's usually at offset 36, but can vary
	dataSize := uint32(0)
	for i := 12; i+8 <= len(data); {
		chunkID := string(data[i : i+4])
		chunkSize := binary.LittleEndian.Uint32(data[i+4 : i+8])
		if chunkID == "data" {
			dataSize = chunkSize
			break
		}
		i += 8 + int(chunkSize)
		// Align to even boundary
		if i%2 != 0 {
			i++
		}
	}

	if dataSize == 0 {
		// Fallback: estimate from total file size minus header
		dataSize = uint32(len(data)) - 44
	}

	totalSamples := float64(dataSize) / float64(blockAlign)
	return totalSamples / float64(sampleRate)
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

const (
	// Voice enrollment API path
	qwenVoiceEnrollmentPath = "/api/v1/services/audio/tts/customization"
	qwenVoiceEnrollmentModel = "qwen-voice-enrollment"
)

// voiceEnrollmentRequest is the request body for voice enrollment.
type voiceEnrollmentRequest struct {
	Model string                 `json:"model"`
	Input voiceEnrollmentInput   `json:"input"`
}

type voiceEnrollmentInput struct {
	Action        string                 `json:"action"`
	TargetModel   string                 `json:"target_model"`
	PreferredName string                 `json:"preferred_name"`
	Audio         voiceEnrollmentAudio   `json:"audio"`
}

type voiceEnrollmentAudio struct {
	Data string `json:"data"`
}

type voiceEnrollmentResponse struct {
	RequestID string `json:"request_id"`
	Output    struct {
		Voice string `json:"voice"`
	} `json:"output"`
}

const maxVoiceSampleSize = 50 * 1024 * 1024 // 50 MB

// CreateVoice enrolls an audio sample and returns a voice ID for synthesis.
func (p *DashScopeProvider) CreateVoice(ctx context.Context, audioPath string, preferredName string) (string, error) {
	info, err := os.Stat(audioPath)
	if err != nil {
		return "", fmt.Errorf("create voice: stat audio file: %w", err)
	}
	if info.Size() > maxVoiceSampleSize {
		return "", fmt.Errorf("create voice: audio file too large (%d bytes, max %d)", info.Size(), maxVoiceSampleSize)
	}

	audioData, err := os.ReadFile(audioPath)
	if err != nil {
		return "", fmt.Errorf("create voice: read audio file: %w", err)
	}

	// Determine MIME type from file extension
	mime := "audio/mpeg"
	switch strings.ToLower(filepath.Ext(audioPath)) {
	case ".wav":
		mime = "audio/wav"
	case ".mp3":
		mime = "audio/mpeg"
	case ".m4a":
		mime = "audio/mp4"
	case ".flac":
		mime = "audio/flac"
	}

	b64 := base64.StdEncoding.EncodeToString(audioData)
	dataURI := fmt.Sprintf("data:%s;base64,%s", mime, b64)

	reqBody := voiceEnrollmentRequest{
		Model: qwenVoiceEnrollmentModel,
		Input: voiceEnrollmentInput{
			Action:        "create",
			TargetModel:   p.cloneModel,
			PreferredName: preferredName,
			Audio:         voiceEnrollmentAudio{Data: dataURI},
		},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("create voice: marshal request: %w", err)
	}

	url := p.endpoint + qwenVoiceEnrollmentPath
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create voice: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return "", &APIError{
			Provider:   "dashscope",
			StatusCode: 0,
			Message:    "voice enrollment network error",
			Err:        err,
		}
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("create voice: read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errResp qwenErrorResponse
		_ = json.Unmarshal(respBody, &errResp)
		msg := errResp.Message
		if msg == "" {
			msg = string(respBody)
		}
		return "", &APIError{
			Provider:   "dashscope",
			StatusCode: resp.StatusCode,
			Message:    fmt.Sprintf("voice enrollment failed: %s", msg),
		}
	}

	var enrollResp voiceEnrollmentResponse
	if err := json.Unmarshal(respBody, &enrollResp); err != nil {
		return "", fmt.Errorf("create voice: parse response: %w", err)
	}

	voiceID := enrollResp.Output.Voice
	if voiceID == "" {
		return "", &APIError{
			Provider:   "dashscope",
			StatusCode: 200,
			Message:    "voice enrollment returned empty voice ID",
		}
	}

	slog.Info("voice cloning enrollment complete",
		"voice_id", voiceID,
		"preferred_name", preferredName,
		"audio_path", audioPath,
	)

	return voiceID, nil
}

// DashScopeFactory creates a DashScope provider via the plugin registry.
func DashScopeFactory(cfg map[string]interface{}) (interface{}, error) {
	return NewDashScopeProvider(DashScopeConfig{
		Endpoint:   stringFromCfg(cfg, "endpoint", defaultDashScopeEndpoint),
		APIKey:     stringFromCfg(cfg, "api_key", ""),
		Model:      stringFromCfg(cfg, "model", defaultDashScopeModel),
		CloneModel: stringFromCfg(cfg, "clone_model", "qwen3-tts-vc-2026-01-22"),
		Format:     stringFromCfg(cfg, "format", defaultDashScopeFormat),
		Voice:      stringFromCfg(cfg, "voice", defaultDashScopeVoice),
		Language:   stringFromCfg(cfg, "language", ""),
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
