package llm

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/sushistack/yt.pipe/internal/domain"
)

// FallbackChain implements the LLM interface with an ordered list of providers.
// If the primary provider fails (after exhausting retries), the next provider is attempted.
type FallbackChain struct {
	providers []LLM
	names     []string
}

// NewFallbackChain creates a fallback chain from an ordered list of providers.
// The first provider is primary; subsequent providers are fallbacks.
func NewFallbackChain(providers []LLM, names []string) (*FallbackChain, error) {
	if len(providers) == 0 {
		return nil, fmt.Errorf("fallback chain: at least one provider is required")
	}
	if len(providers) != len(names) {
		return nil, fmt.Errorf("fallback chain: providers and names must have same length")
	}
	return &FallbackChain{providers: providers, names: names}, nil
}

// Complete tries each provider in order until one succeeds.
func (fc *FallbackChain) Complete(ctx context.Context, messages []Message, opts CompletionOptions) (*CompletionResult, error) {
	var errs []string
	for i, p := range fc.providers {
		result, err := p.Complete(ctx, messages, opts)
		if err == nil {
			if i > 0 {
				slog.Info("llm fallback succeeded",
					"provider", fc.names[i],
					"attempts", i+1,
				)
			}
			return result, nil
		}
		errs = append(errs, fmt.Sprintf("%s: %v", fc.names[i], err))
		if i < len(fc.providers)-1 {
			slog.Warn("llm provider failed, falling back",
				"failed_provider", fc.names[i],
				"next_provider", fc.names[i+1],
				"err", err,
			)
		}
	}
	return nil, &APIError{
		Provider:   "fallback_chain",
		StatusCode: 502,
		Message:    fmt.Sprintf("all LLM providers failed: [%s]", strings.Join(errs, "; ")),
	}
}

// CompleteWithVision tries each provider in order, skipping ErrNotSupported providers.
func (fc *FallbackChain) CompleteWithVision(ctx context.Context, messages []VisionMessage, opts CompletionOptions) (*CompletionResult, error) {
	var errs []string
	for i, p := range fc.providers {
		result, err := p.CompleteWithVision(ctx, messages, opts)
		if err == nil {
			if i > 0 {
				slog.Info("llm vision fallback succeeded",
					"provider", fc.names[i],
					"attempts", i+1,
				)
			}
			return result, nil
		}
		if errors.Is(err, ErrNotSupported) {
			slog.Debug("llm provider does not support vision, skipping",
				"provider", fc.names[i],
			)
			errs = append(errs, fmt.Sprintf("%s: %v", fc.names[i], err))
			continue
		}
		errs = append(errs, fmt.Sprintf("%s: %v", fc.names[i], err))
		if i < len(fc.providers)-1 {
			slog.Warn("llm vision provider failed, falling back",
				"failed_provider", fc.names[i],
				"next_provider", fc.names[i+1],
				"err", err,
			)
		}
	}
	return nil, &APIError{
		Provider:   "fallback_chain",
		StatusCode: 502,
		Message:    fmt.Sprintf("all LLM providers failed for vision: [%s]", strings.Join(errs, "; ")),
	}
}

// GenerateScenario tries each provider in order.
func (fc *FallbackChain) GenerateScenario(ctx context.Context, scpID string, mainText string, facts []domain.FactTag, metadata map[string]string) (*domain.ScenarioOutput, error) {
	var errs []string
	for i, p := range fc.providers {
		result, err := p.GenerateScenario(ctx, scpID, mainText, facts, metadata)
		if err == nil {
			return result, nil
		}
		errs = append(errs, fmt.Sprintf("%s: %v", fc.names[i], err))
		if i < len(fc.providers)-1 {
			slog.Warn("llm provider failed, falling back",
				"failed_provider", fc.names[i],
				"next_provider", fc.names[i+1],
				"err", err,
			)
		}
	}
	return nil, &domain.PluginError{
		Plugin:    "fallback_chain",
		Operation: "generate_scenario",
		Err:       fmt.Errorf("all LLM providers failed: [%s]", strings.Join(errs, "; ")),
	}
}

// RegenerateSection tries each provider in order.
func (fc *FallbackChain) RegenerateSection(ctx context.Context, scenario *domain.ScenarioOutput, sceneNum int, instruction string) (*domain.SceneScript, error) {
	var errs []string
	for i, p := range fc.providers {
		result, err := p.RegenerateSection(ctx, scenario, sceneNum, instruction)
		if err == nil {
			return result, nil
		}
		errs = append(errs, fmt.Sprintf("%s: %v", fc.names[i], err))
		if i < len(fc.providers)-1 {
			slog.Warn("llm provider failed, falling back",
				"failed_provider", fc.names[i],
				"next_provider", fc.names[i+1],
				"err", err,
			)
		}
	}
	return nil, &domain.PluginError{
		Plugin:    "fallback_chain",
		Operation: "regenerate_section",
		Err:       fmt.Errorf("all LLM providers failed: [%s]", strings.Join(errs, "; ")),
	}
}
