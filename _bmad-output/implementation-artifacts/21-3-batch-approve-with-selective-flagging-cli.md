# Story 21.3: Batch Approve with Selective Flagging (CLI)

Status: ready-for-dev

## Story

As a content creator,
I want to review all scenes at once and flag only problematic ones for rework while approving the rest,
so that I can complete scene approval in a single pass instead of one-by-one.

## Acceptance Criteria

1. **Given** a project with generated scenes
   **When** `yt-pipe review batch <scp-id> --asset image` is executed
   **Then** the batch preview is displayed as a table: scene number, mood, status, AI score, image path
   **And** the creator is prompted to enter flagged scene numbers (comma-separated) or `none` for full approval

2. **Given** the creator flags scenes 3 and 7
   **When** `BatchApprove(ctx, projectID, assetType, flaggedScenes)` is called
   **Then** all scenes except 3 and 7 are approved
   **And** scenes 3 and 7 remain in `generated` status for rework
   **And** the response shows: "Approved: 8, Flagged for review: 2"
   **And** `total_scenes`, `flagged_count`, `auto_approved_count` are logged via slog

3. **Given** the creator enters `none` (no flags)
   **When** batch approve runs
   **Then** all scenes are approved

4. **Given** the creator flags a non-existent scene number
   **When** batch approve runs
   **Then** an error is returned listing valid scene numbers

## Tasks / Subtasks

- [ ] Task 1: Add `BatchApprove()` service method (AC: #2, #3, #4)
  - [ ] 1.1 Add `BatchApprovalResult` struct
  - [ ] 1.2 Implement `BatchApprove()` on `ApprovalService`
  - [ ] 1.3 Validate flagged scene numbers against existing scenes
  - [ ] 1.4 Approve all non-flagged generated scenes, skip others
  - [ ] 1.5 Log summary via slog

- [ ] Task 2: Add `yt-pipe review batch` CLI command (AC: #1)
  - [ ] 2.1 Create `internal/cli/review_cmd.go` with `reviewCmd` parent + `reviewBatchCmd`
  - [ ] 2.2 Display batch preview as table (tabwriter)
  - [ ] 2.3 Prompt for flagged scenes using `promptString()`
  - [ ] 2.4 Parse comma-separated input
  - [ ] 2.5 Call `BatchApprove()` and display result
  - [ ] 2.6 Support `--json-output`

- [ ] Task 3: Tests (all ACs)
  - [ ] 3.1 Test `BatchApprove()` — partial flagging
  - [ ] 3.2 Test `BatchApprove()` — no flags (approve all)
  - [ ] 3.3 Test `BatchApprove()` — invalid scene number
  - [ ] 3.4 Test `BatchApprove()` — only generated scenes approved (skip pending/rejected)

## Dev Notes

### Key Patterns
- CLI: follow `scenes_cmd.go` pattern (openDB, NewApprovalService, tabwriter output)
- Prompt: use `promptString()` from `internal/cli/prompt.go`
- Service: reuse existing `ApproveScene()` per scene, iterate non-flagged generated scenes
- Table: `text/tabwriter` with Scene, Mood, Status, AI Score, Image Path columns

### BatchApprove Logic
Only approve scenes currently in `generated` status. Already-approved, pending, or rejected scenes are untouched. This prevents re-approving or approving pending scenes.

### File Structure
- NEW: `internal/cli/review_cmd.go` — review parent + batch subcommand
- MODIFY: `internal/service/approval.go` — add BatchApprovalResult + BatchApprove()
- MODIFY: `internal/service/approval_test.go` — add BatchApprove tests

## Dev Agent Record

### Agent Model Used

### Completion Notes List

### File List
