# Story 12.3: Multi-Stage Pipeline Progress Dashboard

Status: done

## Story

As a creator,
I want real-time progress visibility across multiple parallel stages when running the full pipeline,
So that I can monitor long-running batch jobs and estimate remaining time.

## Acceptance Criteria

1. CLI displays live multi-line progress view during pipeline execution showing all stages
2. Parallel stages (image + TTS) show simultaneous progress bars
3. Display updates in-place using ANSI escape codes when stderr is a TTY
4. Graceful degradation to simple line-by-line output when stderr is not a TTY
5. `yt-pipe status <SCP-ID>` shows current running stages, per-stage progress, elapsed time, and estimated remaining time

## Tasks / Subtasks

- [ ] Task 1: Enhance ProgressTracker for multi-stage display (AC: #1, #2, #3)
  - [ ] Add multi-stage state tracking (map of stage → progress)
  - [ ] Render multi-line display with ANSI escape codes
  - [ ] Update individual stage progress independently
- [ ] Task 2: Add TTY detection and graceful degradation (AC: #4)
  - [ ] Detect if stderr is a TTY
  - [ ] Fall back to simple line-by-line output for non-TTY
- [ ] Task 3: Enhance per-scene progress callbacks (AC: #1)
  - [ ] Add scene-level progress callbacks to image generation
  - [ ] Add scene-level progress callbacks to TTS synthesis
  - [ ] Thread progress through parallel generation
- [ ] Task 4: Enhance status command for live run info (AC: #5)
  - [ ] Write progress state to workspace file during run
  - [ ] Read and display in status command

## Dev Notes

- `internal/pipeline/progress.go` - Current ProgressTracker (single-line, simple)
- The runner calls reportProgress() at stage transitions only, not per-scene
- Need to add per-scene callbacks: modify image_gen.go and tts.go to report scene-level progress
- ANSI: `\033[F` moves cursor up one line, `\033[2K` clears line
- TTY detection: `os.Stderr.Fd()` + `golang.org/x/term` or `isatty` check
- Progress file: write `{project}/progress.json` with current state, read by `yt-pipe status`

### References

- [Source: internal/pipeline/progress.go - ProgressTracker]
- [Source: internal/pipeline/runner.go - reportProgress(), runParallelGeneration()]
- [Source: internal/cli/status_cmd.go - runStatusCmd()]
- [Source: internal/service/image_gen.go - GenerateAllImages()]
- [Source: internal/service/tts.go - SynthesizeAll()]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6

### Completion Notes List

- Rewrote ProgressTracker from single-line to multi-stage concurrent display
- Added `stageState` struct tracking per-stage progress (ScenesTotal, ScenesComplete, Status, StartedAt, CompletedAt)
- TTY detection via `os.ModeCharDevice` (no external dependency)
- Multi-line ANSI rendering: `\033[%dF` cursor up, `\033[2K` clear line, Unicode progress bars (█░)
- Graceful degradation to simple line-by-line output for non-TTY environments
- `writeProgressFile()` writes progress.json for `yt-pipe status` consumption
- Enhanced `status_cmd.go` with `loadLiveProgress()` and live progress display
- Updated progress_test.go with tests for multi-stage, MarkStageDone, progressBar, stageName, isTerminal

### File List

- internal/pipeline/progress.go
- internal/pipeline/progress_test.go
- internal/cli/status_cmd.go
