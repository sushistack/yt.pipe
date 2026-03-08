# Story 5.1: Full Pipeline Orchestration

Status: done

## Story
As a creator, I want to run the entire pipeline from SCP data to CapCut project with a single command, so that I can produce a complete video project with minimal manual steps.

## Acceptance Criteria
- [x] `yt-pipe run <scp-id>` executes all stages in sequence: data_load → scenario_generate → (pause for approval) → image_generate + tts_synthesize (parallel) → timing_resolve → subtitle_generate → assemble
- [x] Each stage's start/end is logged with slog
- [x] Pipeline pauses at scenario approval stage, prompts creator to review and approve
- [x] `yt-pipe scenario approve <project-id>` approves scenario
- [x] `yt-pipe run <scp-id> --resume <project-id>` continues from approved state
- [x] Image generation and TTS synthesis run in parallel using goroutines
- [x] Subtitle generation waits for TTS completion (depends on timing data)
- [x] Assembly waits for all assets to complete
- [x] Individual stage commands: `yt-pipe scenario generate`, `yt-pipe image generate`, `yt-pipe tts generate`, `yt-pipe subtitle generate`, `yt-pipe assemble`
- [x] Each individual command validates required project state before execution

## Implementation

### Pipeline Runner
- `internal/pipeline/runner.go`: Core `Runner` struct with `Run()` and `Resume()` methods
  - `Run()` executes stages 1-3 (data_load → scenario_generate → pause for approval)
  - `Resume()` executes stages 4-8 (parallel gen → timing → subtitle → assemble)
  - `RunStage()` executes a single named stage for individual CLI commands
  - `runParallelGeneration()` uses `sync.WaitGroup` + goroutines for concurrent image/TTS generation
  - `mergeSceneData()` combines image and TTS scene results with timing data
  - `findProject()` looks up the most recent project by SCP ID
  - `ProgressFunc` callback for real-time feedback integration

### CLI Commands
- `internal/cli/run_cmd.go`: Updated `yt-pipe run` with real pipeline execution
  - `--resume <project-id>` flag for post-approval resumption
  - `--dry-run` preserved for configuration verification
  - Human-readable and JSON output formats for run results
- `internal/cli/stage_cmds.go`: Individual stage commands
  - `yt-pipe scenario generate <scp-id>` — Generate scenario from SCP data
  - `yt-pipe scenario approve <project-id>` — Approve generated scenario
  - `yt-pipe image generate <scp-id>` — Generate images for all scenes
  - `yt-pipe tts generate <scp-id>` — Synthesize TTS narration for all scenes
  - `yt-pipe subtitle generate <scp-id>` — Generate subtitles for all scenes
  - `buildRunner()` shared helper creates a fully-configured pipeline Runner
- `internal/cli/plugins.go`: Plugin creation via registry factory pattern
  - `pluginRegistry` global registry, `createPlugins()` instantiates LLM/ImageGen/TTS plugins

### Tests
- `internal/pipeline/runner_test.go`: 9 tests
  - stageResult pass/fail, mergeSceneData, toDomainScenes, parseSceneManifest, pipelineError, progressFunc
- `internal/cli/run_cmd_test.go`: Updated TestRunCmd_NoDryRun to expect plugin error (was placeholder message)

## Dev Agent Record
### Agent Model Used
Claude Opus 4.6

### File List
- `internal/pipeline/runner.go` — New: Pipeline Runner (Run, Resume, RunStage, parallel generation)
- `internal/pipeline/runner_test.go` — New: 9 unit tests
- `internal/cli/run_cmd.go` — Updated: Real pipeline execution, --resume flag, progress tracking
- `internal/cli/run_cmd_test.go` — Updated: Test for plugin error on non-dry-run
- `internal/cli/stage_cmds.go` — New: Individual stage CLI commands
- `internal/cli/plugins.go` — New: Plugin registry and creation

### Change Log
- 2026-03-08: Initial implementation with all acceptance criteria met
