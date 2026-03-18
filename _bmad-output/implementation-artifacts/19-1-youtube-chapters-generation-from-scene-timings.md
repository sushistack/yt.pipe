# Story 19.1: YouTube Chapters Generation from Scene Timings

Status: done

## Story

As a content creator,
I want YouTube chapter timestamps auto-generated from scene timing data,
so that I can paste them into video descriptions without manual timestamp calculation.

## Acceptance Criteria

1. **Given** a project with resolved timings (`timeline.json` exists)
   **When** `yt-pipe chapters <scp-id>` is executed
   **Then** a `chapters.txt` file is written to the project output directory
   **And** the first line is `0:00 Intro`
   **And** each subsequent scene maps to `M:SS Title` format (or `H:MM:SS` if >= 1 hour)
   **And** titles are derived from `Scene.Mood` + `Scene.VisualDesc` (first 30 chars)
   **And** this satisfies EFR1

2. **Given** a project with only 1 scene
   **When** chapters are generated
   **Then** only `0:00 Intro` is output (single-scene edge case)

3. **Given** a project without resolved timings
   **When** `yt-pipe chapters <scp-id>` is executed
   **Then** a clear error message is displayed indicating timings must be resolved first

## Tasks / Subtasks

- [x] Task 1: Add `GenerateChapters()` and `SaveChaptersFile()` to `TimingResolver` (AC: 1, 2)
  - [x] 1.1: Implement `ChapterEntry` struct
  - [x] 1.2: Implement `GenerateChapters(timeline Timeline, scenes []domain.SceneScript) []ChapterEntry`
  - [x] 1.3: Implement `FormatChapters(chapters []ChapterEntry) string`
  - [x] 1.4: Implement `SaveChaptersFile(chapters string, projectPath string) error`
- [x] Task 2: Add `chapters` CLI command (AC: 1, 3)
  - [x] 2.1: Create `internal/cli/chapters_cmd.go`
  - [x] 2.2: Register with `rootCmd`
  - [x] 2.3: Load timeline.json, validate existence, call service, write output
- [x] Task 3: Unit tests (AC: 1, 2, 3)
  - [x] 3.1: Test multi-scene chapters format (M:SS)
  - [x] 3.2: Test single-scene edge case
  - [x] 3.3: Test H:MM:SS for >= 1 hour
  - [x] 3.4: Test title truncation (30 chars)
  - [x] 3.5: Test file output path and content

## Dev Notes

### Architecture & Constraints (Source: architecture.md — EFR1 section)

- **Location**: Add to `internal/service/timing.go` — existing `TimingResolver` service
- **Scope**: ~30 lines of logic. No separate file needed for service code
- **No new dependencies**: Uses only existing `Timeline`, `SceneTiming`, `domain.Scene`
- **No DB migration**: Pure compute + file output
- **No config changes**: Uses existing workspace paths

### Key Types (Source: internal/service/timing.go)

```go
// Already exists — DO NOT modify
type TimingResolver struct {
    logger               *slog.Logger
    defaultSceneDuration float64
}

type Timeline struct {
    TotalDurationSec float64       `json:"total_duration_sec"`
    SceneCount       int           `json:"scene_count"`
    Scenes           []SceneTiming `json:"scenes"`
}

type SceneTiming struct {
    SceneNum    int     `json:"scene_num"`
    StartSec    float64 `json:"start_sec"`
    EndSec      float64 `json:"end_sec"`
    DurationSec float64 `json:"duration_sec"`
    // ... other fields not needed for chapters
}
```

```go
// domain.Scene — already exists at internal/domain/scene.go
type Scene struct {
    SceneNum   int
    Mood       string
    VisualDesc string
    // ... other fields
}
```

### Chapter Title Generation Rules

- Scene 1 (first): Always `"Intro"` (hardcoded)
- Scene N (subsequent): `"{Mood} - {VisualDesc[:30]}"` — truncate VisualDesc to 30 chars
- If Mood is empty, use VisualDesc only
- If VisualDesc is empty, use `"Scene {N}"`

