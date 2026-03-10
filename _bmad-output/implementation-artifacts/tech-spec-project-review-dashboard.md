---
title: 'Project Review Dashboard'
slug: 'project-review-dashboard'
created: '2026-03-10'
status: 'implementation-complete'
stepsCompleted: [1, 2, 3, 4]
tech_stack:
  - 'Go html/template + embed.FS'
  - 'Tailwind CSS (pre-built, embedded)'
  - 'chi router'
  - 'SQLite'
files_to_modify:
  - 'internal/store/migrations/008_review_token.sql (new) — add review_token column to projects'
  - 'internal/domain/project.go (modify) — add ReviewToken field'
  - 'internal/store/project.go (modify) — read/write review_token in CRUD queries'
  - 'internal/service/project.go (modify) — auto-generate review_token on CreateProject'
  - 'internal/api/review.go (new) — review page handler + review token auth helper + asset serving + narration/scene CRUD'
  - 'internal/api/templates/review.html (new) — server-rendered HTML template'
  - 'internal/api/templates/styles.css (new) — pre-built Tailwind CSS bundle'
  - 'internal/api/server.go (modify) — register review routes + embed.FS + auth exemption'
  - 'internal/api/auth.go (modify) — exempt /review/ paths from Bearer auth'
  - 'internal/api/webhook.go (modify) — add review_url field to all webhook event structs'
  - 'internal/service/review.go (new) — ReviewService: narration update, scene add/delete, reject+regenerate'
  - 'internal/service/review_test.go (new) — ReviewService unit tests'
  - 'internal/api/review_test.go (new) — handler tests'
code_patterns:
  - 'chi router handler methods on Server struct'
  - 'WriteJSON/WriteError response helpers'
  - 'SceneDashboardService for data aggregation'
  - 'embed.FS for static assets'
test_patterns:
  - 'testify assertions'
  - 'httptest for handler testing'
  - 'same-package _test.go files'
---

# Tech-Spec: Project Review Dashboard

**Created:** 2026-03-10

## Overview

### Problem Statement

The yt.pipe pipeline generates content assets (scenarios, images, TTS audio) per scene, but there is no UI to visually review these assets. Users must approve/reject via raw API calls, making the review cycle slow and error-prone. When n8n sends Slack/Telegram notifications, there's no link to click for immediate visual review.

### Solution

Add a server-rendered HTML review page that displays all scenes with inline image previews, audio players, and editable narration text. Support per-scene approve/reject actions, scene add/delete, narration editing, and automatic regeneration on rejection. Authenticate via per-project review tokens embedded in URLs for easy sharing via n8n notifications.

### Scope

**In Scope:**
- Server-rendered review dashboard page (`GET /review/{project_id}?token=xxx`)
- Per-scene display: image preview, TTS audio player, narration text, image prompt, approval status badges
- Per-scene approve/reject for image and TTS with auto-regeneration trigger on reject
- Narration inline editing with save/cancel
- Scene append (add to end) and delete (with cascade, gap-tolerant numbering)
- Scenario-level full approval (state transition)
- Asset file serving endpoints (image, audio)
- Per-project review token authentication (UUID, stored in projects table)
- Review token scoped whitelist for allowed endpoints
- Webhook `review_url` field addition
- `embed.FS` HTML templates + pre-built Tailwind CSS
- Responsive layout (mobile-friendly for Slack/Telegram link clicks)
- Deployment guide with nginx reverse proxy config for external exposure

**Out of Scope:**
- SPA framework / separate frontend build pipeline
- Real-time WebSocket updates
- Multi-user concurrent editing
- Direct image/audio upload (pipeline generation only)
- Scene reindexing on delete (gaps allowed)
- Mid-list scene insertion (append only)

## Context for Development

### Codebase Patterns

- **Route registration:** Chi router with middleware stack in `server.go` `setupRouter()`; handlers are `Server` methods
- **Response format:** Standardized JSON envelope via `WriteJSON()` and `WriteError()` helpers
- **Data access:** Service layer abstracts store; `SceneDashboardService.GetDashboard()` already aggregates per-scene approval data, image/TTS paths, prompts, mood presets
- **Asset storage:** Filesystem-based via workspace module; convention: `{projectPath}/scenes/{sceneNum}/image.png`, `audio.mp3`, `subtitle.srt`
- **Approval state machine:** `scene_approvals` table tracks image/TTS independently; statuses: pending → generated → approved (or rejected → generated for retry)
- **Project state machine:** pending → scenario_review → approved → image_review → tts_review → assembling → complete
- **Webhook pattern:** `fanOut()` sends events to all configured URLs in parallel with retry
- **Auth:** Bearer token middleware; health/ready endpoints exempt
- **Scenario data:** `scenario.json` in workspace directory, loaded by dashboard service
- **Domain errors:** `NotFoundError` (404), `ValidationError` (400), `TransitionError` (409), `PluginError` (500/502)

### Files to Reference

