---
title: 'Stacking Review Page'
slug: 'stacking-review-page'
created: '2026-03-12'
status: 'implementation-complete'
stepsCompleted: [1, 2, 3, 4]
tech_stack: ['Go 1.25.7', 'html/template', 'chi router', 'Vanilla JS', 'Tailwind CSS (inline)', 'SQLite']
files_to_modify: ['internal/api/templates/review.html', 'internal/api/templates/styles.css', 'internal/api/review.go', 'internal/api/server.go', 'internal/service/review.go', 'internal/store/scene_approval.go', 'internal/store/bgm.go', 'internal/store/mood_preset.go']
code_patterns: ['Go html/template with {{if}} conditionals for status-based rendering', 'Server-side rendering with embedded templates (embed.FS)', 'Vanilla JS fetch-based API calls with toast notifications', 'Scene numbering is integer-based, maps to filesystem dirs (scenes/N/)', 'DeleteScene does NOT reindex — gaps allowed in scene numbering', 'ReviewService uses per-project mutex locks for concurrency safety']
test_patterns: ['*_test.go same package', 'testify assertions', 'internal/mocks package referenced but missing — prefer integration tests']
---

# Tech-Spec: Stacking Review Page

**Created:** 2026-03-12

## Overview

### Problem Statement

The current review page displays all assets (narration, images, TTS audio) simultaneously regardless of the project's pipeline stage. This makes the step-by-step review flow unclear and overwhelming — reviewers see image/TTS placeholders and action buttons for assets that haven't been generated yet.

### Solution

Transform the unified review page into a stacking UI that progressively reveals assets based on the project's current pipeline status:
- `scenario_review`: Show narration text only (edit + approve scenario)
- `image_review`+: Stack images alongside narration (image approve/reject)
- `tts_review`+: Stack TTS audio alongside narration + images (TTS approve/reject)

### Scope

**In Scope:**
- Project-status-based UI visibility control for scene assets
- Header buttons shown only for the current review stage
- All previously revealed assets remain editable in later stages
- Each scene displayed as a cohesive set (narration + image + TTS)
- Insert new scene between existing scenes (backend API change included)
- Delete individual scenes
- Stage transition visual feedback: stage label headers + next-stage hint bar
- "Reject" button label → "Regenerate" (image/TTS both, backend unchanged)
- Fix existing bug: `DeleteScene` not cleaning up BGM/mood assignment records

**Out of Scope:**
- Backend state machine changes
- New authentication/authorization logic

## Context for Development

### Codebase Patterns

- **Server-side rendering**: Go `html/template` with `embed.FS` — templates are compiled at startup via `initReviewTemplate()`
- **Template data**: `handleReviewPage()` passes `Project`, `Dashboard`, `Token`, `ProjectID`, `StylesCSS` to template
- **Project status available in template**: `{{.Dashboard.ProjectStatus}}` — can be used directly for stacking conditionals
- **Vanilla JS frontend**: No framework; all interactions are `fetch()`-based API calls with full page reload after mutations
- **Scene numbering**: Integer-based, 1-indexed. Gaps allowed after deletion. `scenario.json` uses explicit `SceneNum` field.
- **File system layout**: `{workspace}/scenes/{N}/image.png`, `{workspace}/scenes/{N}/audio.mp3`, `{workspace}/scenes/{N}/prompt.txt`
- **Concurrency**: `ReviewService` uses per-project `sync.Mutex` for safe concurrent mutations
- **State constants**: `domain.StatusScenarioReview`, `domain.StatusApproved`, `domain.StatusImageReview`, `domain.StatusTTSReview`, `domain.StatusGeneratingAssets` (deprecated but must handle), etc.

### Files to Reference

| File | Purpose |
| ---- | ------- |
| `internal/api/templates/review.html` | Main review page HTML template + inline JS |
| `internal/api/templates/styles.css` | Tailwind CSS styles (inline-embedded) |
| `internal/api/review.go` | Review page handler + scene mutation handlers |
| `internal/api/server.go` | Server setup, router, template init, `progressPercent` func |
| `internal/service/review.go` | ReviewService: narration edit, scene add/delete, `allowedMutationStates` |
| `internal/service/scene_dashboard.go` | Dashboard data assembly service |
| `internal/service/approval.go` | Approval state management |
| `internal/domain/project.go` | Project model + status constants + state machine |
| `internal/domain/scene_approval.go` | SceneApproval model |
| `internal/store/scene_approval.go` | DB ops for approvals + manifests |
| `internal/store/bgm.go` | DB ops for BGM assignments |
| `internal/store/mood_preset.go` | DB ops for mood preset assignments |

