# Story 7-3: Pipeline Control API

## Status: Done

## Implementation Summary

### New Files
- `internal/api/pipeline.go` — Pipeline control handlers: `POST /api/v1/projects/:id/run` (async start), `GET /api/v1/projects/:id/status` (real-time status), `POST /api/v1/projects/:id/cancel` (context cancellation), `POST /api/v1/projects/:id/approve` (scenario approval gate)
- `internal/api/pipeline_test.go` — Tests for pipeline start, status polling, cancellation, and duplicate run detection

### Architecture Decisions
- Pipeline runs asynchronously in background goroutine, returns 202 immediately
- Status endpoint returns: current stage, progress percentage, scenes completed, elapsed time
- Cancel triggers context cancellation; current stage checkpoints before stopping
- 409 Conflict returned if pipeline already running for the project
- Approval endpoint resumes pipeline past the human-in-the-loop gate
- Reuses the same pipeline orchestrator as `yt-pipe run`

### Acceptance Criteria Met
- [x] `POST /api/v1/projects/:id/run` starts async pipeline, returns 202
- [x] `GET /api/v1/projects/:id/status` returns real-time progress
- [x] `POST /api/v1/projects/:id/cancel` cancels with checkpoint
- [x] 409 for duplicate run attempts
- [x] `POST /api/v1/projects/:id/approve` triggers scenario approval
- [x] All tests pass