| File | Purpose |
| ---- | ------- |
| `internal/api/server.go` | Route registration pattern, middleware stack |
| `internal/api/scenes.go` | Scene approval handler examples (`handleApproveScene`, `handleRejectScene`) |
| `internal/api/assets.go` | Asset handling, prompt read/write, manifest pattern |
| `internal/api/auth.go` | Bearer token middleware, exempt path pattern |
| `internal/api/response.go` | `WriteJSON()`, `WriteError()` helpers |
| `internal/api/webhook.go` | Webhook event structure, `fanOut()` pattern |
| `internal/service/scene_dashboard.go` | `SceneDashboardService.GetDashboard()` — primary data source |
| `internal/service/approval.go` | `ApprovalService` — approve/reject business logic |
| `internal/domain/project.go` | Project state machine, `CanTransition()` |
| `internal/domain/scene.go` | Scene model fields |
| `internal/domain/scene_approval.go` | Approval state machine, asset type constants |
| `internal/store/store.go` | DB schema, migrations |
| `internal/store/scene_approval.go` | Approval CRUD operations |
| `internal/workspace/project.go` | Workspace file I/O, path conventions |
| `internal/config/types.go` | Config structure, `WebhookConfig` |

### Technical Decisions

1. **Review token auth:** UUID stored as `review_token` column in `projects` table. Auto-generated on project creation. Passed as `?token=xxx` query parameter. `AuthMiddleware` passes through requests with `?token=` query parameter (no Bearer check); actual token validation happens in handler against project's stored token.

2. **Token scoped whitelist:** Review token grants access ONLY to:
   - `GET /review/{project_id}` — page render
   - `GET /api/v1/projects/{id}/scenes/{num}/image` — image serving
   - `GET /api/v1/projects/{id}/scenes/{num}/audio` — audio serving
   - `POST /api/v1/projects/{id}/scenes/{num}/approve` — approve
   - `POST /api/v1/projects/{id}/scenes/{num}/reject` — reject
   - `PATCH /api/v1/projects/{id}/scenes/{num}/narration` — narration edit
   - `POST /api/v1/projects/{id}/scenes` — scene append
   - `DELETE /api/v1/projects/{id}/scenes/{num}` — scene delete
   - `POST /api/v1/projects/{id}/approve` — full scenario approval

3. **Token lifecycle (F1 fix):** Token does NOT auto-expire but CAN be revoked/rotated. Add `POST /api/v1/projects/{id}/review-token/rotate` endpoint (Bearer auth only, NOT review-token accessible) that generates a new UUID and invalidates the old token. Old review URLs immediately stop working. This is the escape hatch for leaked tokens.

4. **Scene deletion:** Cascade delete `scene_approvals` and `scene_manifests` records. Scene numbers maintain gaps (no reindex).

5. **Scene addition:** Append only (next scene_num = max existing + 1). Creates empty narration, no assets.

6. **Auto-regeneration on reject:** Reject action triggers a new Job record for the rejected asset type (image or TTS) for that specific scene. Pipeline runner picks up the job asynchronously.

7. **External exposure:** Application code doesn't need changes. Nginx/Caddy reverse proxy routes `/review/*` and scoped API paths externally; all other `/api/v1/*` internal only. Deployment guide included.

8. **HTML rendering:** `embed.FS` in `internal/api/` embeds `templates/` directory. Single `review.html` template with pre-built Tailwind CSS (NOT CDN — see F10). All interactions via `fetch()` calls to JSON API endpoints (same origin). **CSRF protection (F3 fix):** All mutation `fetch()` calls MUST include a custom header `X-Review-Token: {token}`. Handlers verify this header matches the `?token=` query param. This prevents cross-origin form submissions (browsers don't add custom headers to simple requests without CORS preflight, which the server doesn't allow).

9. **Narration editing:** Updates `scenario.json` in workspace. Service layer handles file I/O. **XSS strategy (F4 fix):** Do NOT call `html.EscapeString()` on input. Instead, store narration as plain text. Go's `html/template` auto-escapes on output — this is the correct defense. Input validation: reject null bytes, enforce max 10000 chars, but do NOT transform the text. This avoids double-encoding. **Concurrency (F5 fix):** `ReviewService` holds a `sync.Mutex` per project (keyed by projectID in a `sync.Map`). All `scenario.json` read-modify-write operations acquire the mutex first. Additionally, before writing, create a `.bak` copy of `scenario.json` (F12 fix) for recovery.

10. **Webhook enhancement:** All webhook events include `review_url` field: `{base_url}/review/{project_id}?token={review_token}`.

11. **Service separation (Party Mode 2 consensus):** Scene CRUD and narration editing go into `ReviewService` (new `internal/service/review.go`), NOT into `SceneDashboardService`. Dashboard service stays read-only. `ReviewService` composes `ApprovalService` + workspace I/O + Job creation.

12. **Image prompt display:** `SceneDashboardEntry.Prompt` currently holds narration text, NOT image prompt. Image prompts are in `{sceneDir}/prompt.txt`. Dashboard service to load image prompts into a new `ImagePrompt` field. UI shows narration prominently with collapsible image prompt section.

13. **Duplicate regeneration job prevention:** Before creating a regen Job on reject, check for existing running jobs of same type+scene (reuse pattern from `assets.go` duplicate job checks).

14. **Existing project token backfill:** Migration `008_review_token.sql` adds column with DEFAULT NULL. Backfill is handled by Go code at app startup (`BackfillReviewTokens()`) to ensure consistent UUID format (F14 fix). No SQL-level backfill.

