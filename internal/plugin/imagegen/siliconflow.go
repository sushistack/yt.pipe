package imagegen

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/sushistack/yt.pipe/internal/plugin"
	"github.com/sushistack/yt.pipe/internal/retry"
)

const (
	defaultSiliconFlowEndpoint = "https://api.siliconflow.com/v1"
	defaultSiliconFlowModel    = "black-forest-labs/FLUX.1-schnell"
	defaultImageWidth          = 1024
	defaultImageHeight         = 576
)

// supportedImageSizes maps model prefixes to their allowed image_size values.
// The first entry in each list is the default for that model family.
var supportedImageSizes = map[string][]string{
	"FLUX.1-schnell": {"1024x576", "576x1024", "1024x1024", "512x1024", "768x512", "768x1024"},
	"FLUX.2-pro":     {"1024x576", "576x1024", "512x512", "768x1024", "1024x768"},
	"FLUX.2-flex":    {"1024x576", "576x1024", "512x512", "768x1024", "1024x768"},
	"FLUX.1-dev":     {"1024x576", "576x1024", "1024x1024", "960x1280", "768x1024", "720x1440", "720x1280"},
	"FLUX-1.1-pro":   {"1024x576", "576x1024", "1024x768"},
	"Qwen-Image":     {"1664x928", "928x1664", "1328x1328", "1472x1140", "1140x1472", "1584x1056", "1056x1584"},
}

// resolveImageSize returns a valid image_size for the given model.
// If the requested WxH is in the model's supported list, it is used as-is.
// Otherwise, the first (default landscape) size for that model family is returned.
func resolveImageSize(model string, width, height int) string {
	requested := fmt.Sprintf("%dx%d", width, height)

	// Find matching model family
	for family, sizes := range supportedImageSizes {
		if strings.Contains(model, family) {
			for _, s := range sizes {
				if s == requested {
					return requested
				}
			}
			return sizes[0] // default for this family
		}
	}

	// Unknown model — pass through as-is
	return requested
}

// SiliconFlowProvider implements ImageGen for the SiliconFlow FLUX API.
type SiliconFlowProvider struct {
	endpoint   string
	apiKey     string
	model      string
	httpClient *http.Client
	pluginCfg  plugin.PluginConfig
}

// SiliconFlowConfig holds configuration for creating a SiliconFlow provider.
type SiliconFlowConfig struct {
	Endpoint string
	APIKey   string
	Model    string
}

// NewSiliconFlowProvider creates a new SiliconFlow image generation provider.
func NewSiliconFlowProvider(cfg SiliconFlowConfig) (*SiliconFlowProvider, error) {
	if cfg.APIKey == "" {
		return nil, &APIError{
			Provider:   "siliconflow",
			StatusCode: 401,
			Message:    "SiliconFlow API authentication failed: check imagegen.api_key config",
		}
	}

	endpoint := cfg.Endpoint
	if endpoint == "" {
		endpoint = defaultSiliconFlowEndpoint
	}

	model := cfg.Model
	if model == "" {
		model = defaultSiliconFlowModel
	}

	pluginCfg := plugin.DefaultPluginConfig("siliconflow")

	return &SiliconFlowProvider{
		endpoint: endpoint,
		apiKey:   cfg.APIKey,
		model:    model,
		httpClient: &http.Client{
			Timeout: pluginCfg.Timeout,
		},
		pluginCfg: pluginCfg,
	}, nil
}

// SiliconFlow API request/response types
type sfImageRequest struct {
	Model         string `json:"model"`
	Prompt        string `json:"prompt"`
	ImageSize     string `json:"image_size,omitempty"`
	BatchSize     int    `json:"batch_size,omitempty"`
	Seed          *int64 `json:"seed,omitempty"`
	NumSteps      int    `json:"num_inference_steps,omitempty"`
	GuidanceScale *float64 `json:"guidance_scale,omitempty"`
}

type sfImageResponse struct {
	Images []sfImage `json:"images"`
}

type sfImage struct {
	URL string `json:"url"`
}

type sfErrorResponse struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error"`
	Message string `json:"message"` // some endpoints use top-level message
}

