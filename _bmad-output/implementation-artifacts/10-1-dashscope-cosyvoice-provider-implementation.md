# Story 10.1: DashScope CosyVoice Provider Implementation

Status: done

## Story

As a creator,
I want the system to connect to DashScope's CosyVoice API for TTS narration,
So that I can generate professional Korean narration audio from my scenarios.

## Acceptance Criteria

1. A `DashScopeProvider` struct in `plugin/tts/dashscope.go` implements the `TTS` interface (Synthesize + SynthesizeWithOverrides)
2. Uses REST mode: `POST /api/v1/services/aigc/text2audio/generation` with `Authorization: Bearer {api_key}`
3. Registered in plugin registry under name `"dashscope"`
4. Returns `SynthesisResult` with audio bytes (MP3), word-level timings (`[]domain.WordTiming`), and duration
5. API errors mapped to `APIError` with `IsRetryable()` (429, 500, 502, 503 retryable)
6. Retry with exponential backoff using existing `retry.Do(ctx, 3, 1*time.Second, fn)`
7. Config: `tts.provider: dashscope`, `tts.model: cosyvoice-v1`, `tts.format: mp3`, `tts.endpoint` (DashScope URL)

## Tasks / Subtasks

- [ ] Create `internal/plugin/tts/errors.go` with TTS APIError (same pattern as imagegen/errors.go)
- [ ] Create `internal/plugin/tts/dashscope.go` implementing TTS interface
- [ ] Update `internal/config/types.go` - add Model, Format, Endpoint to TTSConfig
- [ ] Update `internal/config/config.go` - add TTS defaults for dashscope
- [ ] Update `internal/cli/plugins.go` - register dashscope TTS factory
- [ ] Create `internal/plugin/tts/dashscope_test.go`

## Dev Notes

### Existing Patterns to Follow
- **SiliconFlow provider** (`plugin/imagegen/siliconflow.go`): HTTP client pattern, retry, error handling
- **Plugin registry**: Factory pattern `func(cfg map[string]interface{}) (interface{}, error)`
- **PluginConfig**: `plugin.DefaultPluginConfig("dashscope")` for timeout/retry defaults
- **Error pattern**: `APIError{Provider, StatusCode, Message, Err}` with `IsRetryable()`

### DashScope API Details
- Endpoint: `https://dashscope.aliyuncs.com/api/v1/services/aigc/text2audio/generation`
- Auth: `Authorization: Bearer {api_key}`
- Request: `{"model": "cosyvoice-v1", "input": {"text": "..."}, "parameters": {"voice": "...", "format": "mp3"}}`
- Response: Contains audio data (base64) and word-level timestamps

### Files NOT to Touch (Epic 9 scope)
- `internal/plugin/imagegen/*` - Epic 9 territory
- `internal/service/image_gen.go` - Epic 9 territory

### References
- [Source: _bmad-output/planning-artifacts/epics.md#Story 10.1]
- [Source: internal/plugin/imagegen/siliconflow.go] - pattern reference
- [Source: internal/plugin/tts/interface.go] - interface to implement