15. **Scene deletion with running jobs:** If a running Job exists for the deleted scene, cancel it before deleting (via `jobManager.cancel()` pattern).

16. **Mutation state guards (F6 fix):** ALL mutating review operations (narration edit, scene add, scene delete) MUST check project state. Allowed states for mutations: `scenario_review`, `approved`, `image_review`, `tts_review`. Reject mutations in `pending` (no scenario yet), `assembling` (output in progress), and `complete` (frozen). Return 409 Conflict with explanation.

17. **Token format consistency (F14 fix):** Both migration backfill AND new project creation use the same format: `uuid.New().String()` (Go-style UUID with dashes). Migration backfill changed from `hex(randomblob(16))` to a Go-based backfill executed by the application on startup (after migration adds the column, the app generates UUIDs for NULL tokens).

18. **Tailwind CSS bundling (F10 fix):** Do NOT use Tailwind CDN. Instead, generate a minimal pre-built CSS file using Tailwind CLI during development (`npx tailwindcss -o internal/api/templates/styles.css --minify`) and embed it via `embed.FS`. The CSS file is committed to the repo. No Node.js dependency at runtime.

19. **Scene lazy loading (F11 fix):** For projects with >20 scenes, the review page initially renders only the first 20 scene cards. A "Load More" button or `IntersectionObserver`-based infinite scroll loads additional scenes via `GET /api/v1/projects/{id}/scenes?offset=20&limit=20&token=xxx`. Dashboard service already returns all data; the handler paginates the template rendering or the JS handles client-side pagination.

## Implementation Plan

### Tasks

#### Phase 1: Database & Domain Layer (no dependencies)

- [x] Task 1: Add review_token migration
  - File: `internal/store/migrations/008_review_token.sql` (new)
  - Action: `ALTER TABLE projects ADD COLUMN review_token TEXT DEFAULT NULL;` + create index `idx_projects_review_token ON projects(review_token)`. Backfill is handled by Go code at startup (Task 4) to ensure consistent UUID format (F14 fix).
  - Notes: Migration runs in transaction per existing pattern in `store.go:migrate()`

- [x] Task 2: Add ReviewToken to Project domain model
  - File: `internal/domain/project.go` (modify)
  - Action: Add `ReviewToken string` field to `Project` struct
  - Notes: No validation needed — token is system-generated, not user-input

- [x] Task 3: Update store CRUD to read/write review_token
  - File: `internal/store/project.go` (modify)
  - Action: Update `CreateProject`, `GetProject`, `ListProjects`, `ListProjectsFiltered`, `UpdateProject` to include `review_token` column in SQL queries and Scan calls
  - Notes: `GetProject` must scan review_token. `DeleteProject` needs no change (column deleted with row).

- [x] Task 4: Auto-generate review_token on project creation + backfill
  - File: `internal/service/project.go` (modify)
  - Action: In `CreateProject()`, generate `uuid.New().String()` and set `p.ReviewToken` before calling `store.CreateProject(p)`. Import `github.com/google/uuid` (already a dependency). Also add a `BackfillReviewTokens()` method that queries `SELECT id FROM projects WHERE review_token IS NULL` and updates each with `uuid.New().String()`. Call this from server startup after migrations.
  - Notes: **(F14 fix)** Migration 008 only adds the column (no backfill in SQL). Go-side backfill ensures consistent UUID format across all tokens.

#### Phase 2: Dashboard Enhancement (depends on Phase 1)

- [x] Task 5: Add ImagePrompt field to SceneDashboardEntry
  - File: `internal/service/scene_dashboard.go` (modify)
  - Action: Add `ImagePrompt string \`json:"image_prompt,omitempty"\`` field to `SceneDashboardEntry`. In `GetDashboard()` loop, read `{sceneDir}/prompt.txt` via `os.ReadFile()` and populate `ImagePrompt`. Rename existing `Prompt` field to `Narration` for clarity (or keep for backward compat and add `ImagePrompt` alongside).
  - Notes: Keep `Prompt` field as-is for backward compatibility with existing API consumers. Add `ImagePrompt` as new field. If `prompt.txt` doesn't exist, leave empty string.

#### Phase 3: ReviewService (depends on Phase 1)

