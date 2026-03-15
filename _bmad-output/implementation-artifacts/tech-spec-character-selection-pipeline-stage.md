---
title: 'Character Selection Pipeline Stage'
slug: 'character-selection-pipeline-stage'
created: '2026-03-15'
status: 'implementation-complete'
stepsCompleted: [1, 2, 3, 4]
tech_stack: ['Go 1.25.7', 'chi/v5', 'HTMX', 'SQLite (modernc)', 'html/template', 'DaisyUI', 'testify']
files_to_modify:
  - 'internal/domain/project.go'
  - 'internal/service/pipeline_orchestrator.go'
  - 'internal/pipeline/runner.go'
  - 'internal/api/server.go'
  - 'internal/api/dashboard.go'
  - 'internal/api/pipeline.go'
  - 'internal/api/templates/project_detail.html'
  - 'internal/api/templates/_partials/progress_bar.html'
  - 'internal/store/character.go'
  - 'internal/service/character.go'
  - 'internal/cli/serve_cmd.go'
  - 'internal/cli/run_cmd.go'
  - 'internal/cli/stage_cmds.go'
files_to_create:
  - 'internal/store/migrations/011_character_candidates.sql'
  - 'internal/api/character_handlers.go'
  - 'internal/api/templates/_partials/character_section.html'
code_patterns:
  - 'Handler pattern: func (s *Server) handle*(w, r) with WriteError/WriteJSON'
  - 'Service constructor: NewXxxService(store *store.Store)'
  - 'Domain errors: NotFoundError, ValidationError, DependencyError'
  - 'Store CRUD: scanCharacters helper, JSON aliases, RFC3339 timestamps'
  - 'Dashboard data: projectDetailData struct with DependenciesMet map'
  - 'UI stage mapping: backend stages → UI stages (images/tts → "assets")'
  - 'HTMX partial rendering: check HX-Request header, render block only'
  - 'Job system: jobManager tracks async operations with progress'
  - 'Pipeline runner: created separately in CLI (run_cmd.go, stage_cmds.go) and API (serve_cmd.go)'
  - 'Path safety: strconv.Atoi + range check for numeric URL params (see handleDashboardImage pattern)'
test_patterns:
  - 'Same-package *_test.go with setupTestXxx(t) helper'
  - 'testify assert/require'
  - ':memory: SQLite for store tests'
  - 'build tag -tags=integration for integration tests'
---

# Tech-Spec: Character Selection Pipeline Stage

**Created:** 2026-03-15

## Overview

### Problem Statement

The pipeline currently has no character creation/selection stage, so there is no way to ensure consistent character appearance across generated scene images. Characters need to be established before image generation begins.

### Solution

Add a `character` stage between `scenario` and `images` in the pipeline. The dashboard provides inline UI (below the stage progress bar) for generating candidate character images, displaying them in a card grid, and selecting one. The selected character's reference image is automatically injected into ImageGenService. Existing characters for the same SCP ID can be reused across projects.

### Scope

**In Scope:**
- New `character` stage in state machine and pipeline runner
- 4 API endpoints: generate candidates, list candidates, select candidate, get current character
- Dashboard inline UI with HTMX polling for async generation
- Hard block on `images` stage when no character is selected
- SCP ID-based character reuse across projects
- DB migration for `character_candidates` table

**Out of Scope:**
- Migration of existing projects (will be manually deleted)
- Approval stages (being removed entirely)
- WebSocket/SSE real-time updates
- Character editing UI

## Context for Development

### Codebase Patterns

- **State machine**: `domain/project.go` — `StageOrder` array + `ValidStages` map + `SetStage()` progress marker (not a gate). Currently: `[pending, scenario, images, tts, complete]`
- **Pipeline stages**: `service/pipeline_orchestrator.go` — separate `PipelineStage` string type with finer-grained constants (`data_load`, `scenario_generate`, `scenario_approval`, `image_generate`, etc.)
- **Runner flow**: `pipeline/runner.go:RunWithOptions()` → data_load → scenario_generate → approval gate → `resumeFromApproval()` → (skipApproval branch at L310) → `runSkipApprovalPath()` or `runApprovalPath()` → images/tts → timing → subtitle → assemble
- **Runner creation (3 call sites)**:
  1. `internal/cli/run_cmd.go:90` — CLI `run` command
  2. `internal/cli/stage_cmds.go:204` — CLI stage commands via `buildRunner()` helper
  3. `tests/integration/pipeline_test.go:275` — integration tests
