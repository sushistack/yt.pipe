# Story 8-1: Gemini LLM Provider Implementation

## Status: Done

## Implementation Summary

### New Files
- `internal/plugin/llm/openai.go` — `OpenAICompatibleProvider` struct implementing the `LLM` interface for all OpenAI-compatible APIs (Gemini, Qwen, DeepSeek). Includes `GeminiFactory`, `QwenFactory`, `DeepSeekFactory` for plugin registry registration. HTTP-based API client with retry (exponential backoff, max 3 retries). Token usage logged at INFO level.
- `internal/plugin/llm/errors.go` — `APIError` type with `IsRetryable()` implementing `retry.RetryableError`. Retryable: 429, 500, 502, 503. Non-retryable: 400, 401, 403.
- `internal/plugin/llm/openai_test.go` — Tests using `httptest` server to mock OpenAI-compatible API responses, retry behavior, error handling

### Modified Files
- `internal/plugin/llm/interface.go` — Added `Complete(ctx, messages, opts)` method to `LLM` interface; added `Message`, `CompletionOptions`, `CompletionResult` types
- `internal/config/types.go` — Added `Endpoint` and `Fallback` fields to `LLMConfig`
- `internal/config/config.go` — Changed LLM default provider from `openai` to `gemini`
- `internal/mocks/mock_LLM.go` — Added `Complete` method mock manually

### Architecture Decisions
- Single `OpenAICompatibleProvider` reused for all providers (Gemini, Qwen, DeepSeek) — only config differs, no code duplication
- Each provider registered via its own Factory function in the plugin registry
- Retry uses existing `internal/retry` package with exponential backoff + jitter
- JSON response parsing helpers: `extractJSON()`, `parseScenarioJSON()`, `parseSceneJSON()`

### Acceptance Criteria Met
- [x] `OpenAICompatibleProvider` connects to Gemini's OpenAI-compatible endpoint
- [x] Response parsed into `domain.ScenarioOutput` with scenes, narration, fact tags
- [x] Token usage (input/output) logged at INFO level
- [x] Retryable errors (429, 500, 503) retried with exponential backoff
- [x] Non-retryable errors (400, 401, 403) returned immediately
- [x] Provider registered in plugin registry under "gemini"
- [x] All tests pass