// Generate creates a single image from a prompt using the SiliconFlow FLUX API.
func (p *SiliconFlowProvider) Generate(ctx context.Context, prompt string, opts GenerateOptions) (*ImageResult, error) {
	width := opts.Width
	if width == 0 {
		width = defaultImageWidth
	}
	height := opts.Height
	if height == 0 {
		height = defaultImageHeight
	}
	model := p.model
	if opts.Model != "" {
		model = opts.Model
	}

	imageSize := resolveImageSize(model, width, height)

	// Parse back resolved dimensions for the result
	if parts := strings.SplitN(imageSize, "x", 2); len(parts) == 2 {
		if w, err := strconv.Atoi(parts[0]); err == nil {
			width = w
		}
		if h, err := strconv.Atoi(parts[1]); err == nil {
			height = h
		}
	}

	reqBody := sfImageRequest{
		Model:     model,
		Prompt:    prompt,
		ImageSize: imageSize,
		BatchSize: 1,
	}
	if opts.Seed != 0 {
		seed := opts.Seed
		reqBody.Seed = &seed
	}

	var result *ImageResult
	start := time.Now()

	err := retry.Do(ctx, p.pluginCfg.MaxRetries, p.pluginCfg.BaseDelay, func() error {
		var genErr error
		result, genErr = p.doGenerate(ctx, reqBody, width, height)
		return genErr
	})

	elapsed := time.Since(start)
	if err != nil {
		slog.Error("siliconflow image generation failed",
			"model", model,
			"duration_ms", elapsed.Milliseconds(),
			"err", err,
		)
		return nil, err
	}

	slog.Info("siliconflow image generated",
		"model", model,
		"width", result.Width,
		"height", result.Height,
		"format", result.Format,
		"size_bytes", len(result.ImageData),
		"duration_ms", elapsed.Milliseconds(),
	)

	return result, nil
}

func (p *SiliconFlowProvider) doGenerate(ctx context.Context, reqBody sfImageRequest, width, height int) (*ImageResult, error) {
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	url := p.endpoint + "/images/generations"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, &APIError{
			Provider:   "siliconflow",
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

	// Handle rate limiting with Retry-After header
	if resp.StatusCode == http.StatusTooManyRequests {
		msg := "rate limited"
		if retryAfter := resp.Header.Get("Retry-After"); retryAfter != "" {
			if seconds, parseErr := strconv.Atoi(retryAfter); parseErr == nil {
				msg = fmt.Sprintf("rate limited (retry after %ds)", seconds)
			}
		}
		return nil, &APIError{
			Provider:   "siliconflow",
			StatusCode: resp.StatusCode,
			Message:    msg,
		}
	}

	if resp.StatusCode != http.StatusOK {
		var errResp sfErrorResponse
		_ = json.Unmarshal(respBody, &errResp)
		msg := errResp.Error.Message
		if msg == "" {
			msg = errResp.Message
		}
		if msg == "" {
			msg = string(respBody)
		}
		return nil, &APIError{
			Provider:   "siliconflow",
			StatusCode: resp.StatusCode,
			Message:    msg,
		}
	}

	var sfResp sfImageResponse
	if err := json.Unmarshal(respBody, &sfResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	if len(sfResp.Images) == 0 {
		return nil, &APIError{
			Provider:   "siliconflow",
			StatusCode: 200,
			Message:    "empty response: no images returned",
		}
	}

	// SiliconFlow returns base64-encoded image data in the URL field
	imgData, err := decodeImageData(ctx, sfResp.Images[0].URL, p.httpClient)
	if err != nil {
		return nil, fmt.Errorf("decode image data: %w", err)
	}

	return &ImageResult{
		ImageData: imgData,
		Format:    "png",
		Width:     width,
		Height:    height,
	}, nil
}

// decodeImageData handles both base64-encoded data and URL-based image responses.
func decodeImageData(ctx context.Context, urlOrData string, client *http.Client) ([]byte, error) {
	// Check if it's a URL (starts with http:// or https://)
	if strings.HasPrefix(urlOrData, "http://") || strings.HasPrefix(urlOrData, "https://") {
		return downloadImage(ctx, urlOrData, client)
	}

	// Try as data URI (data:image/png;base64,...)
	if strings.HasPrefix(urlOrData, "data:") {
		if idx := strings.Index(urlOrData, ","); idx > 0 {
			urlOrData = urlOrData[idx+1:]
		}
	}

	// Try base64 decode
	decoded, err := base64.StdEncoding.DecodeString(urlOrData)
	if err == nil && len(decoded) > 0 {
		return decoded, nil
	}

	return nil, fmt.Errorf("cannot decode image data: not a valid URL or base64")
}

func downloadImage(ctx context.Context, url string, client *http.Client) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create image download request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download image: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download image: HTTP %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

// SiliconFlowFactory creates a SiliconFlow provider via the plugin registry.
func SiliconFlowFactory(cfg map[string]interface{}) (interface{}, error) {
	return NewSiliconFlowProvider(SiliconFlowConfig{
		Endpoint: stringFromMap(cfg, "endpoint", defaultSiliconFlowEndpoint),
		APIKey:   stringFromMap(cfg, "api_key", ""),
		Model:    stringFromMap(cfg, "model", defaultSiliconFlowModel),
	})
}

func stringFromMap(cfg map[string]interface{}, key, defaultVal string) string {
	if v, ok := cfg[key]; ok {
		if s, ok := v.(string); ok && s != "" {
			return s
		}
	}
	return defaultVal
}
