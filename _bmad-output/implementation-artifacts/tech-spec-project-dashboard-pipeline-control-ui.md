---
title: 'Project Dashboard & Pipeline Control UI'
slug: 'project-dashboard-pipeline-control-ui'
created: '2026-03-14'
status: 'ready-for-dev'
stepsCompleted: [1, 2, 3, 4]
tech_stack: ['Go', 'HTMX', 'Go html/template', 'chi', 'SQLite']
files_to_modify: ['internal/domain/project.go', 'internal/domain/errors.go', 'internal/domain/project_test.go', 'internal/domain/errors_test.go', 'internal/service/project.go', 'internal/service/project_test.go', 'internal/service/scenario.go', 'internal/service/scenario_test.go', 'internal/service/image_gen.go', 'internal/service/image_gen_test.go', 'internal/service/tts.go', 'internal/service/assembler.go', 'internal/service/assembler_test.go', 'internal/service/approval.go', 'internal/service/approval_test.go', 'internal/service/review.go', 'internal/service/review_test.go', 'internal/service/scene_dashboard_test.go', 'internal/api/projects.go', 'internal/api/pipeline.go', 'internal/api/review.go', 'internal/api/assets.go', 'internal/api/assets_test.go', 'internal/api/scenes.go', 'internal/api/server.go', 'internal/api/webhook.go', 'internal/api/templates/review.html', 'internal/pipeline/runner.go', 'internal/cli/stage_cmds.go', 'internal/store/project.go']
files_to_create: ['internal/store/migrations/009_simplify_stages.sql', 'internal/api/dashboard.go', 'internal/api/helpers.go', 'internal/api/static/htmx.min.js', 'internal/api/templates/_layout.html', 'internal/api/templates/_partials/progress_bar.html', 'internal/api/templates/_partials/project_card.html', 'internal/api/templates/_partials/scene_card.html', 'internal/api/templates/_partials/toast.html', 'internal/api/templates/dashboard.html', 'internal/api/templates/project_detail.html']
code_patterns: ['chi router with global AuthMiddleware(enabled, apiKey) via r.Use()', 'html/template with existing go:embed templates/* in server.go:20', 'Service layer wraps store with DB transactions', 'Domain models with validation methods (Transition, CanTransition)', 'execution_logs table records all state changes with previous/new status', 'Pipeline runner uses status-based switch for flow control (runApprovalPath, runSkipApprovalPath)', 'Scene approval system uses TransitionError — shared with project transitions', 'Auto-trigger logic in review.go and assets.go (all images approved → TTS, all TTS approved → assembly)', 'NotifyStateChange() calls scattered in assets.go, pipeline.go, webhook.go — not centralized in TransitionProject']
test_patterns: ['*_test.go in same package, testify assertions', 'internal/mocks package referenced but missing (assembler_test.go)', 'No existing integration tests for state transitions']
---

# Tech-Spec: Project Dashboard & Pipeline Control UI

**Created:** 2026-03-14

## Overview

### Problem Statement

Currently the yt.pipe system only provides individual project review pages (`/review/{id}?token=xxx`). There is no way to see all projects at a glance, track their pipeline progress, or manage the full lifecycle from a single UI. Users must create new projects for the same SCP-# every time, cannot roll back approved states, and depend on n8n to trigger pipeline stage transitions.

### Solution

Add an HTMX-powered dashboard UI that provides: (1) a project list page with state filtering and SCP-# search, (2) a project detail page with a progress bar showing the current pipeline stage and the existing review content below, (3) state rollback capabilities for backward transitions, (4) direct pipeline stage triggers (image gen, TTS, assembly) from the UI, (5) SCP-# to projects relationship visibility, and (6) project deletion. This eliminates the hard dependency on n8n for pipeline orchestration.

### Scope

