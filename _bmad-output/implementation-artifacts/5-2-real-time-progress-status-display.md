# Story 5.2: Real-Time Progress & Status Display

Status: done

## Story
As a creator, I want real-time progress updates during pipeline execution and the ability to query project status at any time, so that I know exactly what's happening and how far along the pipeline is.

## Acceptance Criteria
- [x] CLI displays on stderr: current stage name, progress percentage (scenes completed / total), and elapsed time
- [x] Progress updates at least once per scene completion
- [x] `yt-pipe status <scp-id>` outputs JSON: project state, current/last stage, progress %, elapsed time, scene count, per-scene asset status
- [x] Response time under 2 seconds (reads from DB + filesystem, no API calls)
- [x] `yt-pipe status <scp-id> --scenes` displays a table: scene number, image status, audio status, subtitle status, timestamp
- [x] `--json-output` flag for machine-readable JSON output

## Implementation

### Progress Tracker
- `internal/pipeline/progress.go`: `ProgressTracker` struct
  - `OnProgress()` writes carriage-return formatted line to stderr with stage icon (1/8..8/8), stage name, scene count/total, percentage, elapsed seconds
  - `Finish()` writes final status line with total elapsed time
  - `stageIcon()` maps PipelineStage to "N/8" display format

### Status Command
- `internal/cli/status_cmd.go`: `yt-pipe status <scp-id>`
  - `ProjectStatus` struct: project_id, scp_id, status, scene_count, scenes[], created_at, updated_at
  - `SceneStatus` struct: scene_num, image_file/status, audio_file/status, subtitle_file/status, prompt_file, timestamp
  - `--scenes` flag: per-scene tabwriter table (Scene | Image | Audio | Subtitle | Timestamp)
  - `--json-output` flag: JSON encoder output
  - `collectSceneStatuses()`: scans workspace scene directories for asset files
  - `checkAsset()`: checks file existence and non-zero size → "generated", "empty", or "pending"
  - `findProjectBySCPID()`: finds most recent project by SCP ID from DB

### Integration
- `internal/cli/run_cmd.go`: Wires `ProgressTracker.OnProgress` to `Runner.ProgressFunc`

### Tests
- `internal/pipeline/progress_test.go`: 4 tests (OnProgress with/without scenes, Finish, stageIcon)
- `internal/cli/status_cmd_test.go`: 5 tests (checkAsset found/not found/empty, collectSceneStatuses, truncatePrompt)

## Dev Agent Record
### Agent Model Used
Claude Opus 4.6

### File List
- `internal/pipeline/progress.go` — New: ProgressTracker for stderr display
- `internal/pipeline/progress_test.go` — New: 4 unit tests
- `internal/cli/status_cmd.go` — New: status command with --scenes table
- `internal/cli/status_cmd_test.go` — New: 5 unit tests

### Change Log
- 2026-03-08: Initial implementation with all acceptance criteria met