### Technical Decisions

1. **Stacking via Go template conditionals**: Use `{{if}}` blocks keyed on `ProjectStatus` to show/hide image and TTS sections. No new API endpoints needed for stacking itself.
2. **Scene insertion requires renumbering**: Inserting between scenes requires renaming only **existing** filesystem dirs, updating `scenario.json` SceneNum fields, and updating DB records. Gaps are handled by operating on actual existing items only (not iterating a contiguous range).
3. **Insert API**: Extend `POST /api/v1/projects/{id}/scenes` with optional `after` query param (e.g., `?after=3`). If `after` is absent or empty string, append to end (backward compatible). If `after` is present but non-integer, return 400.
4. **Reject → Regenerate**: Label-only change in HTML template. Backend `reject` endpoint unchanged.
5. **Progress bar**: Use template conditional display — hide progress bar during `scenario_review`/`approved`/`pending`, show stage-specific progress otherwise. Keep `progressPercent` function signature unchanged.
6. **All states editable**: Add `StatusAssembling`, `StatusComplete`, and `StatusGeneratingAssets` (deprecated compat) to `allowedMutationStates`.
7. **`approved` state UI**: Same as `scenario_review` (narration only) but with "Image generation pending..." hint instead of "Approve Scenario" button.
8. **Scene insert guard**: Reject insert if ANY running generation job exists (`image_generate` OR `tts_generate`). Check both job types. Return 409 Conflict.
9. **Renumbering atomicity (MANDATORY)**:
   - **Filesystem**: Use temp-rename pattern: `scenes/N` → `scenes/N_shift_tmp` for all affected dirs first, then `scenes/N_shift_tmp` → `scenes/N+1`. If any rename fails, reverse all completed renames.
   - **DB**: All 4 table renumber operations MUST run in a single SQLite transaction (`store.db.Begin()` → renumber → `tx.Commit()`). On failure, `tx.Rollback()`.
   - **Order**: Filesystem renames first → DB transaction → scenario.json update. If DB tx fails, rollback filesystem renames.
10. **Gap-aware renumbering**: Do NOT iterate a contiguous range. Use `os.ReadDir` to find actually existing scene dirs > afterNum, sort descending, rename only those. SQL `WHERE scene_num > ?` naturally skips gaps.

### Stacking UI Visibility Matrix

| Status | Narration | Image | TTS | Header Button | Hint Bar |
| ------ | --------- | ----- | --- | ------------- | -------- |
| `pending` | Edit | Hidden | Hidden | (none) | "Scenario generation pending..." |
| `scenario_review` | Edit | Hidden | Hidden | Approve Scenario | "Next: Image generation" |
| `approved` | Edit | Hidden | Hidden | (none) | "Image generation pending..." |
| `generating_assets` | Edit | Show + Actions | Hidden | Approve All Images | "Next: TTS generation" |
| `image_review` | Edit | Show + Actions | Hidden | Approve All Images | "Next: TTS generation" |
| `tts_review` | Edit | Show | Show + Actions | Approve All TTS | (none) |
| `assembling` | Edit | Show | Show | (none) | "Assembling..." |
| `complete` | Edit | Show | Show | (none) | (none) |

## Implementation Plan

### Tasks

- [x] Task 0: Fix `DeleteScene` to clean up BGM and mood assignment records
  - File: `internal/service/review.go` — `DeleteScene()` method
  - Action: After existing `store.DeleteSceneApprovals()` and `store.DeleteSceneManifest()` calls, add:
    - `store.DeleteSceneBGM(projectID, sceneNum)`
    - `store.DeleteSceneMood(projectID, sceneNum)`
  - File: `internal/store/bgm.go`
  - Action: Add `DeleteSceneBGM(projectID string, sceneNum int) error` — DELETE FROM scene_bgm_assignments WHERE project_id = ? AND scene_num = ?
  - File: `internal/store/mood_preset.go`
  - Action: Add `DeleteSceneMoodAssignment` if not already handling single-scene delete (check existing `DeleteSceneMoodAssignment` — it exists but verify it matches the needed signature)
  - Notes: This fixes a pre-existing bug where deleting a scene leaves orphan BGM/mood records. MUST be done before InsertScene to prevent renumbering orphans into wrong positions.

- [x] Task 1: Expand `allowedMutationStates` to all project states
  - File: `internal/service/review.go`
  - Action: Add to `allowedMutationStates` map: `domain.StatusAssembling: true`, `domain.StatusComplete: true`, `domain.StatusGeneratingAssets: true`
  - Notes: Allows narration edits, scene add/delete in all project states including legacy `generating_assets`

