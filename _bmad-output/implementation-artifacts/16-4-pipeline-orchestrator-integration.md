# Story 16-4: Pipeline Orchestrator Integration

## Overview
Integrate image_review and tts_review stages into the pipeline orchestrator with per-scene approval gates.

## Changes

### `internal/pipeline/runner.go` — Pipeline Flow Modification
- Add `SkipApproval` field to `RunOptions`
- Modify `resumeFromApproval()`:
  - With --skip-approval: transition approved → image_review, run all images, auto-approve all, transition → tts_review, run all TTS, auto-approve all, transition → assembling
  - Without --skip-approval: transition → image_review, generate per-scene, pause for approval. Resume transitions → tts_review, generate per-scene, pause again.
- Add `ResumeFromImageReview()` and `ResumeFromTTSReview()` for approval resume flow
- Update `Resume()` to handle image_review and tts_review states
- Backward compat: existing --auto-approve + skip-approval → old generating_assets path preserved via StatusGeneratingAssets

### `internal/cli/run_cmd.go` — CLI Flag
- Add `--skip-approval` flag for image/TTS approval bypass
- Pass to RunOptions

### `internal/pipeline/runner.go` — Checkpoint/Resume Updates
- `findExistingCheckpoint()` handles image_review/tts_review states
- Resume detects current state and resumes appropriately

### Test Updates
- Existing integration tests work with --skip-approval (backward compat)
- New unit tests for approval flow state transitions

## Acceptance Criteria
- [x] --skip-approval flag auto-approves image and TTS stages
- [x] Pipeline pauses at image_review and tts_review without --skip-approval
- [x] Resume from image_review and tts_review works correctly
- [x] Existing tests pass with --skip-approval path
- [x] Checkpoint/resume compatibility maintained