- [x] Task 6: Create ReviewService
  - File: `internal/service/review.go` (new)
  - Action: Create `ReviewService` struct with dependencies: `*store.Store`, `*slog.Logger`, workspace base path, `projectLocks sync.Map` (keyed by projectID → `*sync.Mutex`). Implement methods:
    - `getProjectLock(projectID string) *sync.Mutex` — get-or-create mutex for project from sync.Map
    - `validateMutationState(projectID string) error` — **(F6 fix)** check project status is in `{scenario_review, approved, image_review, tts_review}`. Return `TransitionError` (409) for `pending`, `assembling`, `complete`.
    - `UpdateNarration(projectID string, sceneNum int, text string) error` — validate mutation state, acquire project lock **(F5 fix)**, backup `scenario.json` to `scenario.json.bak` **(F12 fix)**, load, update scene narration, write back atomically via `workspace.WriteFileAtomic()`, release lock
    - `AddScene(projectID string, narration string) (*domain.Scene, error)` — validate mutation state, acquire lock, determine next scene_num (max existing + 1), create scene dir, update `scenario.json`, init approval records (pending), update project scene_count, release lock
    - `DeleteScene(projectID string, sceneNum int) error` — validate mutation state, acquire lock, delete `scene_approvals` + `scene_manifests` for scene, remove from `scenario.json`, remove scene dir, update project scene_count, release lock. Do NOT reindex remaining scenes.
    - `RejectAndRegenerate(projectID string, sceneNum int, assetType string, jobs *jobManager) (string, error)` — call `ApprovalService.RejectScene()`, check for duplicate running job (same project+type), create new Job record, return jobID. Caller (handler) launches the background goroutine.
  - Notes: `LoadScenarioFromFile` and `WriteFileAtomic` already exist in service/workspace packages. Reuse them. Input validation for narration: reject null bytes, max 10000 chars, do NOT html-escape (F4 fix — `html/template` handles output escaping).

- [x] Task 7: Add store methods for scene deletion cascade
  - File: `internal/store/scene_approval.go` (modify)
  - Action: Add `DeleteSceneApprovals(projectID string, sceneNum int) error` — `DELETE FROM scene_approvals WHERE project_id=? AND scene_num=?`. Add similar method to scene_manifests store if not present: `DeleteSceneManifest(projectID string, sceneNum int) error`.
  - Notes: These are called by `ReviewService.DeleteScene()`. Existing `DeleteProject` cascade pattern for reference.

#### Phase 4: Auth Layer (depends on Phase 1)

- [x] Task 8: Update AuthMiddleware for review-scoped token pass-through
  - File: `internal/api/auth.go` (modify)
  - Action: Add pass-through condition after the `/health`/`/ready` check that ONLY bypasses Bearer auth for review-scoped paths. Create `isReviewScopedPath(path string) bool` helper that returns true for: (1) `strings.HasPrefix(path, "/review/")`, (2) review-allowed API paths matching patterns: `/api/v1/projects/*/scenes/*/image`, `/audio`, `/approve`, `/reject`, `/narration`, `/api/v1/projects/*/scenes` (for POST add), `/api/v1/projects/*/approve`. Only when BOTH `?token=` is present AND path matches review scope, bypass Bearer auth.
  - Notes: **Security-critical (Party Mode 3 + F2 fix):** Without path restriction, ANY endpoint could be accessed by appending `?token=garbage`. Non-review endpoints (e.g., `GET /api/v1/config`) MUST still require Bearer auth even if `?token=` is present. **Implementation MUST use chi's `RouteContext` to extract the matched route pattern** (e.g., `/api/v1/projects/{id}/scenes/{num}/image`) and compare against an allowlist of exact route patterns — NOT string-glob matching against the raw URL path. This prevents path manipulation attacks like `/api/v1/projects/X/scenes/1/image/../../config`. Actual review token validation still happens in handlers.

- [x] Task 9: Create review token validation helper
  - File: `internal/api/review.go` (new, part of)
  - Action: Create helper function `validateReviewToken(s *Server, w http.ResponseWriter, r *http.Request, projectID string) (*domain.Project, bool)` that: (1) gets project from store, (2) extracts `?token=` from query, (3) compares with `project.ReviewToken` using `subtle.ConstantTimeCompare`, (4) returns project+true on success, writes 401/403 error+false on failure.
  - Notes: All review handlers call this first. Returns the loaded project to avoid double DB query.

#### Phase 5: API Handlers (depends on Phases 2, 3, 4)

- [x] Task 10: Create review page handler
  - File: `internal/api/review.go` (new, part of)
  - Action: Implement `handleReviewPage(w http.ResponseWriter, r *http.Request)`:
    1. Extract `project_id` from URL, validate review token (Task 9 helper)
    2. Call `SceneDashboardService.GetDashboard()` for data
    3. Execute `review.html` template with dashboard data + review token
    4. Write rendered HTML response with `Content-Type: text/html`
  - Notes: Template receives: project meta, scenes array, review token (for JS fetch calls), base API URL

- [x] Task 11: Create asset serving handlers
  - File: `internal/api/review.go` (new, part of)
  - Action: Implement `handleServeImage` and `handleServeAudio`:
    1. Validate review token
    2. **Parse and validate `sceneNum` as positive integer (F9 fix)** — use `strconv.Atoi()`, reject non-numeric values with 400
    3. Build file path: `filepath.Join(project.WorkspacePath, "scenes", strconv.Itoa(sceneNum), "image.png"|"audio.mp3")` — using validated integer ensures no path injection
    4. Validate path doesn't escape workspace: `filepath.Clean()` + `strings.HasPrefix(cleaned, project.WorkspacePath)`
    5. Check `os.Stat()` — return 404 if file doesn't exist
    6. `http.ServeFile(w, r, path)` — sets Content-Type automatically
  - Notes: Path traversal prevention is critical. The integer validation (step 2) + filepath.Join with known filenames ("image.png"/"audio.mp3") is the primary defense.