- **API pipeline execution**: `internal/api/pipeline.go:292` — `executeFullPipeline()` uses `s.pipelineRunner` field (currently always nil because `serve_cmd.go` never calls `WithPipelineRunner()`). `WithPipelineRunner()` is defined at `server.go:84` but unused.
- **Dashboard UI stages**: Mapped from backend stages to UI stages. `images`/`tts` → `"assets"`. Uses `UIStageOrder` in `dashboard.go`
- **Handler pattern**: All handlers are `func (s *Server) handle*(w http.ResponseWriter, r *http.Request)`. Use `WriteJSON(w, r, status, data)` and `WriteError(w, r, status, code, msg)` from `api/response.go`
- **HTMX partials**: `handleProjectDetail()` checks `r.Header.Get("HX-Request")` — if true, renders only `project_detail_content` block; otherwise full page with layout
- **Job system**: `jobManager` in `api/jobs.go` tracks async operations. Handlers start goroutines, store job IDs, dashboard polls job status. Job types checked in `handleProjectDetail()` L346: `"image_generate"`, `"tts_generate"`, `"assembly"`.
- **Pipeline stage labels**: `pipelineStageLabel()` in `dashboard.go:409-425` maps stage strings to human-readable labels for dashboard display.
- **Template registration**: `initDashboardTemplates()` has a hardcoded `partialFiles` slice. All partials must be registered here to be available in templates.
- **ImageGenService wiring gap**: `SetCharacterService()` exists but is **never called** in pipeline runner. Character auto-reference feature is implemented but currently disabled.
- **Store migration pattern**: Sequential `NNN_name.sql` files in `internal/store/migrations/`, embedded via `//go:embed`, auto-applied on startup. Currently at 010.
- **Path safety pattern**: Existing `handleDashboardImage` uses `strconv.Atoi` to validate numeric URL params before constructing filesystem paths.

### Files to Reference

| File | Purpose |
| ---- | ------- |
| `internal/domain/project.go` | Stage constants, StageOrder, ValidStages, SetStage() |
| `internal/domain/character.go` | Character model struct |
| `internal/domain/errors.go` | DependencyError, ValidationError, NotFoundError |
| `internal/service/character.go` | CharacterService with GenerateCandidates, SelectCandidate, CheckExistingCharacter |
| `internal/service/pipeline_orchestrator.go` | PipelineStage constants, PipelineCheckpoint |
| `internal/service/image_gen.go` | ImageGenService with SetCharacterService, SetSelectedCharacterImage |
| `internal/pipeline/runner.go` | Runner struct, RunWithOptions, resumeFromApproval (L292), runSkipApprovalPath (L319), runApprovalPath (L351), RunStage (L649) |
| `internal/api/server.go` | Server struct, setupRouter(), ServerOption pattern, WithPipelineRunner (L84, unused) |
| `internal/api/pipeline.go` | handleRunPipeline (L136), executeFullPipeline (L292, uses s.pipelineRunner) |
| `internal/api/dashboard.go` | handleProjectDetail (L291), projectDetailData (L259), computeDependencies (L428), pipelineStageLabel (L409), initDashboardTemplates with partialFiles |
| `internal/api/templates/project_detail.html` | Project detail page with progress bar, pipeline actions, scene cards |
| `internal/api/templates/_partials/progress_bar.html` | DaisyUI steps component with HTMX stage-click |
| `internal/store/character.go` | Character CRUD SQL (CreateCharacter, ListCharactersBySCPID, scanCharacters helper) |
| `internal/store/store.go` | Store init, migration engine, embed FS |
| `internal/store/voice_cache.go` | UpdateSelectedImagePath (L65-80) |
| `internal/cli/serve_cmd.go` | Server bootstrap — creates Server with opts but NO Runner (L33-142) |
| `internal/cli/run_cmd.go` | CLI run — creates Runner at L90 |
| `internal/cli/stage_cmds.go` | CLI stages — creates Runner via buildRunner() at L204 |
| `internal/cli/character_cmd.go` | CLI character commands (generate, select) |

### Technical Decisions

