# Story 18.7: Scene Dashboard Enhancement for n8n Polling

Status: done

## Story

As a n8n workflow orchestrator,
I want the scene dashboard endpoint to include per-scene approval flags and aggregate status fields,
so that I can efficiently poll for approval completion without making multiple API calls.

## Acceptance Criteria

1. **Given** GET /projects/{id}/scenes is called
   **When** the dashboard is returned
   **Then** each scene entry includes: `image_approved` (bool), `tts_approved` (bool), `prompt` (string), `assets` (object with image/audio/subtitle paths)

2. **Given** GET /projects/{id}/scenes is called
   **When** the dashboard is returned
   **Then** the response includes aggregate fields: `all_images_approved`, `all_tts_approved`, `all_approved` (bool)
   **And** count fields: `approved_image_count`, `approved_tts_count`, `total_scenes`

3. **Given** all scenes have approved images
   **When** the dashboard is queried
   **Then** `all_images_approved` is true
   **And** `approved_image_count` equals `total_scenes`

4. **Given** a mix of approved and unapproved scenes
   **When** the dashboard is queried
   **Then** `all_approved` is false
   **And** individual scene entries accurately reflect their approval status

5. **Given** the approval webhook fires `scene_approved` when a scene is approved
   **When** n8n polls the dashboard
   **Then** the approval flags reflect the latest state
   **And** `all_approved` transitions to true when all scenes are approved for all asset types

## Tasks / Subtasks

- [x] Task 1: Extend SceneDashboardEntry with per-scene fields (AC: #1)
  - [x] 1.1: Add `ImageApproved bool` field with json:"image_approved"
  - [x] 1.2: Add `TTSApproved bool` field with json:"tts_approved"
  - [x] 1.3: Add `Prompt string` field with json:"prompt"
  - [x] 1.4: Add `Assets *SceneAssets` field with json:"assets"
  - [x] 1.5: Define SceneAssets struct with ImagePath, AudioPath, SubtitlePath
- [x] Task 2: Extend SceneDashboard with aggregate fields (AC: #2, #3)
  - [x] 2.1: Add `ApprovedImageCount int` field
  - [x] 2.2: Add `ApprovedTTSCount int` field
  - [x] 2.3: Add `AllImagesApproved bool` field
  - [x] 2.4: Add `AllTTSApproved bool` field
  - [x] 2.5: Add `AllApproved bool` field (all images AND all TTS approved)
- [x] Task 3: Update GetDashboard to populate new fields (AC: #1-#4)
  - [x] 3.1: Load image and TTS approvals from store
  - [x] 3.2: Set per-scene ImageApproved/TTSApproved based on approval status
  - [x] 3.3: Build SceneAssets from workspace paths
  - [x] 3.4: Load scenario text for prompt field
  - [x] 3.5: Compute aggregate counts and boolean flags
  - [x] 3.6: Compute summaries for all states (not just review states)
- [x] Task 4: Write unit tests (AC: #1-#5)
  - [x] 4.1: Test dashboard with all scenes approved
  - [x] 4.2: Test dashboard with mixed approval states
  - [x] 4.3: Test dashboard with no approvals
  - [x] 4.4: Test aggregate flag calculations
  - [x] 4.5: Test per-scene prompt and asset path population

## Dev Notes

### Key Architecture Decisions

- **Flat aggregate booleans for n8n polling**: `all_images_approved`, `all_tts_approved`, `all_approved` allow n8n to use simple condition checks without iterating scenes
- **Per-scene approval booleans**: `image_approved` and `tts_approved` provide granular status without requiring separate API calls
- **SceneAssets struct**: Groups file paths for easy consumption by n8n HTTP nodes
- **Always compute summaries**: Removed state check gate — summaries are computed for all project states to support flexible n8n polling

### References

- [Source: internal/service/scene_dashboard.go] - SceneDashboard, SceneDashboardEntry, SceneAssets, GetDashboard
- [Source: internal/api/scenes.go] - handleGetScenes handler
- [Source: internal/service/approval.go] - ApprovalService.GetApprovalStatus

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6

### Completion Notes List

- Extended SceneDashboardEntry with ImageApproved, TTSApproved, Prompt, Assets fields
- Added SceneAssets struct with ImagePath, AudioPath, SubtitlePath
- Extended SceneDashboard with AllImagesApproved, AllTTSApproved, AllApproved, ApprovedImageCount, ApprovedTTSCount
- Updated GetDashboard to load approvals, compute aggregates, build asset paths
- Summaries now computed for all project states (not gated by review state)
- All tests pass, no regressions

### File List

- internal/service/scene_dashboard.go (modified) - SceneDashboardEntry, SceneAssets, SceneDashboard, GetDashboard
- internal/service/scene_dashboard_test.go (modified) - Dashboard enhancement tests