- [x] Task 12: Create narration edit handler
  - File: `internal/api/review.go` (new, part of)
  - Action: Implement `handleUpdateNarration`:
    1. Validate review token
    2. Verify `X-Review-Token` header matches query token **(F3 CSRF fix)**
    3. Parse JSON body: `{ "narration": "..." }`
    4. Input validation: non-empty, max 10000 chars, reject null bytes. Do NOT call `html.EscapeString()` — store as plain text **(F4 fix)**. `html/template` auto-escapes on output.
    5. Call `ReviewService.UpdateNarration()`
    6. Return updated narration JSON
  - Notes: All mutation handlers (12, 13, 14, and Approve All) must verify `X-Review-Token` header for CSRF protection.

- [x] Task 13: Create scene add/delete handlers
  - File: `internal/api/review.go` (new, part of)
  - Action: Implement `handleAddScene` (POST) and `handleDeleteScene` (DELETE):
    - Add: parse `{ "narration": "..." }`, call `ReviewService.AddScene()`, return new scene data
    - Delete: validate scene exists, check/cancel running jobs for scene, call `ReviewService.DeleteScene()`, return success
  - Notes: Delete needs access to `jobManager` to cancel running jobs

- [x] Task 14: Create reject-with-regeneration handler
  - File: `internal/api/review.go` (new, part of)
  - Action: Implement `handleRejectAndRegenerate`:
    1. Validate review token
    2. Extract `?type=image|tts`
    3. Call `ReviewService.RejectAndRegenerate()` — returns jobID
    4. Launch background goroutine: `go s.executeImageGeneration(...)` or `go s.executeTTSGeneration(...)` (reuse existing functions from `assets.go`)
    5. Return `{ "job_id": "...", "status": "rejected", "regeneration_started": true }`
  - Notes: Duplicate job check inside `RejectAndRegenerate()`. If dup exists, still reject but return `"regeneration_started": false` with existing job_id. Must also validate project state allows regeneration (check `validImageGenStates`/`validTTSGenStates` from `assets.go`) — return 409 if project state doesn't permit asset generation.

#### Phase 6: HTML Template (depends on Phase 5)

- [x] Task 15: Create review.html template
  - File: `internal/api/templates/review.html` (new), `internal/api/templates/styles.css` (new)
  - Action: Server-rendered HTML with **pre-built Tailwind CSS** (embedded via `embed.FS`, NOT CDN — F10 fix). Generate CSS: `npx tailwindcss -i input.css -o internal/api/templates/styles.css --minify` during development, commit the output. Structure:
    - **Header**: Project meta (SCP ID, status badge, progress bar), sticky "Approve All" button with unapproved count
    - **Scene cards** (vertical scroll list): Each card contains:
      - Scene number badge + approval status badges (image/TTS)
      - Image preview: `<img src="/api/v1/projects/{{.ProjectID}}/scenes/{{.SceneNum}}/image?token={{.Token}}" />`  with placeholder if no image
      - Audio player: `<audio controls src="/api/v1/projects/{{.ProjectID}}/scenes/{{.SceneNum}}/audio?token={{.Token}}"></audio>` with placeholder if no audio
      - Narration text: `<textarea>` with save/cancel buttons (JS fetch PATCH)
      - Collapsible image prompt section (details/summary HTML element)
      - Action buttons: Approve Image, Reject Image (→ regen), Approve TTS, Reject TTS (→ regen)
      - Delete button (top-right trash icon, confirmation modal)
    - **Generating state indicator**: When `ImageStatus`/`TTSStatus` is `"pending"` or `"rejected"` AND a running Job exists for that asset type, show loading spinner + "Generating..." text instead of placeholder. Check job status via dashboard data or a lightweight polling mechanism.
    - **Add Scene button** at bottom of list
    - **Lazy loading (F11 fix):** For projects with >20 scenes, render first 20 cards. "Load More" button triggers JS fetch to `GET /api/v1/projects/{id}/scenes?offset=20&limit=20&token=xxx` and appends new cards to DOM.
    - **JavaScript**: fetch() calls for all actions, update DOM on success, error toasts. **All mutation fetch() calls MUST include `X-Review-Token` header** matching the query token (F3 CSRF fix).
  - Notes: Responsive via Tailwind breakpoints. Mobile: single-column cards. Desktop: wider cards. All API calls include `?token={{.Token}}` query param. No external JS dependencies. No CDN dependencies — works offline/air-gapped.

- [x] Task 16: Embed templates in server
  - File: `internal/api/server.go` (modify)
  - Action: Add `//go:embed templates/*` directive and `embed.FS` variable. Parse templates in `NewServer()` or lazily. Add `reviewTmpl *template.Template` field to `Server` struct.
  - Notes: Template parsed once at startup, executed per request.

#### Phase 7: Route Registration (depends on Phases 5, 6)

