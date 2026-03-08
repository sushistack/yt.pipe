# Story 5.5: Checkpoint, Resume & Error Recovery

Status: done

## Story
As a creator, I want the pipeline to preserve progress on failure and provide clear recovery instructions, so that I never lose completed work and can quickly fix and resume.

## Acceptance Criteria
- [x] Checkpoint recorded on stage completion: completed stage, scene-level progress, timestamp
- [x] All generated artifacts persisted to disk via atomic writes
- [x] Failed pipeline preserves previously completed scene artifacts on disk
- [x] Error includes: failed stage name, scene number, error cause, specific CLI recovery command
- [x] Error logged as structured JSON with all fields
- [x] Recovery command resumes from failed point, not from beginning
- [x] Previously completed scenes not re-processed on resume
- [x] Abnormal termination (kill signal) does not corrupt existing project data
- [x] Project can resume from last checkpoint after restart

## Implementation

### Checkpoint Manager
- `internal/pipeline/checkpoint.go`:
  - `CheckpointManager` struct: save/load pipeline checkpoints
  - `SaveStageCheckpoint()`: records completed stage with scenesDone count, persists to `checkpoint.json` via atomic write
  - `LoadCheckpoint()`: loads checkpoint from project workspace, returns nil if not found
  - `GetResumeStage()`: determines next stage after last completed (data_load→scenario_generate→...→assemble)
  - `ShouldSkipStage()`: checks if a stage already completed per checkpoint
  - `BuildRecoveryCommand()`: generates stage-specific CLI recovery commands:
    - data_load/scenario_generate → `yt-pipe run <scp-id>`
    - image_generate → `yt-pipe image generate <scp-id> [--scene N]`
    - tts_synthesize → `yt-pipe tts generate <scp-id> [--scene N]`
    - subtitle_generate → `yt-pipe subtitle generate <scp-id>`
    - assemble → `yt-pipe assemble <scp-id>`
  - `CheckProjectIntegrity()`: verifies essential files are non-empty after abnormal termination

### Existing Infrastructure Used
- `internal/service/pipeline_orchestrator.go`:
  - `PipelineCheckpoint`: checkpoint data structure with stages array
  - `SaveCheckpoint()`/`LoadCheckpoint()`: atomic JSON persistence
  - `RecordStage()`: appends stage to checkpoint
  - `HasCompletedStage()`: checks stage completion
  - `PipelineError`: structured error with stage, scene_num, cause, recover_cmd
- `internal/workspace/project.go`:
  - `WriteFileAtomic()`: temp file + rename for corruption-safe writes

### Tests
- `internal/pipeline/checkpoint_test.go`: 9 tests
  - SaveAndLoad: round-trip checkpoint persistence
  - MultipleStages: accumulating stage completions
  - LoadNoCheckpoint: nil return for missing checkpoint
  - GetResumeStage: 5 cases (nil, empty, after data_load, after image_generate, after assemble)
  - ShouldSkipStage: completed stages skipped, pending stages not skipped
  - BuildRecoveryCommand: 6 cases (each stage with/without scene number)
  - CheckProjectIntegrity: empty file detected, valid file passes, no file OK (early stage)

## Dev Agent Record
### Agent Model Used
Claude Opus 4.6

### File List
- `internal/pipeline/checkpoint.go` — New: CheckpointManager, BuildRecoveryCommand, CheckProjectIntegrity
- `internal/pipeline/checkpoint_test.go` — New: 9 unit tests

### Change Log
- 2026-03-08: Initial implementation with all acceptance criteria met