- Polling pattern (not SSE/WebSocket) for candidate generation status — consistent with existing dashboard patterns
- Single `character` pipeline stage (no separate generate/approve sub-stages)
- Character selection is a hard gate — pipeline runner refuses to proceed to images without it
- SCP ID-based reuse: `CheckExistingCharacter(scpID)` allows cross-project character sharing
- **Candidate storage: DB** — New `character_candidates` table (id, project_id, scp_id, candidate_num, image_path, description, status, created_at) rather than filesystem-only
- **Gate implementation: Runner level** — Character gate placed at TOP of `resumeFromApproval()` (L292, after `projectSvc` creation at L300, BEFORE `skipApproval` branch at L310). This covers both `runSkipApprovalPath` and `runApprovalPath`. Additional gate in `RunStage(StageImageGenerate)` at L663. Both return `&domain.DependencyError{Action: "image_generate", Missing: []string{"character"}}`.
- **Loading UI: Skeleton cards** — 4 skeleton placeholder cards shown during generation, HTMX `hx-trigger="every 2s"` polls candidates endpoint, swaps to real images when status transitions `generating` → `ready`
- **Runner wiring via RunnerConfig**: Add `CharacterSvc *service.CharacterService` field to `RunnerConfig` struct (NOT changing `NewRunner()` function signature). All 3 call sites (`run_cmd.go:90`, `stage_cmds.go:204`, `pipeline_test.go:275`) pass it via config.
- **API pipeline wiring**: `serve_cmd.go` must create `pipeline.NewRunner()` with `CharacterSvc` in config and pass via `api.WithPipelineRunner(runner)`. This also fixes the existing gap where `executeFullPipeline()` always fails with "pipeline runner not configured".
- **Character replacement**: "Generate New" replaces existing character for the SCP ID. `SelectCandidate()` already handles this (updates existing record if found).
- **Retry on failure**: Generation failure shows error message + "Retry" button in UI. Candidates table stores `status: "failed"` with error detail.
- **Character section placement**: Below progress bar, above pipeline actions. Collapsed with thumbnail when not in `character` stage; expanded when active.
- **Path safety**: `handleCandidateImage` validates `{num}` via `strconv.Atoi` + range check (1-10). Never use raw string URL params in filesystem paths.
- **Stage guard**: `handleGenerateCharacters` validates project is in `scenario` or `character` stage before proceeding. Prevents character generation from wrong stages (pending, images, complete).
- **Route namespace**: Candidate image serving on `/dashboard/` (consistent with existing scene image serving). JSON data operations on `/api/v1/`. No mixing.

## Implementation Plan

### Tasks

#### Layer 1: Domain & Storage (no external dependencies)

- [x] Task 1: Add `character` stage to domain state machine
  - File: `internal/domain/project.go`
  - Action: Add `StageCharacter = "character"` constant. Insert into `ValidStages` map and `StageOrder` array between `StageScenario` and `StageImages`: `[pending, scenario, character, images, tts, complete]`
  - Notes: `StageIndex()` automatically adjusts since it iterates `StageOrder`

- [x] Task 2: Add `StageCharacterSelect` pipeline stage constant
  - File: `internal/service/pipeline_orchestrator.go`
  - Action: Add `StageCharacterSelect PipelineStage = "character_select"` to the const block, before `StageImageGenerate`
  - Notes: Do NOT reference `StageScenarioApproval` for positioning — approval stages are being removed. Place it logically before image generation.

- [x] Task 3: Add `CharacterCandidate` domain model
  - File: `internal/domain/character.go`
  - Action: Add struct:
    ```go
    type CharacterCandidate struct {
        ID           string
        ProjectID    string
        SCPID        string
        CandidateNum int
        ImagePath    string
        Description  string
        Status       string // "pending", "generating", "ready", "failed"
        ErrorDetail  string
        CreatedAt    time.Time
    }
    ```
  - Notes: Must be defined BEFORE store CRUD methods that reference it (Task 5).