- [x] Task 17: Register review routes
  - File: `internal/api/server.go` (modify)
  - Action: In `setupRouter()`, add routes BEFORE the `/api/v1` group:
    ```
    r.Get("/review/{project_id}", s.handleReviewPage)
    ```
    Inside `/api/v1` group, add:
    ```
    r.Get("/projects/{id}/scenes/{num}/image", s.handleServeImage)
    r.Get("/projects/{id}/scenes/{num}/audio", s.handleServeAudio)
    r.Patch("/projects/{id}/scenes/{num}/narration", s.handleUpdateNarration)
    r.Post("/projects/{id}/scenes", s.handleAddScene)
    r.Delete("/projects/{id}/scenes/{num}", s.handleDeleteScene)
    ```
    ```
    r.Post("/projects/{id}/approve-all", s.handleApproveAll)
    r.Post("/projects/{id}/review-token/rotate", s.handleRotateReviewToken)
    ```
    Update existing reject handler to support `?regen=true` OR add separate route for reject+regen.
  - Notes: Existing `/projects/{id}/scenes/{num}/reject` stays for backward compat. `handleRotateReviewToken` is Bearer-auth only (not review-token accessible). Update `isReviewScopedPath` allowlist to include `/approve-all` but NOT `/review-token/rotate`.

- [x] Task 18: Add ReviewService to Server dependencies
  - File: `internal/api/server.go` (modify)
  - Action: Add `reviewSvc *service.ReviewService` field to `Server` struct. Initialize in `NewServer()`: `reviewSvc: service.NewReviewService(st, slog.Default(), cfg.WorkspacePath)`. Add `WithReviewService` option if needed.
  - Notes: ReviewService needs store, logger, and workspace path.

#### Phase 8: Token Rotation + Approve All + Rate Limiting (depends on Phase 1)

- [x] Task 23: Create token rotation endpoint (F1 fix)
  - File: `internal/api/review.go` (part of)
  - Action: Implement `handleRotateReviewToken`: `POST /api/v1/projects/{id}/review-token/rotate`. **Bearer auth only** (NOT review-token accessible). Generate new `uuid.New().String()`, update project in DB, return new token. Old review URLs immediately stop working.
  - Notes: Register in `/api/v1` group (requires Bearer auth). This is the escape hatch for leaked tokens.

- [x] Task 24: Create "Approve All" handler (F13 fix)
  - File: `internal/api/review.go` (part of)
  - Action: Implement `handleApproveAll`: `POST /api/v1/projects/{id}/approve-all?type=image|tts&token=xxx`. Verify CSRF header. Iterate all scenes for the asset type, call `ApprovalService.ApproveScene()` for each `generated` scene. Skip scenes that are `pending` (not yet generated) or already `approved`. Return summary: `{ "approved": N, "skipped": M, "all_approved": bool }`. If `all_approved`, fire webhook.
  - Notes: This is distinct from the existing `handleApprovePipeline` (which transitions project state). This bulk-approves individual scene assets. Scenes in `pending` status are skipped (not yet generated — can't approve what doesn't exist).

- [x] Task 25: Add CSRF verification helper (F3 fix)
  - File: `internal/api/review.go` (part of)
  - Action: Create `verifyCsrfToken(r *http.Request) bool` that checks `r.Header.Get("X-Review-Token")` matches `r.URL.Query().Get("token")`. All mutation handlers (Tasks 12, 13, 14, 24) call this and return 403 on mismatch.
  - Notes: This prevents cross-origin form submission attacks. Browsers don't add custom headers without CORS preflight.

- [x] Task 26: Add rate limiting middleware for review endpoints (F8 fix)
  - File: `internal/api/review.go` (part of) or `internal/api/ratelimit.go` (new)
  - Action: Simple in-memory rate limiter per IP for review-scoped endpoints. Use `golang.org/x/time/rate` or a simple token bucket: 30 requests/minute per IP for mutation endpoints, 120 requests/minute for reads. Return 429 Too Many Requests when exceeded.
  - Notes: Applied only to review-scoped routes (not the entire API). In-memory is acceptable for single-instance deployment. If `golang.org/x/time/rate` is not already a dependency, implement a simple counter-based limiter to avoid new deps.

#### Phase 9: Webhook Enhancement (independent)

- [x] Task 19: Add review_url to webhook events (renumber: was Phase 8)
  - File: `internal/api/webhook.go` (modify)
  - Action: Add `ReviewURL string \`json:"review_url,omitempty"\`` to all event structs: `WebhookEvent`, `JobCompleteEvent`, `JobFailedEvent`, `SceneApprovedEvent`, `AllApprovedEvent`. Update all `Notify*` methods to accept `reviewURL string` parameter. In each Notify method, set `event.ReviewURL = reviewURL`.
  - Notes: Callers must pass the review URL. Build it as: `fmt.Sprintf("/review/%s?token=%s", projectID, project.ReviewToken)`. The base URL prefix (scheme+host) should come from config — add `API.BaseURL` config field if not present, or let n8n construct the full URL from the relative path.

- [x] Task 20: Update webhook callers to pass review_url
  - File: `internal/api/scenes.go`, `internal/api/assets.go`, `internal/api/projects.go` (modify)
  - Action: At each `s.webhooks.Notify*()` call site, fetch the project's ReviewToken and build review URL. Pass to Notify method.
  - Notes: Most callers already have the project loaded. Just need to add the URL construction.

#### Phase 10: Tests (depends on all above)

