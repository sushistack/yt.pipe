# Story 7-7: API Authentication Middleware

## Status: Done

## Implementation Summary

### New Files
- `internal/api/auth.go` — Bearer token authentication middleware: validates `Authorization: Bearer <key>` header, configurable via `api.auth.enabled` and `api.auth.key` (or `YTP_API_KEY` env var), excludes `/health` and `/ready` from auth
- `internal/api/auth_test.go` — Tests for valid/invalid key, missing header, health endpoint exclusion, disabled auth mode

### Architecture Decisions
- Middleware pattern: wraps Chi router, applied to `/api/v1/*` routes only
- Health/readiness endpoints excluded from auth (for load balancers/monitoring)
- Failed auth attempts logged with client IP and timestamp (key value never logged)
- Successful auth logs key prefix only for identification
- When `api.auth.enabled: false`, all endpoints accessible without key + startup warning logged
- API key sourced from config or `YTP_API_KEY` environment variable

### Acceptance Criteria Met
- [x] 401 for requests without `Authorization` header (when auth enabled)
- [x] 401 for invalid API key with security audit logging
- [x] Successful auth with `Authorization: Bearer <valid-key>`
- [x] `/health` and `/ready` accessible without auth
- [x] Auth disabled mode with startup warning
- [x] All tests pass