- [x] Task 2: Add `InsertScene` method with atomic renumbering
  - File: `internal/service/review.go`
  - Action: Add `InsertScene(projectID string, afterSceneNum int, narration string) (int, error)` method
  - Notes:
    - Validate narration (reject empty, null bytes, >10000 chars — same rules as `UpdateNarration`)
    - Validate mutation state
    - Acquire project lock
    - Check no running generation jobs: call `store.GetRunningJobByProjectAndType()` for BOTH `"image_generate"` AND `"tts_generate"`. If either is running, return a `domain.ConflictError` (or similar) that the handler maps to 409
    - Validate `afterSceneNum` is within range [0, maxNum] (0 = insert at beginning)
    - **Filesystem renumbering** (temp-rename pattern):
      1. `os.ReadDir(scenesDir)` to get actually existing scene dirs
      2. Filter dirs where parsed number > afterSceneNum, sort descending
      3. Phase 1: rename each `scenes/N` → `scenes/N_shift_tmp`
      4. Phase 2: rename each `scenes/N_shift_tmp` → `scenes/N+1`
      5. On any failure in Phase 1/2, reverse completed renames
    - **DB renumbering** (single transaction):
      1. `tx := store.db.Begin()`
      2. `RenumberSceneApprovals(tx, projectID, afterSceneNum, +1)`
      3. `RenumberSceneManifests(tx, projectID, afterSceneNum, +1)`
      4. `RenumberSceneBGM(tx, projectID, afterSceneNum, +1)`
      5. `RenumberSceneMoods(tx, projectID, afterSceneNum, +1)`
      6. `tx.Commit()` — on failure, `tx.Rollback()` and reverse filesystem renames
    - **scenario.json**: Update all SceneNum > afterSceneNum to SceneNum+1, then append new scene
    - Create new scene dir at afterSceneNum+1 via `workspace.InitSceneDir()`
    - Init approval records + update project scene count

- [x] Task 3: Add store helper methods for scene renumbering (transaction-aware)
  - File: `internal/store/scene_approval.go`
  - Action: Add methods that accept `*sql.Tx`:
    - `RenumberSceneApprovalsTx(tx *sql.Tx, projectID string, afterNum int, delta int) error`
    - `RenumberSceneManifestsTx(tx *sql.Tx, projectID string, afterNum int, delta int) error`
  - File: `internal/store/bgm.go`
  - Action: Add `RenumberSceneBGMTx(tx *sql.Tx, projectID string, afterNum int, delta int) error`
  - File: `internal/store/mood_preset.go`
  - Action: Add `RenumberSceneMoodsTx(tx *sql.Tx, projectID string, afterNum int, delta int) error`
  - Notes: All use `UPDATE ... SET scene_num = scene_num + ? WHERE project_id = ? AND scene_num > ? ORDER BY scene_num DESC`. The `ORDER BY DESC` prevents unique constraint violations when delta > 0. All accept `*sql.Tx` so they can participate in a single transaction.

- [x] Task 4: Update `handleAddScene` handler to support positional insert
  - File: `internal/api/review.go`
  - Action: Modify `handleAddScene()`:
    - Parse `after` query param: `after := r.URL.Query().Get("after")`
    - If `after == ""` (absent or empty): call existing `s.reviewSvc.AddScene()` (backward compatible)
    - If `after` is present: parse with `strconv.Atoi(after)`. On parse error → 400 Bad Request
    - If valid int: call `s.reviewSvc.InsertScene(projectID, afterNum, narration)`
    - Map `domain.ConflictError` to 409 Conflict response
  - Notes: No new route — same `POST /api/v1/projects/{id}/scenes` endpoint

- [x] Task 5: Refactor `review.html` header to stacking buttons
  - File: `internal/api/templates/review.html`
  - Action: Replace current always-visible "Approve All Images" and "Approve All TTS" buttons with status-conditional rendering. Locate the header `<div class="flex items-center gap-2">` section:
    - `scenario_review`: Show "Approve Scenario" button only
    - `approved`/`pending`: No action buttons
    - `image_review`/`generating_assets`: Show "Approve All Images" button only
    - `tts_review`: Show "Approve All TTS" button only
    - `assembling`/`complete`: No action buttons
  - Notes: Use `{{if eq .Dashboard.ProjectStatus "..."}}` Go template conditionals. Use semantic anchors (element structure) not line numbers.

