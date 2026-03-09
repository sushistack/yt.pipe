# API Contracts — yt.pipe

> Auto-generated project documentation (2026-03-09)

## Overview

yt.pipe exposes a RESTful HTTP API via the **chi/v5** router with 20 endpoints across 6 functional domains. The API follows a consistent JSON envelope format and supports Bearer token authentication.

**Base URL**: `http://{host}:{port}` (default: `localhost:8080`)

## Middleware Stack (Applied in Order)

| Order | Middleware | Description |
|-------|-----------|-------------|
| 1 | RecoveryMiddleware | Catches panics, returns 500 |
| 2 | RequestIDMiddleware | UUID per request → `X-Request-ID` header |
| 3 | LoggingMiddleware | Structured log with method, path, status, duration |
| 4 | AuthMiddleware | Bearer token auth (configurable on/off) |

## Authentication

- **Type**: Bearer Token via `Authorization: Bearer {token}`
- **Config**: `api.auth.enabled` (bool), `api.auth.key` (string)
- **Exempt**: `/health`, `/ready` always bypass auth
- **Failure**: HTTP 401 with `UNAUTHORIZED` code
- **Security**: Constant-time comparison (timing attack prevention)

## Standard Response Envelope

```json
{
  "success": true,
  "data": { ... },
  "error": { "code": "string", "message": "string" },
  "timestamp": "2026-03-09T...",
  "request_id": "uuid"
}
```

---

## Endpoints

### Health & Readiness (Public)

| Method | Path | Description | Auth |
|--------|------|-------------|------|
| GET | `/health` | Health check → `{"status":"ok","version":"..."}` | No |
| GET | `/ready` | Readiness check (DB + workspace) | No |

### Projects (CRUD)

| Method | Path | Description | Auth |
|--------|------|-------------|------|
| POST | `/api/v1/projects` | Create project `{"scp_id":"..."}` → 201 | Yes |
| GET | `/api/v1/projects` | List projects (filter: `state`, `scp_id`, pagination) | Yes |
| GET | `/api/v1/projects/{id}` | Get project by ID | Yes |
| DELETE | `/api/v1/projects/{id}` | Delete project (pending/complete only) | Yes |

### Pipeline Control

| Method | Path | Description | Auth |
|--------|------|-------------|------|
| POST | `/api/v1/projects/{id}/run` | Start pipeline (async job) → 202 | Yes |
| GET | `/api/v1/projects/{id}/status` | Pipeline progress (stage, %, scenes) | Yes |
| POST | `/api/v1/projects/{id}/cancel` | Cancel running pipeline | Yes |
| POST | `/api/v1/projects/{id}/approve` | Approve scenario → state transition | Yes |

### Scene Dashboard & Approval

| Method | Path | Description | Auth |
|--------|------|-------------|------|
| GET | `/api/v1/projects/{id}/scenes` | Scene dashboard with approval statuses | Yes |
| POST | `/api/v1/projects/{id}/scenes/{num}/approve` | Approve scene asset (`?type=image\|tts`) | Yes |
| POST | `/api/v1/projects/{id}/scenes/{num}/reject` | Reject scene asset (`?type=image\|tts`) | Yes |

### Asset Management

| Method | Path | Description | Auth |
|--------|------|-------------|------|
| POST | `/api/v1/projects/{id}/images/generate` | Regenerate images for scenes → 202 | Yes |
| POST | `/api/v1/projects/{id}/tts/generate` | Regenerate TTS for scenes → 202 | Yes |
| PUT | `/api/v1/projects/{id}/scenes/{num}/prompt` | Update image prompt | Yes |
| POST | `/api/v1/projects/{id}/feedback` | Submit asset feedback | Yes |

### Configuration & Plugins

| Method | Path | Description | Auth |
|--------|------|-------------|------|
| GET | `/api/v1/config` | Get config (keys masked) | Yes |
| PATCH | `/api/v1/config` | Partial config update (mutable fields only) | Yes |
| GET | `/api/v1/plugins` | List plugins by type with active provider | Yes |
| PUT | `/api/v1/plugins/{type}/active` | Switch active plugin provider | Yes |

---

## Error Codes

| Code | HTTP | Description |
|------|------|-------------|
| `UNAUTHORIZED` | 401 | Missing/invalid Bearer token |
| `NOT_FOUND` | 404 | Resource not found |
| `INVALID_REQUEST` | 400 | Malformed JSON body |
| `VALIDATION_ERROR` | 400 | Business rule violation |
| `CONFLICT` | 409 | Invalid state transition |
| `DB_UNAVAILABLE` | 503 | Database connection failed |
| `WORKSPACE_UNAVAILABLE` | 503 | Workspace directory not accessible |
| `INTERNAL_ERROR` | 500 | Unexpected server error |

## Webhook Notifications

On project state transitions, webhook events fire to configured URLs:

```json
{
  "event": "state_change",
  "project_id": "string",
  "scp_id": "string",
  "previous_state": "string",
  "new_state": "string",
  "timestamp": "RFC3339"
}
```

- Fire-and-forget delivery (async goroutine)
- Exponential backoff retry: 1s → 2s → 4s (max 3 retries)

## Server Configuration

| Parameter | Default | Description |
|-----------|---------|-------------|
| `api.host` | localhost | Bind address |
| `api.port` | 8080 | Listen port |
| Read Timeout | 15s | HTTP read timeout |
| Write Timeout | 30s | HTTP write timeout |
| Idle Timeout | 60s | HTTP idle timeout |
