# Story 12.2: Cross-Stage Checkpoint & Resume

Status: done

## Story

As a creator,
I want to resume the pipeline from the exact stage that failed when using real providers,
So that I don't waste API calls and time re-running completed stages after fixing a config issue.

## Acceptance Criteria

1. When pipeline fails at any stage and is re-run, completed stages are detected via checkpoints and skipped
2. CLI displays "Resuming from stage: X (N stages already completed)"
3. Per-scene checkpoint for image and TTS: partially completed scenes are not re-processed
4. Image and TTS maintain separate checkpoint state (parallel independence)
5. `--force` flag clears all checkpoints and starts from stage 1
6. Existing artifacts are backed up to `{project}/backup/{timestamp}/` before overwriting with --force

## Tasks / Subtasks

- [ ] Task 1: Integrate checkpoint saving into Runner.Run() (AC: #1, #2)
  - [ ] After each stage completes, call SaveCheckpoint()
  - [ ] On Run() entry, check for existing checkpoint and skip completed stages
  - [ ] Log resume message when skipping stages
- [ ] Task 2: Integrate checkpoint into Runner.Resume() (AC: #1, #3, #4)
  - [ ] Check checkpoint before each post-approval stage
  - [ ] Skip image scenes already completed (per-scene checkpoint via manifest)
  - [ ] Skip TTS scenes already completed (independent of image checkpoint)
- [ ] Task 3: Add --force flag (AC: #5, #6)
  - [ ] Add --force flag to run command
  - [ ] Clear checkpoints when --force is set
  - [ ] Backup existing artifacts before overwriting

## Dev Notes

- `internal/service/pipeline_orchestrator.go` has PipelineCheckpoint, SaveCheckpoint, LoadCheckpoint, HasCompletedStage, RecordStage already implemented but NOT integrated into runner
- `internal/pipeline/runner.go` Run() and Resume() need checkpoint integration
- Per-scene skip logic already exists in TTSService.ShouldSkipScene() and ImageGenService (manifest hash check)
- workspace.WriteFileAtomic() for safe checkpoint writes
- Backup: create `{project}/backup/{timestamp}/` dir and copy existing output

### References

- [Source: internal/service/pipeline_orchestrator.go - PipelineCheckpoint]
- [Source: internal/pipeline/runner.go - Run(), Resume()]
- [Source: internal/service/tts.go - ShouldSkipScene()]
- [Source: internal/service/image_gen.go - manifest hash checking]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6

### Completion Notes List

- Integrated existing CheckpointManager into pipeline runner via `findExistingCheckpoint()` and `saveCheckpointAfterStage()`
- `saveCheckpointAfterStage` delegates to `CheckpointManager.SaveStageCheckpoint()` avoiding duplication
- `RunWithOptions` with `Force=true` calls `BackupAndClearCheckpoints()` before checkpoint check
- Extracted `resumeFromApproval()` shared by both `Run()` and `Resume()` to avoid code duplication
- Code review: removed duplicate checkpoint save logic, consolidated to use CheckpointManager

### File List

- internal/pipeline/runner.go
