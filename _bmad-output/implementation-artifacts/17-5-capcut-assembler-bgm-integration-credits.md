# Story 17.5: CapCut Assembler Integration — BGM Placement & Credits

Status: done

## Story

As a developer,
I want BGM tracks integrated into the CapCut project output with per-scene volume, fade, and ducking controls, and license credits auto-appended,
So that assembled videos include background music with proper attribution.

## Acceptance Criteria

1. **Given** `AssembleInput` with non-empty `BGMAssignments`
   **When** `Assemble()` is called
   **Then** `buildDraftProject` receives BGM assignments and creates a separate "bgm" audio track
   **And** the BGM track is appended after video, audio, and text tracks

2. **Given** a BGM assignment for scene N
   **When** the BGM track is built
   **Then** a `AudioMaterial` is created with the BGM file path
   **And** a `Segment` is placed at the correct timeline position matching scene N
   **And** volume is converted from dB to linear using `dbToLinear()`

3. **Given** `AssembleInput` with empty `BGMAssignments`
   **When** `Assemble()` is called
   **Then** no BGM track is added (backward compatible)
   **And** existing behavior is unchanged

4. **Given** the `dbToLinear` function in `types.go`
   **When** called with various dB values
   **Then** 0 dB returns 1.0, -6 dB returns ~0.5, -20 dB returns 0.1

5. **Given** the assembler service
   **When** assembly runs with a BGM service configured
   **Then** `WithBGMService()` injects the BGM service
   **And** `AppendBGMCredits()` merges BGM credit entries into the output

## Tasks / Subtasks

- [x] Task 1: Modify `buildDraftProject` to accept BGM assignments (AC: #1, #3)
  - [x] 1.1: Add `bgmAssignments []output.BGMAssignment` parameter
  - [x] 1.2: Call `buildBGMTrack` when assignments are non-empty
  - [x] 1.3: Append BGM track after video/audio/text tracks
- [x] Task 2: Implement `buildBGMTrack` (AC: #2)
  - [x] 2.1: Build scene timeline position map from scenes
  - [x] 2.2: Create `AudioMaterial` per BGM assignment
  - [x] 2.3: Create `Segment` aligned to scene timeline position
  - [x] 2.4: Apply `dbToLinear` volume conversion
  - [x] 2.5: Set track Name="bgm", IsDefaultName=false
- [x] Task 3: Add `dbToLinear` to types.go (AC: #4)
  - [x] 3.1: Implement `math.Pow(10, db/20)` conversion
- [x] Task 4: Integrate in assembler service (AC: #5)
  - [x] 4.1: `WithBGMService()` optional setter on `AssemblerService`
  - [x] 4.2: `AppendBGMCredits()` in `assembler.go` — merge credits into assembly
- [x] Task 5: Run full test suite
  - [x] 5.1: `make test` passes
  - [x] 5.2: `make lint` passes

## Dev Notes

### buildBGMTrack Architecture

The BGM track is a **separate audio track** from the narration audio track. This keeps BGM independent for editing in CapCut:

```go
func buildBGMTrack(scenes []domain.Scene, assignments []output.BGMAssignment, materials *Materials) Track {
    bgmTrack := Track{
        Type: "audio",
        Name: "bgm",
        IsDefaultName: false,
    }
    // Build scene position map, then for each assignment:
    // 1. Create AudioMaterial with BGM file path
    // 2. Create Segment at scene's timeline position
    // 3. Apply dbToLinear volume conversion
    return bgmTrack
}
```

### dB to Linear Conversion

```go
// dbToLinear converts decibels to linear volume (0 dB = 1.0, -6 dB ≈ 0.5).
func dbToLinear(db float64) float64 {
    return math.Pow(10, db/20)
}
```

| dB | Linear |
|----|--------|
| 0 | 1.0 |
| -6 | ~0.501 |
| -12 | ~0.251 |
| -20 | 0.1 |

### AssemblerService BGM Integration

```go
// Optional setter — BGM is opt-in
func (s *AssemblerService) WithBGMService(bgmSvc *BGMService) {
    s.bgmSvc = bgmSvc
}
```

The assembler service builds `AssembleInput.BGMAssignments` from confirmed scene assignments and appends BGM credits via `AppendBGMCredits()`.

### Files Touched

| File | Change |
|------|--------|
| `internal/plugin/output/capcut/capcut.go` | Modified — `buildDraftProject` accepts BGM assignments; new `buildBGMTrack` function |
| `internal/plugin/output/capcut/types.go` | Modified — added `dbToLinear` volume conversion |
| `internal/service/assembler.go` | Modified — `WithBGMService()` setter, BGM assignment building, `AppendBGMCredits()` |

### Design Decision: Separate Track vs Inline

BGM is placed on a **separate named track** (`Name: "bgm"`) rather than mixing segments into the existing narration audio track. This ensures:
- Independent volume/mute control in CapCut editor
- Clear visual separation on timeline
- No interference with narration audio segments

### References

- [Source: internal/plugin/output/capcut/capcut.go] — buildDraftProject, buildBGMTrack
- [Source: internal/plugin/output/capcut/types.go] — dbToLinear
- [Source: internal/service/assembler.go] — WithBGMService, AppendBGMCredits

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6

### Completion Notes List

- Modified `buildDraftProject` to accept `bgmAssignments` parameter (zero-value = no BGM track)
- Implemented `buildBGMTrack` with scene timeline position mapping and per-scene BGM segments
- Added `dbToLinear` function in types.go for dB-to-linear volume conversion
- Added `WithBGMService()` optional setter to `AssemblerService`
- Integrated BGM credits into assembly output via `AppendBGMCredits()`
- BGM track is separate from narration audio for independent editing
- All existing tests pass, backward compatible with empty BGMAssignments

### File List

- `internal/plugin/output/capcut/capcut.go` (modified) — buildDraftProject with BGM param, buildBGMTrack function
- `internal/plugin/output/capcut/types.go` (modified) — dbToLinear volume conversion
- `internal/service/assembler.go` (modified) — WithBGMService, BGM assignment building, AppendBGMCredits
