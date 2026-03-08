package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewOpenAICompatibleProvider_MissingAPIKey(t *testing.T) {
	_, err := NewOpenAICompatibleProvider(OpenAIConfig{
		ProviderName: "gemini",
		Endpoint:     "https://example.com",
		APIKey:       "",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "authentication failed")
}

func TestNewOpenAICompatibleProvider_MissingEndpoint(t *testing.T) {
	_, err := NewOpenAICompatibleProvider(OpenAIConfig{
		ProviderName: "gemini",
		Endpoint:     "",
		APIKey:       "test-key",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "endpoint URL is required")
}

func TestComplete_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/chat/completions", r.URL.Path)
		assert.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var req chatRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		assert.Equal(t, "test-model", req.Model)
		assert.Len(t, req.Messages, 1)
		assert.Equal(t, "user", req.Messages[0].Role)

		resp := chatResponse{
			ID: "test-id",
			Choices: []chatChoice{
				{Message: chatMessage{Role: "assistant", Content: "Hello response"}},
			},
			Usage: chatUsage{PromptTokens: 10, CompletionTokens: 5},
			Model: "test-model",
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider, err := NewOpenAICompatibleProvider(OpenAIConfig{
		ProviderName: "test",
		Endpoint:     server.URL,
		APIKey:       "test-key",
		Model:        "test-model",
	})
	require.NoError(t, err)

	result, err := provider.Complete(context.Background(), []Message{
		{Role: "user", Content: "Hello"},
	}, CompletionOptions{})
	require.NoError(t, err)

	assert.Equal(t, "Hello response", result.Content)
	assert.Equal(t, 10, result.InputTokens)
	assert.Equal(t, 5, result.OutputTokens)
	assert.Equal(t, "test-model", result.Model)
}

func TestComplete_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(errorResponse{
			Error: struct {
				Message string `json:"message"`
				Type    string `json:"type"`
				Code    string `json:"code"`
			}{Message: "Invalid API key"},
		})
	}))
	defer server.Close()

	provider, err := NewOpenAICompatibleProvider(OpenAIConfig{
		ProviderName: "test",
		Endpoint:     server.URL,
		APIKey:       "bad-key",
		Model:        "test-model",
	})
	require.NoError(t, err)

	_, err = provider.Complete(context.Background(), []Message{
		{Role: "user", Content: "Hello"},
	}, CompletionOptions{})
	require.Error(t, err)

	var apiErr *APIError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, 401, apiErr.StatusCode)
	assert.False(t, apiErr.IsRetryable())
}

func TestComplete_RetryableError(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount < 3 {
			w.WriteHeader(http.StatusTooManyRequests)
			json.NewEncoder(w).Encode(errorResponse{
				Error: struct {
					Message string `json:"message"`
					Type    string `json:"type"`
					Code    string `json:"code"`
				}{Message: "Rate limited"},
			})
			return
		}
		resp := chatResponse{
			Choices: []chatChoice{
				{Message: chatMessage{Role: "assistant", Content: "Success after retry"}},
			},
			Usage: chatUsage{PromptTokens: 10, CompletionTokens: 5},
			Model: "test-model",
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider, err := NewOpenAICompatibleProvider(OpenAIConfig{
		ProviderName: "test",
		Endpoint:     server.URL,
		APIKey:       "test-key",
		Model:        "test-model",
	})
	require.NoError(t, err)
	// Speed up retries for test
	provider.pluginCfg.BaseDelay = 1

	result, err := provider.Complete(context.Background(), []Message{
		{Role: "user", Content: "Hello"},
	}, CompletionOptions{})
	require.NoError(t, err)
	assert.Equal(t, "Success after retry", result.Content)
	assert.Equal(t, 3, callCount)
}

func TestComplete_EmptyChoices(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := chatResponse{
			Choices: []chatChoice{},
			Usage:   chatUsage{},
			Model:   "test-model",
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider, err := NewOpenAICompatibleProvider(OpenAIConfig{
		ProviderName: "test",
		Endpoint:     server.URL,
		APIKey:       "test-key",
		Model:        "test-model",
	})
	require.NoError(t, err)

	_, err = provider.Complete(context.Background(), []Message{
		{Role: "user", Content: "Hello"},
	}, CompletionOptions{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty response")
}

func TestExtractJSON(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"plain json", `{"key":"val"}`, `{"key":"val"}`},
		{"code fence", "```json\n{\"key\":\"val\"}\n```", `{"key":"val"}`},
		{"bare fence", "```\n{\"key\":\"val\"}\n```", `{"key":"val"}`},
		{"with whitespace", "  \n```json\n{\"key\":\"val\"}\n```  \n", `{"key":"val"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractJSON(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGeminiFactory(t *testing.T) {
	_, err := GeminiFactory(map[string]interface{}{
		"api_key": "test-key",
	})
	require.NoError(t, err)
}

func TestGeminiFactory_NoKey(t *testing.T) {
	_, err := GeminiFactory(map[string]interface{}{})
	require.Error(t, err)
}

func TestProviderName(t *testing.T) {
	provider, err := NewOpenAICompatibleProvider(OpenAIConfig{
		ProviderName: "gemini",
		Endpoint:     "https://example.com",
		APIKey:       "test-key",
		Model:        "test",
	})
	require.NoError(t, err)
	assert.Equal(t, "gemini", provider.ProviderName())
}