- [x] Task 21: ReviewService unit tests
  - File: `internal/service/review_test.go` (new)
  - Action: Test `UpdateNarration`, `AddScene`, `DeleteScene`, `RejectAndRegenerate` with in-memory store (`store.New(":memory:")`). Cover: happy path, not-found errors, duplicate job prevention, scene count update after add/delete.
  - Notes: Follow existing test patterns from `internal/service/` tests.

- [x] Task 22: Review handler tests
  - File: `internal/api/review_test.go` (new)
  - Action: Test with `httptest.NewRecorder` + `chi.NewRouter`:
    - Review page: valid token → 200 HTML, invalid token → 401, missing token → 401, wrong project → 403
    - Asset serving: valid image → 200 + correct Content-Type, missing file → 404, path traversal attempt → 400/403
    - Narration edit: valid → 200, empty narration → 400, XSS payload → sanitized
    - Scene add: valid → 201, scene delete: valid → 200, delete with running job → cancelled + deleted
    - Cross-project token → 403
  - Notes: Use test helpers to create project with review_token in in-memory DB.

### Acceptance Criteria

#### Authentication & Authorization
- [ ] AC-1: Given a project with review_token "abc123", when GET `/review/{id}?token=abc123`, then return 200 with rendered HTML page
- [ ] AC-2: Given a project with review_token "abc123", when GET `/review/{id}?token=wrong`, then return 401 Unauthorized
- [ ] AC-3: Given a project with review_token "abc123", when GET `/review/{id}` (no token), then return 401 Unauthorized
- [ ] AC-4: Given project A's token, when accessing project B's review endpoints, then return 403 Forbidden
- [ ] AC-5: Given auth enabled with Bearer token, when GET `/api/v1/projects/{id}/scenes/1/image?token=valid_review_token`, then return 200 (review token bypasses Bearer auth)

#### Review Page Display
- [ ] AC-6: Given a project with 5 scenes, when loading review page, then display all 5 scene cards with image preview, audio player, narration text, and approval status badges
- [ ] AC-7: Given a scene with no generated image, when loading review page, then display placeholder image in that scene's card
- [ ] AC-8: Given a scene with image prompt in `prompt.txt`, when loading review page, then display image prompt in collapsible section

#### Asset Serving
- [ ] AC-9: Given a scene with `image.png` in workspace, when GET `.../scenes/1/image?token=xxx`, then return 200 with `Content-Type: image/png`
- [ ] AC-10: Given a scene with no audio file, when GET `.../scenes/1/audio?token=xxx`, then return 404
- [ ] AC-11: Given a path traversal attempt (`scenes/../../../etc/passwd`), when serving asset, then return 400/403 (blocked)

#### Narration Editing
- [ ] AC-12: Given a valid narration update, when PATCH `.../scenes/1/narration`, then update `scenario.json` and return 200 with updated text
- [ ] AC-13: Given empty narration text, when PATCH `.../scenes/1/narration`, then return 400 validation error
- [ ] AC-14: Given narration with `<script>` tags, when PATCH `.../scenes/1/narration`, then sanitize HTML and store clean text

#### Scene Add/Delete
- [ ] AC-15: Given a project with scenes 1,2,3, when POST `.../scenes` with narration, then create scene 4 with pending approvals and return 201
- [ ] AC-16: Given a project with scenes 1,2,3, when DELETE `.../scenes/2`, then delete scene 2, its approvals, and its manifests. Remaining scenes: 1, 3 (gap preserved)
- [ ] AC-17: Given scene 2 has a running image_generate job, when DELETE `.../scenes/2`, then cancel the job before deleting

#### Approve/Reject with Regeneration
- [ ] AC-18: Given a generated scene image, when POST `.../scenes/1/reject?type=image&regen=true`, then reject the image AND create a new image_generate job
- [ ] AC-19: Given an already-running regen job for scene 1 image, when rejecting scene 1 image again, then reject but do NOT create duplicate job; return existing job_id
- [ ] AC-20: Given all scenes approved for images, when approving last scene, then fire `all_approved` webhook with `review_url` field

#### Webhook Enhancement
- [ ] AC-21: Given a state_change webhook event, when fired, then payload includes `review_url` field with format `/review/{project_id}?token={review_token}`

#### Auth Scope Security
- [ ] AC-24: Given `?token=garbage` on a non-review endpoint (e.g., `GET /api/v1/config?token=garbage`), when auth is enabled, then return 401 Unauthorized (Bearer auth NOT bypassed for non-review paths)

