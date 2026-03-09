# Story 18.4: Assembly Endpoint and Prompt Update Persistence

Status: done

## Story

As a n8n workflow orchestrator,
I want to trigger CapCut project assembly and update image prompts via API,
so that the final video project can be assembled after all scenes are approved and prompts can be refined iteratively.

## Acceptance Criteria

1. **Given** POST /projects/{id}/assemble is called
   **When** the project is in "tts_review" or "approved" state
   **And** all scenes have approved images and TTS
   **Then** a background assembly job is started
   **And** 202 Accepted is returned with a job_id

2. **Given** not all scenes have approved assets
   **When** POST /assemble is called
   **Then** the handler returns 409 INVALID_STATE listing which asset types are not fully approved

3. **Given** PUT /projects/{id}/scenes/{num}/prompt is called with a new prompt
   **When** the scene number is valid
   **Then** the prompt is persisted to workspace as `scenes/{num}/prompt.txt`
   **And** the content hash in the manifest is invalidated for incremental build detection
   **And** the response includes the updated prompt and timestamp

4. **Given** an assembly job completes
   **When** the assembler service produces a CapCut project
   **Then** the job record is updated to "complete" with the output path
   **And** a job_complete webhook is fired

5. **Given** an assembly job fails
   **When** the error occurs
   **Then** the job record is updated to "failed" with the error message
   **And** a job_failed webhook is fired

## Tasks / Subtasks

- [x] Task 1: Implement handleAssemble endpoint (AC: #1, #2)
  - [x] 1.1: Validate project state against validAssemblyStates
  - [x] 1.2: Check all scenes have approved images via store.AllApproved
  - [x] 1.3: Check all scenes have approved TTS via store.AllApproved
  - [x] 1.4: Return 409 with descriptive message if approvals incomplete
  - [x] 1.5: Check for duplicate running assembly job (in-memory + DB)
  - [x] 1.6: Create DB job record and start background goroutine
- [x] Task 2: Implement executeAssembly background worker (AC: #4, #5)
  - [x] 2.1: Load scenes from workspace directory via loadScenesFromWorkspace
  - [x] 2.2: Call assemblerSvc.Assemble with loaded scenes
  - [x] 2.3: Update job record and fire webhook on complete/fail
  - [x] 2.4: Handle panic recovery in defer
- [x] Task 3: Implement loadScenesFromWorkspace utility (AC: #4)
  - [x] 3.1: Read scene directories from workspace
  - [x] 3.2: Parse manifest.json per scene directory into domain.Scene
- [x] Task 4: Implement handleUpdatePrompt endpoint (AC: #3)
  - [x] 4.1: Parse project ID and scene number from URL
  - [x] 4.2: Validate scene number range
  - [x] 4.3: Write prompt to workspace via workspace.WriteFileAtomic
  - [x] 4.4: Invalidate content hash in manifest store
  - [x] 4.5: Return updated prompt with timestamp
- [x] Task 5: Write unit tests (AC: #1-#5)
  - [x] 5.1: Test assembly with fully approved scenes
  - [x] 5.2: Test 409 for unapproved scenes (images, TTS, both)
  - [x] 5.3: Test 409 for invalid project state
  - [x] 5.4: Test prompt update with workspace persistence
  - [x] 5.5: Test prompt validation (empty, invalid scene number)

## Dev Notes

### Key Architecture Decisions

- **Approval gate**: Assembly requires both image AND TTS approvals for all scenes; error message lists which types are missing
- **Workspace file I/O**: Uses workspace.WriteFileAtomic for prompt persistence (atomic write to prevent corruption)
- **Content hash invalidation**: Setting manifest.ContentHash to "" triggers incremental build to regenerate the scene
- **Scene loading**: loadScenesFromWorkspace reads manifest.json from each scene directory, skipping unreadable entries

### References

- [Source: internal/api/assets.go] - handleAssemble, executeAssembly, handleUpdatePrompt, loadScenesFromWorkspace
- [Source: internal/service/assembler.go] - AssemblerService.Assemble
- [Source: internal/workspace/] - WriteFileAtomic utility

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6

### Completion Notes List

- Implemented handleAssemble with dual approval validation (images + TTS)
- executeAssembly loads scenes from workspace, calls assemblerSvc.Assemble, fires webhooks
- Implemented loadScenesFromWorkspace and parseSceneManifestJSON utilities
- Implemented handleUpdatePrompt with atomic file write and content hash invalidation
- Route: POST /projects/{id}/assemble, PUT /projects/{id}/scenes/{num}/prompt
- All tests pass, no regressions

### File List

- internal/api/assets.go (modified) - handleAssemble, executeAssembly, loadScenesFromWorkspace, parseSceneManifestJSON, handleUpdatePrompt
- internal/api/server.go (modified) - Route registration for /assemble and /scenes/{num}/prompt
- internal/api/assets_test.go (modified) - Assembly and prompt update tests