- [x] Task 4: Create `character_candidates` DB migration
  - File: `internal/store/migrations/011_character_candidates.sql` (NEW)
  - Action: Create table:
    ```sql
    CREATE TABLE IF NOT EXISTS character_candidates (
        id            TEXT PRIMARY KEY,
        project_id    TEXT NOT NULL,
        scp_id        TEXT NOT NULL,
        candidate_num INTEGER NOT NULL,
        image_path    TEXT NOT NULL DEFAULT '',
        description   TEXT NOT NULL DEFAULT '',
        status        TEXT NOT NULL DEFAULT 'pending',
        error_detail  TEXT NOT NULL DEFAULT '',
        created_at    TEXT NOT NULL DEFAULT (datetime('now'))
    );
    CREATE INDEX idx_character_candidates_project ON character_candidates(project_id);
    ```
  - Notes: `status` values: `pending`, `generating`, `ready`, `failed`. No FK to projects (projects may be deleted independently).

- [x] Task 5: Add candidate CRUD to store layer
  - File: `internal/store/character.go`
  - Action: Add methods:
    - `CreateCandidateBatch(projectID, scpID string, count int) error` — insert N rows with status `pending`, UUID for each id
    - `ListCandidatesByProject(projectID string) ([]*domain.CharacterCandidate, error)` — SELECT ordered by candidate_num
    - `UpdateCandidateStatus(id, status, imagePath, description, errorDetail string) error` — update individual candidate
    - `DeleteCandidatesByProject(projectID string) error` — cleanup old candidates
  - Notes: Follow existing `scanCharacters` helper pattern for row scanning.

#### Layer 2: Service Logic

- [x] Task 6: Extend CharacterService for DB-backed candidates
  - File: `internal/service/character.go`
  - Action: Modify `GenerateCandidates()` signature to include `projectID`:
    ```go
    func (cs *CharacterService) GenerateCandidates(ctx context.Context, projectID, scpID string, count int, workspacePath string) ([]*domain.CharacterCandidate, error)
    ```
    Implementation:
    1. Call `store.DeleteCandidatesByProject(projectID)` to clear old candidates
    2. Call `store.CreateCandidateBatch(projectID, scpID, count)` with status `pending`
    3. For each candidate: update status to `generating` → run LLM+ImageGen → update to `ready` (or `failed` with error detail)
    4. Return `[]*domain.CharacterCandidate` instead of `[]CandidateResult`
  - Add new method: `ListCandidates(projectID string) ([]*domain.CharacterCandidate, error)` — delegates to store
  - Add new method: `GetCandidateGenerationStatus(projectID string) (string, error)` — returns aggregate status: `"empty"` (no rows), `"generating"` (any pending/generating), `"ready"` (all ready), `"failed"` (any failed, none generating)
  - Notes: Keep filesystem writes for actual image files; DB tracks metadata + status. Update CLI `character generate` command in `character_cmd.go` to pass a project ID or handle the new signature.

- [x] Task 7: Add character gate to pipeline runner
  - File: `internal/pipeline/runner.go`
  - Action:
    1. Add `characterSvc *service.CharacterService` field to `Runner` struct
    2. Add `CharacterSvc *service.CharacterService` field to `RunnerConfig` struct
    3. In `NewRunner()`: assign `cfg.CharacterSvc` to `r.characterSvc`
    4. **Character gate in `resumeFromApproval()` — EXACT PLACEMENT**: Insert at L300 (after `projectSvc := service.NewProjectService(r.store)` and `approvalSvc` creation), BEFORE the `skipApproval` branch at L310. This ensures BOTH `runSkipApprovalPath` and `runApprovalPath` are gated:
       ```go
       // Character gate: must have selected character before image generation
       // Placed before skipApproval branch to cover both paths
       if r.characterSvc != nil {
           char, err := r.characterSvc.CheckExistingCharacter(project.SCPID)
           if err != nil {
               return nil, fmt.Errorf("pipeline: check character: %w", err)
           }
           if char == nil || char.SelectedImagePath == "" {
               return nil, &domain.DependencyError{
                   Action: "image_generate", Missing: []string{"character"},
               }
           }
           // Wire selected character image for all downstream ImageGenService instances
           r.selectedCharacterImagePath = char.SelectedImagePath
       }
       ```
    5. Add `selectedCharacterImagePath string` field to Runner (set in gate, used in image gen calls)
    6. In every `NewImageGenService()` call within runner (4 locations in `runApprovalPath` L380, `runParallelGeneration` L748, `runImageGenerateStage` L815, `RunImageRegenerate` L848): add after creation:
       ```go
       imgSvc.SetCharacterService(r.characterSvc)
       if r.selectedCharacterImagePath != "" {
           _ = imgSvc.SetSelectedCharacterImage(r.selectedCharacterImagePath)
       }
       ```
    7. In `RunStage()` case `StageImageGenerate` (L663): add character gate check (same as step 4 logic)
  - Notes: `r.characterSvc` is nil-safe — existing code without character service continues to work. `selectedCharacterImagePath` avoids re-querying DB at each image gen call.