- [x] Task 6: Refactor scene card layout to stacking UI
  - File: `internal/api/templates/review.html`
  - Action: Restructure each scene card inside `{{range .Dashboard.Scenes}}` into stacking sections. Define template variables at top of range block:
    ```
    {{$status := $.Dashboard.ProjectStatus}}
    {{$showImages := or (eq $status "image_review") (eq $status "generating_assets") (eq $status "tts_review") (eq $status "assembling") (eq $status "complete")}}
    {{$showTTS := or (eq $status "tts_review") (eq $status "assembling") (eq $status "complete")}}
    ```
    - **Narration section** (always visible): Keep existing textarea + Save/Cancel
    - **Image section** (`{{if $showImages}}`): Image preview + image prompt collapsible + Approve/Regenerate Image buttons
    - **TTS section** (`{{if $showTTS}}`): Audio player + Approve/Regenerate TTS buttons
    - **Hint bars** (`{{if not $showImages}}`): Grey bar with next-stage text per Visibility Matrix. Same pattern for TTS hint.
  - Notes: Use the `$showImages`/`$showTTS` variables consistently across Tasks 6, 7

- [x] Task 7: Add scene status badges conditional rendering
  - File: `internal/api/templates/review.html`
  - Action: In scene card header, wrap IMG badge with `{{if $showImages}}` and TTS badge with `{{if $showTTS}}`. During `scenario_review`/`approved`/`pending`, both badges are hidden.

- [x] Task 8: Rename "Reject" buttons to "Regenerate"
  - File: `internal/api/templates/review.html`
  - Action: Change button labels inside the image and TTS action sections:
    - `Reject Image` → `Regenerate Image`
    - `Reject TTS` → `Regenerate TTS`
  - Notes: JS function `rejectScene()` stays the same — only the visible label changes

- [x] Task 9: Add inter-scene insert buttons
  - File: `internal/api/templates/review.html`
  - Action:
    - **Before** the `{{range .Dashboard.Scenes}}` loop, add "Insert at beginning" button:
      ```html
      <div class="flex justify-center py-1">
        <button onclick="insertScene(0)" class="text-gray-400 hover:text-indigo-600 text-sm transition" title="Insert scene at beginning">
          + Insert Scene
        </button>
      </div>
      ```
    - **After** each scene card closing `</div>` (inside the range loop), add:
      ```html
      <div class="flex justify-center py-1">
        <button onclick="insertScene({{.SceneNum}})" class="text-gray-400 hover:text-indigo-600 text-sm transition" title="Insert scene after #{{.SceneNum}}">
          + Insert Scene
        </button>
      </div>
      ```

- [x] Task 10: Add `insertScene()` JS function
  - File: `internal/api/templates/review.html`
  - Action: Add JS function in `<script>` section:
    ```javascript
    async function insertScene(afterNum) {
      const narration = prompt('Enter narration for the new scene:');
      if (!narration) return;
      const result = await apiFetch(API_BASE + '/scenes?after=' + afterNum, {
        method: 'POST',
        body: JSON.stringify({ narration: narration })
      });
      if (result) {
        showToast('Scene inserted after #' + afterNum, 'success');
        setTimeout(() => location.reload(), 500);
      }
    }
    ```
  - Notes: On 409 response (running jobs), `apiFetch` already shows error toast from backend response

- [x] Task 11: Update progress bar for stacking context
  - File: `internal/api/templates/review.html`
  - Action: Wrap the progress bar `<div class="h-1 bg-gray-200">` in conditional:
    - `{{if or (eq $status "pending") (eq $status "scenario_review") (eq $status "approved")}}`: Show status text label instead of progress bar (e.g., "Scenario Review")
    - `{{else if eq $status "image_review"}}` or `generating_assets`: Show progress bar with image approval percentage
    - `{{else if eq $status "tts_review"}}`: Show progress bar with TTS approval percentage
    - `{{else}}`: Show 100% / "Complete"
  - Notes: Keep existing `progressPercent` template func unchanged. For image-only or TTS-only progress, compute inline: `{{.Dashboard.ApprovedImageCount}} * 100 / {{.Dashboard.TotalScenes}}`

- [x] Task 12: Add stage label and hint bar styles
  - File: `internal/api/templates/styles.css`
  - Action: Add CSS classes:
    - `.stage-hint-bar` — grey rounded bar with muted text for next-stage hints (use Tailwind: `bg-gray-100 text-gray-400 rounded-lg p-4 text-center text-sm`)
    - `.stage-label` — small uppercase label above each asset section (use Tailwind: `text-xs font-semibold text-gray-500 uppercase mb-1`)
  - Notes: These can also be done inline with Tailwind utility classes directly in the template if preferred

### Acceptance Criteria

