package llm

import (
	"bytes"
	"context"
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

// OpenAICompatibleProvider implements the LLM interface for any OpenAI-compatible API.
// Works with Gemini, Qwen, DeepSeek, and standard OpenAI endpoints.
type OpenAICompatibleProvider struct {
	providerName string
	endpoint     string // base URL (e.g., "https://generativelanguage.googleapis.com/v1beta/openai")
	apiKey       string
	model        string
	temperature  float64
	maxTokens    int
	httpClient   *http.Client
	pluginCfg    plugin.PluginConfig
}

// OpenAIConfig holds configuration for creating an OpenAI-compatible provider.
type OpenAIConfig struct {
	ProviderName string
	Endpoint     string
	APIKey       string
	Model        string
	Temperature  float64
	MaxTokens    int
}

// NewOpenAICompatibleProvider creates a new OpenAI-compatible LLM provider.
func NewOpenAICompatibleProvider(cfg OpenAIConfig) (*OpenAICompatibleProvider, error) {
	if cfg.APIKey == "" {
		return nil, &APIError{
			Provider:   cfg.ProviderName,
			StatusCode: 401,
			Message:    fmt.Sprintf("%s API authentication failed: check llm.api_key config", cfg.ProviderName),
		}
	}
	if cfg.Endpoint == "" {
		return nil, &APIError{
			Provider:   cfg.ProviderName,
			StatusCode: 400,
			Message:    "endpoint URL is required",
		}
	}

	pluginCfg := plugin.DefaultPluginConfig(cfg.ProviderName)

	temp := cfg.Temperature
	if temp == 0 {
		temp = 0.7
	}
	maxTokens := cfg.MaxTokens
	if maxTokens == 0 {
		maxTokens = 4096
	}

	return &OpenAICompatibleProvider{
		providerName: cfg.ProviderName,
		endpoint:     strings.TrimRight(cfg.Endpoint, "/"),
		apiKey:       cfg.APIKey,
		model:        cfg.Model,
		temperature:  temp,
		maxTokens:    maxTokens,
		httpClient: &http.Client{
			Timeout: pluginCfg.Timeout,
		},
		pluginCfg: pluginCfg,
	}, nil
}

// openAI API request/response types
type chatRequest struct {
	Model       string          `json:"model"`
	Messages    []chatMessage   `json:"messages"`
	Temperature float64         `json:"temperature,omitempty"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatResponse struct {
	ID      string         `json:"id"`
	Choices []chatChoice   `json:"choices"`
	Usage   chatUsage      `json:"usage"`
	Model   string         `json:"model"`
}

type chatChoice struct {
	Message chatMessage `json:"message"`
}

type chatUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
}

// Vision-specific request types for OpenAI multimodal format.
type visionChatRequest struct {
	Model       string              `json:"model"`
	Messages    []visionChatMessage `json:"messages"`
	Temperature float64             `json:"temperature,omitempty"`
	MaxTokens   int                 `json:"max_tokens,omitempty"`
}

type visionChatMessage struct {
	Role    string              `json:"role"`
	Content []visionContentPart `json:"content"`
}

type visionContentPart struct {
	Type     string          `json:"type"`
	Text     string          `json:"text,omitempty"`
	ImageURL *visionImageURL `json:"image_url,omitempty"`
}

type visionImageURL struct {
	URL string `json:"url"`
}

type errorResponse struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error"`
}

// Complete sends messages to the LLM and returns the completion result.
func (p *OpenAICompatibleProvider) Complete(ctx context.Context, messages []Message, opts CompletionOptions) (*CompletionResult, error) {
	model := p.model
	if opts.Model != "" {
		model = opts.Model
	}
	temp := p.temperature
	if opts.Temperature > 0 {
		temp = opts.Temperature
	}
	maxTokens := p.maxTokens
	if opts.MaxTokens > 0 {
		maxTokens = opts.MaxTokens
	}

	chatMsgs := make([]chatMessage, len(messages))
	for i, m := range messages {
		chatMsgs[i] = chatMessage{Role: m.Role, Content: m.Content}
	}

	reqBody := chatRequest{
		Model:       model,
		Messages:    chatMsgs,
		Temperature: temp,
		MaxTokens:   maxTokens,
	}

	var result *CompletionResult
	start := time.Now()

	err := retry.Do(ctx, p.pluginCfg.MaxRetries, p.pluginCfg.BaseDelay, func() error {
		resp, err := p.doRequest(ctx, reqBody)
		if err != nil {
			return err
		}
		result = resp
		return nil
	})

	elapsed := time.Since(start)
	if err != nil {
		slog.Error("llm completion failed",
			"provider", p.providerName,
			"model", model,
			"duration_ms", elapsed.Milliseconds(),
			"err", err,
		)
		return nil, err
	}

	slog.Info("llm completion succeeded",
		"provider", p.providerName,
		"model", result.Model,
		"input_tokens", result.InputTokens,
		"output_tokens", result.OutputTokens,
		"duration_ms", elapsed.Milliseconds(),
	)

	return result, nil
}

// CompleteWithVision sends multimodal messages to a vision-capable LLM.
func (p *OpenAICompatibleProvider) CompleteWithVision(ctx context.Context, messages []VisionMessage, opts CompletionOptions) (*CompletionResult, error) {
	model := p.model
	if opts.Model != "" {
		model = opts.Model
	}
	temp := p.temperature
	if opts.Temperature > 0 {
		temp = opts.Temperature
	}
	maxTokens := p.maxTokens
	if opts.MaxTokens > 0 {
		maxTokens = opts.MaxTokens
	}

	visionMsgs := make([]visionChatMessage, len(messages))
	for i, m := range messages {
		parts := make([]visionContentPart, 0, len(m.Content))
		for _, cp := range m.Content {
			switch cp.Type {
			case "text":
				parts = append(parts, visionContentPart{Type: "text", Text: cp.Text})
			case "image_url":
				parts = append(parts, visionContentPart{Type: "image_url", ImageURL: &visionImageURL{URL: cp.ImageURL}})
			default:
				slog.Warn("unknown vision content part type, skipping",
					"type", cp.Type,
					"provider", p.providerName,
				)
			}
		}
		visionMsgs[i] = visionChatMessage{Role: m.Role, Content: parts}
	}

	reqBody := visionChatRequest{
		Model:       model,
		Messages:    visionMsgs,
		Temperature: temp,
		MaxTokens:   maxTokens,
	}

	var result *CompletionResult
	start := time.Now()

	err := retry.Do(ctx, p.pluginCfg.MaxRetries, p.pluginCfg.BaseDelay, func() error {
		resp, err := p.doVisionRequest(ctx, reqBody)
		if err != nil {
			return err
		}
		result = resp
		return nil
	})

	elapsed := time.Since(start)
	if err != nil {
		slog.Error("llm vision completion failed",
			"provider", p.providerName,
			"model", model,
			"duration_ms", elapsed.Milliseconds(),
			"err", err,
		)
		return nil, err
	}

	slog.Info("llm vision completion succeeded",
		"provider", p.providerName,
		"model", result.Model,
		"input_tokens", result.InputTokens,
		"output_tokens", result.OutputTokens,
		"duration_ms", elapsed.Milliseconds(),
	)

	return result, nil
}

func (p *OpenAICompatibleProvider) doVisionRequest(ctx context.Context, reqBody visionChatRequest) (*CompletionResult, error) {
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal vision request: %w", err)
	}

	url := p.endpoint + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, &APIError{
			Provider:   p.providerName,
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
		var errResp errorResponse
		_ = json.Unmarshal(respBody, &errResp)
		msg := errResp.Error.Message
		if msg == "" {
			msg = string(respBody)
		}
		return nil, &APIError{
			Provider:   p.providerName,
			StatusCode: resp.StatusCode,
			Message:    msg,
		}
	}

	var chatResp chatResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	if len(chatResp.Choices) == 0 {
		return nil, &APIError{
			Provider:   p.providerName,
			StatusCode: 200,
			Message:    "empty response: no choices returned",
		}
	}

	return &CompletionResult{
		Content:      chatResp.Choices[0].Message.Content,
		InputTokens:  chatResp.Usage.PromptTokens,
		OutputTokens: chatResp.Usage.CompletionTokens,
		Model:        chatResp.Model,
	}, nil
}