#### Layer 3: API Endpoints

- [x] Task 8: Add character API handlers
  - File: `internal/api/character_handlers.go` (NEW)
  - Action: Implement 4 handlers:
    1. `handleGenerateCharacters(w, r)` — POST `/api/v1/projects/{id}/characters/generate`
       - Get project from store, validate exists
       - **Stage guard**: validate `project.Status` is `domain.StageScenario` or `domain.StageCharacter`. Otherwise return 409 with `"INVALID_STAGE"` error.
       - Check `requirePlugin(w, r, "llm")` and `requirePlugin(w, r, "imagegen")`
       - Set project stage to `character` via `projectSvc.SetProjectStage()`
       - Start async job via `s.jobs` with type `"character_generate"`: goroutine calls `s.characterSvc.GenerateCandidates(ctx, project.ID, project.SCPID, 4, project.WorkspacePath)`
       - Return 202 with `{job_id: "..."}`
    2. `handleListCandidates(w, r)` — GET `/api/v1/projects/{id}/characters/candidates`
       - **Validate project exists** first (get from store, return 404 if not found)
       - Get candidates from `s.characterSvc.ListCandidates(project.ID)`
       - Get aggregate status from `s.characterSvc.GetCandidateGenerationStatus(project.ID)`
       - Return `{status: "generating|ready|empty|failed", candidates: [...]}`
    3. `handleSelectCharacter(w, r)` — POST `/api/v1/projects/{id}/characters/select`
       - Get project from store, validate exists
       - Parse `{candidate_num: N}` from request body
       - Call `s.characterSvc.SelectCandidate(project.SCPID, N, project.WorkspacePath)`
       - Set project stage to `character` (confirms stage)
       - Return 200 with selected character data
    4. `handleGetCharacter(w, r)` — GET `/api/v1/projects/{id}/characters`
       - Get project from store, validate exists
       - Call `s.characterSvc.CheckExistingCharacter(project.SCPID)`
       - Return character data or 404 if none
  - Notes: Follow existing handler pattern. All 4 handlers start by fetching project from store to validate existence and extract SCPID/WorkspacePath.

- [x] Task 9: Add `WithCharacterService` server option and wire routes
  - File: `internal/api/server.go`
  - Action:
    1. Add `characterSvc *service.CharacterService` field to `Server` struct
    2. Add `WithCharacterService(svc *service.CharacterService) ServerOption`
    3. In `setupRouter()` under `/api/v1` route group, add:
       ```go
       // Character management
       r.Post("/projects/{id}/characters/generate", s.handleGenerateCharacters)
       r.Get("/projects/{id}/characters/candidates", s.handleListCandidates)
       r.Post("/projects/{id}/characters/select", s.handleSelectCharacter)
       r.Get("/projects/{id}/characters", s.handleGetCharacter)
       ```
    4. In `setupRouter()` under dashboard pages section, add candidate image route:
       ```go
       r.Get("/dashboard/projects/{id}/characters/candidates/{num}/image", s.handleCandidateImage)
       ```
  - Notes: Place API routes after scene routes, before jobs routes. Dashboard route alongside other dashboard asset routes.

#### Layer 4: Dashboard UI