- [x] AC 1: Given a project in `scenario_review` status, when viewing the review page, then only narration text and "Approve Scenario" button are visible; image and TTS sections show hint bars with "Next: Image generation" text
- [x] AC 2: Given a project in `image_review` status, when viewing the review page, then narration and image sections are visible with approve/regenerate actions; TTS section shows hint bar; header shows "Approve All Images" only
- [x] AC 3: Given a project in `tts_review` status, when viewing the review page, then all three sections (narration, image, TTS) are visible; TTS has approve/regenerate actions; header shows "Approve All TTS" only
- [x] AC 4: Given a project in `complete` status, when editing a narration and clicking Save, then the narration is updated successfully (all states are editable)
- [x] AC 5: Given a project with scenes [1, 2, 3], when inserting a scene after scene 2, then a new scene 3 is created, old scene 3 becomes scene 4, filesystem directories are renamed, and all DB records (approvals, manifests, BGM, moods) are renumbered correctly
- [x] AC 6: Given a project with a running image generation OR tts generation job, when attempting to insert a scene, then the API returns 409 Conflict with an appropriate error message
- [x] AC 7: Given any scene card, when clicking "Regenerate Image", then the backend `reject` endpoint is called and regeneration is triggered (same behavior as old "Reject Image", label only changed)
- [x] AC 8: Given a scene card between scene 2 and scene 3, when clicking the "+ Insert Scene" button between them and entering narration text, then a new scene is inserted at position 3 and existing scene 3+ are shifted
- [x] AC 9: Given a project in `approved` status, when viewing the review page, then narration is editable and a hint bar shows "Image generation pending..."
- [x] AC 10: Given a project with scenes [1, 3, 5] (gaps from deletion), when inserting after scene 1, then renumbering correctly handles the gap: only existing dirs (3, 5) are renamed to (4, 6), new scene 2 is created
- [x] AC 11: Given a project in `pending` status, when viewing the review page, then narration is editable, no action buttons shown, hint bar shows "Scenario generation pending..."
- [x] AC 12: Given a project in legacy `generating_assets` status, when viewing the review page, then it renders identically to `image_review` (narration + images visible)
- [x] AC 13: Given a scene is deleted, then BGM and mood assignment records for that scene are also deleted (no orphans)
- [x] AC 14: Given an insert with `?after=` (empty string), when the API is called, then it falls back to append behavior (same as no `after` param)
- [x] AC 15: Given an insert with empty narration text, then the API returns 400 Bad Request with validation error

## Additional Context

### Dependencies

- No external library additions required
- Existing `store` package DB methods for scene_approvals, manifests, bgm_assignments, mood_assignments
- `workspace.InitSceneDir()` for creating new scene directories
- `workspace.WriteFileAtomic()` for safe file writes
- Job manager (`s.jobs`) and `store.GetRunningJobByProjectAndType()` for insert guard check
- `database/sql` `*sql.Tx` for transaction support in renumber operations

### Testing Strategy

- **Integration tests** (preferred — no mocks dependency):
  - Full flow: create project → scenario_review (narration only visible) → approve → image_review (images appear) → approve all → tts_review (TTS appears) → approve all → complete
  - InsertScene: test at beginning (after=0), middle, end positions with and without gaps
  - InsertScene: test 409 when generation job is running
  - InsertScene: verify all 4 DB tables + filesystem + scenario.json are consistent after insert
  - DeleteScene: verify BGM and mood records are cleaned up (regression test for Task 0 fix)
- **Manual testing**:
  - Visual verification of stacking UI at each project status
  - Test "Regenerate" button triggers same behavior as old "Reject"
  - Test editing narration in `complete` status
  - Test legacy `generating_assets` project renders correctly
  - Test `pending` project renders with hint bars
- **Unit tests** (if feasible without mocks):
  - Store renumber SQL operations with gap edge cases
  - Narration validation in InsertScene

### Notes

- **High risk: Scene renumbering** — Filesystem dir rename + multi-table DB update. Mitigation: temp-rename pattern for filesystem + single SQLite transaction for DB. Order: filesystem first → DB tx → scenario.json. Rollback filesystem if DB fails.
- **Bug fix included**: `DeleteScene` BGM/mood orphan cleanup (Task 0) — prerequisite for safe renumbering.
- **Legacy compat**: `generating_assets` status handled in visibility matrix and mutation states.
- **Known UX debt**: `prompt()` dialog for scene insertion/addition is a minimal UX. Future improvement could use inline form or modal.
- **Future consideration**: Drag-and-drop scene reordering — more natural UX but significantly more complex.
- **Future consideration**: Real-time UI updates via SSE/WebSocket instead of full page reload after mutations.