#### Regeneration State Validation
- [ ] AC-25: Given project in `pending` state, when POST `.../scenes/1/reject?type=image&regen=true`, then return 409 Conflict (project state doesn't allow image generation)

#### Token Lifecycle (F1)
- [ ] AC-26: Given a leaked review token, when POST `/api/v1/projects/{id}/review-token/rotate` with Bearer auth, then old token is invalidated and new UUID token returned
- [ ] AC-27: Given a rotated token, when using the old token on any review endpoint, then return 401 Unauthorized
- [ ] AC-28: Given no Bearer auth, when POST `.../review-token/rotate?token=review_token`, then return 401 (rotation requires Bearer, not review token)

#### CSRF Protection (F3)
- [ ] AC-29: Given a mutation request (PATCH narration) with valid `?token=` but missing `X-Review-Token` header, then return 403 Forbidden
- [ ] AC-30: Given a mutation request with `X-Review-Token` header not matching `?token=`, then return 403 Forbidden

#### Mutation State Guards (F6)
- [ ] AC-31: Given project in `complete` state, when PATCH `.../scenes/1/narration`, then return 409 Conflict
- [ ] AC-32: Given project in `assembling` state, when DELETE `.../scenes/1`, then return 409 Conflict
- [ ] AC-33: Given project in `pending` state, when POST `.../scenes` (add scene), then return 409 Conflict

#### Concurrency (F5)
- [ ] AC-34: Given two simultaneous narration edits on different scenes of the same project, when both complete, then both edits are preserved in `scenario.json` (no data loss)

#### Approve All (F13)
- [ ] AC-35: Given 5 scenes with 3 `generated` and 2 `pending` images, when POST `.../approve-all?type=image`, then approve the 3 generated scenes, skip the 2 pending, return `{ "approved": 3, "skipped": 2 }`
- [ ] AC-36: Given all images already approved, when POST `.../approve-all?type=image`, then return `{ "approved": 0, "skipped": 0, "all_approved": true }`

#### Rate Limiting (F8)
- [ ] AC-37: Given >30 mutation requests in 1 minute from same IP, then return 429 Too Many Requests

#### Lazy Loading (F11)
- [ ] AC-38: Given a project with 30 scenes, when loading review page, then render first 20 scene cards with "Load More" button; clicking it loads remaining 10

#### Mobile & UX (F7)
- [ ] AC-39: Given review page on a 375px-wide viewport, then all scene cards render in single-column layout with no horizontal scroll
- [ ] AC-40: Given a failed fetch() call (e.g., network error), then display error toast with message

#### Project Creation
- [ ] AC-22: Given a new project creation request, when project is created, then `review_token` is auto-generated (non-empty UUID with dashes)
- [ ] AC-23: Given existing projects before migration, when migration 008 runs and app starts, then all existing projects receive a backfilled review_token in UUID format

## Additional Context

### Dependencies

- No new Go runtime dependencies required (rate limiter uses stdlib or simple implementation)
- `github.com/google/uuid` — already in go.mod (used for job IDs)
- Tailwind CSS — pre-built at development time via `npx tailwindcss` (dev dependency only, NOT runtime). Output committed to repo as `internal/api/templates/styles.css`.
- Existing services: `SceneDashboardService`, `ApprovalService`, `ProjectService`

### Testing Strategy

- **Unit tests** (`internal/service/review_test.go`):
  - `ReviewService.UpdateNarration` — happy path, not found, file I/O error
  - `ReviewService.AddScene` — happy path, scene_count increment, approval init
  - `ReviewService.DeleteScene` — happy path, cascade delete, gap preservation
  - `ReviewService.RejectAndRegenerate` — happy path, duplicate job prevention
- **Handler tests** (`internal/api/review_test.go`):
  - Review token validation: valid/invalid/missing/cross-project
  - Review page render: 200 HTML with valid token
  - Asset serving: image/audio with correct Content-Type, 404, path traversal blocked
  - Narration CRUD: update, XSS sanitization, validation
  - Scene CRUD: add/delete with proper cascade
- **Manual testing steps**:
  1. Create project via API, note review_token in response
  2. Open `/review/{id}?token=xxx` in browser
  3. Verify scene cards render with images and audio
  4. Edit narration, save, reload — verify persistence
  5. Reject image — verify regeneration job created
  6. Add/delete scene — verify scene count and UI update
  7. Test mobile layout on phone/responsive mode

### Notes

- **Party Mode 1 consensus:** All agents agreed on architecture (Winston), UX patterns (Sally), priority (John — all P0), implementation approach (Amelia), and test coverage (Quinn)
- **Party Mode 2 consensus:** (1) Auth: AuthMiddleware pass-through + handler validation (Winston), (2) ReviewService separated from DashboardService (Winston/Amelia), (3) Image prompt separate field + collapsible UI (Sally), (4) Existing project token backfill in migration (Quinn)
- **Party Mode 3 consensus:** (1) Task 8 security fix: restrict pass-through to review-scoped paths only — prevent `?token=` bypass on arbitrary endpoints (Winston/Amelia), (2) AC-24: non-review endpoint token bypass test (Quinn), (3) AC-25: project state validation on regen (Quinn), (4) Generating state loading spinner UX (Sally)
- **Adversarial Review fixes applied:** F1 (token rotation), F2 (chi RouteContext matching), F3 (CSRF X-Review-Token header), F4 (no input escaping, rely on html/template output escaping), F5 (per-project sync.Mutex), F6 (mutation state guards), F7 (15 new ACs), F8 (rate limiting), F9 (sceneNum integer validation), F10 (pre-built Tailwind CSS), F11 (lazy loading >20 scenes), F12 (scenario.json .bak backup), F13 (Approve All handler task), F14 (consistent UUID format)
- Mobile-responsive design critical — primary access via Slack/Telegram notification links
- Scene cards: vertical scroll list, image preview + audio player + narration in single card
- Delete confirmation modal to prevent accidental deletion
- Sticky "approve all" button at top with unapproved count badge
- Image prompt shown in collapsible section below narration text per scene card
