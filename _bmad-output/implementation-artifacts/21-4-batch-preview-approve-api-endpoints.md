# Story 21.4: Batch Preview & Approve API Endpoints

Status: ready-for-dev

## Story

As a n8n workflow orchestrator,
I want REST API endpoints for batch preview and approval,
so that external automation tools can perform bulk scene review and approval.

## Acceptance Criteria

1. **Given** the API server is running with initialized services
   **When** `GET /api/v1/projects/{id}/preview?asset_type=image` is called
   **Then** the response contains a JSON array of `BatchPreviewItem` objects
   **And** response conforms to n8n-compatible flat JSON structure (NFR11)

2. **Given** a valid project with generated scenes
   **When** `POST /api/v1/projects/{id}/batch-approve` is called with `{ "asset_type": "image", "flagged_scenes": [3, 7] }`
   **Then** all non-flagged scenes are approved
   **And** the response includes `{ approved: N, flagged: M }`

3. **Given** `flagged_scenes` is an empty array
   **When** batch approve is called via API
   **Then** all scenes are approved (same as CLI `none`)

4. **Given** an invalid project ID
   **When** either endpoint is called
   **Then** `404 NOT_FOUND` is returned with consistent error structure (FR40)

## Tasks / Subtasks

- [ ] Task 1: Add GET /api/v1/projects/{id}/preview handler (AC: #1, #4)
  - [ ] 1.1 Add handler `handleBatchPreview` to review.go
  - [ ] 1.2 Read `asset_type` query param (default: image)
  - [ ] 1.3 Call `ApprovalService.GetBatchPreview()`
  - [ ] 1.4 Return flat JSON array via WriteJSON

- [ ] Task 2: Add POST /api/v1/projects/{id}/batch-approve handler (AC: #2, #3, #4)
  - [ ] 2.1 Add handler `handleBatchApprove` to review.go
  - [ ] 2.2 Parse JSON body with asset_type and flagged_scenes
  - [ ] 2.3 Call `ApprovalService.BatchApprove()`
  - [ ] 2.4 Return result via WriteJSON

- [ ] Task 3: Register routes in server.go (AC: #1, #2)
  - [ ] 3.1 Add GET /projects/{id}/preview route
  - [ ] 3.2 Add POST /projects/{id}/batch-approve route
  - [ ] 3.3 Add to reviewScopedRoutes for auth

- [ ] Task 4: Tests (all ACs)
  - [ ] 4.1 Test GET preview endpoint returns BatchPreviewItem array
  - [ ] 4.2 Test POST batch-approve with flagged scenes
  - [ ] 4.3 Test POST batch-approve with empty flagged_scenes
  - [ ] 4.4 Test 404 for invalid project ID

## Dev Notes

### Existing Patterns
- Handler signature: `func (s *Server) handleXxx(w http.ResponseWriter, r *http.Request)`
- Service creation in handler: `service.NewApprovalService(s.store, slog.Default())`
- Response: `WriteJSON(w, r, statusCode, data)` / `WriteError(w, r, statusCode, code, msg)`
- Error mapping: `writeServiceError(w, r, err)` handles NotFoundError → 404
- Auth: `requireReviewAuth(s, w, r, projectID)` for dual Bearer/review-token auth
- Rate limit: `checkReadRateLimit` for GET, `checkMutationRateLimit` for POST

### File Structure
- MODIFY: `internal/api/review.go` — add handleBatchPreview, handleBatchApprove
- MODIFY: `internal/api/server.go` — register routes
- MODIFY: `internal/api/auth.go` — add to reviewScopedRoutes

## Dev Agent Record

### Agent Model Used

### Completion Notes List

### File List
