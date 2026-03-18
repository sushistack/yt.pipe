package llm

import (
	"context"
	"fmt"
	"testing"

	"github.com/sushistack/yt.pipe/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubLLM is a simple LLM stub for fallback testing.
type stubLLM struct {
	name       string
	completeOK bool
	genOK      bool
	regenOK    bool
}

func (s *stubLLM) CompleteWithVision(ctx context.Context, messages []VisionMessage, opts CompletionOptions) (*CompletionResult, error) {
	return nil, ErrNotSupported
}

func (s *stubLLM) Complete(ctx context.Context, messages []Message, opts CompletionOptions) (*CompletionResult, error) {
	if s.completeOK {
		return &CompletionResult{Content: "response from " + s.name, Model: s.name}, nil
	}
	return nil, &APIError{Provider: s.name, StatusCode: 500, Message: "server error"}
}

func (s *stubLLM) GenerateScenario(ctx context.Context, scpID string, mainText string, facts []domain.FactTag, metadata map[string]string) (*domain.ScenarioOutput, error) {
	if s.genOK {
		return &domain.ScenarioOutput{SCPID: scpID, Title: "from " + s.name}, nil
	}
	return nil, &domain.PluginError{Plugin: s.name, Operation: "gen", Err: fmt.Errorf("failed")}
}

func (s *stubLLM) RegenerateSection(ctx context.Context, scenario *domain.ScenarioOutput, sceneNum int, instruction string) (*domain.SceneScript, error) {
	if s.regenOK {
		return &domain.SceneScript{SceneNum: sceneNum, Narration: "from " + s.name}, nil
	}
	return nil, &domain.PluginError{Plugin: s.name, Operation: "regen", Err: fmt.Errorf("failed")}
}

func TestFallbackChain_Complete_PrimarySucceeds(t *testing.T) {
	primary := &stubLLM{name: "gemini", completeOK: true}
	fallback := &stubLLM{name: "qwen", completeOK: true}

	chain, err := NewFallbackChain([]LLM{primary, fallback}, []string{"gemini", "qwen"})
	require.NoError(t, err)

	result, err := chain.Complete(context.Background(), []Message{{Role: "user", Content: "test"}}, CompletionOptions{})
	require.NoError(t, err)
	assert.Equal(t, "response from gemini", result.Content)
}

func TestFallbackChain_Complete_FallsBack(t *testing.T) {
	primary := &stubLLM{name: "gemini", completeOK: false}
	fallback := &stubLLM{name: "qwen", completeOK: true}

	chain, err := NewFallbackChain([]LLM{primary, fallback}, []string{"gemini", "qwen"})
	require.NoError(t, err)

	result, err := chain.Complete(context.Background(), []Message{{Role: "user", Content: "test"}}, CompletionOptions{})
	require.NoError(t, err)
	assert.Equal(t, "response from qwen", result.Content)
}

func TestFallbackChain_Complete_AllFail(t *testing.T) {
	p1 := &stubLLM{name: "gemini", completeOK: false}
	p2 := &stubLLM{name: "qwen", completeOK: false}
	p3 := &stubLLM{name: "deepseek", completeOK: false}

	chain, err := NewFallbackChain([]LLM{p1, p2, p3}, []string{"gemini", "qwen", "deepseek"})
	require.NoError(t, err)

	_, err = chain.Complete(context.Background(), []Message{{Role: "user", Content: "test"}}, CompletionOptions{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "all LLM providers failed")
	assert.Contains(t, err.Error(), "gemini")
	assert.Contains(t, err.Error(), "qwen")
	assert.Contains(t, err.Error(), "deepseek")
}

func TestFallbackChain_GenerateScenario_FallsBack(t *testing.T) {
	primary := &stubLLM{name: "gemini", genOK: false}
	fallback := &stubLLM{name: "qwen", genOK: true}

	chain, err := NewFallbackChain([]LLM{primary, fallback}, []string{"gemini", "qwen"})
	require.NoError(t, err)

	result, err := chain.GenerateScenario(context.Background(), "SCP-173", "", nil, nil)
	require.NoError(t, err)
	assert.Equal(t, "from qwen", result.Title)
}

// visionStubLLM supports vision for fallback chain testing.
type visionStubLLM struct {
	stubLLM
	visionOK bool
}

func (s *visionStubLLM) CompleteWithVision(ctx context.Context, messages []VisionMessage, opts CompletionOptions) (*CompletionResult, error) {
	if s.visionOK {
		return &CompletionResult{Content: "vision from " + s.name, Model: s.name}, nil
	}
	return nil, &APIError{Provider: s.name, StatusCode: 500, Message: "vision error"}
}

func TestFallbackChain_CompleteWithVision_Success(t *testing.T) {
	primary := &visionStubLLM{stubLLM: stubLLM{name: "qwen-vl"}, visionOK: true}
	fallback := &visionStubLLM{stubLLM: stubLLM{name: "gpt4v"}, visionOK: true}

	chain, err := NewFallbackChain([]LLM{primary, fallback}, []string{"qwen-vl", "gpt4v"})
	require.NoError(t, err)

	result, err := chain.CompleteWithVision(context.Background(), []VisionMessage{
		{Role: "user", Content: []ContentPart{{Type: "text", Text: "test"}}},
	}, CompletionOptions{})
	require.NoError(t, err)
	assert.Equal(t, "vision from qwen-vl", result.Content)
}

func TestFallbackChain_CompleteWithVision_SkipsUnsupported(t *testing.T) {
	// Primary returns ErrNotSupported, fallback supports vision
	primary := &stubLLM{name: "gemini"}                                       // returns ErrNotSupported from default stub
	fallback := &visionStubLLM{stubLLM: stubLLM{name: "qwen-vl"}, visionOK: true}

	chain, err := NewFallbackChain([]LLM{primary, fallback}, []string{"gemini", "qwen-vl"})
	require.NoError(t, err)

	result, err := chain.CompleteWithVision(context.Background(), []VisionMessage{
		{Role: "user", Content: []ContentPart{{Type: "text", Text: "test"}}},
	}, CompletionOptions{})
	require.NoError(t, err)
	assert.Equal(t, "vision from qwen-vl", result.Content)
}

func TestFallbackChain_CompleteWithVision_FallsBackOnError(t *testing.T) {
	primary := &visionStubLLM{stubLLM: stubLLM{name: "qwen-vl"}, visionOK: false}
	fallback := &visionStubLLM{stubLLM: stubLLM{name: "gpt4v"}, visionOK: true}

	chain, err := NewFallbackChain([]LLM{primary, fallback}, []string{"qwen-vl", "gpt4v"})
	require.NoError(t, err)

	result, err := chain.CompleteWithVision(context.Background(), []VisionMessage{
		{Role: "user", Content: []ContentPart{{Type: "text", Text: "test"}}},
	}, CompletionOptions{})
	require.NoError(t, err)
	assert.Equal(t, "vision from gpt4v", result.Content)
}

func TestFallbackChain_CompleteWithVision_AllFail(t *testing.T) {
	p1 := &stubLLM{name: "gemini"}                                          // ErrNotSupported
	p2 := &visionStubLLM{stubLLM: stubLLM{name: "qwen-vl"}, visionOK: false} // API error

	chain, err := NewFallbackChain([]LLM{p1, p2}, []string{"gemini", "qwen-vl"})
	require.NoError(t, err)

	_, err = chain.CompleteWithVision(context.Background(), []VisionMessage{
		{Role: "user", Content: []ContentPart{{Type: "text", Text: "test"}}},
	}, CompletionOptions{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "all LLM providers failed for vision")
}

func TestFallbackChain_EmptyProviders(t *testing.T) {
	_, err := NewFallbackChain(nil, nil)
	require.Error(t, err)
}

func TestFallbackChain_MismatchedLengths(t *testing.T) {
	_, err := NewFallbackChain(
		[]LLM{&stubLLM{name: "a"}},
		[]string{"a", "b"},
	)
	require.Error(t, err)
}
