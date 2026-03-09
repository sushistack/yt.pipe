# Story 17.1: OutputAssembler Plugin Interface Extension

Status: done

## Story

As a developer,
I want the OutputAssembler plugin interface extended with BGMAssignment and CreditEntry types in AssembleInput,
So that output assembly plugins can receive BGM placement and licensing information.

## Acceptance Criteria

1. **Given** the existing `output.Assembler` interface in `internal/plugin/output/interface.go`
   **When** the `AssembleInput` struct is extended
   **Then** new fields are added: `BGMAssignments []BGMAssignment` and `Credits []CreditEntry`
   **And** `BGMAssignment` struct: `SceneNum int`, `FilePath string`, `VolumeDB float64`, `FadeInMs int`, `FadeOutMs int`, `DuckingDB float64`
   **And** `CreditEntry` struct: `Type string` (e.g. "bgm", "cc-by-sa"), `Text string`
   **And** zero-value (nil/empty slice) means no BGM and no additional credits (backward compatible)

2. **Given** the existing CapCut implementation in `internal/plugin/output/capcut/capcut.go`
   **When** it receives `AssembleInput` with empty `BGMAssignments` and `Credits`
   **Then** existing behavior is unchanged — no BGM tracks, no extra credits
   **And** all existing unit tests pass without modification

3. **Given** the existing mock in `internal/mocks/mock_assembler.go`
   **When** it is updated
   **Then** it still implements `output.Assembler` interface correctly

4. **Given** all call sites of `Assemble()`
   **When** they pass `AssembleInput{}` with zero-value new fields
   **Then** no call site changes are required for backward compatibility

## Tasks / Subtasks

- [x] Task 1: Extend `AssembleInput` in interface.go (AC: #1)
  - [x] 1.1: Add `BGMAssignment` struct definition
  - [x] 1.2: Add `CreditEntry` struct definition
  - [x] 1.3: Add `BGMAssignments []BGMAssignment` field to `AssembleInput`
  - [x] 1.4: Add `Credits []CreditEntry` field to `AssembleInput`
- [x] Task 2: Verify CapCut assembler compiles without changes (AC: #2)
  - [x] 2.1: Confirm `capcut.go` compiles — it uses `AssembleInput` fields it needs, new zero-value fields are ignored
  - [x] 2.2: Run existing tests to confirm green
- [x] Task 3: Update mock if needed (AC: #3)
  - [x] 3.1: Verify `mock_assembler.go` still compiles (signature unchanged, only struct extended)
- [x] Task 4: Run full test suite (AC: #4)
  - [x] 4.1: `make test` passes
  - [x] 4.2: `make lint` passes

## Dev Notes

### Critical Architecture Decision: Extend Struct, NOT Signature

The architecture document shows a different signature (`Assemble(ctx, *Project, *AssembleOptions)`), but the **actual codebase** uses:

```go
// internal/plugin/output/interface.go — CURRENT
type Assembler interface {
    Assemble(ctx context.Context, input AssembleInput) (*AssembleResult, error)
    Validate(ctx context.Context, outputPath string) error
}
```

Follow the **Epic 14 pattern** (ImageGen interface extension): add new fields to the existing `AssembleInput` struct rather than changing the method signature. This ensures:
- Zero call-site changes
- Zero-value fields (nil slices) mean "no BGM"
- All existing tests pass without modification

### Exact Changes Required

**File: `internal/plugin/output/interface.go`**

Add these types and extend `AssembleInput`:

```go
// BGMAssignment represents a BGM track placement for a specific scene.
type BGMAssignment struct {
    SceneNum  int
    FilePath  string
    VolumeDB  float64 // base volume relative to 0dB
    FadeInMs  int     // fade-in duration at segment start
    FadeOutMs int     // fade-out duration at segment end
    DuckingDB float64 // volume reduction during narration
}

// CreditEntry represents a single credit line (BGM, CC-BY-SA, etc.)
type CreditEntry struct {
    Type string // e.g. "bgm", "cc-by-sa"
    Text string
}

// In AssembleInput, add:
// BGMAssignments []BGMAssignment // BGM tracks to place; nil/empty = no BGM
// Credits        []CreditEntry   // additional credits; nil/empty = no extra credits
```

### Files Touched

| File | Change |
|------|--------|
| `internal/plugin/output/interface.go` | Add BGMAssignment, CreditEntry structs + extend AssembleInput |

### Files NOT Touched (backward compatible)

| File | Reason |
|------|--------|
| `internal/plugin/output/capcut/capcut.go` | Uses `input.Scenes`, `input.OutputDir` etc. — new zero-value fields ignored |
| `internal/plugin/output/capcut/types.go` | No changes needed yet (BGM track types added in Story 17.5) |
| `internal/service/assembler.go` | Builds AssembleInput — new fields default to nil |
| `internal/mocks/mock_assembler.go` | Interface signature unchanged |
| `internal/service/assembler_test.go` | Tests pass AssembleInput — zero-value new fields are fine |

### Pattern Reference: Epic 14 (ImageGen Extension)

Epic 14 added `CharacterRefs []CharacterRef` to `GenerateOptions` struct. Same pattern:
- New struct types defined alongside the interface
- Field added to existing options struct
- Zero-value = backward compatible
- No signature change, no call-site changes

### Project Structure Notes

- All plugin interfaces live in `internal/plugin/{type}/interface.go`
- Domain types (BGM model, SceneBGMAssignment) belong in `domain/bgm.go` — that's Story 17.2, NOT this story
- This story only extends the plugin interface layer

### Testing Standards

- Existing tests must pass with no changes (`make test`)
- No new tests needed — this is a pure type addition with no logic change
- `make lint` (`go vet ./...`) must pass

### References

- [Source: internal/plugin/output/interface.go] — current Assembler interface
- [Source: internal/plugin/imagegen/interface.go] — Epic 14 extension pattern
- [Source: _bmad-output/planning-artifacts/architecture.md#Plugin Interface Changes] — architecture spec
- [Source: _bmad-output/planning-artifacts/epics.md#Story 17.1] — acceptance criteria

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6

### Debug Log References

None — clean implementation with no issues.

### Completion Notes List

- Added `BGMAssignment` struct (SceneNum, FilePath, VolumeDB, FadeInMs, FadeOutMs, DuckingDB) to `internal/plugin/output/interface.go`
- Added `CreditEntry` struct (Type, Text) to `internal/plugin/output/interface.go`
- Extended `AssembleInput` with `BGMAssignments []BGMAssignment` and `Credits []CreditEntry` fields
- Followed Epic 14 pattern: struct extension, no signature change, full backward compatibility
- All existing tests pass (20 packages), no call-site changes needed
- `go build ./...` compiles cleanly, `go vet ./...` passes

### File List

- `internal/plugin/output/interface.go` (modified) — added BGMAssignment, CreditEntry structs + extended AssembleInput