func (p *OpenAICompatibleProvider) doRequest(ctx context.Context, reqBody chatRequest) (*CompletionResult, error) {
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	url := p.endpoint + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, &APIError{
			Provider:   p.providerName,
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
		var errResp errorResponse
		_ = json.Unmarshal(respBody, &errResp)
		msg := errResp.Error.Message
		if msg == "" {
			msg = string(respBody)
		}
		return nil, &APIError{
			Provider:   p.providerName,
			StatusCode: resp.StatusCode,
			Message:    msg,
		}
	}

	var chatResp chatResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	if len(chatResp.Choices) == 0 {
		return nil, &APIError{
			Provider:   p.providerName,
			StatusCode: 200,
			Message:    "empty response: no choices returned",
		}
	}

	return &CompletionResult{
		Content:      chatResp.Choices[0].Message.Content,
		InputTokens:  chatResp.Usage.PromptTokens,
		OutputTokens: chatResp.Usage.CompletionTokens,
		Model:        chatResp.Model,
	}, nil
}

// GenerateScenario generates a complete scenario from SCP data using a single LLM call.
// For the 4-stage pipeline, use the ScenarioPipeline service which calls Complete() directly.
func (p *OpenAICompatibleProvider) GenerateScenario(ctx context.Context, scpID string, mainText string, facts []domain.FactTag, metadata map[string]string) (*domain.ScenarioOutput, error) {
	prompt := buildScenarioPrompt(scpID, mainText, facts, metadata)

	result, err := p.Complete(ctx, []Message{
		{Role: "system", Content: "You are a professional video scenario writer for SCP Foundation content. Generate structured scenarios in JSON format."},
		{Role: "user", Content: prompt},
	}, CompletionOptions{})
	if err != nil {
		return nil, &domain.PluginError{
			Plugin:    p.providerName,
			Operation: "generate_scenario",
			Err:       err,
		}
	}

	scenario, err := parseScenarioJSON(result.Content, scpID)
	if err != nil {
		return nil, &domain.PluginError{
			Plugin:    p.providerName,
			Operation: "parse_scenario",
			Err:       err,
		}
	}

	return scenario, nil
}

