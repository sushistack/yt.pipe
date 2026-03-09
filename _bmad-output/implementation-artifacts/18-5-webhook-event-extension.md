# Story 18.5: Webhook Event Extension

Status: done

## Story

As a n8n workflow orchestrator,
I want to receive webhook notifications for job completion, job failure, scene approval, and all-approved events,
so that I can react to pipeline state changes in real time without polling.

## Acceptance Criteria

1. **Given** a job completes successfully
   **When** the `job_complete` event is fired
   **Then** webhook payload includes: event, project_id, scp_id, job_id, job_type, result, timestamp
   **And** the payload is flat JSON (no nested objects) for n8n compatibility

2. **Given** a job fails
   **When** the `job_failed` event is fired
   **Then** webhook payload includes: event, project_id, scp_id, job_id, job_type, error, failed_scene, timestamp

3. **Given** a single scene's asset is approved
   **When** the `scene_approved` event is fired
   **Then** webhook payload includes: event, project_id, scp_id, scene_num, asset_type, timestamp

4. **Given** all scenes of a specific asset type are approved
   **When** the `all_approved` event is fired
   **Then** webhook payload includes: event, project_id, scp_id, asset_type, timestamp

5. **Given** the WebhookNotifier is nil (no URLs configured)
   **When** any Notify method is called
   **Then** the method returns silently without panic (nil-safe)

## Tasks / Subtasks

- [x] Task 1: Define webhook event structs (AC: #1, #2, #3, #4)
  - [x] 1.1: Define `JobCompleteEvent` struct with flat JSON tags
  - [x] 1.2: Define `JobFailedEvent` struct with flat JSON tags including `failed_scene`
  - [x] 1.3: Define `SceneApprovedEvent` struct with flat JSON tags
  - [x] 1.4: Define `AllApprovedEvent` struct with flat JSON tags
- [x] Task 2: Implement notification methods on WebhookNotifier (AC: #1-#5)
  - [x] 2.1: `NotifyJobComplete(projectID, scpID, jobID, jobType, result)` — nil-safe
  - [x] 2.2: `NotifyJobFailed(projectID, scpID, jobID, jobType, errMsg, failedScene)` — nil-safe
  - [x] 2.3: `NotifySceneApproved(projectID, scpID, sceneNum, assetType)` — nil-safe
  - [x] 2.4: `NotifyAllApproved(projectID, scpID, assetType)` — nil-safe
- [x] Task 3: Wire webhooks into scene approval handler (AC: #3, #4)
  - [x] 3.1: Fire `scene_approved` in handleApproveScene after successful approval
  - [x] 3.2: Check if all scenes of the asset type are now approved
  - [x] 3.3: Fire `all_approved` if all scenes are approved
- [x] Task 4: Wire webhooks into pipeline and generation handlers (AC: #1, #2)
  - [x] 4.1: Fire `job_complete` in executeScenarioOnly, executeFullPipeline, executeImageGeneration, executeTTSGeneration, executeAssembly
  - [x] 4.2: Fire `job_failed` on errors in all execution handlers with failed_scene where applicable
- [x] Task 5: Write unit tests (AC: #1-#5)
  - [x] 5.1: Test each webhook event struct marshals to correct flat JSON
  - [x] 5.2: Test nil-safety for all Notify methods
  - [x] 5.3: Test webhook delivery with retry and error handling
  - [x] 5.4: Test scene approval webhook integration in handleApproveScene

## Dev Notes

### Key Architecture Decisions

- **Flat JSON payloads**: All webhook event structs use flat (non-nested) JSON fields for n8n HTTP Request node compatibility
- **Nil-safe pattern**: Every Notify method starts with `if wn == nil { return }` guard
- **Fan-out delivery**: Each URL is notified independently in a goroutine; failures are logged but don't block
- **Exponential backoff retry**: 1s → 2s → 4s with configurable max retries (default: 3)

### References

- [Source: internal/api/webhook.go] - WebhookNotifier, event structs, Notify methods
- [Source: internal/api/scenes.go] - Webhook integration in handleApproveScene
- [Source: internal/api/pipeline.go] - Webhook calls in pipeline execution
- [Source: internal/api/assets.go] - Webhook calls in generation/assembly execution

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6

### Completion Notes List

- Added 4 new event structs: JobCompleteEvent, JobFailedEvent, SceneApprovedEvent, AllApprovedEvent
- Added 4 Notify methods: NotifyJobComplete, NotifyJobFailed, NotifySceneApproved, NotifyAllApproved
- All methods are nil-safe (guard on `wn == nil`)
- Wired scene_approved and all_approved webhooks into handleApproveScene
- Wired job_complete and job_failed webhooks into all execution handlers
- All payloads are flat JSON for n8n compatibility
- All tests pass, no regressions

### File List

- internal/api/webhook.go (modified) - Event structs, Notify methods
- internal/api/scenes.go (modified) - Webhook integration in handleApproveScene
- internal/api/pipeline.go (modified) - Webhook calls in pipeline execution
- internal/api/assets.go (modified) - Webhook calls in generation/assembly execution
- internal/api/webhook_test.go (modified) - Webhook event and nil-safety tests
