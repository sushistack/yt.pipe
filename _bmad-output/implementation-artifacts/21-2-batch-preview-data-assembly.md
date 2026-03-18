# Story 21.2: Batch Preview Data Assembly

Status: review

## Story

As a content creator,
I want a preview listing of all scenes with image, narration excerpt, mood, and AI score,
so that I can quickly scan the entire project and decide which scenes need attention.

## Acceptance Criteria

1. **Given** a project with generated assets
   **When** `GetBatchPreview(ctx, projectID, assetType)` is called
   **Then** it returns `[]BatchPreviewItem` with each scene's:
   - `SceneNum` (int)
   - `ImagePath` (string)
   - `NarrationFirst` (string — first sentence of narration)
   - `Mood` (string)
   - `ValidationScore` (*int — nil if not validated)
   - `Status` (string — generated, approved, rejected)

2. **Given** a project where EFR3 (image validation) was not enabled
   **When** batch preview is generated
   **Then** `ValidationScore` is nil for all scenes
   **And** the preview is still functional with all other fields populated

3. **Given** a project with mixed scene statuses (some auto-approved, some generated, some rejected)
   **When** batch preview is generated
   **Then** all scenes are included with their current status
   **And** scenes are ordered by scene number

## Tasks / Subtasks

- [x] Task 1: Define `BatchPreviewItem` domain model (AC: #1)
  - [x] 1.1 Add `BatchPreviewItem` struct to `internal/service/approval.go`
  - [x] 1.2 Include all fields: SceneNum, ImagePath, NarrationFirst, Mood, ValidationScore, Status

- [x] Task 2: Implement `GetBatchPreview()` service method (AC: #1, #2, #3)
  - [x] 2.1 Add method to `ApprovalService`
  - [x] 2.2 Load project via store to get WorkspacePath
  - [x] 2.3 Load scenario from file for narration + mood data
  - [x] 2.4 Load approvals via `ListApprovalsByProject()`
  - [x] 2.5 Load validation scores via new `ListAllSceneValidationScores()` (all statuses)
  - [x] 2.6 Build image paths from workspace structure
  - [x] 2.7 Extract first sentence using `domain.SplitNarrationSentences()`
  - [x] 2.8 Assemble `[]BatchPreviewItem` ordered by scene number
  - [x] 2.9 Handle missing scenario gracefully (empty narration/mood)

- [x] Task 3: Tests (all ACs)
  - [x] 3.1 Test full preview assembly with all fields populated
  - [x] 3.2 Test nil ValidationScore when no scores exist (AC #2)
  - [x] 3.3 Test mixed statuses (generated, approved, rejected) (AC #3)
  - [x] 3.4 Test scene ordering by scene number
  - [x] 3.5 Test missing scenario file (graceful degradation)

## Dev Notes

### Key Design Decision
Added `ListAllSceneValidationScores()` store method (no status filter) separate from `ListSceneValidationScores()` (generated-only filter). This avoids modifying the existing method used by auto-approval.

## Dev Agent Record

### Agent Model Used
Claude Opus 4.6 (1M context)

### Completion Notes List
- Added `BatchPreviewItem` struct with JSON tags for API compatibility
- Added `GetBatchPreview()` method combining project, scenario, approvals, and validation scores
- Added `ListAllSceneValidationScores()` store method without status filter
- 4 batch preview tests covering: full assembly, nil scores, mixed statuses, missing scenario
- All existing tests pass (no regressions)

### File List
- internal/service/approval.go (modified — added BatchPreviewItem, GetBatchPreview)
- internal/service/approval_test.go (modified — added 4 batch preview tests)
- internal/store/scene_approval.go (modified — added ListAllSceneValidationScores)
