// Package llm defines the interface for language model plugins.
package llm

import (
	"context"
	"errors"

	"github.com/sushistack/yt.pipe/internal/domain"
)

//go:generate go run github.com/vektra/mockery/v2@latest --name=LLM --output=../../../internal/mocks --outpkg=mocks

// ErrNotSupported indicates a provider does not support the requested operation.
var ErrNotSupported = errors.New("operation not supported by this provider")

// Message represents a chat message for LLM completion.
type Message struct {
	Role    string `json:"role"`    // "system", "user", "assistant"
	Content string `json:"content"`
}

// VisionMessage represents a multimodal message with text and images.
type VisionMessage struct {
	Role    string        `json:"role"` // "system", "user", "assistant"
	Content []ContentPart `json:"content"`
}

// ContentPart represents a single part of a multimodal message content.
type ContentPart struct {
	Type     string `json:"type"`               // "text" or "image_url"
	Text     string `json:"text,omitempty"`      // used when Type == "text"
	ImageURL string `json:"image_url,omitempty"` // used when Type == "image_url" (base64 data URI or URL)
}

// CompletionOptions configures a single LLM completion call.
type CompletionOptions struct {
	Temperature float64 `json:"temperature,omitempty"`
	MaxTokens   int     `json:"max_tokens,omitempty"`
	Model       string  `json:"model,omitempty"` // override default model
}

// CompletionResult holds the response from an LLM completion call.
type CompletionResult struct {
	Content      string `json:"content"`
	InputTokens  int    `json:"input_tokens"`
	OutputTokens int    `json:"output_tokens"`
	Model        string `json:"model"`
}

// LLM defines the interface for language model plugins.
type LLM interface {
	// Complete sends messages to the LLM and returns the completion result.
	// This is the low-level method used by the 4-stage scenario pipeline.
	Complete(ctx context.Context, messages []Message, opts CompletionOptions) (*CompletionResult, error)

	// CompleteWithVision sends multimodal messages to a vision-capable LLM.
	// Returns ErrNotSupported if the provider does not support vision.
	CompleteWithVision(ctx context.Context, messages []VisionMessage, opts CompletionOptions) (*CompletionResult, error)

	// GenerateScenario generates a complete scenario from SCP data.
	GenerateScenario(ctx context.Context, scpID string, mainText string, facts []domain.FactTag, metadata map[string]string) (*domain.ScenarioOutput, error)

	// RegenerateSection regenerates a single scene's script based on instruction.
	RegenerateSection(ctx context.Context, scenario *domain.ScenarioOutput, sceneNum int, instruction string) (*domain.SceneScript, error)
}
