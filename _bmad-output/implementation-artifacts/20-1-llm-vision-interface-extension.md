# Story 20.1: LLM Vision Interface Extension

Status: done

## Story

As a system,
I want the LLM plugin interface to support multimodal (text + image) completions,
So that vision-capable models can evaluate images alongside text prompts.

## Acceptance Criteria

1. `CompleteWithVision(ctx, []VisionMessage, CompletionOptions) (*CompletionResult, error)` added to LLM interface
2. `VisionMessage` struct with `Role string` and `Content []ContentPart`
3. `ContentPart` struct with `Type string` ("text" or "image_url"), `Text string`, `ImageURL string`
4. `ErrNotSupported` sentinel error added to `plugin/llm` package (follows `imagegen.ErrNotSupported` pattern)
5. `OpenAICompatibleProvider.CompleteWithVision()` serializes to OpenAI multimodal format and reuses `doRequest`-equivalent logic with retry
6. `FallbackChain.CompleteWithVision()` tries each provider in order, skipping `ErrNotSupported` providers
7. Mock regenerated via `make generate` — all existing tests pass without modification
8. Existing `Complete()` signature is NOT changed

## Tasks / Subtasks

- [x] Task 1: Add types and interface method (AC: #1, #2, #3, #4)
  - [x] 1.1 Add `VisionMessage`, `ContentPart` structs to `internal/plugin/llm/interface.go`
  - [x] 1.2 Add `ErrNotSupported` sentinel error to `internal/plugin/llm/interface.go`
  - [x] 1.3 Add `CompleteWithVision()` method to `LLM` interface

- [x] Task 2: Implement OpenAI-compatible vision request (AC: #5)
  - [x] 2.1 Add multimodal chat message types (`visionChatMessage`, `visionContentPart`) to `openai.go`
  - [x] 2.2 Add `doVisionRequest()` that builds OpenAI multimodal format request
  - [x] 2.3 Implement `CompleteWithVision()` on `OpenAICompatibleProvider` with retry logic

- [x] Task 3: Implement FallbackChain support (AC: #6)
  - [x] 3.1 Add `CompleteWithVision()` to `FallbackChain` — same pattern as `Complete()`

- [x] Task 4: Regenerate mocks and verify (AC: #7, #8)
  - [x] 4.1 Run `make generate` to regenerate `mock_LLM.go`
  - [x] 4.2 Run `make test` to verify zero breakage
  - [x] 4.3 Run `make lint` to verify clean

- [x] Task 5: Add unit tests
  - [x] 5.1 Test `CompleteWithVision()` success path (mock HTTP server)
  - [x] 5.2 Test `ErrNotSupported` return and `errors.Is()` check
  - [x] 5.3 Test vision message serialization to OpenAI multimodal JSON format
  - [x] 5.4 Test `FallbackChain.CompleteWithVision()` fallback behavior (success, skip unsupported, fallback on error)
  - [x] 5.5 Test `FallbackChain.CompleteWithVision()` all-fail error aggregation

## Dev Notes

### Architecture Constraints

- **DO NOT change `Complete()` signature** — this is the most critical constraint
- Follow `imagegen.ErrNotSupported` pattern exactly: `var ErrNotSupported = errors.New("operation not supported by this provider")`
- The `ErrNotSupported` error goes in `internal/plugin/llm/interface.go` alongside type definitions

### OpenAI Multimodal Format

The vision request must serialize messages as:
```json
{
  "model": "qwen-vl-max",
  "messages": [
    {
      "role": "user",
      "content": [
        {"type": "text", "text": "Evaluate this image..."},
        {"type": "image_url", "image_url": {"url": "data:image/png;base64,..."}}
      ]
    }
  ]
}
```

Note: The `content` field changes from `string` to `[]object` for vision messages. This means `doRequest()` cannot be directly reused because `chatMessage.Content` is typed as `string`. Create a parallel `doVisionRequest()` with vision-specific message types.

### Vision-specific request types (add to openai.go)

```go
type visionChatRequest struct {
    Model       string               `json:"model"`
    Messages    []visionChatMessage   `json:"messages"`
    Temperature float64              `json:"temperature,omitempty"`
    MaxTokens   int                  `json:"max_tokens,omitempty"`
}

type visionChatMessage struct {
    Role    string             `json:"role"`
    Content []visionContentPart `json:"content"`
}

type visionContentPart struct {
    Type     string            `json:"type"`
    Text     string            `json:"text,omitempty"`
    ImageURL *visionImageURL   `json:"image_url,omitempty"`
}

type visionImageURL struct {
    URL string `json:"url"`
}
```

### Response Parsing

Response format is identical to `Complete()` — same `chatResponse` struct works. Reuse the response parsing logic.

### Retry Logic

Reuse existing `retry.Do()` pattern from `Complete()`:
```go
err := retry.Do(ctx, p.pluginCfg.MaxRetries, p.pluginCfg.BaseDelay, func() error { ... })
```

### FallbackChain Pattern

Follow exact pattern from `FallbackChain.Complete()`:
- Try each provider in order
- On `ErrNotSupported`, skip to next provider (not a failure, just unsupported)
- On other errors, fall back to next provider
- If all fail, return aggregated error

### Existing Code Patterns to Follow

| Pattern | Location | Description |
|---------|----------|-------------|
| Sentinel error | `internal/plugin/imagegen/interface.go:46` | `var ErrNotSupported = errors.New(...)` |
| HTTP request | `internal/plugin/llm/openai.go:182-245` | `doRequest()` — build, send, parse |
| Retry wrapper | `internal/plugin/llm/openai.go:151-158` | `retry.Do()` around HTTP call |
| Fallback chain | `internal/plugin/llm/fallback.go:32-59` | Try each provider, aggregate errors |
| go:generate | `internal/plugin/llm/interface.go:10` | mockery directive for LLM mock |

### Project Structure Notes

Files to modify (in order):
1. `internal/plugin/llm/interface.go` — Add types + interface method
2. `internal/plugin/llm/openai.go` — Add vision request types + `CompleteWithVision()` impl
3. `internal/plugin/llm/fallback.go` — Add `CompleteWithVision()` delegation
4. `internal/mocks/mock_LLM.go` — Auto-regenerated by `make generate`

Test files:
- `internal/plugin/llm/openai_test.go` — Vision request serialization + HTTP mock tests
- `internal/plugin/llm/fallback_test.go` — Vision fallback chain tests

### References

- [Source: _bmad-output/planning-artifacts/architecture.md#LLM Interface Vision Extension]
- [Source: _bmad-output/planning-artifacts/epics.md#Story 20.1]
- [Source: internal/plugin/imagegen/interface.go:46 — ErrNotSupported pattern]
- [Source: internal/plugin/llm/openai.go:122-180 — Complete() implementation]
- [Source: internal/plugin/llm/fallback.go:32-59 — FallbackChain.Complete()]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Debug Log References

### Completion Notes List

- ErrNotSupported placed in interface.go (alongside types) rather than errors.go — keeps related types co-located
- VisionMessage/ContentPart types defined with JSON tags for potential future serialization
- doVisionRequest() parallels doRequest() but with vision-specific message types
- FallbackChain.CompleteWithVision() skips ErrNotSupported (debug log) vs falls back on other errors (warn log)
- stubLLM in fallback_test.go updated with default CompleteWithVision() returning ErrNotSupported
- visionStubLLM added for vision-specific fallback chain testing
- All 8 acceptance criteria satisfied
- make test: all packages pass, make lint: clean

### Change Log

- 2026-03-18: Story 20.1 implementation complete — LLM Vision interface extension

### File List

- internal/plugin/llm/interface.go (modified — added VisionMessage, ContentPart, ErrNotSupported, CompleteWithVision)
- internal/plugin/llm/openai.go (modified — added vision request types, CompleteWithVision, doVisionRequest)
- internal/plugin/llm/fallback.go (modified — added CompleteWithVision with ErrNotSupported skip logic)
- internal/plugin/llm/openai_test.go (modified — added 4 vision tests + ErrNotSupported test)
- internal/plugin/llm/fallback_test.go (modified — added visionStubLLM + 4 vision fallback tests + stubLLM CompleteWithVision)
- internal/mocks/mock_LLM.go (regenerated — includes CompleteWithVision mock)