- [x] Task 10: Update dashboard UI stage mapping and data
  - File: `internal/api/dashboard.go`
  - Action:
    1. Add `"character"` to `UIStageOrder`: `["pending", "scenario", "character", "assets", "assemble", "complete"]`
    2. In `projectDetailData` struct: add `Character *domain.Character`, `CharacterCandidates []*domain.CharacterCandidate`, and `CharacterStatus string` fields
    3. In `handleProjectDetail()`: load character data:
       ```go
       if s.characterSvc != nil {
           data.Character, _ = s.characterSvc.CheckExistingCharacter(project.SCPID)
           data.CharacterCandidates, _ = s.characterSvc.ListCandidates(project.ID)
           data.CharacterStatus, _ = s.characterSvc.GetCandidateGenerationStatus(project.ID)
       }
       ```
    4. **Character dependency** — set OUTSIDE `computeDependencies()` (which is filesystem-based), directly in `handleProjectDetail()`:
       ```go
       data.DependenciesMet["character"] = data.Character != nil && data.Character.SelectedImagePath != ""
       ```
    5. Add `"character_select"` to `pipelineStageLabel()` map:
       ```go
       "character_select": "Generating Characters..."
       ```
    6. Add `"character_generate"` to job type polling in `handleProjectDetail()` (alongside existing `"image_generate"`, `"tts_generate"`, `"assembly"` checks):
       ```go
       case "character_generate":
           data.Job.StageLabel = "Generating character candidates..."
       ```
  - Notes: Backend `"character"` stage maps directly to UI `"character"` (no folding like images/tts → assets). `computeDependencies()` function signature unchanged.

- [x] Task 11: Create character section template partial
  - File: `internal/api/templates/_partials/character_section.html` (NEW)
  - Action: Create HTMX-enabled character management section with these states:
    - **No character + no candidates + stage is `character`**: "Generate Characters" button
    - **Candidates generating**: 4 skeleton cards (`animate-pulse bg-base-300`) with polling:
      ```html
      <div id="character-candidates"
           hx-get="/api/v1/projects/{{.ProjectID}}/characters/candidates"
           hx-trigger="every 2s"
           hx-target="#character-candidates"
           hx-swap="innerHTML">
      ```
    - **Candidates ready**: 4 image cards with description, clickable to select:
      ```html
      <div class="card cursor-pointer hover:ring-2 ring-primary"
           hx-post="/api/v1/projects/{{.ProjectID}}/characters/select"
           hx-vals='{"candidate_num": {{.CandidateNum}}}'
           hx-target="#project-content" hx-swap="innerHTML">
          <img src="/dashboard/projects/{{.ProjectID}}/characters/candidates/{{.CandidateNum}}/image" />
          <p>{{.Description}}</p>
      </div>
      ```
    - **Candidates failed**: Error message + "Retry" button (same as Generate button, re-triggers generation)
    - **Character selected (stage > character)**: Collapsed card with selected character thumbnail + description + "Generate New" option
    - **Existing character from other project (stage is `character`, Character != nil)**: Reuse prompt with character card + "Reuse This Character" / "Generate New" buttons
  - Notes: Use DaisyUI card/collapse components. Image cards served via `/dashboard/` route (Task 14).

- [x] Task 12: Register character section partial in template init
  - File: `internal/api/dashboard.go`
  - Action: In `initDashboardTemplates()`, add to `partialFiles` slice:
    ```go
    "templates/_partials/character_section.html",
    ```
  - Notes: Without this registration, `{{template "character_section" .}}` will fail at runtime.

- [x] Task 13: Integrate character section into project detail page
  - File: `internal/api/templates/project_detail.html`
  - Action:
    1. After progress bar card (line ~48), before pipeline actions card (line ~73), insert:
       ```html
       <!-- Character Selection -->
       {{template "character_section" .}}
       ```
    2. In pipeline actions: when stage is `character`, show only character-related actions (hide image/tts/assemble buttons until character is selected)
    3. When stage is `scenario`, the "Generate Assets" button should transition to `character` stage first (instead of directly generating assets)
  - Notes: Character section is always rendered but visually collapsed (DaisyUI collapse) when not in `character` stage. Shows selected character thumbnail as a compact badge when stage > character.

- [x] Task 14: Update progress bar for character stage
  - File: `internal/api/templates/_partials/progress_bar.html`
  - Action: No code change needed — template iterates `.StageOrder` dynamically. Verify that the `character` stage renders correctly with proper step-primary highlighting after Task 10 updates `UIStageOrder`.

