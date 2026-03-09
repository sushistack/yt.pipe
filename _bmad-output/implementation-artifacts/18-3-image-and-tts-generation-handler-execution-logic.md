# Story 18.3: Image and TTS Generation Handler Execution Logic

Status: done

## Story

As a n8n workflow orchestrator,
I want the image and TTS generation endpoints to execute real generation logic in background goroutines,
so that I can trigger selective scene regeneration and track per-scene progress.

## Acceptance Criteria

1. **Given** POST /projects/{id}/images/generate is called with a list of scene numbers
   **When** the project is in "approved" or "image_review" state
   **Then** a background goroutine generates images for the specified scenes
   **And** returns 202 Accepted with a job_id immediately
   **And** per-scene progress is tracked and visible via GET /jobs/{jobId}

2. **Given** POST /projects/{id}/tts/generate is called with a list of scene numbers
   **When** the project is in "approved" or "tts_review" state
   **Then** a background goroutine generates TTS for the specified scenes
   **And** returns 202 Accepted with a job_id immediately

3. **Given** an empty scenes array is submitted
   **When** the handler processes the request
   **Then** all scenes (1 to scene_count) are generated

4. **Given** a generation job is already running for the same project and type
   **When** a duplicate request is received
   **Then** the handler returns 409 CONFLICT with the existing job_id
   **And** checks both in-memory jobManager and DB for running jobs

5. **Given** the project is in an invalid state for generation
   **When** the generation endpoint is called
   **Then** the handler returns 409 CONFLICT with a descriptive message

6. **Given** a scene generation fails mid-execution
   **When** the error occurs
   **Then** the job is marked as failed with the specific scene number in the error message
   **And** a job_failed webhook is fired with the failed_scene field

## Tasks / Subtasks

- [x] Task 1: Implement handleGenerateImages with state and duplicate validation (AC: #1, #3, #4, #5)
  - [x] 1.1: Validate project state against validImageGenStates map
  - [x] 1.2: Parse scenes array from request body, expand empty to all scenes
  - [x] 1.3: Check for duplicate via jobManager.getByType and store.GetRunningJobByProjectAndType
  - [x] 1.4: Create DB job record and start background goroutine
- [x] Task 2: Implement executeImageGeneration background worker (AC: #1, #6)
  - [x] 2.1: Iterate scenes with context cancellation checks
  - [x] 2.2: Read manual prompt from workspace if exists
  - [x] 2.3: Call imageGenSvc.GenerateSceneImage per scene
  - [x] 2.4: Update progress after each scene completion
  - [x] 2.5: On failure, fire job_failed webhook with failed_scene
- [x] Task 3: Implement handleGenerateTTS with state and duplicate validation (AC: #2, #3, #4, #5)
  - [x] 3.1: Validate project state against validTTSGenStates map
  - [x] 3.2: Parse scenes array, expand empty to all scenes
  - [x] 3.3: Duplicate job detection (in-memory + DB)
  - [x] 3.4: Create DB job record and start background goroutine
- [x] Task 4: Implement executeTTSGeneration background worker (AC: #2, #6)
  - [x] 4.1: Iterate scenes with context cancellation checks
  - [x] 4.2: Call ttsSvc.SynthesizeScene per scene
  - [x] 4.3: Update progress and fire webhooks on completion/failure
- [x] Task 5: Add helper functions (AC: #1, #2, #3)
  - [x] 5.1: `makeSceneRange(count)` — generates [1..count] slice
  - [x] 5.2: `validateSceneNumbers(scenes, maxScene)` — validates scene number range
  - [x] 5.3: `parseIntParam(r, name)` — parses URL integer parameter
- [x] Task 6: Write unit tests (AC: #1-#6)
  - [x] 6.1: Test image generation with valid state and scenes
  - [x] 6.2: Test TTS generation with valid state and scenes
  - [x] 6.3: Test 409 CONFLICT for duplicate running jobs
  - [x] 6.4: Test 409 CONFLICT for invalid project state
  - [x] 6.5: Test empty scenes expansion to all scenes
  - [x] 6.6: Test 502 when plugin is unavailable

## Dev Notes

### Key Architecture Decisions

- **State validation maps**: `validImageGenStates` and `validTTSGenStates` are declared as package-level maps for clean state checking
- **Typed job tracking**: Uses `startTyped`/`getByType`/`removeTyped` from Story 18-2 to support concurrent image and TTS jobs
- **Dual duplicate detection**: Checks both in-memory jobManager (fast) and DB (covers server restart scenario)
- **Per-scene progress**: Progress percentage calculated as `completed * 100 / total` after each scene

### References

- [Source: internal/api/assets.go] - Image/TTS generation handlers and background workers
- [Source: internal/service/image_gen.go] - ImageGenService.GenerateSceneImage
- [Source: internal/service/tts.go] - TTSService.SynthesizeScene

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6

### Completion Notes List

- Implemented handleGenerateImages with state validation, duplicate detection, empty scenes expansion
- Implemented executeImageGeneration with per-scene progress, manual prompt reading, webhook notifications
- Implemented handleGenerateTTS with parallel validation pattern
- Implemented executeTTSGeneration with per-scene progress and webhook notifications
- Added makeSceneRange, validateSceneNumbers, parseIntParam helpers
- Both handlers return 202 Accepted immediately, execution runs in background goroutines
- 409 CONFLICT for duplicate jobs includes existing job_id in error message
- All tests pass, no regressions

### File List

- internal/api/assets.go (modified) - handleGenerateImages, handleGenerateTTS, executeImageGeneration, executeTTSGeneration, helpers
- internal/api/assets_test.go (modified) - Generation handler tests with state/duplicate/progress coverage
