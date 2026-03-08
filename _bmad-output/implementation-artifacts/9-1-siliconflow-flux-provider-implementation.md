# Story 9.1: SiliconFlow FLUX Provider Implementation

Status: done

## Story

As a creator,
I want the system to connect to SiliconFlow's FLUX API for image generation,
So that I can generate cinematic images for my SCP video scenes.

## Acceptance Criteria

1. A `SiliconFlowProvider` struct in `plugin/imagegen/siliconflow.go` implements the `ImageGen` interface (Generate)
2. Uses SiliconFlow API: `POST {endpoint}/images/generations` with `Authorization: Bearer {api_key}`
3. Registered in plugin registry under name `"siliconflow"` via `SiliconFlowFactory`
4. Returns `ImageResult` with image bytes (PNG), format, width, and height
5. Handles both base64-encoded and URL-based image responses via `decodeImageData`
6. API errors mapped to `APIError` with `IsRetryable()` (0, 429, 500, 502, 503 retryable)
7. Retry with exponential backoff using existing `retry.Do(ctx, maxRetries, baseDelay, fn)`
8. Config: `imagegen.provider: siliconflow`, `imagegen.model: black-forest-labs/FLUX.1-schnell`, dimensions 1920x1080
9. Retry-After header handling for rate limits

## Implementation Summary

### Files Created
- `internal/plugin/imagegen/errors.go` — APIError type with IsRetryable()
- `internal/plugin/imagegen/siliconflow.go` — SiliconFlow FLUX provider
- `internal/plugin/imagegen/siliconflow_test.go` — 17 tests

### Files Modified
- `internal/config/types.go` — Added Endpoint, Width, Height to ImageGenConfig
- `internal/config/config.go` — Added imagegen defaults (endpoint, model, dimensions)
- `internal/cli/plugins.go` — Registered SiliconFlowFactory + all LLM/TTS factories in init()

### Test Coverage
- Provider creation (success, custom model, no API key)
- Base64 and URL image responses
- Custom dimensions and seed
- Rate limiting (429 with Retry-After)
- Server error retries (500 → retry → success)
- Client error no-retry (400)
- Empty response handling
- Context cancellation
- Factory function
- APIError.IsRetryable
- decodeImageData (base64, data URI)

## References
- [Source: _bmad-output/planning-artifacts/epics.md#Story 9.1]
- [Source: internal/plugin/imagegen/interface.go] — interface implemented
- [Source: internal/plugin/llm/openai.go] — pattern reference