- [x] Task 15: Add candidate image serving route with path safety
  - File: `internal/api/dashboard.go`
  - Action: Add handler:
    ```go
    func (s *Server) handleCandidateImage(w http.ResponseWriter, r *http.Request) {
        projectID := chi.URLParam(r, "id")
        // Validate candidate number — SECURITY: prevent path traversal
        num, err := strconv.Atoi(chi.URLParam(r, "num"))
        if err != nil || num < 1 || num > 10 {
            WriteError(w, r, http.StatusBadRequest, "INVALID_CANDIDATE",
                "candidate number must be 1-10")
            return
        }

        project, err := s.store.GetProject(projectID)
        if err != nil {
            WriteError(w, r, http.StatusNotFound, "NOT_FOUND", "project not found")
            return
        }

        imgPath := filepath.Join(project.WorkspacePath, project.SCPID,
            "characters", fmt.Sprintf("candidate_%d.png", num))
        http.ServeFile(w, r, imgPath)
    }
    ```
  - Notes: Route registered in Task 9 under `/dashboard/` namespace. `strconv.Atoi` + range check eliminates path traversal risk — `num` is always a validated integer.

#### Layer 5: Wiring & Integration

- [x] Task 16: Wire CharacterService in API server bootstrap (`serve_cmd.go`)
  - File: `internal/cli/serve_cmd.go` (L33-142, `runServeCmd` function)
  - Action:
    1. Create `CharacterService` and wire plugins:
       ```go
       characterSvc := service.NewCharacterService(db)
       if plugins.LLM != nil {
           characterSvc.SetLLM(plugins.LLM)
       }
       if plugins.ImageGen != nil {
           characterSvc.SetImageGen(plugins.ImageGen)
       }
       ```
    2. Create `pipeline.Runner` with CharacterService (currently missing entirely from serve_cmd.go):
       ```go
       runner := pipeline.NewRunner(db, plugins.LLM, plugins.ImageGen, plugins.TTS, assembler, g, slog.Default(), pipeline.RunnerConfig{
           SCPDataPath:   c.SCPDataPath,
           WorkspacePath: c.WorkspacePath,
           Voice:         c.TTS.Voice,
           // ... other config fields ...
           CharacterSvc:  characterSvc,
       })
       ```
    3. Add to server options:
       ```go
       opts = append(opts,
           api.WithPipelineRunner(runner),
           api.WithCharacterService(characterSvc),
       )
       ```
  - Notes: This ALSO fixes the existing bug where `executeFullPipeline()` always fails with "pipeline runner not configured". The runner needs the same config fields as `run_cmd.go:90` — reference that for the complete `RunnerConfig`.

- [x] Task 17: Wire CharacterService in CLI runner bootstrap
  - File: `internal/cli/run_cmd.go` (L90) and `internal/cli/stage_cmds.go` (L204, `buildRunner()`)
  - Action: In both files, create `CharacterService` and pass via config:
    ```go
    characterSvc := service.NewCharacterService(db)
    characterSvc.SetLLM(llmPlugin)
    characterSvc.SetImageGen(imgPlugin)

    runner := pipeline.NewRunner(db, ..., pipeline.RunnerConfig{
        // ... existing fields ...
        CharacterSvc: characterSvc,
    })
    ```
  - Notes: `stage_cmds.go` uses a `buildRunner()` helper function — add CharacterService creation inside it. Integration test at `pipeline_test.go:275` should also be updated to pass CharacterSvc (can be nil for existing tests).

### Acceptance Criteria

#### Happy Path

- [x] AC 1: Given a project in `scenario` stage, when user clicks "Generate Characters" on dashboard, then project stage transitions to `character`, 4 skeleton cards appear, and candidates are generated asynchronously via job system
- [x] AC 2: Given candidates with status `ready`, when user clicks a candidate card, then the character is created/updated in DB with `selected_image_path` set
- [x] AC 3: Given a selected character, when pipeline runner reaches image generation (via BOTH `runSkipApprovalPath` and `runApprovalPath`), then `ImageGenService` has character reference image loaded and `CharacterService` wired for auto-reference
- [x] AC 4: Given a project with stage `character` and a selected character, when user triggers image generation (via UI or pipeline), then images are generated using the selected character as reference
- [x] AC 5: Given SCP-173 has an existing character from a previous project, when a new project for SCP-173 reaches `character` stage, then the UI shows "Reuse This Character" option with the existing character's thumbnail
- [x] AC 6: Given user selects "Reuse This Character", then the project skips candidate generation and proceeds to `images` stage eligibility

#### Error Handling