// RegenerateSection regenerates a single scene's script based on instruction.
func (p *OpenAICompatibleProvider) RegenerateSection(ctx context.Context, scenario *domain.ScenarioOutput, sceneNum int, instruction string) (*domain.SceneScript, error) {
	scenarioJSON, _ := json.Marshal(scenario)
	prompt := fmt.Sprintf(
		"Here is the current scenario:\n```json\n%s\n```\n\nRegenerate scene %d with this instruction: %s\n\nReturn ONLY the regenerated scene as JSON with fields: scene_num, narration, visual_description, fact_tags (array of {key, content}), mood.",
		string(scenarioJSON), sceneNum, instruction,
	)

	result, err := p.Complete(ctx, []Message{
		{Role: "system", Content: "You are a professional video scenario writer. Regenerate the requested scene based on the instruction. Return valid JSON only."},
		{Role: "user", Content: prompt},
	}, CompletionOptions{})
	if err != nil {
		return nil, &domain.PluginError{
			Plugin:    p.providerName,
			Operation: "regenerate_section",
			Err:       err,
		}
	}

	scene, err := parseSceneJSON(result.Content)
	if err != nil {
		return nil, &domain.PluginError{
			Plugin:    p.providerName,
			Operation: "parse_scene",
			Err:       err,
		}
	}

	return scene, nil
}

// ProviderName returns the name of this provider.
func (p *OpenAICompatibleProvider) ProviderName() string {
	return p.providerName
}

func buildScenarioPrompt(scpID string, mainText string, facts []domain.FactTag, metadata map[string]string) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Generate a video scenario for %s.\n\n", scpID))
	b.WriteString("## SCP Document\n\n")
	b.WriteString(mainText)
	b.WriteString("\n\n## Facts\n\n")
	for _, f := range facts {
		b.WriteString(fmt.Sprintf("- %s: %s\n", f.Key, f.Content))
	}
	b.WriteString("\n## Metadata\n\n")
	for k, v := range metadata {
		b.WriteString(fmt.Sprintf("- %s: %s\n", k, v))
	}
	b.WriteString("\n## Instructions\n\n")
	b.WriteString("Generate a structured video scenario with 8-12 scenes. For each scene include:\n")
	b.WriteString("- scene_num (integer)\n- narration (Korean text for video narration)\n")
	b.WriteString("- visual_description (English, for image generation)\n")
	b.WriteString("- fact_tags (array of {key, content} referencing source facts)\n")
	b.WriteString("- mood (one word: tense, mysterious, horror, dramatic, etc.)\n\n")
	b.WriteString("Return a JSON object with: scp_id, title, scenes (array), metadata (object with template_version).")
	return b.String()
}

func parseScenarioJSON(content string, scpID string) (*domain.ScenarioOutput, error) {
	// Extract JSON from potential markdown code blocks
	cleaned := extractJSON(content)

	var raw struct {
		SCPID    string `json:"scp_id"`
		Title    string `json:"title"`
		Scenes   []struct {
			SceneNum    int    `json:"scene_num"`
			Narration   string `json:"narration"`
			VisualDesc  string `json:"visual_description"`
			FactTags    []struct {
				Key     string `json:"key"`
				Content string `json:"content"`
			} `json:"fact_tags"`
			Mood string `json:"mood"`
		} `json:"scenes"`
		Metadata map[string]any `json:"metadata"`
	}

	if err := json.Unmarshal([]byte(cleaned), &raw); err != nil {
		return nil, fmt.Errorf("parse scenario JSON: %w", err)
	}

	scenario := &domain.ScenarioOutput{
		SCPID:    scpID,
		Title:    raw.Title,
		Metadata: raw.Metadata,
	}
	if scenario.Metadata == nil {
		scenario.Metadata = map[string]any{}
	}

	for _, s := range raw.Scenes {
		scene := domain.SceneScript{
			SceneNum:          s.SceneNum,
			Narration:         s.Narration,
			VisualDescription: s.VisualDesc,
			Mood:              s.Mood,
		}
		for _, ft := range s.FactTags {
			scene.FactTags = append(scene.FactTags, domain.FactTag{Key: ft.Key, Content: ft.Content})
		}
		scenario.Scenes = append(scenario.Scenes, scene)
	}

	return scenario, nil
}

