# Epic 21 Retrospective: Automated Approval & Batch Review (EFR4, EFR5)

**Date:** 2026-03-18
**Status:** Done
**Stories:** 4/4 completed

## Summary

Epic 21 implemented AI validation score-based auto-approval and batch review capabilities, reducing manual scene approval overhead from one-by-one review to efficient batch operations. The system auto-approves high-scoring scenes and provides batch preview + selective flagging through both CLI and REST API.

## Stories Completed

| Story | Title | Key Deliverable |
|-------|-------|----------------|
| 21-1 | Auto-Approve by Validation Score | `AutoApproveByScore()` + `AutoApproval` config + pipeline integration |
| 21-2 | Batch Preview Data Assembly | `GetBatchPreview()` + `BatchPreviewItem` with narration/mood/score |
| 21-3 | Batch Approve with Selective Flagging (CLI) | `BatchApprove()` + `yt-pipe review batch` command |
| 21-4 | Batch Preview & Approve API Endpoints | `GET /preview` + `POST /batch-approve` API endpoints |

## What Went Well

1. **Clean layering** — Each story built precisely on the previous: config → store → service → CLI → API. Zero circular dependencies or rework needed
2. **Reuse of existing infrastructure** — `ApproveScene()`, `ListApprovalsByProject()`, `SplitNarrationSentences()`, `LoadScenarioFromFile()` all reused without modification. No wheel reinvention
3. **Scene-level score from shot-level data** — The SQL `MIN(validation_score)` aggregation across shots per scene solved the schema mismatch cleanly (validation scores on `shot_manifests`, approvals on `scene_approvals`)
4. **Code review caught real issues** — H1 finding (store bypass → service call) prevented potential state machine violations in production
5. **Consistent patterns** — All new code follows established service/store/CLI/API patterns from prior epics. No new paradigms introduced

## What Could Be Improved

1. **Two similar store methods** — `ListSceneValidationScores` (generated-only) and `ListAllSceneValidationScores` (all statuses) share identical SQL logic except one WHERE clause. Could be unified with an optional status filter parameter
2. **No integration test for pipeline auto-approval path** — The `runApprovalPath` auto-approval logic in `runner.go` is covered by unit tests on the service/store layers but lacks an integration test that exercises the full pipeline flow
3. **BatchPreviewItem image path is hardcoded** — Uses `image.png` (scene-level backward compat). If the project uses only shot-level cuts without a scene-level copy, the path may not resolve. Should verify file existence or return the first available shot image

## Architecture Decisions

- **AutoApproval config as separate struct** — Not nested under `ImageValidation` to keep config concerns separate. `auto_approval.enabled && image_validation.enabled` are checked together at the call site
- **Config validation as warning, not error** — `auto_approval.enabled` without `image_validation.enabled` logs a warning but doesn't fail config loading. This is consistent with the existing validation pattern
- **`BatchApprove` only approves `generated` scenes** — Already-approved, pending, and rejected scenes are untouched. This prevents double-approvals and ensures rejected scenes go through the regeneration cycle
- **API endpoints use n8n-compatible flat JSON** — `GET /preview` returns a flat array (not nested in data envelope). `POST /batch-approve` accepts `flagged_scenes` as a simple int array

## Previous Retro Action Items Follow-Through

From Epic 20 retrospective:
- ✅ **Epic 21 leverages `validation_score`** — Core premise of Epic 21 fulfilled
- ⏳ **Code duplication in openai.go** — Not addressed in Epic 21 (not in scope). Still relevant for future refactoring
- ⏳ **Live API test** — Not added in Epic 21. Consider for a dedicated test sprint

## Files Changed

### New Files
- `internal/cli/review_cmd.go` — `yt-pipe review batch` CLI command

### Modified Files
- `internal/config/types.go` — `AutoApproval` struct
- `internal/config/config.go` — defaults + validation warning
- `internal/config/config_test.go` — 3 new tests
- `internal/service/approval.go` — `AutoApproveByScore()`, `BatchPreviewItem`, `GetBatchPreview()`, `BatchApprovalResult`, `BatchApprove()`
- `internal/service/approval_test.go` — 13 new tests (auto-approve + batch preview + batch approve)
- `internal/store/scene_approval.go` — `SceneValidationScore`, `ListSceneValidationScores()`, `ListAllSceneValidationScores()`
- `internal/store/scene_approval_test.go` — 4 new store tests
- `internal/pipeline/runner.go` — auto-approval fields + `runApprovalPath` integration
- `internal/cli/run_cmd.go` — pass `AutoApproval` config to `RunnerConfig`
- `internal/cli/serve_cmd.go` — pass `AutoApproval` config to `RunnerConfig`
- `internal/cli/stage_cmds.go` — pass `AutoApproval` config to `RunnerConfig`
- `internal/api/review.go` — `handleBatchPreview()`, `handleBatchApprove()` handlers
- `internal/api/server.go` — route registration
- `internal/api/auth.go` — review-scoped routes
- `internal/api/scenes_test.go` — 5 new API tests

## Metrics

- **Total new tests:** ~25
- **New source files:** 1
- **Modified source files:** 15
- **Migration:** 0 (reused Epic 20's `validation_score` column)
- **Config additions:** `auto_approval` section with 2 fields (enabled, threshold)
- **New CLI commands:** `yt-pipe review batch <project-id> --asset <type>`
- **New API endpoints:** 2 (`GET /preview`, `POST /batch-approve`)

## Lessons Learned

1. **Shot vs Scene schema gap** — The `validation_score` lives on `shot_manifests` but approval is per-scene. This required a SQL JOIN with MIN aggregation. Future features should be aware of this two-level hierarchy
2. **Separate store methods > parameterized methods** — Adding `ListAllSceneValidationScores` (no status filter) was cleaner than adding a boolean parameter to the existing `ListSceneValidationScores`. Keeps each method's contract simple
3. **Code review as quality gate works** — The review caught the store-bypass issue in `AutoApproveByScore` that could have caused state machine violations. Worth the overhead

## Next Steps

- No Epic 23 is currently planned. Epic 21 and 22 complete the Phase 2 Enhancement PRD
- Consider live integration testing for the full auto-approval pipeline flow
- Monitor auto-approval threshold tuning in production (start at 90, adjust down)
- The `review batch` CLI and `/batch-approve` API are ready for n8n workflow integration
