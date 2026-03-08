// Package llm defines the interface for language model plugins.
package llm

import (
	"context"

	"github.com/sushistack/yt.pipe/internal/domain"
)

//go:generate go run github.com/vektra/mockery/v2@latest --name=LLM --output=../../../internal/mocks --outpkg=mocks

// Message represents a chat message for LLM completion.
type Message struct {
	Role    string `json:"role"`    // "system", "user", "assistant"
	Content string `json:"content"`
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

	// GenerateScenario generates a complete scenario from SCP data.
	GenerateScenario(ctx context.Context, scpID string, mainText string, facts []domain.FactTag, metadata map[string]string) (*domain.ScenarioOutput, error)

	// RegenerateSection regenerates a single scene's script based on instruction.
	RegenerateSection(ctx context.Context, scenario *domain.ScenarioOutput, sceneNum int, instruction string) (*domain.SceneScript, error)
}
