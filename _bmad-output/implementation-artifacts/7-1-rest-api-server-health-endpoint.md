# Story 7-1: REST API Server & Health Endpoint

## Status: Done

## Implementation Summary

### New Files
- `internal/api/server.go` — HTTP server setup with Chi router, graceful shutdown (30s timeout), configurable port (default 8080)
- `internal/api/server_test.go` — Server startup, shutdown, and configuration tests
- `internal/api/health.go` — `GET /health` (200 + version) and `GET /ready` (SQLite + workspace checks, 503 on failure)
- `internal/api/middleware.go` — Request ID generation (`X-Request-ID` header), structured logging middleware (method, path, status, duration)
- `internal/api/response.go` — Standard JSON response helpers (success, error, validation error)
- `internal/api/doc.go` — Package documentation
- `internal/cli/serve_cmd.go` — `yt-pipe serve` CLI command with `--port` flag

### Architecture Decisions
- Chi router chosen for lightweight, stdlib-compatible HTTP routing
- Server reuses the same service layer instances as CLI (no code duplication)
- All responses use `Content-Type: application/json`
- Health endpoint is unauthenticated (for load balancers/monitoring)
- Graceful shutdown: stops accepting new requests, waits for in-flight (max 30s), then exits

### Acceptance Criteria Met
- [x] `yt-pipe serve` starts HTTP server on configurable port
- [x] `GET /health` returns 200 with `{"status": "ok", "version": "..."}`
- [x] `GET /ready` checks SQLite connectivity and workspace directory
- [x] Request ID generated and included in logs and response headers
- [x] Graceful shutdown on SIGTERM/SIGINT
- [x] All tests pass
