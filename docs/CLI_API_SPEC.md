# yt.pipe — CLI & REST API Specification

> Version: 1.0 | Updated: 2026-03-09

---

## Table of Contents

1. [CLI Commands](#1-cli-commands)
2. [REST API Endpoints](#2-rest-api-endpoints)
3. [Authentication](#3-authentication)
4. [Response Format](#4-response-format)
5. [Error Codes](#5-error-codes)

---

## 1. CLI Commands

**Binary**: `yt-pipe`
**Global Flags**: `--config <path>`, `--verbose`, `--json-output`

### 1.1 Initialization & Configuration

| Command | Description | Flags |
|---------|-------------|-------|
| `yt-pipe init` | Interactive configuration wizard | `--force`, `--non-interactive`, `--scp-data-path`, `--workspace-path`, `--llm-api-key`, `--imagegen-api-key`, `--tts-provider`, `--tts-api-key` |
| `yt-pipe config show` | Display merged config (secrets masked) | — |
| `yt-pipe config validate` | Validate configuration values | — |

### 1.2 Pipeline Execution

| Command | Description | Flags |
|---------|-------------|-------|
| `yt-pipe run <scp-id>` | Run full pipeline | `--dry-run`, `--resume <project-id>`, `--auto-approve`, `--skip-approval`, `--force` |
| `yt-pipe serve` | Start HTTP API server | `--port` |

### 1.3 Individual Stage Commands

| Command | Description | Flags |
|---------|-------------|-------|
| `yt-pipe scenario generate <scp-id>` | Generate scenario from SCP data | — |
| `yt-pipe scenario approve <project-id>` | Approve generated scenario | — |
| `yt-pipe image generate <scp-id>` | Generate images for all scenes | `--parallel`, `--force` |
| `yt-pipe image regenerate <scp-id>` | Regenerate specific scene images | `--scenes <3,5,7>`, `--scene <num>`, `--edit-prompt <instruction>` |
| `yt-pipe tts generate <scp-id>` | Synthesize TTS narration | `--force`, `--scenes <3,5>` |
| `yt-pipe tts register-voice` | Register voice clone with DashScope | `--audio <path>` (required), `--name <name>` (required) |
| `yt-pipe subtitle generate <scp-id>` | Generate subtitles | — |
| `yt-pipe assemble <scp-id>` | Assemble CapCut project | `--check-license` |

### 1.4 Scene Management (Epic 16)

| Command | Description | Flags |
|---------|-------------|-------|
| `yt-pipe scenes <project-id>` | Show scene asset mapping dashboard | `--scene <num>` |
| `yt-pipe scenes approve <project-id>` | Approve scene asset | `--type <image\|tts>` (required), `--scene <num>`, `--all` |
| `yt-pipe scenes reject <project-id>` | Reject scene asset | `--type <image\|tts>` (required), `--scene <num>` (required) |

### 1.5 Project Status & Monitoring

| Command | Description | Flags |
|---------|-------------|-------|
| `yt-pipe status <scp-id>` | Project status and progress | `--scenes`, `--json-output` |
| `yt-pipe logs <scp-id>` | Execution history and summary | `--summary`, `--limit <num>` |
| `yt-pipe metrics [scp-id]` | Pipeline metrics and feedback | — |
| `yt-pipe feedback <scp-id>` | Submit quality feedback | `--scene <num>`, `--type <image\|audio\|subtitle\|scenario>`, `--rating <good\|bad\|neutral>`, `--comment <text>` |
| `yt-pipe clean <scp-id>` | Clean workspace files | `--all`, `--dry-run`, `--status` |

### 1.6 Prompt Template Management (Epic 13)

| Command | Description | Flags |
|---------|-------------|-------|
| `yt-pipe template list` | List templates | `--category <scenario\|image\|tts\|caption>` |
| `yt-pipe template show <id>` | Show template content | `--version <num>` |
| `yt-pipe template create` | Create new template | `--category` (required), `--name` (required), `--file` (required) |
| `yt-pipe template update <id>` | Update template (creates new version) | `--file` (required) |
| `yt-pipe template rollback <id>` | Rollback to specific version | `--version <num>` (required) |
| `yt-pipe template delete <id>` | Delete template | — |
| `yt-pipe template override <id>` | Per-project template override | `--project` (required), `--file`, `--delete` |

### 1.7 Character ID Cards (Epic 14)

| Command | Description | Flags |
|---------|-------------|-------|
| `yt-pipe character create` | Create character ID card | `--scp-id` (required), `--name` (required), `--aliases`, `--visual`, `--style`, `--prompt-base` |
| `yt-pipe character list` | List characters | `--scp-id` |
| `yt-pipe character show <id>` | Show character detail | — |
| `yt-pipe character update <id>` | Update character | `--name`, `--aliases`, `--visual`, `--style`, `--prompt-base` |
| `yt-pipe character delete <id>` | Delete character | — |

### 1.8 TTS Mood Presets (Epic 15)

| Command | Description | Flags |
|---------|-------------|-------|
| `yt-pipe mood list` | List mood presets | — |
| `yt-pipe mood create` | Create preset | `--name` (required), `--emotion` (required), `--speed`, `--pitch`, `--description` |
| `yt-pipe mood show <id>` | Show preset detail | — |
| `yt-pipe mood update <id>` | Update preset | `--name`, `--emotion`, `--speed`, `--pitch`, `--description` |
| `yt-pipe mood delete <id>` | Delete preset | — |
| `yt-pipe mood review <project-id>` | Review auto-mapped mood assignments | `--confirm-all`, `--confirm <num>`, `--reassign <num>`, `--preset <id>` |

### 1.9 BGM Library (Epic 17)

| Command | Description | Flags |
|---------|-------------|-------|
| `yt-pipe bgm add` | Register BGM | `--name` (required), `--file` (required), `--moods`, `--license-type`, `--credit`, `--source`, `--duration` |
| `yt-pipe bgm list` | List BGMs | `--mood <tag>` |
| `yt-pipe bgm show <id>` | Show BGM detail | — |
| `yt-pipe bgm update <id>` | Update BGM | `--name`, `--moods`, `--license-type`, `--credit` |
| `yt-pipe bgm delete <id>` | Delete BGM | — |
| `yt-pipe bgm review <project-id>` | Review BGM recommendations | `--confirm-all`, `--confirm <num>`, `--reassign <num>`, `--bgm <id>`, `--adjust <num>`, `--volume <dB>`, `--fade-in <ms>`, `--fade-out <ms>`, `--ducking <dB>` |

---

## 2. REST API Endpoints

**Base URL**: `http://{host}:{port}`
**Default**: `http://localhost:8080`
**Content-Type**: `application/json`

### 2.1 Health & Readiness

| Method | Path | Description | Auth | Status |
|--------|------|-------------|------|--------|
| `GET` | `/health` | Server health check | No | `200` |
| `GET` | `/ready` | Readiness probe (DB + workspace) | No | `200` / `503` |

**`GET /health` Response**:
```json
{ "status": "ok", "version": "dev" }
```

**`GET /ready` Response**:
```json
{ "status": "ready" }
```

### 2.2 Projects

| Method | Path | Description | Auth | Status |
|--------|------|-------------|------|--------|
| `POST` | `/api/v1/projects` | Create project | Yes | `201` |
| `GET` | `/api/v1/projects` | List projects | Yes | `200` |
| `GET` | `/api/v1/projects/{id}` | Get project detail | Yes | `200` |
| `DELETE` | `/api/v1/projects/{id}` | Delete project | Yes | `200` |

**`POST /api/v1/projects` Request**:
```json
{ "scp_id": "SCP-173" }
```

**`GET /api/v1/projects` Query Params**:
| Param | Type | Default | Description |
|-------|------|---------|-------------|
| `state` | string | — | Filter by project state |
| `scp_id` | string | — | Filter by SCP ID |
| `limit` | int | 20 | Max results (max: 100) |
| `offset` | int | 0 | Pagination offset |

**`DELETE /api/v1/projects/{id}`**: Only `pending` or `complete` state projects can be deleted.

### 2.3 Pipeline Control

| Method | Path | Description | Auth | Status |
|--------|------|-------------|------|--------|
| `POST` | `/api/v1/projects/{id}/run` | Start pipeline (async) | Yes | `202` |
| `GET` | `/api/v1/projects/{id}/status` | Real-time pipeline status | Yes | `200` |
| `POST` | `/api/v1/projects/{id}/cancel` | Cancel running pipeline | Yes | `200` |
| `POST` | `/api/v1/projects/{id}/approve` | Approve scenario | Yes | `200` |

**`POST /api/v1/projects/{id}/run` Request**:
```json
{ "dryRun": false }
```

**`POST /api/v1/projects/{id}/run` Response** (202):
```json
{
  "job_id": "uuid",
  "project_id": "uuid",
  "status": "running"
}
```

**`GET /api/v1/projects/{id}/status` Response**:
```json
{
  "project_id": "uuid",
  "scp_id": "SCP-173",
  "state": "generating",
  "job_id": "uuid",
  "job_status": "running",
  "stage": "image_generate",
  "progress_pct": 62.5,
  "scenes_total": 8,
  "scenes_complete": 5,
  "elapsed_sec": 45.2
}
```

### 2.4 Scene Dashboard & Approval (Epic 16)

| Method | Path | Description | Auth | Status |
|--------|------|-------------|------|--------|
| `GET` | `/api/v1/projects/{id}/scenes` | Scene asset dashboard | Yes | `200` |
| `POST` | `/api/v1/projects/{id}/scenes/{num}/approve` | Approve scene asset | Yes | `200` |
| `POST` | `/api/v1/projects/{id}/scenes/{num}/reject` | Reject scene asset | Yes | `200` |

**`POST .../scenes/{num}/approve` Query Params**: `type=image|tts` (required)

**`GET .../scenes` Response**:
```json
{
  "project_id": "uuid",
  "project_status": "generating",
  "scenes": [
    {
      "scene_num": 1,
      "image_status": "approved",
      "image_path": "scenes/1/image.png",
      "tts_status": "pending",
      "tts_path": "",
      "image_attempts": 1,
      "tts_attempts": 0,
      "mood_preset": "tense",
      "bgm_name": "Dark Ambient"
    }
  ],
  "image_summary": { "total": 8, "approved": 5 },
  "tts_summary": { "total": 8, "approved": 3 }
}
```

### 2.5 Asset Management

| Method | Path | Description | Auth | Status |
|--------|------|-------------|------|--------|
| `POST` | `/api/v1/projects/{id}/images/generate` | Selective image regeneration | Yes | `202` |
| `POST` | `/api/v1/projects/{id}/tts/generate` | Selective TTS regeneration | Yes | `202` |
| `PUT` | `/api/v1/projects/{id}/scenes/{num}/prompt` | Update image prompt | Yes | `200` |
| `POST` | `/api/v1/projects/{id}/feedback` | Submit feedback | Yes | `201` |

**`POST .../images/generate` Request**:
```json
{ "scenes": [3, 5, 7] }
```

**`PUT .../scenes/{num}/prompt` Request**:
```json
{ "prompt": "A dark corridor with SCP-173 statue..." }
```

**`POST .../feedback` Request**:
```json
{
  "scene_num": 3,
  "asset_type": "image",
  "rating": "good",
  "comment": "Great atmosphere"
}
```

### 2.6 Configuration & Plugins

| Method | Path | Description | Auth | Status |
|--------|------|-------------|------|--------|
| `GET` | `/api/v1/config` | Get config (secrets masked) | Yes | `200` |
| `PATCH` | `/api/v1/config` | Partial config update | Yes | `200` |
| `GET` | `/api/v1/plugins` | List registered plugins | Yes | `200` |
| `PUT` | `/api/v1/plugins/{type}/active` | Switch active plugin | Yes | `200` |

**`PATCH /api/v1/config` Mutable Fields**:
| Field | Type | Constraint |
|-------|------|------------|
| `log_level` | string | `debug`, `info`, `warn`, `error` |
| `log_format` | string | `json`, `text` |
| `llm.model` | string | — |
| `llm.temperature` | float | — |
| `llm.max_tokens` | int | — |
| `tts.voice` | string | — |
| `tts.speed` | float | — |

**`GET /api/v1/plugins` Response**:
```json
{
  "plugins": [
    { "type": "llm", "active": "gemini", "available": ["gemini", "qwen", "deepseek"] },
    { "type": "imagegen", "active": "siliconflow", "available": ["siliconflow"] },
    { "type": "tts", "active": "dashscope", "available": ["dashscope"] },
    { "type": "output", "active": "capcut", "available": ["capcut"] }
  ]
}
```

**`PUT /api/v1/plugins/{type}/active` Request**:
```json
{ "provider": "qwen" }
```

---

## 3. Authentication

| Config Key | Environment Variable | Default |
|------------|---------------------|---------|
| `api.auth.enabled` | `YTP_API_AUTH_ENABLED` | `false` |
| `api.auth.key` | `YTP_API_AUTH_KEY` | — |

When enabled, all `/api/v1/*` endpoints require the `Authorization: Bearer <key>` header.
`/health` and `/ready` are always public.

### Middleware Stack

```
Request → Recovery → RequestID → Logging → Auth → Handler
```

| Middleware | Function |
|-----------|----------|
| **Recovery** | Catch panics, return 500 |
| **RequestID** | Generate UUID per request |
| **Logging** | Structured request logging |
| **Auth** | API key validation (if enabled) |

---

## 4. Response Format

All API responses follow a consistent JSON envelope:

```json
{
  "success": true,
  "data": { ... },
  "error": { "code": "NOT_FOUND", "message": "project not found" },
  "timestamp": "2026-03-09T12:00:00Z",
  "request_id": "550e8400-e29b-41d4-a716-446655440000"
}
```

---

## 5. Error Codes

| Code | HTTP | Description |
|------|------|-------------|
| `NOT_FOUND` | 404 | Resource not found |
| `VALIDATION_ERROR` | 400 | Input validation failed |
| `INVALID_REQUEST` | 400 | Malformed request |
| `CONFLICT` | 409 | State transition conflict |
| `UNAUTHORIZED` | 401 | Missing or invalid API key |
| `INTERNAL_ERROR` | 500 | Server internal error |
| `DB_UNAVAILABLE` | 503 | Database inaccessible |
| `WORKSPACE_UNAVAILABLE` | 503 | Workspace inaccessible |
