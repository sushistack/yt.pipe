# Story 7-2: Project CRUD API

## Status: Done

## Implementation Summary

### New Files
- `internal/api/projects.go` — REST handlers for project CRUD: `POST /api/v1/projects`, `GET /api/v1/projects`, `GET /api/v1/projects/:id`, `DELETE /api/v1/projects/:id`
- `internal/api/projects_test.go` — Handler tests with mock store for all CRUD operations

### Modified Files
- `internal/store/project.go` — Added `ListByState()`, `ListBySCPID()` for query filtering, `Delete()` for project archival

### Architecture Decisions
- Project listing supports filtering via query params: `?state=approved`, `?scp_id=SCP-173`
- Pagination with `?limit=20&offset=0` (default limit 20)
- DELETE archives project (marks as archived in SQLite, not hard delete)
- 404 response includes `project_id` for debugging
- 400 response includes field-level validation error details

### Acceptance Criteria Met
- [x] `POST /api/v1/projects` creates project and returns 201
- [x] `GET /api/v1/projects/:id` returns full project JSON
- [x] `GET /api/v1/projects` returns paginated list with filtering
- [x] `DELETE /api/v1/projects/:id` archives project
- [x] 404 for missing projects, 400 for validation errors
- [x] All tests pass
