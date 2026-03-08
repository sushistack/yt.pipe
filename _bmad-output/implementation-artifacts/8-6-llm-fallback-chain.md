# Story 8-6: LLM Fallback Chain (Gemini → Qwen → DeepSeek)

## Status: Done

## Implementation Summary

### New Files
- `internal/plugin/llm/fallback.go` — `FallbackChain` struct implementing full `LLM` interface. `NewFallbackChain(providers []LLM, names []string)` constructor. Tries providers in order: primary → fallback 1 → fallback 2. Warning log on fallback activation. Collects all errors if all providers fail.
- `internal/plugin/llm/fallback_test.go` — Tests with `stubLLM`: primary success, primary fail + fallback success, all providers fail with collected errors

### Modified Files
- `internal/config/types.go` — Added `LLMFallbackItem` struct and `Fallback []LLMFallbackItem` to `LLMConfig`

### Architecture Decisions
- `FallbackChain` wraps ordered list of `LLM` interface implementations
- Each provider (Gemini, Qwen, DeepSeek) is an `OpenAICompatibleProvider` with different config
- Fallback configured in YAML: `llm.fallback: [{provider: "qwen", model: "qwen-max"}, {provider: "deepseek", model: "deepseek-chat"}]`
- Each provider independently retries (via its own retry logic) before falling back to next
- Warning log: "Primary LLM failed, falling back to {provider_name}"
- Final error message lists all attempted providers with individual error messages
- No code duplication between providers — only config differs

### Acceptance Criteria Met
- [x] `FallbackChain` implements full `LLM` interface
- [x] Ordered provider chain: primary → fallback 1 → fallback 2
- [x] Warning log on fallback activation
- [x] All-fail error lists all providers and their errors
- [x] Qwen/DeepSeek reuse same `OpenAICompatibleProvider` code
- [x] All tests pass
