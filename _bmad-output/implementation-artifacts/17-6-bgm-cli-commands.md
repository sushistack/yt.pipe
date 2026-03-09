# Story 17.6: BGM CLI Commands

Status: done

## Story

As a content creator,
I want CLI commands to manage the BGM library and review/confirm auto-recommended BGM assignments,
So that I can add BGMs, browse the library, and approve or override LLM recommendations from the terminal.

## Acceptance Criteria

1. **Given** the CLI
   **When** `yt-pipe bgm add --name "Track1" --file /path/to/audio.mp3 --moods "tense,dark" --license-type cc_by --credit "Artist Name"` is run
   **Then** a new BGM is registered and its ID is printed

2. **Given** BGMs exist in the library
   **When** `yt-pipe bgm list` is run
   **Then** a table is printed with ID, NAME, MOOD_TAGS, LICENSE, DURATION columns
   **And** `--mood <tag>` flag filters by mood tag

3. **Given** a BGM exists
   **When** `yt-pipe bgm show <bgm-id>` is run
   **Then** all BGM details are printed (ID, Name, File, Mood Tags, Duration, License Type/Source, Credit Text)

4. **Given** a BGM exists
   **When** `yt-pipe bgm update <bgm-id> --name "New Name" --moods "calm"` is run
   **Then** only the specified fields are updated

5. **Given** a BGM exists without scene assignments
   **When** `yt-pipe bgm delete <bgm-id>` is run
   **Then** the BGM is removed and confirmation is printed

6. **Given** a project with BGM assignments
   **When** `yt-pipe bgm review <project-id>` is run
   **Then** a table is printed with SCENE, BGM, VOLUME, FADE_IN, FADE_OUT, DUCKING, STATUS columns
   **And** `--confirm-all` confirms all pending recommendations
   **And** `--confirm <scene-num>` confirms a specific scene
   **And** `--reassign <scene-num> --bgm <bgm-id>` replaces the BGM for a scene
   **And** `--adjust <scene-num> --volume -6 --fade-in 1000 --fade-out 1500 --ducking -10` adjusts parameters

## Tasks / Subtasks

- [x] Task 1: Create `bgm` parent command (AC: all)
  - [x] 1.1: Define `bgmCmd` with `Use: "bgm"` and `Short: "Manage BGM preset library"`
  - [x] 1.2: Register on `rootCmd`
- [x] Task 2: Implement `bgm add` subcommand (AC: #1)
  - [x] 2.1: Flags: `--name`, `--file`, `--moods`, `--license-type`, `--credit`, `--source`, `--duration`
  - [x] 2.2: Parse comma-separated moods
  - [x] 2.3: Call `svc.CreateBGM()` and print result
- [x] Task 3: Implement `bgm list` subcommand (AC: #2)
  - [x] 3.1: Flag: `--mood` for tag filter
  - [x] 3.2: Use `tabwriter` for formatted output
  - [x] 3.3: Route to `SearchByMoodTags` or `ListBGMs` based on `--mood` flag
- [x] Task 4: Implement `bgm show` subcommand (AC: #3)
  - [x] 4.1: `cobra.ExactArgs(1)` for BGM ID
  - [x] 4.2: Print all fields with aligned formatting
- [x] Task 5: Implement `bgm update` subcommand (AC: #4)
  - [x] 5.1: Flags: `--name`, `--moods`, `--license-type`, `--credit`
  - [x] 5.2: Call `svc.UpdateBGM()` with merge semantics
- [x] Task 6: Implement `bgm delete` subcommand (AC: #5)
  - [x] 6.1: `cobra.ExactArgs(1)` for BGM ID
  - [x] 6.2: Call `svc.DeleteBGM()` and print confirmation
- [x] Task 7: Implement `bgm review` subcommand (AC: #6)
  - [x] 7.1: Flags: `--confirm-all`, `--confirm`, `--reassign`, `--bgm`, `--adjust`, `--volume`, `--fade-in`, `--fade-out`, `--ducking`
  - [x] 7.2: `--confirm-all` — iterate pending and confirm each
  - [x] 7.3: `--confirm <scene>` — confirm specific scene
  - [x] 7.4: `--reassign <scene> --bgm <id>` — reassign with validation
  - [x] 7.5: `--adjust <scene>` — update volume/fade/ducking params
  - [x] 7.6: Default (no action flags) — display assignment table with status
- [x] Task 8: Add `formatDuration` helper
  - [x] 8.1: Convert milliseconds to `m:ss` format
- [x] Task 9: Run full test suite
  - [x] 9.1: `make test` passes
  - [x] 9.2: `make lint` passes

## Dev Notes

### Command Structure

```
yt-pipe bgm
  ├── add      — Register a new BGM file
  ├── list     — List all BGMs (with optional --mood filter)
  ├── show     — Show BGM details
  ├── update   — Update a BGM
  ├── delete   — Delete a BGM
  └── review   — Review pending BGM recommendations
```

### openBGMService Helper

```go
func openBGMService(cmd *cobra.Command) (*service.BGMService, *store.Store, func(), error) {
    // Opens DB, creates BGMService with nil LLM (not needed for CLI ops)
    // Returns service, store (for direct queries), cleanup function, error
}
```

LLM is passed as `nil` because CLI operations don't invoke auto-recommendation — that happens during pipeline execution.

### Review Command Design

The `review` subcommand is multi-modal based on flags:
1. **No flags** → display table of all assignments with status
2. **`--confirm-all`** → batch confirm all pending
3. **`--confirm N`** → confirm scene N
4. **`--reassign N --bgm <id>`** → replace BGM for scene N
5. **`--adjust N`** → update audio parameters for scene N

This avoids multiple subcommands for what is essentially a single review workflow.

### Duration Formatting

```go
func formatDuration(ms int64) string {
    d := time.Duration(ms) * time.Millisecond
    m := int(d.Minutes())
    s := int(d.Seconds()) % 60
    return fmt.Sprintf("%d:%02d", m, s)
}
```

### Files Touched

| File | Change |
|------|--------|
| `internal/cli/bgm_cmd.go` | New — all BGM CLI commands (add, list, show, update, delete, review) |

### Pattern Reference

Follows the same CLI pattern as Epic 14 (`character_cmd.go`), Epic 15 (`mood_cmd.go`), and Epic 13 (`template_cmd.go`):
- Parent command with subcommands
- `openXxxService` helper for DB/service setup
- `tabwriter` for list output
- `cobra.ExactArgs` for ID parameters
- `cmd.OutOrStdout()` for testable output

### References

- [Source: internal/cli/bgm_cmd.go] — full implementation
- [Pattern: internal/cli/character_cmd.go] — Epic 14 CLI pattern
- [Pattern: internal/cli/mood_cmd.go] — Epic 15 CLI pattern

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6

### Completion Notes List

- Implemented 6 subcommands: add, list, show, update, delete, review
- `bgm add` with --name, --file, --moods, --license-type, --credit, --source, --duration flags
- `bgm list` with optional --mood filter routing to SearchByMoodTags
- `bgm show` with aligned key-value output
- `bgm update` with partial field updates (merge semantics)
- `bgm delete` with store-level delete protection
- `bgm review` with multi-modal flag handling (confirm-all, confirm, reassign, adjust, default table display)
- `openBGMService` helper with nil LLM (CLI doesn't need auto-recommendation)
- `formatDuration` helper for millisecond-to-mm:ss formatting
- All tests pass, builds clean

### File List

- `internal/cli/bgm_cmd.go` (new) — BGM CLI commands (add, list, show, update, delete, review)