func parseSceneJSON(content string) (*domain.SceneScript, error) {
	cleaned := extractJSON(content)

	var raw struct {
		SceneNum    int    `json:"scene_num"`
		Narration   string `json:"narration"`
		VisualDesc  string `json:"visual_description"`
		FactTags    []struct {
			Key     string `json:"key"`
			Content string `json:"content"`
		} `json:"fact_tags"`
		Mood string `json:"mood"`
	}

	if err := json.Unmarshal([]byte(cleaned), &raw); err != nil {
		return nil, fmt.Errorf("parse scene JSON: %w", err)
	}

	scene := &domain.SceneScript{
		SceneNum:          raw.SceneNum,
		Narration:         raw.Narration,
		VisualDescription: raw.VisualDesc,
		Mood:              raw.Mood,
	}
	for _, ft := range raw.FactTags {
		scene.FactTags = append(scene.FactTags, domain.FactTag{Key: ft.Key, Content: ft.Content})
	}

	return scene, nil
}

// extractJSON strips markdown code fences from LLM output.
func extractJSON(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```json") {
		s = strings.TrimPrefix(s, "```json")
		if idx := strings.LastIndex(s, "```"); idx >= 0 {
			s = s[:idx]
		}
	} else if strings.HasPrefix(s, "```") {
		s = strings.TrimPrefix(s, "```")
		if idx := strings.LastIndex(s, "```"); idx >= 0 {
			s = s[:idx]
		}
	}
	return strings.TrimSpace(s)
}

// GeminiFactory creates a Gemini provider via the plugin registry.
func GeminiFactory(cfg map[string]interface{}) (interface{}, error) {
	return NewOpenAICompatibleProvider(OpenAIConfig{
		ProviderName: "gemini",
		Endpoint:     stringFromCfg(cfg, "endpoint", "https://generativelanguage.googleapis.com/v1beta/openai"),
		APIKey:       stringFromCfg(cfg, "api_key", ""),
		Model:        stringFromCfg(cfg, "model", "gemini-2.0-flash"),
		Temperature:  floatFromCfg(cfg, "temperature", 0.7),
		MaxTokens:    intFromCfg(cfg, "max_tokens", 4096),
	})
}

// QwenFactory creates a Qwen provider via the plugin registry.
func QwenFactory(cfg map[string]interface{}) (interface{}, error) {
	return NewOpenAICompatibleProvider(OpenAIConfig{
		ProviderName: "qwen",
		Endpoint:     stringFromCfg(cfg, "endpoint", "https://dashscope.aliyuncs.com/compatible-mode/v1"),
		APIKey:       stringFromCfg(cfg, "api_key", ""),
		Model:        stringFromCfg(cfg, "model", "qwen-max"),
		Temperature:  floatFromCfg(cfg, "temperature", 0.7),
		MaxTokens:    intFromCfg(cfg, "max_tokens", 4096),
	})
}

// DeepSeekFactory creates a DeepSeek provider via the plugin registry.
func DeepSeekFactory(cfg map[string]interface{}) (interface{}, error) {
	return NewOpenAICompatibleProvider(OpenAIConfig{
		ProviderName: "deepseek",
		Endpoint:     stringFromCfg(cfg, "endpoint", "https://api.deepseek.com/v1"),
		APIKey:       stringFromCfg(cfg, "api_key", ""),
		Model:        stringFromCfg(cfg, "model", "deepseek-chat"),
		Temperature:  floatFromCfg(cfg, "temperature", 0.7),
		MaxTokens:    intFromCfg(cfg, "max_tokens", 4096),
	})
}

func stringFromCfg(cfg map[string]interface{}, key, defaultVal string) string {
	if v, ok := cfg[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return defaultVal
}

func floatFromCfg(cfg map[string]interface{}, key string, defaultVal float64) float64 {
	if v, ok := cfg[key]; ok {
		switch f := v.(type) {
		case float64:
			return f
		case float32:
			return float64(f)
		case int:
			return float64(f)
		}
	}
	return defaultVal
}

func intFromCfg(cfg map[string]interface{}, key string, defaultVal int) int {
	if v, ok := cfg[key]; ok {
		switch i := v.(type) {
		case int:
			return i
		case float64:
			return int(i)
		}
	}
	return defaultVal
}