- [x] AC 7: Given no character selected for a project, when pipeline runner tries to execute `image_generate` stage (via `resumeFromApproval`, `RunStage`, or `executeFullPipeline`), then it returns `DependencyError{Action: "image_generate", Missing: ["character"]}` (HTTP 409)
- [x] AC 8: Given candidate generation fails (LLM or ImageGen error), when user views candidates page, then failed candidates show error message and "Retry" button is available
- [x] AC 9: Given a project without LLM/ImageGen plugins configured, when user clicks "Generate Characters", then API returns 502 with appropriate plugin unavailable message
- [x] AC 10: Given a project in `pending` or `images` stage, when user calls POST `/characters/generate`, then API returns 409 with `INVALID_STAGE` error

#### Edge Cases

- [x] AC 11: Given candidates are still `generating`, when user polls GET `/characters/candidates`, then response has `status: "generating"` with partial results (ready candidates shown, pending shown as skeleton)
- [x] AC 12: Given a project is deleted, when character candidates exist for it, then `handleListCandidates` returns 404 because project existence is validated first (not relying on empty query results)
- [x] AC 13: Given user clicks "Generate New" when existing character exists, when new candidates are generated, then old candidates are deleted and new ones created; existing character record is updated (not duplicated) upon selection

## Additional Context

### Dependencies

- **Existing CharacterService**: Already has `GenerateCandidates()`, `SelectCandidate()`, `CheckExistingCharacter()` — reuse, extend for DB candidates
- **ImageGenService.SetCharacterService()**: Method exists but unwired in pipeline runner — must wire it
- **ImageGenService.SetSelectedCharacterImage()**: Method exists — call after character selection
- **Pipeline Runner**: Must add `characterSvc *CharacterService` field and inject it via `RunnerConfig`
- **DB migration 011**: New `character_candidates` table; depends on existing `characters` table from migration 004
- **Dashboard template system**: Embed FS (`//go:embed all:templates`), DaisyUI components, HTMX attributes
- **Job system**: Character generation should use `jobManager` for async tracking (same pattern as image/tts generation)
- **serve_cmd.go pipeline runner**: Currently missing — creating it also fixes existing `executeFullPipeline()` nil runner bug

### Testing Strategy

**Unit Tests:**
- `internal/store/character_test.go` — Add tests for `CreateCandidateBatch`, `ListCandidatesByProject`, `UpdateCandidateStatus`, `DeleteCandidatesByProject` using `:memory:` SQLite
- `internal/service/character_test.go` — Test `GetCandidateGenerationStatus()` aggregation logic with various candidate status combinations
- `internal/domain/project_test.go` — Verify `StageCharacter` is in `ValidStages` and `StageOrder`, and `StageIndex("character")` returns correct position (2)

**Integration Tests (build tag: `integration`):**
- `internal/pipeline/runner_test.go` — Test character gate: runner with nil character returns `DependencyError`; runner with valid character proceeds to image generation. Test BOTH skip-approval and approval paths.
- `internal/api/character_handlers_test.go` — HTTP handler tests: generate (202 + job + stage guard), list candidates (status variants + project validation), select (200 + character created), get character (200 or 404)

**Manual Testing:**
1. Start server with LLM + ImageGen plugins
2. Create project → generate scenario → reach `character` stage
3. Click "Generate Characters" → verify skeleton cards + job indicator in dashboard → verify cards populate
4. Click a card → verify selection → verify stage progression
5. Create another project with same SCP ID → verify reuse option appears
6. Try to generate images without character → verify 409 block
7. Try to generate characters from `pending` stage → verify 409 stage guard
8. Test full pipeline with `--auto-approve` → verify character gate fires before parallel generation

### Notes

- Existing projects will be manually deleted by user — no backward compatibility needed
- All approval-related stages are being removed from the state machine
- `ImageGenService.SetCharacterService()` has been implemented but never wired — this spec fixes that gap
- The `CandidateResult` struct in `service/character.go` may be deprecated in favor of the new `domain.CharacterCandidate` model — but keep it for backward compatibility with CLI `character generate` command until CLI is updated
- Candidate images are stored on filesystem (workspace path); only metadata/status tracked in DB
- Creating the pipeline runner in `serve_cmd.go` (Task 16) also fixes the pre-existing bug where `executeFullPipeline()` could never succeed via API
- Consider adding `project_id` to the character selection flow so the same SCP character can be customized per-project in the future (out of scope for now)