**In Scope:**
1. Project list page (state filter, SCP-# search, pagination)
2. Project detail page (progress bar + review content)
3. Replace state machine with dependency-based stage model — `status` becomes a progress marker, not a gate
4. Direct pipeline stage triggers from UI (image generation, TTS, assembly) — enabled by dependency check, not state
5. SCP-# ↔ Projects relationship display (project history per SCP)
6. Project deletion from UI

**Out of Scope:**
- Removing n8n webhook support (keep for backward compatibility, just no longer required)
- Changing auth system (keep existing API key + review token)
- Mobile optimization

## Context for Development

### Codebase Patterns

- **Router**: chi with global `AuthMiddleware(enabled, apiKey)` function applied via `r.Use()` to entire router. Review pages exempt via path check inside the middleware. No per-route auth middleware — all routes get Bearer auth by default.
- **Templates**: `go:embed` HTML templates in `internal/api/templates/` — review.html (425 lines) as precedent
- **Service layer**: Wraps store calls with DB transactions, records `execution_logs` for state changes (previous_status → new_status)
- **Domain validation**: `Transition()` method on `Project` struct checks `allowedTransitions` map → TO BE REPLACED
- **Pipeline runner**: `internal/pipeline/runner.go` (1142 lines) — biggest consumer of state machine, uses `resumableStatuses` map and `runApprovalPath()` with status-based switch
- **Scene approvals**: Independent from project status — `scene_approval.go` tracks per-scene approval states (keep unchanged)
- **Existing APIs**: `ListProjectsFiltered(state, scpID, limit, offset)`, `DeleteProject`, pipeline triggers (`/images/generate`, `/tts/generate`, `/assemble`) all exist
- **Review page**: Vanilla JS with fetch API for mutations, `X-Review-Token` header for CSRF
- **Refactor blast radius**: 28+ files reference `Transition|StatusApproved|StatusGeneratingAssets|TransitionError`, 31+ files reference old status string literals

### Files to Reference

| File | Purpose | Refactor Impact |
| ---- | ------- | --------------- |
| `internal/domain/project.go` | 8 status constants (incl. `StatusAssembling`), `allowedTransitions` map, `Transition()`, `CanTransition()`, `AllowedTransitions()` | **HIGH** — Replace entire state machine with `SetStage()` + 5 stage constants |
| `internal/domain/errors.go` | `TransitionError{Current, Requested, Allowed}` — **shared by both project transitions AND scene approval system** | **HIGH** — Keep `TransitionError` for scene approvals, add `DependencyError` for generation actions |
| `internal/service/project.go` | `TransitionProject()` — atomic transition with DB tx + execution_log recording | **HIGH** — Replace with `SetProjectStage()`, simplify to direct status update |
| `internal/service/scenario.go` | `GenerateScenario()` → `StatusScenarioReview`; `ApproveScenario()` → `StatusApproved`; `RegenerateSection()` uses `TransitionError` | **HIGH** — Remove `StatusApproved` reference, update all transition calls |
| `internal/api/review.go` | 4 `TransitionProject()` calls + **auto-trigger logic** (all images approved → trigger TTS; all TTS approved → trigger assembly) | **HIGH** — Replace transitions, redesign auto-trigger to use dependency checks instead of state transitions |
| `internal/api/assets.go` | 3 `TransitionProject()` calls (lines 663, 793), `StatusApproved` refs, `NotifyStateChange()` calls — **primary location of transition + webhook calls for asset completion** | **HIGH** — Replace transitions, update webhook calls, redesign auto-trigger logic |
| `internal/pipeline/runner.go` | 1142 lines — `resumableStatuses` map, `runApprovalPath()` + `runSkipApprovalPath()` status switches, multiple `TransitionProject()` calls | **HIGH** — Major refactor: dependency-based logic replaces status-based branching in both paths |
| `internal/service/approval.go` | 3 `TransitionError` returns for scene approval validation | **MED** — `TransitionError` is KEPT (not removed) since scene approvals still use it. Update status constant references only |
| `internal/api/scenes.go` | 2 `TransitionError` type assertions for error handling | **MED** — `TransitionError` is KEPT. Update status constant references only |
| `internal/service/review.go` | `StatusApproved`, `StatusAssembling`, `StatusGeneratingAssets` refs + `TransitionError` return | **MED** — Update status constants, keep `TransitionError` for review validation |
| `internal/cli/stage_cmds.go` | `runScenarioApproveCmd` calls `ApproveScenario()`, `buildRunner()` | **MED** — Update approve command for new stage model |
| `internal/api/server.go` | Route definitions, global `AuthMiddleware()` via `r.Use()`, existing `//go:embed templates/*` | **MED** — Add `/dashboard/` routes + `//go:embed static` for HTMX |
| `internal/api/projects.go` | Project REST API handlers (list/delete already exist) | **MED** — Add `PATCH /stage` endpoint, update status references |
| `internal/api/pipeline.go` | Pipeline trigger API handlers + `NotifyStateChange()` calls | **MED** — Replace state checks with dependency checks |
| `internal/service/image_gen.go` | Image generation service — **no `TransitionProject` calls** (transitions are in `assets.go`) | **LOW** — Remove old status constant references only |
| `internal/service/tts.go` | TTS generation service — **no `TransitionProject` calls** (transitions are in `assets.go`) | **LOW** — Remove old status constant references only |
| `internal/service/assembler.go` | Assembly service, validates scene assets | **LOW** — Minor stage constant updates |
| `internal/store/project.go` | Project CRUD, filtered listing | **LOW** — Status column values change |
| `internal/api/templates/review.html` | Existing frontend template (425 lines) | **LOW** — Update JS status constant strings |
| `internal/api/webhook.go` | Webhook notifier — `NotifyStateChange()` definition | **LOW** — Stage names change in payload |
| `internal/domain/scene_approval.go` | Scene approval statuses and transitions | **NONE** — Keep unchanged |
| `internal/store/scene_approval.go` | Approval CRUD, bulk reset queries | **NONE** — Keep unchanged |
| *Test files* | `*_test.go` for all modified files above (12+ test files) | **MED** — Update status constants, `TransitionError` handling in tests |

### Technical Decisions

- **HTMX over React**: No separate build pipeline, single Go binary deployment, consistent with existing embedded HTML pattern
- **HTMX delivery**: `//go:embed` static file (`internal/api/static/htmx.min.js`) — single binary principle, no CDN dependency
- **Go `html/template`**: UI templates use `html/template` for XSS protection. **Never use `template.HTML()` typecast** — all output must be auto-escaped
- **Dashboard auth**: Global `AuthMiddleware(enabled, apiKey)` already applied to entire router via `r.Use()`. Dashboard routes under `/dashboard/` automatically get Bearer auth. Review pages are exempt via path check inside the middleware.
- **Dashboard ↔ Review page coexistence**: review.html kept unchanged for external sharing (review token auth). project_detail.html is admin-only (Bearer auth). Scene card rendering shared via `_partials/scene_card.html` to avoid duplication
- **URL structure**: `GET /dashboard/` (list), `GET /dashboard/projects/{id}` (detail), `PATCH /api/v1/projects/{id}/stage` (set stage — replaces transition/rewind endpoints)
- **Template structure**: `html/template` partials via `{{define}}`/`{{template}}`:
  ```
  templates/
    _layout.html              # base layout (head, nav, footer, HTMX script)
    _partials/
      progress_bar.html       # reusable step indicator component
      project_card.html       # project list row
      scene_card.html         # scene card (extracted from review.html)
      toast.html              # toast notifications
    dashboard.html            # project list page
    project_detail.html       # project detail (progress + review content)
    review.html               # existing review page (backward compat, unchanged)
  ```
- **HTMX partial/full response**: Detect `HX-Request` header to return partial HTML (HTMX) vs full page (browser navigation). Add `isHTMX(r *http.Request) bool` helper
- **State machine → Dependency-based stage model**: Replace `allowedTransitions` map with a dependency graph. `status` field becomes a progress marker ("last reached stage"), not a gate that restricts actions. Any earlier stage is always accessible.
- **Remove `approved`, `generating_assets`, and `assembling` statuses**: Simplify from 8 to 5 stages: `pending`, `scenario`, `images`, `tts`, `complete`. DB migration required. `assembling` maps to `complete` (assembly was in progress = near completion).
- **Stage definitions and dependencies**:
  ```
  pending    → no dependencies (initial state)
  scenario   → scenario text exists
  images     → scenario exists (dependency: scenario)  ┐ parallel — independent
  tts        → scenario exists (dependency: scenario)  ┘ of each other
  complete   → images + tts exist (dependency: images, tts)
  ```
  Note: images and tts are independent of each other — only assembly (complete) requires both.
- **`status` field semantics change**: Instead of "current enforced state", it means "highest stage reached". User can freely set status to any stage ≤ current. Setting status backward is just moving the progress marker — no side effects on assets or approvals.
- **Action availability = dependency check, not state check**: "Generate images" button is enabled if scenario exists, regardless of current status. "Assemble" is enabled if images + TTS exist. The UI queries asset existence, not project status.
- **Remove `Project.Transition()` validation**: Replace `allowedTransitions` with simple `Project.SetStage(stage)`. API layer validates stage string via `IsValidStage()` (400 Bad Request for invalid). Service layer has no validation — just sets the value. Service layer checks dependencies before triggering generation actions (not stage transitions).
- **`TransitionError` is KEPT for scene approvals**: `TransitionError` is shared between project transitions and scene approval system (`approval.go`, `scenes.go`, `review.go`). Since scene approvals are out of scope, `TransitionError` must remain. Add new `DependencyError` type for generation action prerequisites. Do NOT remove `TransitionError`.
- **Auto-trigger logic redesign**: `review.go` and `assets.go` contain auto-trigger logic (all images approved → auto-trigger TTS; all TTS approved → auto-trigger assembly). These currently use `TransitionProject()`. Replace with: check dependency satisfaction after each approval, then call generation service directly + `SetProjectStage()`. The auto-trigger behavior is preserved but uses dependency checks instead of state transitions.
- **Asset/approval preservation**: All generated files stay in workspace. All approval states stay in DB. User controls what to regenerate by rejecting individual scenes in the review UI. This avoids unnecessary API costs and gives full control to the user.
- **Progress bar UX**: Step indicator shows all 5 stages. `images` and `tts` are visually side-by-side (parallel branch) since they're independent. Current stage highlighted. Each completed stage shows a checkmark based on **asset existence** (not stage order). Clicking any stage moves the marker (no side effects). Generation buttons enabled by dependency check, not stage position.
- **`StageOrder` is for rendering order, not dependency order**: `StageOrder` provides a consistent display sequence for the progress bar. Since `images` and `tts` are parallel, `StageIndex()` comparison should NOT be used for "is stage X ahead of Y" logic — use dependency checks instead. `StageIndex()` is only for progress bar highlighting.
- **Webhook centralization**: Currently `NotifyStateChange()` is called from scattered locations (`assets.go`, `pipeline.go`, `webhook.go`), NOT from `TransitionProject()`. After refactor, centralize webhook calls inside `SetProjectStage()` service method so every stage change fires a webhook automatically. This replaces the scattered callsite pattern.
- **HTMX Bearer token injection**: Dashboard templates inject the Bearer token via `<meta name="api-key" content="{{.APIKey}}">` in `_layout.html`. HTMX picks it up via `document.addEventListener("htmx:configRequest", (e) => { e.detail.headers["Authorization"] = "Bearer " + ... })`. The API key is passed from handler to template data.
- **CSRF for dashboard**: Since Bearer tokens are sent via `Authorization` header (not cookies), dashboard state-mutating operations are inherently CSRF-safe — CSRF attacks cannot set custom headers from cross-origin requests. No additional CSRF token needed.
- **Async generation race condition**: When a generation job completes, it calls `SetProjectStage()` to advance the marker. If user has manually rolled back the stage during generation, the job completion will overwrite it. **Accepted trade-off**: generation jobs always set stage on completion. User can re-roll back if needed. The cost is free (no side effects). Alternative (job checking current stage before setting) adds complexity for minimal benefit.
- **Pipeline trigger API full paths**: `POST /api/v1/projects/{id}/images/generate`, `POST /api/v1/projects/{id}/tts/generate`, `POST /api/v1/projects/{id}/assemble` — HTMX buttons must use these full paths.
- **SCP grouping**: List page groups projects by SCP-# with collapsible sections (Phase 2)
- **Separate HTML files**: `dashboard.html` (list), `project_detail.html` (detail), shared `_layout.html` + partials
- **HTMX navigation**: `hx-boost` for SPA-like page transitions without full reload (Phase 2)
- **Pipeline triggers**: Already exist as API endpoints (`/images/generate`, `/tts/generate`, `/assemble`) — dashboard adds UI buttons only, no new backend needed
- **Phased delivery**:
  - Phase 1 (MVP): Stage model refactor + dashboard (list + detail with progress bar + pipeline triggers + project deletion)
  - Phase 2: SCP grouping with collapsible sections, advanced filters, `hx-boost` SPA transitions

## Implementation Plan

### Tasks

> **Implementation strategy**: Tasks are grouped into 3 atomic blocks. Each block must be completed as a unit (all files within a block change together in one commit) to avoid intermediate compile errors. Block A is the critical refactor — it touches domain, service, API, CLI, store, and pipeline all at once because removing the state machine breaks all callers simultaneously.

#### Block A: Stage Model Refactor (atomic — single commit)

All files in this block must change together (30+ files). The old state machine removal + new stage model introduction cannot be split across commits without breaking the build.

- [ ] Task 1: Create stage simplification migration
  - File: `internal/store/migrations/009_simplify_stages.sql`
  - Action: Create SQL migration that:
    1. Updates `projects.status` values: `scenario_review` → `scenario`, `image_review` → `images`, `tts_review` → `tts`, `generating_assets` → `scenario` (safe default — user re-triggers from dashboard), `approved` → `scenario`, `assembling` → `complete` (assembly was in progress = near completion)
    2. Add CHECK constraint for valid stage values: `pending`, `scenario`, `images`, `tts`, `complete`
  - Notes: All 8 old statuses mapped: `pending` (keep), `scenario_review` → `scenario`, `approved` → `scenario`, `image_review` → `images`, `tts_review` → `tts`, `generating_assets` → `scenario`, `assembling` → `complete`, `complete` (keep). Ensure no migration 009 conflicts with parallel development.

- [ ] Task 2: Replace state machine with stage model in domain
  - File: `internal/domain/project.go`
  - Action:
    1. Remove `StatusScenarioReview`, `StatusApproved`, `StatusImageReview`, `StatusTTSReview`, `StatusGeneratingAssets` constants
    2. Add new constants: `StagePending = "pending"`, `StageScenario = "scenario"`, `StageImages = "images"`, `StageTTS = "tts"`, `StageComplete = "complete"`
    3. Add `var ValidStages = map[string]bool{...}` for validation and `var StageOrder = []string{...}` for progress bar rendering order
    4. Remove `allowedTransitions` map, `Transition()`, `CanTransition()`, `AllowedTransitions()` methods
    5. Add `SetStage(stage string)` method — sets `p.Status = stage`, updates `p.UpdatedAt`
    6. Add `IsValidStage(stage string) bool` helper — checks against `ValidStages` map
    7. Add `StageIndex(stage string) int` helper for progress bar rendering position (NOT for dependency logic)
  - Notes: `IsValidStage()` used by API layer to reject invalid stage strings (e.g. `"foo"`). `StageIndex()` is for UI rendering only — dependency checks use asset existence.

- [ ] Task 3: Add DependencyError (keep TransitionError for scene approvals)
  - File: `internal/domain/errors.go`
  - Action:
    1. **KEEP `TransitionError`** — it is used by `approval.go`, `scenes.go`, `review.go` for scene approval validation (out of scope for this refactor)
    2. Add `DependencyError{Action string, Missing []string}` struct with `Error()` returning `"cannot {Action}: missing dependencies: {Missing}"`
  - Notes: `TransitionError` shared with scene approval system. Removing it would break scene approvals at compile time. `DependencyError` is the new type for generation action prerequisites.

- [ ] Task 4: Replace TransitionProject with SetProjectStage
  - File: `internal/service/project.go`
  - Action:
    1. Remove `TransitionProject(ctx, projectID, newStatus)` method
    2. Add `SetProjectStage(ctx, projectID, stage string) error` — direct DB update + execution_log recording (keep logging `previous_status → new_status`)
    3. No state-machine validation (no `allowedTransitions` check). Stage string validation (`IsValidStage()`) is done at API layer, not here — internal callers pass known constants.
    4. **Centralize webhook**: call `NotifyStateChange()` inside `SetProjectStage()` after successful DB update. This replaces scattered webhook calls in `assets.go`, `pipeline.go`.
  - Notes: `execution_logs` recording preserved for audit trail. Webhook centralization ensures every stage change fires a notification without callsite scatter.

- [ ] Task 5: Update scenario service for stage model
  - File: `internal/service/scenario.go`
  - Action:
    1. `GenerateScenario()`: replace `TransitionProject(ctx, project.ID, domain.StatusScenarioReview)` with `SetProjectStage(ctx, project.ID, domain.StageScenario)`
    2. `ApproveScenario()`: remove `TransitionProject(ctx, projectID, domain.StatusApproved)` — scenario approval no longer changes project stage
    3. `RegenerateSection()`: remove status check that used `TransitionError` — regeneration always allowed if scenario exists

- [ ] Task 6: Update image generation service
  - File: `internal/service/image_gen.go`
  - Action:
    1. **No `TransitionProject()` calls exist in this file** — transitions for image completion are in `assets.go` (Task 10a)
    2. Add dependency check before generation: verify scenario exists in workspace (return `DependencyError` if not)
    3. Remove any old status constant references (`StatusApproved`, `StatusGeneratingAssets`)

- [ ] Task 7: Update TTS service
  - File: `internal/service/tts.go`
  - Action:
    1. **No `TransitionProject()` calls exist in this file** — transitions for TTS completion are in `assets.go` (Task 10a)
    2. Add dependency check before generation: verify scenario exists
    3. Remove any old status constant references

- [ ] Task 8: Update assembler service
  - File: `internal/service/assembler.go`
  - Action:
    1. Replace `TransitionProject()` calls with `SetProjectStage(ctx, projectID, domain.StageComplete)`
    2. Add dependency check: verify both images and TTS audio exist in workspace
    3. Remove old status references

- [ ] Task 9: Refactor pipeline runner from status-based to dependency-based
  - File: `internal/pipeline/runner.go`
  - Action:
    1. Remove `resumableStatuses` map
    2. Replace **both** `runApprovalPath()` and `runSkipApprovalPath()` status switches with dependency-based flow:
       - Check scenario exists → run image gen if needed → run TTS if needed → run assembly if both exist
    3. Replace all `TransitionProject()` calls with `SetProjectStage()`
    4. Remove `StatusApproved`, `StatusGeneratingAssets` references
    5. Runner determines what to do based on asset existence, not project status
  - Notes: Both approval and skip-approval paths use `StatusGeneratingAssets` and status-based branching — both must be refactored. Keep existing checkpoint/incremental build patterns.

- [ ] Task 10: Update API handlers for stage model
  - Files: `internal/api/projects.go`, `internal/api/pipeline.go`, `internal/api/review.go`, `internal/api/assets.go`, `internal/api/scenes.go`, `internal/api/webhook.go`
  - Action:
    1. `projects.go`: Add `PATCH /api/v1/projects/{id}/stage` handler — validate stage with `domain.IsValidStage()`, reject invalid with 400 Bad Request, call `SetProjectStage()` for valid. Update existing handlers for new constants.
    2. `pipeline.go`: Replace state-based precondition checks with dependency checks (asset existence). Return `DependencyError` as 409 Conflict. Remove scattered `NotifyStateChange()` calls (now centralized in `SetProjectStage()`).
    3. **`review.go` (HIGH impact)**: Has 4 `TransitionProject()` calls + auto-trigger logic. Replace transitions to `StatusTTSReview`/`StatusAssembling` with `SetProjectStage(StageImages/StageTTS/StageComplete)`. Redesign auto-trigger: after all images approved → call TTS generation service + `SetProjectStage(StageTTS)`; after all TTS approved → call assembly service + `SetProjectStage(StageComplete)`. Use dependency checks instead of state transitions.
    4. **`assets.go` (HIGH impact)**: Has 3 `TransitionProject()` calls (lines 663, 793) + `NotifyStateChange()` calls + `StatusApproved` refs. Replace transitions with `SetProjectStage()`. Remove `NotifyStateChange()` calls (centralized). Update auto-trigger logic same as review.go.
    5. `scenes.go`: Update `TransitionError` type assertions — keep them since `TransitionError` is preserved for scene approvals. Update status constant references only.
    6. `webhook.go`: Stage names change in payload, remove redundant `NotifyStateChange()` callsites that are now in `SetProjectStage()`.

- [ ] Task 10a: Update service files that use TransitionError/status constants
  - Files: `internal/service/approval.go`, `internal/service/review.go`
  - Action:
    1. `approval.go`: **Keep `TransitionError` usage** (scene approvals). Update status constant references (`StatusApproved` → appropriate new constant, `StatusImageReview` → `StageImages`, etc.)
    2. `review.go`: Update `StatusApproved`, `StatusAssembling`, `StatusGeneratingAssets` refs. Keep `TransitionError` return for review validation. Update the `reviewableStatuses` map to use new stage constants.

- [ ] Task 10b: Update all test files for new stage model
  - Files: `internal/domain/project_test.go`, `internal/domain/errors_test.go`, `internal/service/project_test.go`, `internal/service/scenario_test.go`, `internal/service/approval_test.go`, `internal/service/review_test.go`, `internal/service/image_gen_test.go`, `internal/service/assembler_test.go`, `internal/service/scene_dashboard_test.go`, `internal/api/assets_test.go`
  - Action:
    1. Replace old status constants with new stage constants in all test assertions
    2. Update `TransitionProject()` call expectations to `SetProjectStage()`
    3. Keep `TransitionError` assertions in approval/scenes tests (unchanged behavior)
  - Notes: Compile-check all test files as part of Block A. 12+ test files must be updated.

- [ ] Task 11: Update review.html status strings
  - File: `internal/api/templates/review.html`
  - Action:
    1. Update JS status string comparisons: `scenario_review` → `scenario`, `image_review` → `images`, `tts_review` → `tts`
    2. Remove references to `approved`, `generating_assets` states
  - Notes: No structural changes — just string value updates.

- [ ] Task 12: Update CLI stage commands
  - File: `internal/cli/stage_cmds.go`
  - Action:
    1. Update `runScenarioApproveCmd` — scenario approval no longer changes project stage
    2. Replace old status constant references
    3. Update any `TransitionProject()` calls to `SetProjectStage()`

- [ ] Task 13: Update store for new stage values
  - File: `internal/store/project.go`
  - Action:
    1. Update any hardcoded status string references to new stage values
    2. Ensure `ListProjectsFiltered` works with new stage values in filter parameter

#### Block B: HTMX Dashboard UI (independent of Block A for templates, depends on Block A for handlers)

Templates (Tasks 14-21) can be authored in parallel with Block A. Handlers (Tasks 22-24) require Block A to be complete.

- [ ] Task 14: Add HTMX static file
  - File: `internal/api/static/htmx.min.js`
  - Action: Download htmx.min.js (v2.0.x) and place in static directory with `//go:embed` directive
  - Notes: Single binary principle — no CDN dependency.

- [ ] Task 15: Create base layout template
  - File: `internal/api/templates/_layout.html`
  - Action: Create HTML layout with:
    1. `<head>`: htmx script, basic CSS, `<meta name="api-key" content="{{.APIKey}}">` for HTMX auth
    2. JS: `htmx.on("htmx:configRequest", (e) => { e.detail.headers["Authorization"] = "Bearer " + document.querySelector('meta[name="api-key"]').content })` — inject Bearer token into all HTMX requests
    3. HTMX error handling: `htmx.on("htmx:responseError", ...)` → show toast with error message
    4. Global loading indicator: `hx-indicator` class for spinner during HTMX requests
    5. Nav bar (Dashboard link), `{{block "content" .}}` placeholder, footer
  - Notes: Minimal CSS — functional, not fancy. Auth token injection is critical for all HTMX state-mutating operations.

- [ ] Task 16: Create progress bar partial
  - File: `internal/api/templates/_partials/progress_bar.html`
  - Action: Create step indicator showing 5 stages. `images` and `tts` shown side-by-side as parallel branch. Each stage shows checkmark based on asset existence (from `.DependenciesMet` map), not stage order. Current stage marker highlighted separately. Each stage clickable via HTMX `hx-patch` to set stage.
  - Notes: Receives `.CurrentStage`, `.StageOrder`, `.DependenciesMet` map from handler. Checkmarks = dependency met, highlight = current marker position.

- [ ] Task 17: Create project card partial
  - File: `internal/api/templates/_partials/project_card.html`
  - Action: Create list row showing: SCP-#, project ID, current stage badge, created date, link to detail page

- [ ] Task 18: Create scene card partial (new for dashboard)
  - File: `internal/api/templates/_partials/scene_card.html`
  - Action: Create scene card for dashboard's project detail page. Show scene image, TTS audio player, approval status.
  - Notes: **Not extracted from review.html** — this is a new, simpler partial for the dashboard. review.html keeps its own inline scene rendering unchanged (Phase 1). Phase 2 may unify both into this partial.

- [ ] Task 19: Create toast notification partial
  - File: `internal/api/templates/_partials/toast.html`
  - Action: Create HTMX-compatible toast for success/error feedback. Auto-dismiss after 3 seconds. Triggered by `HX-Trigger: showToast` response header.

- [ ] Task 20: Create dashboard list page
  - File: `internal/api/templates/dashboard.html`
  - Action: Create project list page extending `_layout.html`. Include: stage filter dropdown, SCP-# search input, paginated project list using `project_card.html` partial. HTMX for filter/search without page reload.
  - Notes: Pagination: "Load more" button at bottom using `hx-get` with `?page=N&stage=X&scp=Y` — appends results via `hx-swap="beforeend"`. Default 20 per page. Filter/search changes reset to page 1 and replace entire list (`hx-swap="innerHTML"`).

- [ ] Task 21: Create project detail page
  - File: `internal/api/templates/project_detail.html`
  - Action: Create detail page extending `_layout.html`. Top section: progress bar partial + pipeline action buttons (Generate Images, Generate TTS, Assemble). Bottom section: scene cards with approval states. Action buttons enabled/disabled based on dependency check data. Delete project button with `hx-confirm` dialog.
  - Notes:
    - Pipeline buttons use `hx-post` to full API paths: `POST /api/v1/projects/{id}/images/generate`, `POST /api/v1/projects/{id}/tts/generate`, `POST /api/v1/projects/{id}/assemble`
    - Delete uses existing `DELETE /api/v1/projects/{id}`
    - Generation buttons show `hx-indicator` spinner during async job submission
    - Generation jobs are async — button shows "Submitted" state after 200 response. Job progress is NOT tracked in this UI (Phase 2 — use polling or SSE). User refreshes to see updated dependency checkmarks.

- [ ] Task 22: Create dashboard handlers
  - File: `internal/api/dashboard.go`
  - Action:
    1. `HandleDashboardList(w, r)`: load filtered projects, render dashboard.html (full) or project_card partials (HTMX). Use `isHTMX(r)` helper. Pagination: default 20 per page, `?page=N` query param, HTMX replaces project list container.
    2. `HandleProjectDetail(w, r)`: load project + scenes + approvals, compute dependency satisfaction map, render project_detail.html (full) or content partial (HTMX). Pass `APIKey` to template data for HTMX auth header injection.
    3. Add `computeDependencies(project *domain.Project, ws *workspace.Workspace) map[string]bool`:
       - `scenario`: `ws.ScenarioPath(project.ID)` file exists AND `project.SceneCount > 0`
       - `images`: all scenes have image files at `ws.SceneImagePath(project.ID, sceneN)` (check all `SceneCount` scenes)
       - `tts`: all scenes have audio files at `ws.SceneAudioPath(project.ID, sceneN)` (check all `SceneCount` scenes)
       - `complete`: both `images` and `tts` are true
  - Notes: Asset existence = filesystem check via workspace helper methods. Checks all scenes, not just one. "All scenes" means `SceneCount` files present.

- [ ] Task 23: Add `isHTMX()` helper
  - File: `internal/api/helpers.go`
  - Action: Add `isHTMX(r *http.Request) bool` helper — checks `HX-Request` header.
  - Notes: Separate file for reusability across dashboard and future HTMX handlers.

- [ ] Task 24: Register dashboard routes and static serving
  - File: `internal/api/server.go`
  - Action:
    1. Add `//go:embed static` for HTMX file serving (new embed directive — separate from existing `//go:embed templates/*` at line 20)
    2. `//go:embed templates/*` already exists (line 20, `var templatesFS embed.FS`) — new template files are auto-included. Add dashboard template parsing logic in a new `initDashboardTemplates()` method (existing `initReviewTemplate()` only parses review.html).
    3. Register routes (global `AuthMiddleware` already applied via `r.Use()` — dashboard gets Bearer auth automatically):
       - `GET /dashboard/` → `HandleDashboardList`
       - `GET /dashboard/projects/{id}` → `HandleProjectDetail`
    4. Register `PATCH /api/v1/projects/{id}/stage` → stage set handler
    5. Add static file handler: `GET /static/*` → embedded file server (exempt from auth via path check in `AuthMiddleware`)
  - Notes: No need to add auth middleware per-route — global middleware handles it. Static files need auth exemption path added to `AuthMiddleware`.

### Acceptance Criteria

#### Stage Model

- [ ] AC 1: Given a project at any stage, when `PATCH /api/v1/projects/{id}/stage` is called with a valid stage, then the project stage is updated and `execution_logs` records the change
- [ ] AC 2: Given a project at stage `images`, when stage is set to `scenario`, then the stage marker moves backward with no side effects (no asset deletion, no approval reset)
- [ ] AC 3: Given a project with no scenario file, when "Generate Images" is triggered, then a 409 response with `DependencyError` listing `["scenario"]` is returned
- [ ] AC 4: Given a project with scenario + images + TTS, when "Assemble" is triggered, then assembly proceeds regardless of current stage value
- [ ] AC 5: Given `PATCH /api/v1/projects/{id}/stage` with `{"stage": "foo"}`, when called, then 400 Bad Request is returned with "invalid stage" error (validated via `IsValidStage()`)

#### Dashboard List

- [ ] AC 6: Given multiple projects exist, when `GET /dashboard/` is accessed with Bearer auth, then all projects are listed with SCP-#, stage badge, and creation date
- [ ] AC 7: Given projects at various stages, when stage filter is applied, then only matching projects are shown (HTMX partial update, no full page reload)
- [ ] AC 8: Given projects for SCP-173 and SCP-682, when searching "173", then only SCP-173 projects appear

#### Project Detail

- [ ] AC 9: Given a project with scenario + images done but no TTS, when detail page loads, then progress bar shows `scenario` and `images` with checkmarks, `tts` without checkmark, and current stage marker at `images`
- [ ] AC 10: Given a project with scenario but no images, when detail page loads, then "Generate Images" button is enabled and "Assemble" button is disabled
- [ ] AC 11: Given a project detail page, when a progress bar stage is clicked, then stage marker updates via HTMX without page reload
- [ ] AC 12: Given a project detail page, when "Delete" button is clicked and confirmed, then project is deleted and user is redirected to dashboard list

#### Auth & Security

- [ ] AC 13: Given no Bearer token, when `GET /dashboard/` is accessed, then 401 Unauthorized is returned
- [ ] AC 14: Given a review token, when `GET /dashboard/` is accessed, then 401 Unauthorized is returned (review tokens cannot access dashboard)

#### Backward Compatibility

- [ ] AC 15: Given existing review page URLs (`/review/{id}?token=xxx`), when accessed, then review page works unchanged with new stage values
- [ ] AC 16: Given a stage change via dashboard, when webhook fires, then payload contains `previous_state` and `new_state` with new stage names

#### DB Migration

- [ ] AC 17: Given existing projects with all 8 old status values (`pending`, `scenario_review`, `approved`, `image_review`, `tts_review`, `generating_assets`, `assembling`, `complete`), when migration 009 runs, then all are correctly mapped (`assembling` → `complete`, `generating_assets`/`approved` → `scenario`, others by name) with no data loss and CHECK constraint passes

## Additional Context

### Dependencies

- HTMX v2.0.x minified JS via `//go:embed` (no CDN, no new Go module dependencies)
- No new Go module dependencies — uses existing `html/template`, `embed`, `chi`, `sqlite`

### Testing Strategy

**Unit Tests:**
- `internal/domain/project_test.go`: Test `SetStage()` sets status correctly, `StageIndex()` returns correct order
- `internal/domain/errors_test.go`: Test `DependencyError.Error()` message format
- `internal/service/project_test.go`: Test `SetProjectStage()` updates DB + records execution_log
- `internal/service/scenario_test.go`: Test `GenerateScenario()` sets stage to `scenario`, `ApproveScenario()` no longer changes project stage

**Integration Tests:**
- Stage set API: `PATCH /stage` with valid/invalid stages, auth checks
- Dashboard list: `GET /dashboard/` with filters, pagination, auth
- Project detail: `GET /dashboard/projects/{id}` with dependency data
- Migration: verify old status values correctly mapped

**Manual Testing:**
- Navigate dashboard list → filter by stage → search by SCP-#
- Open project detail → verify progress bar reflects actual stage
- Click earlier stage in progress bar → verify stage moves backward
- Trigger image generation → verify dependency check works
- Trigger assembly without TTS → verify 409 with dependency error
- Access dashboard without auth → verify 401
- Access dashboard with review token → verify 401
- Verify existing review pages still work with new stage values

### Security Considerations (Red Team Analysis)

- **Dashboard auth**: Global `AuthMiddleware` already applied via `r.Use()` — all `/dashboard/` routes automatically require Bearer token. No additional per-route middleware needed.
- **Stage change is free**: Moving the progress marker has no side effects and no cost. Generation actions (image, TTS, assembly) are gated by dependency checks, not stage.
- **No global rate limit**: Rate limiting only exists on review page mutations (per-IP, in-memory). Dashboard API endpoints have no rate limit. Acceptable for admin-only dashboard — add rate limiting in Phase 2 if needed.
- **XSS prevention**: `html/template` auto-escaping, never use `template.HTML()` typecast for user-supplied data
- **CSRF protection**: Dashboard uses Bearer token via `Authorization` header (injected by JS from meta tag). Since CSRF attacks cannot set custom headers from cross-origin requests, dashboard is inherently CSRF-safe. No additional CSRF token needed.
- **Race condition (async jobs)**: When a generation job completes, it overwrites the stage marker via `SetProjectStage()`. If user rolled back during generation, the completion will advance the marker again. **Accepted trade-off** — stage movement is free, user can re-roll back.
- **Review token isolation**: Review tokens cannot access dashboard routes; Bearer tokens cannot access review-token-only routes

### Implementation Order (block-based)

3 atomic blocks. Block A is the critical refactor, Block B templates can start in parallel, Block C ties everything together.

| Block | Tasks | Scope | Depends On |
|-------|-------|-------|------------|
| **A** (Stage Model Refactor) | 1-13 (incl. 10a, 10b) | Migration + domain + service + all callers (API, CLI, pipeline, store, review.html) + all test files (30+ files) | nothing — **must be atomic single commit** |
| **B** (Dashboard UI) | 14-24 | HTMX static + templates + handlers + helpers + route registration | Templates (14-21) parallel with A; Handlers + routes (22-24) wait for A |

**Critical path**: Block A (atomic) → Block B handlers/routes

**Block A validation strategy**: Since 30+ files change atomically, use this incremental validation approach during development:
1. Start with domain changes (Tasks 2-3) — run `go vet ./internal/domain/...`
2. Add service changes (Tasks 4-8) — run `go vet ./internal/service/...` (will show remaining callers)
3. Use compiler errors as a checklist: `go build ./...` after each sub-group to find missed references
4. Final `go test ./...` after all Block A files are updated
5. Only commit when all tests pass

**Refactor scope**: 23 files confirmed via grep. All callers of `TransitionProject()`, `Project.Transition()`, and old status constants must be updated. `TransitionError` is KEPT (used by scene approvals) — only project-specific usages of old status constants are removed.

### Notes

- **High-risk item**: `pipeline/runner.go` (1142 lines) — both `runApprovalPath()` and `runSkipApprovalPath()` must be refactored. Test thoroughly after changes.
- **High-risk item**: `review.go` and `assets.go` auto-trigger logic — not a simple string replace. Must redesign to use dependency checks + direct service calls instead of state transitions.
- **Block A atomicity**: 30+ files change in single commit. Use `go build ./...` as incremental validation (see validation strategy above). Do not commit until all tests pass.
- **`TransitionError` preserved**: Scene approval system (`approval.go`, `scenes.go`, `review.go`) uses `TransitionError`. It must NOT be removed. Only project-specific uses of old status constants are updated.
- **`image_gen.go`/`tts.go` have NO TransitionProject calls**: Actual transition calls for asset completion live in `assets.go`. Tasks 6/7 only remove old status constant references.
- **Webhook centralization**: Move `NotifyStateChange()` into `SetProjectStage()` service method. Remove scattered calls from `assets.go`, `pipeline.go`. This is a behavior change — verify no duplicate notifications.
- **Migration maps all 8 statuses**: `generating_assets` → `scenario` (safe default), `assembling` → `complete`, `approved` → `scenario`. Ensure no migration 009 number conflicts.
- **n8n impact**: Webhook payloads will use new stage names. n8n workflows may need update.
- **HTMX auth**: Bearer token injected via `<meta>` tag + JS `htmx:configRequest` event. Critical for all state-mutating HTMX operations.
- **Async job UX**: Generation jobs are submitted async. Dashboard shows "Submitted" state, not live progress. User refreshes to see updated checkmarks. Phase 2: polling or SSE for live updates.
- **Pagination**: "Load more" pattern with `hx-swap="beforeend"`. Default 20 per page. Filter/search resets to page 1.
- **scene_card.html**: New partial for dashboard only (Phase 1). review.html keeps its own inline rendering unchanged. Phase 2 may unify both.
- **`isHTMX()` helper**: Placed in `internal/api/helpers.go` for reusability, not in `dashboard.go`.
- **Progress bar parallel stages**: `images` and `tts` are visually side-by-side. Checkmarks based on asset existence (dependency check), not stage order position.
- **`go:embed templates/*` already exists**: server.go line 20. New templates auto-included. Need new `initDashboardTemplates()` for parsing (existing `initReviewTemplate()` only parses review.html).
- Stage marker movement is free and side-effect-free — no asset deletion, no approval reset
- Project deletion: included in Phase 1 via dashboard detail page (existing `DELETE /api/v1/projects/{id}` API)
- Phase 2 items deferred: SCP grouping, `hx-boost` SPA transitions, advanced filters, review.html partial unification, async job progress tracking, dashboard rate limiting
