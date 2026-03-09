# Story 15.1: TTS Plugin Interface Extension

Status: done

## Story

As a developer,
I want the TTS plugin interface extended with TTSOptions and MoodPreset parameters,
So that TTS plugins can apply mood-specific voice parameters for scene-appropriate narration.

## Acceptance Criteria

1. **Given** the existing TTS interface `Synthesize(ctx, text string, voice string) (*SynthesisResult, error)` and `SynthesizeWithOverrides(ctx, text string, voice string, overrides map[string]string) (*SynthesisResult, error)`
   **When** the interface is updated in `internal/plugin/tts/interface.go`
   **Then** the signatures become `Synthesize(ctx, text, voice string, opts *TTSOptions) (*SynthesisResult, error)` and `SynthesizeWithOverrides(ctx, text, voice string, overrides map[string]string, opts *TTSOptions) (*SynthesisResult, error)`
   **And** `TTSOptions` struct contains `MoodPreset *MoodPreset`
   **And** `MoodPreset` struct contains `Speed float64`, `Emotion string`, `Pitch float64`, `Params map[string]any`
   **And** `opts` being `nil` or `MoodPreset` being `nil` uses default TTS tone (backward compatible)

2. **Given** the existing DashScope implementation in `internal/plugin/tts/dashscope.go`
   **When** the signature is updated
   **Then** the implementation accepts `opts *TTSOptions` and uses default parameters when `opts` or `MoodPreset` is nil
   **And** all existing unit tests for DashScope are updated to pass the new signature
   **And** no existing functionality is broken

3. **Given** all call sites that use the TTS interface
   **When** the interface gains a new parameter
   **Then** all callers are updated to pass `nil` as opts (backward compatible behavior)
   **And** mock TTS implementation is updated to match the new interface

## Tasks / Subtasks

- [x] Task 1: Add MoodPreset and TTSOptions structs to interface.go (AC: #1)
  - [x] Define `MoodPreset` with Speed, Emotion, Pitch, Params fields
  - [x] Define `TTSOptions` with `MoodPreset *MoodPreset` field
  - [x] Update `Synthesize` signature to include `opts *TTSOptions`
  - [x] Update `SynthesizeWithOverrides` signature to include `opts *TTSOptions`
- [x] Task 2: Update DashScope implementation (AC: #2)
  - [x] Update `Synthesize()` and `SynthesizeWithOverrides()` public methods
  - [x] Update internal `synthesize()` method signature
  - [x] Verify nil opts handling (backward compatible)
- [x] Task 3: Update mock TTS (AC: #3)
  - [x] Update `internal/mocks/mock_tts.go` with new method signatures
- [x] Task 4: Update all callers (AC: #3)
  - [x] Update `internal/service/tts.go` — pass `nil` as opts
  - [x] Update `internal/service/tts_test.go` — update mock expectations with `mock.Anything` for opts
  - [x] Update `tests/integration/pipeline_test.go` — pass `nil` as opts
- [x] Task 5: Update and add tests
  - [x] Update all existing DashScope tests to pass `nil` as opts
  - [x] Add `TestSynthesize_WithMoodPreset` — verify opts with mood preset accepted
  - [x] Add `TestSynthesize_NilOpts` — verify backward compatibility

## Dev Notes

### Architecture Decision: Last parameter vs wrapper struct

Adding `opts *TTSOptions` as the **last parameter** (rather than wrapping all params in a struct) was chosen because:
- Minimal diff — only appending one parameter to each method
- `nil` is the natural zero-value for "no options" — backward compatible
- Matches the pattern used in the LLM interface (`CompletionOptions` as last param)

### Key Files Modified

| File | Change |
|------|--------|
| `internal/plugin/tts/interface.go` | Added `MoodPreset`, `TTSOptions` structs; updated TTS interface methods |
| `internal/plugin/tts/dashscope.go` | Updated method signatures to accept `opts *TTSOptions` |
| `internal/plugin/tts/dashscope_test.go` | Updated all test calls + added mood preset tests |
| `internal/mocks/mock_tts.go` | Updated mock methods to match new interface |
| `internal/service/tts.go` | Updated calls to pass `nil` opts |
| `internal/service/tts_test.go` | Updated mock expectations with `mock.Anything` for opts |
| `tests/integration/pipeline_test.go` | Updated Synthesize call to pass `nil` opts |

### Testing Standards

- Framework: `testify` (assert + require) for service tests, stdlib for plugin tests
- Run: `make test` (go test ./...)
- Lint: `make lint` (go vet ./...)

### References

- [Source: _bmad-output/planning-artifacts/epics.md — Story 15.1 AC]
- [Source: internal/plugin/tts/interface.go — TTS interface]

## Dev Agent Record

### Agent Model Used
Claude Opus 4.6

### Completion Notes List
- Added `MoodPreset` struct (Speed, Emotion, Pitch, Params) and `TTSOptions` struct to interface.go
- Updated TTS interface: both methods gain `opts *TTSOptions` as last parameter
- DashScope implementation updated — opts parameter threaded through to internal synthesize()
- Mock TTS updated to match new interface signatures
- All callers updated to pass `nil` opts (backward compatible)
- Added tests: `TestSynthesize_WithMoodPreset`, `TestSynthesize_NilOpts`
- All existing tests pass with updated signatures
- `go vet ./...` passes clean

### File List
- `internal/plugin/tts/interface.go` (modified)
- `internal/plugin/tts/dashscope.go` (modified)
- `internal/plugin/tts/dashscope_test.go` (modified)
- `internal/mocks/mock_tts.go` (modified)
- `internal/service/tts.go` (modified)
- `internal/service/tts_test.go` (modified)
- `tests/integration/pipeline_test.go` (modified)