### Timestamp Format Rules

- `0:00` — always for first entry
- `M:SS` — for timestamps < 1 hour (e.g., `1:23`, `12:05`)
- `H:MM:SS` — for timestamps >= 1 hour (e.g., `1:02:30`)
- Use `int(startSec)` — no fractional seconds

### File Output

- Output path: `{projectPath}/output/chapters.txt`
- Use `workspace.WriteFileAtomic()` for safe writes
- Ensure `output/` dir exists (use `os.MkdirAll`)
- Format: plain text, one chapter per line, newline-terminated

### CLI Command Pattern (Source: internal/cli/assemble_cmd.go)

```go
// Follow this exact pattern for chapters_cmd.go
var chaptersCmd = &cobra.Command{
    Use:   "chapters <scp-id>",
    Short: "Generate YouTube chapter timestamps from scene timings",
    Args:  cobra.ExactArgs(1),
    RunE:  runChapters,
}

func init() {
    rootCmd.AddCommand(chaptersCmd)
}
```

CLI flow:
1. `cfg := GetConfig()` — load config
2. `cmd.SilenceUsage = true`
3. Resolve project path from scp-id via workspace
4. Check `{projectPath}/timeline.json` exists — if not, return error
5. Load timeline.json + scene data (scenes needed for Mood/VisualDesc)
6. Call `TimingResolver.GenerateChapters()` → `FormatChapters()` → `SaveChaptersFile()`
7. Print success message to stdout

### Error Handling

- Missing timeline.json: Return `domain.DependencyError{Action: "generate chapters", Missing: []string{"timeline.json — run pipeline first"}}`
- File write failure: Wrap with `fmt.Errorf("chapters: save: %w", err)`

### Project Structure Notes

- `internal/service/timing.go` — add GenerateChapters, FormatChapters, SaveChaptersFile methods
- `internal/service/timing_test.go` — add test cases
- `internal/cli/chapters_cmd.go` — new file for CLI command
- No other files need modification

### Testing Standards

- Same package `service` test (not `service_test`)
- Use `testify/assert` and `testify/require`
- Use `t.TempDir()` for file system tests
- Test data inline — no external test fixtures needed

### References

- [Source: internal/service/timing.go] — TimingResolver, Timeline, SceneTiming types
- [Source: internal/domain/scene.go] — Scene with Mood, VisualDesc
- [Source: internal/domain/errors.go] — DependencyError pattern
- [Source: internal/cli/assemble_cmd.go] — CLI command pattern
- [Source: internal/workspace/project.go] — WriteFileAtomic, InitSceneDir
- [Source: architecture.md — EFR1 section] — Architecture design decisions
- [Source: epics.md — Epic 19, Story 19.1] — Full acceptance criteria

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Completion Notes List

- Implemented ChapterEntry struct, GenerateChapters(), FormatChapters(), SaveChaptersFile() on TimingResolver
- Used domain.SceneScript (from scenario.json) instead of domain.Scene for Mood/VisualDescription access
- Created chapters CLI command following existing assemble_cmd.go pattern
- Fixed pre-existing mock build errors by adding CompleteWithVision to MockLLM and mockLLMForBGM
- 8 test functions covering all ACs: multi-scene, single-scene, H:MM:SS, title truncation, empty mood/desc, file output, timestamp formatting
- All tests pass, no regressions, lint clean

### Change Log

- 2026-03-18: Implemented Story 19.1 — YouTube chapters generation from scene timings

### File List

- internal/service/timing.go (modified — added ChapterEntry, GenerateChapters, FormatChapters, SaveChaptersFile, helper functions)
- internal/service/timing_test.go (modified — added 8 chapter test functions)
- internal/cli/chapters_cmd.go (new — CLI command for `yt-pipe chapters <scp-id>`)
- internal/mocks/mock_LLM.go (modified — added CompleteWithVision mock method)
- internal/service/bgm_test.go (modified — added CompleteWithVision to mockLLMForBGM)
