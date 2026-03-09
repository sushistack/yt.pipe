# Story 14.1: ImageGen Plugin Interface Extension

Status: review

## Story

As a developer,
I want the ImageGen plugin interface extended with CharacterRef and character reference support in GenerateOptions,
So that image generation plugins can receive character visual references for consistent imagery.

## Acceptance Criteria

1. **Given** the existing ImageGen interface `Generate(ctx, prompt string, opts GenerateOptions) (*ImageResult, error)`
   **When** `GenerateOptions` is updated in `internal/plugin/imagegen/interface.go`
   **Then** `CharacterRef` struct is added with `Name string`, `VisualDescriptor string`, `ImagePromptBase string`
   **And** `GenerateOptions` gains a `CharacterRefs []CharacterRef` field
   **And** nil/empty `CharacterRefs` is equivalent to no character references (backward compatible)

2. **Given** the existing SiliconFlow implementation in `internal/plugin/imagegen/siliconflow.go`
   **When** `CharacterRefs` is present in `GenerateOptions`
   **Then** the implementation ignores `CharacterRefs` if nil or empty (existing behavior preserved)
   **And** all existing unit tests pass without modification (zero-value `CharacterRefs` is nil)

3. **Given** all call sites that use `imagegen.GenerateOptions{}`
   **When** the struct gains a new field
   **Then** zero-value initialization (`GenerateOptions{}`) keeps `CharacterRefs` as nil — no call site changes needed

## Tasks / Subtasks

- [x] Task 1: Add CharacterRef struct to interface.go (AC: #1)
  - [x] Define `CharacterRef` with Name, VisualDescriptor, ImagePromptBase fields
  - [x] Add `CharacterRefs []CharacterRef` field to existing `GenerateOptions` struct
- [x] Task 2: Verify SiliconFlow backward compatibility (AC: #2)
  - [x] Confirm SiliconFlow's Generate() does not reference CharacterRefs (it only uses Width, Height, Model, Style, Seed)
  - [x] Run existing tests — no changes needed since `GenerateOptions{}` keeps CharacterRefs nil
- [x] Task 3: Verify all call sites (AC: #3)
  - [x] Confirm call sites use `GenerateOptions{}` or field-named initialization — no breakage
  - [x] Run `make test && make lint` to confirm zero regressions

## Dev Notes

### Architecture Decision: Extend existing GenerateOptions vs new ImageGenOptions

The architecture doc envisions a separate `ImageGenOptions` type, but the current codebase already has `GenerateOptions` as part of the interface signature. **Extending `GenerateOptions`** is the correct approach because:
- No interface signature change required (`Generate(ctx, prompt, opts GenerateOptions)` stays the same)
- All 15+ call sites use `GenerateOptions{}` value initialization — new field defaults to nil
- SiliconFlow implementation doesn't need modification — it only reads Width/Height/Model/Style/Seed
- Adding a separate opts parameter would break the interface and require updating all implementations + callers

### Key Files to Modify

| File | Change |
|------|--------|
| `internal/plugin/imagegen/interface.go` | Add `CharacterRef` struct + `CharacterRefs` field to `GenerateOptions` |

### Files That Must NOT Change (backward compat verification)

| File | Reason |
|------|--------|
| `internal/plugin/imagegen/siliconflow.go` | Does not reference CharacterRefs — zero changes |
| `internal/plugin/imagegen/siliconflow_test.go` | Uses `GenerateOptions{}` — nil CharacterRefs by default |
| `internal/service/image_gen.go` | Passes opts through — no field access on CharacterRefs |
| `internal/service/image_gen_test.go` | Uses `imagegen.GenerateOptions{}` — nil CharacterRefs |
| `internal/pipeline/runner.go` | Stores `imagegen.GenerateOptions` — no CharacterRefs access |
| `internal/cli/stage_cmds.go` | Creates `imagegen.GenerateOptions{Width:..., Height:...}` — safe |
| `internal/cli/run_cmd.go` | Creates `imagegen.GenerateOptions{}` — safe |
| `tests/integration/pipeline_test.go` | Uses `imagegen.GenerateOptions{Width:..., Height:...}` — safe |

### Testing Standards

- Framework: `testify` (assert + require)
- Run: `make test` (go test ./...)
- Lint: `make lint` (go vet ./...)
- This story has ZERO new tests needed — only verify existing tests pass

### Project Structure Notes

- Module: `github.com/sushistack/yt.pipe`
- Go 1.25.7
- Plugin interface pattern: interface.go defines types, concrete providers implement
- `//go:generate mockery` directive on ImageGen interface — mock may need regeneration if interface changes, but since we're only adding a struct and a field (not changing the method signature), no mock regeneration needed

### References

- [Source: _bmad-output/planning-artifacts/architecture.md — Plugin Interface Extensions section]
- [Source: _bmad-output/planning-artifacts/epics.md — Story 14.1 AC]
- [Source: internal/plugin/imagegen/interface.go — current ImageGen interface]

## Dev Agent Record

### Agent Model Used
Claude Opus 4.6

### Completion Notes List
- Added `CharacterRef` struct with Name, VisualDescriptor, ImagePromptBase fields to interface.go
- Added `CharacterRefs []CharacterRef` field to existing `GenerateOptions` struct
- All existing tests pass (imagegen, service, pipeline, store packages)
- SiliconFlow implementation unchanged — backward compatible
- All call sites unaffected — `GenerateOptions{}` keeps CharacterRefs nil
- `make lint` passes clean

### File List
- `internal/plugin/imagegen/interface.go` (modified)
