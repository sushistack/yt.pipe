# Story 21.1: Auto-Approve by Validation Score

Status: review

## Story

As a content creator,
I want high-scoring scenes automatically approved after image validation,
so that I only need to review scenes the AI flagged as potentially problematic.

## Acceptance Criteria

1. **Given** `auto_approval.enabled: true` and `threshold: 80` in config
   **When** `AutoApproveByScore(ctx, projectID, assetType, threshold)` is called
   **Then** all scenes with `validation_score >= threshold` are auto-approved via existing `ApproveScene()`
   **And** auto-approved scenes are logged as `"auto-approved (score: N)"`
   **And** the method returns two lists: `autoApproved []int` and `reviewRequired []int`

2. **Given** a scene with `validation_score < threshold`
   **When** auto-approval runs
   **Then** the scene remains in `generated` status (review queue)

3. **Given** a scene with `validation_score == NULL` (validation not run)
   **When** auto-approval runs
   **Then** the scene remains in `generated` status (review queue)

4. **Given** `auto_approval.enabled: true` but `image_validation.enabled: false`
   **When** the config is loaded
   **Then** a warning is logged: "auto_approval requires image_validation to be enabled"
   **And** auto-approval is effectively disabled (no scores to evaluate)

5. **Given** all scenes are auto-approved (all scores >= threshold)
   **When** auto-approval completes
   **Then** the next pipeline stage transition is triggered (image_review → tts generation)

6. **Given** config `auto_approval` section
   **When** loaded
   **Then** `AutoApproval` struct contains: `Enabled` (bool, default false), `Threshold` (int, default 80)

## Tasks / Subtasks

- [x] Task 1: Add `AutoApproval` config struct (AC: #6)
  - [x] 1.1 Add `AutoApproval` struct to `internal/config/types.go`
  - [x] 1.2 Register defaults in `internal/config/config.go` `setDefaults()`
  - [x] 1.3 Add config validation: warn if `auto_approval.enabled && !image_validation.enabled`

- [x] Task 2: Add `AutoApproveByScore()` to ApprovalService (AC: #1, #2, #3)
  - [x] 2.1 Add method to `internal/service/approval.go`
  - [x] 2.2 Query store for scenes with `generated` status + validation scores
  - [x] 2.3 Apply threshold logic (>= approve, < keep, NULL keep)
  - [x] 2.4 Call existing `ApproveScene()` for qualifying scenes
  - [x] 2.5 Log each auto-approved scene with slog

- [x] Task 3: Add store method to fetch validation scores for scenes (AC: #1, #3)
  - [x] 3.1 Add `ListSceneValidationScores(projectID, assetType)` to store
  - [x] 3.2 JOIN scene_approvals with shot_manifests.validation_score

- [x] Task 4: Config validation at startup (AC: #4)
  - [x] 4.1 Add validation logic in config `Validate()` function
  - [x] 4.2 Log warning and effectively disable auto-approval when image_validation disabled

- [x] Task 5: Pipeline integration for auto-approval trigger (AC: #5)
  - [x] 5.1 Wire `AutoApproveByScore()` call after image generation + validation completes
  - [x] 5.2 If all scenes auto-approved, skip pause and proceed to TTS generation

- [x] Task 6: Tests (all ACs)
  - [x] 6.1 Unit tests for `AutoApproveByScore()` — threshold boundary (79, 80, 81)
  - [x] 6.2 Test NULL validation_score handling
  - [x] 6.3 Test config warning when EFR3 disabled + EFR4 enabled
  - [x] 6.4 Test all-approved triggers next stage (via AllApproved check)
  - [x] 6.5 Test mixed scores — partial approval (multi-shot scenes with min aggregation)

## Dev Notes

### Key Design Decision: Scene-Level Score from Shot-Level Scores
The `validation_score` column is on `shot_manifests` (per-shot), but approval is per-scene.
The store query uses `MIN(validation_score)` across all shots in a scene as the scene-level score.
If any shot has NULL validation_score, the entire scene score is NULL (conservative approach).

### References

- [Source: internal/service/approval.go — AutoApproveByScore method]
- [Source: internal/store/scene_approval.go — ListSceneValidationScores query]
- [Source: internal/config/types.go — AutoApproval struct]
- [Source: internal/pipeline/runner.go — runApprovalPath auto-approval integration]

## Dev Agent Record

### Agent Model Used
Claude Opus 4.6 (1M context)

### Debug Log References

### Completion Notes List
- Added `AutoApproval` config struct with Enabled (default false) and Threshold (default 80)
- Added config validation warning when auto_approval.enabled but image_validation.disabled
- Added `ListSceneValidationScores()` store method using JOIN of scene_approvals and shot_manifests with MIN aggregation
- Added `AutoApproveByScore()` service method that applies threshold logic and calls existing ApproveScene()
- Integrated auto-approval into pipeline runner's approval path — auto-approves after image generation, skips pause if all approved
- Wired auto-approval config to RunnerConfig in run_cmd.go, serve_cmd.go, stage_cmds.go
- Added 8 new tests: 5 service tests (threshold boundary, NULL handling, mixed scores, no generated scenes, all above threshold) + 3 config tests (defaults, warning, no-warning)
- All existing tests pass (no regressions), full suite green

### File List
- internal/config/types.go (modified — added AutoApproval struct and Config field)
- internal/config/config.go (modified — added setDefaults for auto_approval, added validation warning)
- internal/config/config_test.go (modified — added 3 tests for auto-approval config)
- internal/service/approval.go (modified — added AutoApproveByScore method)
- internal/service/approval_test.go (modified — added 5 auto-approve tests)
- internal/store/scene_approval.go (modified — added SceneValidationScore type and ListSceneValidationScores method)
- internal/pipeline/runner.go (modified — added autoApproval fields to Runner/RunnerConfig, integrated into runApprovalPath)
- internal/cli/run_cmd.go (modified — pass AutoApproval config to RunnerConfig)
- internal/cli/serve_cmd.go (modified — pass AutoApproval config to RunnerConfig)
- internal/cli/stage_cmds.go (modified — pass AutoApproval config to RunnerConfig)
