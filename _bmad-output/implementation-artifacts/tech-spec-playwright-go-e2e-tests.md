---
title: 'Playwright-Go E2E Test Framework'
slug: 'playwright-go-e2e-tests'
created: '2026-03-15'
status: 'implementation-complete'
stepsCompleted: [1, 2, 3, 4]
tech_stack: ['Go 1.25.7', 'playwright-go', 'Chromium', 'HTMX', 'chi/v5', 'modernc.org/sqlite', 'testify']
files_to_modify:
  - 'go.mod'
  - 'Makefile'
  - '.github/workflows/deploy.yml'
  - 'tests/e2e/setup_test.go (new)'
  - 'tests/e2e/helpers_test.go (new)'
  - 'tests/e2e/pipeline_test.go (new)'
  - 'tests/e2e/dashboard_test.go (new)'
  - 'tests/e2e/scenes_test.go (new)'
  - 'tests/e2e/characters_test.go (new)'
code_patterns: ['NewServer(store, cfg, ...ServerOption)', 'store.New(":memory:")', 'testify assert/require', '//go:build tags', 'ServerOption DI (8 With* funcs)', 'net.Listen(":0") for random port', 't.Cleanup() for teardown', 'CharacterService.SetLLM/SetImageGen', 'WithPluginStatus(all true)']
test_patterns: ['//go:build e2e', 'playwright.Run()', 'page.Goto()', 'page.Locator().WaitFor()', 'page.OnDialog()', 'require.NoError for setup', 'assert.* for assertions', 'table-driven tests']
---

# Tech-Spec: Playwright-Go E2E Test Framework

**Created:** 2026-03-15

## Overview

### Problem Statement

Browser-level bugs are not caught by unit tests. During character selection implementation (2026-03-15), 6 browser-level bugs were discovered that unit tests could not detect: HTMX hx-post rendering raw JSON, missing stage in validStates map causing 409 errors, template condition ordering causing UI not updating after clicks, etc. The project has zero browser-level test coverage.

### Solution

Introduce `playwright-go` to run E2E browser tests within the Go test ecosystem. Tests start an in-process server (in-memory SQLite) and drive Chromium to verify full user flows. Runs locally via `go test -tags=e2e` and in CI as a post-unit-test GitHub Actions job.

### Scope

**In Scope:**
- playwright-go setup and test helpers (in-process server, browser lifecycle)
- Pipeline flow (priority): create project → generate scenario → generate/select character → generate images → generate TTS → assemble — stage-based transitions
- Dashboard: SCP accordion groups, filtering, SCP search, pagination
- Scene editing: narration update, image/TTS regeneration, scene insert/delete
- Character candidate polling + select/deselect
- HTMX polling (3s auto-refresh during jobs, 2s for character candidates)
- JavaScript-driven flows: project creation (fetch API), scene insert (modal), asset generation buttons
- Native browser dialog handling (`hx-confirm` → `page.OnDialog()`)
- `//go:build e2e` build tag, `make test-e2e` target
- GitHub Actions job (post-unit-test, parallel with build-and-push)
- Local execution: `go test -tags=e2e ./tests/e2e/...`

**Out of Scope:**
- Real external API calls (LLM, TTS, ImageGen — all mocked at plugin interface level)
- Performance/load testing
- Cross-browser (Chromium only)
- Review page (external share link — separate sprint)

## Context for Development

### Codebase Patterns

- **Server construction**: `api.NewServer(store, config, ...ServerOption)` — 8 `With*` options for DI
  - `WithScenarioService`, `WithImageGenService`, `WithTTSService`, `WithAssemblerService`, `WithCharacterService`, `WithPipelineRunner`, `WithRegistry`, `WithPluginStatus`
  - Default services created internally: `ProjectService`, `ReviewService`, `jobManager`
  - **CRITICAL**: Default `pluginStatus` is `map[string]bool{"llm": false, "imagegen": false, "tts": false, "output": false}` — must override with `WithPluginStatus(map[string]bool{"llm": true, "imagegen": true, "tts": true, "output": true})` or all plugin-gated handlers return 502
- **CharacterService special wiring**: Constructor only takes `*store.Store`. Must call `SetLLM(llm)` and `SetImageGen(ig)` after construction, before passing to server — otherwise `GenerateCandidates()` returns "provider not set" error
- **Existing test pattern**: `store.New(":memory:")` + `config.Config{WorkspacePath: t.TempDir()}` + `httptest.NewRequest/NewRecorder` + `srv.Router().ServeHTTP(w, req)`
- **Stage-based model**: `pending → scenario → character → images → tts → complete` (progress markers, not gates — any valid stage can be set directly via `PATCH /api/v1/projects/{id}/stage`)
- **UI interaction patterns**:
  - **HTMX**: Stage filter dropdown, SCP search (300ms debounce), project detail polling (3s), character polling (2s), scene image/TTS regeneration, scene delete, stage transition via progress bar
  - **JavaScript (fetch API)**: Project creation (`createProject()`), scenario generation (`runScenario()`), character generation (`generateCharacters()`), character selection (`selectCharacter()`), character deselection (`deselectCharacter()`), asset generation (`generateImages()`, `generateTTS()`), assembly (`runAssemble()`), scene insert (`insertScene()`)
  - **Native dialogs**: `hx-confirm` triggers browser `confirm()` on delete, stage change, regenerate
- **Dashboard structure**: Projects are organized in **SCP accordion groups** (collapsible), not flat cards. Each group shows SCP ID, project count, and expandable project list
- **SceneDashboardService**: Created on-the-fly in dashboard handlers (`service.NewSceneDashboardService(s.store, slog.Default())`), NOT injected — no ServerOption needed
- **Auth**: Bearer token via `Authorization` header, can be disabled via `AuthConfig.Enabled = false`
- **Templates**: Go `html/template` with `//go:embed all:templates`, partials in `_partials/`, HTMX partial vs full page via `isHTMX(r)` check
- **Webhooks**: `NewWebhookNotifier(cfg.Webhooks)` safely returns nil if no URLs configured. All webhook methods nil-check before operating. No special config needed for E2E.
- **Layer rule**: domain → store → service → api/cli (reverse import forbidden)

### Plugin Interfaces (Mock Targets)

| Interface | Package | Methods | Notes |
| --------- | ------- | ------- | ----- |
| `LLM` | `internal/plugin/llm` | `Complete()`, `GenerateScenario()`, `RegenerateSection()` | Returns `*CompletionResult`, `*domain.ScenarioOutput`, `*domain.SceneScript` |
| `ImageGen` | `internal/plugin/imagegen` | `Generate()`, `Edit()` | Returns `*ImageResult` (ImageData []byte, Format, Width, Height) |
| `TTS` | `internal/plugin/tts` | `Synthesize()`, `SynthesizeWithOverrides()` | Returns `*SynthesisResult` (AudioData []byte, WordTimings, DurationSec) |
| `Assembler` | `internal/plugin/output` | `Assemble()`, `Validate()` | Returns `*AssembleResult`. Validate checks output path validity |

- mockery v2 registered via `//go:generate` but `internal/mocks/` package doesn't exist yet (known issue)
- E2E fakes will be hand-written in `tests/e2e/helpers_test.go` — simpler than mockery for static behavior

### Service Constructor Signatures (Exact)

```go
// Required for E2E fake wiring:
service.NewProjectService(s *store.Store) *ProjectService
service.NewScenarioService(s *store.Store, l llm.LLM, ps *ProjectService) *ScenarioService
service.NewImageGenService(ig imagegen.ImageGen, s *store.Store, logger *slog.Logger) *ImageGenService
service.NewTTSService(t tts.TTS, g *glossary.Glossary, s *store.Store, logger *slog.Logger) *TTSService
service.NewCharacterService(s *store.Store) *CharacterService  // then call .SetLLM() and .SetImageGen()
service.NewAssemblerService(a output.Assembler, ps *ProjectService) *AssemblerService
service.NewReviewService(s *store.Store, logger *slog.Logger) *ReviewService
```

**IMPORTANT**: `TTSService` requires `*glossary.Glossary` — MUST use `glossary.New()`, NOT nil. Nil causes panic in `buildOverrides()` which calls `s.glossary.Entries()`.

### requirePlugin Handler Gates

These handlers call `s.requirePlugin()` and return 502 if the plugin is not enabled in `pluginStatus`:

| Handler | Plugin(s) Required |
| ------- | ------------------ |
| `handleRunPipeline` | `llm` |
| `handleGenerateCharacters` | `llm` AND `imagegen` |
| `handleGenerateImages` | `imagegen` |
| `handleGenerateTTS` | `tts` |
| `handleAssemble` | `output` |

### Server Startup Details

- `NewServer()` chain: create defaults → apply ServerOptions → `initReviewTemplate()` → `initDashboardTemplates()` → `setupRouter()`
- `Start()` blocks on `http.ListenAndServe()` — E2E needs goroutine wrapper
- **E2E pattern uses `http.Serve(listener, srv.Router())`** which bypasses `Start()`. This means `s.httpServer` is nil and `Shutdown()` cannot be called. Cleanup must use `listener.Close()` directly.
- `Router()` returns `chi.Router` for direct use
- Middleware stack: Recovery → RequestID → Logging → Auth (with path exemptions)

### Seed Strategy

**`seedProjectAtStage()` uses store-level seeding + API stage PATCH, NOT pipeline runner:**

The pipeline runner requires disk-based SCP data, workspace files, and full orchestration stack — too complex for E2E helpers. Instead:

1. **Create project** via `POST /api/v1/projects {"scp_id": "SCP-173"}`
2. **Seed data directly in store**: Insert scenes (narration, visual desc, image prompt) and scene assets (image/audio file paths pointing to temp files) via `store.DB()` SQL or store methods
3. **Set stage** via `PATCH /api/v1/projects/{id}/stage {"stage": "scenario"}` (or target stage)
4. **For character stage**: Also insert character candidates in store, then `POST /api/v1/projects/{id}/characters/select {"candidate_num": 1}`

This exercises real API handlers for stage transitions and character selection while avoiding the pipeline runner dependency entirely.

### Files to Reference

| File | Purpose |
| ---- | ------- |
| `internal/api/server.go` | Server struct (14 fields), NewServer, 8 With* options, Start/Shutdown/Router, default pluginStatus |
| `internal/api/server_test.go` | `setupTestServer()` — in-memory store + tmpDir + config pattern |
| `internal/api/dashboard_test.go` | `setupDashboardServer()` — auth toggle, HTMX partial testing |
| `internal/api/dashboard.go` | `initDashboardTemplates()`, `isHTMX()`, `SceneDashboardService` created on-the-fly |
| `internal/api/templates/_layout.html` | JS functions: `createProject()`, `runScenario()`, `generateCharacters()`, `selectCharacter()`, etc. |
| `internal/api/templates/dashboard.html` | SCP accordion groups, "New Project" modal with SCP search |
| `internal/api/templates/project_detail.html` | Stage actions (JS onclick), delete (hx-delete + hx-confirm), polling |
| `internal/api/templates/_partials/progress_bar.html` | Clickable stages (hx-patch + hx-confirm), disabled when job running or pending |
| `internal/api/templates/_partials/scene_card.html` | Scene actions, `insert_scene_btn` template (JS onclick insertScene) |
| `internal/api/templates/_partials/character_section.html` | 5 states: generating (polling), ready, failed, empty, selected |
| `internal/domain/project.go` | 6 stage constants, ValidStages map, StageOrder slice |
| `internal/store/store.go` | `New(":memory:")`, embedded migrations, WAL mode |
| `internal/config/config.go` | Config struct — minimum: WorkspacePath + API.Host/Port |
| `internal/glossary/glossary.go` | `glossary.New()` — no params, returns empty glossary |
| `internal/service/character.go` | `NewCharacterService(store)` + `SetLLM()` + `SetImageGen()` — both required for GenerateCandidates |
| `internal/plugin/llm/interface.go` | LLM interface (3 methods) |
| `internal/plugin/imagegen/interface.go` | ImageGen interface (2 methods), GenerateOptions, EditOptions |
| `internal/plugin/tts/interface.go` | TTS interface (2 methods), SynthesisResult, TTSOptions |
| `internal/plugin/output/interface.go` | Assembler interface (2 methods): Assemble, Validate |
| `tests/integration/pipeline_test.go` | `testStore()`, `testWorkspace()`, `skipIfNoKey()` patterns |
| `Makefile` | 10 targets (no e2e yet) |
| `.github/workflows/deploy.yml` | 4 jobs: test → build-and-push → deploy → notify. Uses actions/checkout@v5 |
| `_bmad-output/project-context.md` | Testing rules, layer architecture, coding conventions |

### Technical Decisions

1. **Separate spec file**: Uses `tech-spec-playwright-go-e2e-tests.md` (not `tech-spec-wip.md`) since another spec is in progress in a different session.
2. **In-process server via `net.Listen(":0")`**: Random free port, goroutine-based start, `listener.Close()` for cleanup (not `srv.Shutdown()` since `Start()` is bypassed). No Docker needed locally.
3. **Mock at plugin interface level**: Hand-written fakes for `llm.LLM`, `imagegen.ImageGen`, `tts.TTS`, `output.Assembler` in `helpers_test.go`. Injected via service constructors → `ServerOption`. Full HTTP → handler → service → store chain exercises real code.
4. **Service wiring for mocks**: Create real services with fake plugins using exact constructor signatures. `CharacterService` additionally requires `SetLLM()` and `SetImageGen()` after construction. All plugins enabled via `WithPluginStatus(map[string]bool{"llm": true, "imagegen": true, "tts": true, "output": true})`.
5. **Auth disabled for E2E**: `AuthConfig.Enabled = false` — auth middleware is already unit-tested. E2E focuses on UI flow.
6. **HTMX verification via DOM change detection**: Use `page.Locator().WaitFor()` to detect UI updates from polling. Tests verify "UI updated correctly" rather than "HTTP request happened".
7. **Build tag `e2e`**: Three-tier: `go test ./...` (unit) → `go test -tags=e2e` (browser) → `go test -tags=integration` (real APIs).
8. **Test isolation**: Each test function gets fresh server + fresh in-memory DB. `t.Parallel()` safe via port 0.
9. **Chromium only**: Fastest to install, sufficient coverage.
10. **CI browser install**: `go run github.com/playwright-community/playwright-go/cmd/playwright install --with-deps chromium`
11. **Naming conventions**: Follow project standard — `TestMethodName_Scenario`, `require.NoError` for setup, `assert.*` for assertions, table-driven where applicable.
12. **Playwright shared instance**: `TestMain(m *testing.M)` initializes `playwright.Run()` once at package level. Each test creates a new `browser.NewContext()` → `context.NewPage()` for cookie/storage isolation. Avoids ~1-2s startup per test.
13. **Seed via store + PATCH stage (not pipeline runner)**: `seedProjectAtStage()` inserts scenes/assets directly into store via SQL, then sets stage via API PATCH. Avoids needing pipeline runner, workspace files, or SCP data on disk. Fast and deterministic.
14. **CI runs after unit tests, not deploy**: E2E uses in-process server, not the deployed instance. `needs: test` (not `needs: deploy`) so E2E runs parallel with build-and-push.
15. **No testdata directory**: Fake plugins return canned responses without reading SCP text. `seedProject` only needs `POST /api/v1/projects {"scp_id": "SCP-173"}`. No file-based SCP data required.
16. **Native dialog handling**: All `hx-confirm` triggers and JS `confirm()` calls produce native browser dialogs. Tests MUST register `page.OnDialog()` to accept/dismiss before clicking buttons that trigger confirms.
17. **JavaScript-driven interactions**: Project creation, scenario generation, character actions, asset generation, and scene insert all use JavaScript `fetch()` + `onclick` handlers (NOT HTMX). Playwright handles these natively — just click the button and wait for navigation/DOM change.

### Directory Structure

```
tests/e2e/
├── setup_test.go        # TestMain — package-level playwright init, shared browser instance
├── helpers_test.go      # StartTestServer(), newPage(), seedProject(), seedProjectAtStage(), fake plugins
├── pipeline_test.go     # Pipeline stage flow (priority 1)
├── dashboard_test.go    # Dashboard filtering, SCP search, pagination
├── scenes_test.go       # Scene editing, insert/delete, narration update
└── characters_test.go   # Character generation polling, select/deselect
```

## Implementation Plan

### Tasks

#### Task 1: Add playwright-go dependency

- **File:** `go.mod`
- **Action:** `go get github.com/playwright-community/playwright-go`
- **Notes:** This is the only new dependency. Run `go mod tidy` after.

#### Task 2a: Create TestMain setup (`tests/e2e/setup_test.go`)

- **File:** `tests/e2e/setup_test.go` (new)
- **Action:**
  1. `//go:build e2e` build tag at top
  2. Package-level vars: `var pw *playwright.Playwright`, `var browser playwright.Browser`
  3. **`TestMain(m *testing.M)`**:
     - `pw, err = playwright.Run()` — install/start playwright once
     - `browser, err = pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{Headless: playwright.Bool(true)})`
     - `code := m.Run()`
     - `browser.Close()`, `pw.Stop()`
     - `os.Exit(code)`
- **Notes:** Single playwright + browser instance shared across all tests. Each test creates isolated `BrowserContext` via `newPage()` helper.

#### Task 2b: Create test helpers (`tests/e2e/helpers_test.go`)

- **File:** `tests/e2e/helpers_test.go` (new)
- **Action:** Create the E2E test foundation with:
  1. `//go:build e2e` build tag at top
  2. **`fakeLLM` struct** implementing `llm.LLM`:
     - `GenerateScenario()` → returns a canned `*domain.ScenarioOutput` with 3 scenes (narration, visual desc, image prompt, mood, fact tags)
     - `RegenerateSection()` → returns a canned `*domain.SceneScript`
     - `Complete()` → returns a canned `*CompletionResult`
  3. **`fakeImageGen` struct** implementing `imagegen.ImageGen`:
     - `Generate()` → returns a 1x1 PNG byte slice as `*ImageResult`
     - `Edit()` → returns same 1x1 PNG
  4. **`fakeTTS` struct** implementing `tts.TTS`:
     - `Synthesize()` → returns a minimal WAV header byte slice as `*SynthesisResult` with fake `WordTimings`
     - `SynthesizeWithOverrides()` → delegates to `Synthesize()`
  5. **`fakeAssembler` struct** implementing `output.Assembler`:
     - `Assemble()` → returns a canned `*AssembleResult` (creates a dummy output file in workspace)
     - `Validate()` → returns nil
  6. **`StartTestServer(t *testing.T) (baseURL string, st *store.Store)`** helper:
     - Creates `store.New(":memory:")`
     - Creates config:
       ```go
       cfg := &config.Config{
           WorkspacePath: t.TempDir(),
           API: config.APIConfig{Host: "127.0.0.1", Port: 0},
           // Auth disabled — no Bearer token needed
           // Webhooks empty — NewWebhookNotifier safely returns nil
       }
       ```
     - Wires fake plugins into real services:
       ```go
       fl := &fakeLLM{}
       fig := &fakeImageGen{}
       ft := &fakeTTS{}
       fa := &fakeAssembler{}

       projectSvc := service.NewProjectService(st)
       scenarioSvc := service.NewScenarioService(st, fl, projectSvc)
       imageGenSvc := service.NewImageGenService(fig, st, slog.Default())
       ttsSvc := service.NewTTSService(ft, glossary.New(), st, slog.Default())  // glossary.New() NOT nil
       characterSvc := service.NewCharacterService(st)
       characterSvc.SetLLM(fl)        // REQUIRED — GenerateCandidates checks for nil
       characterSvc.SetImageGen(fig)   // REQUIRED — GenerateCandidates checks for nil
       assemblerSvc := service.NewAssemblerService(fa, projectSvc)
       ```
     - Creates server with ALL required options:
       ```go
       srv := api.NewServer(st, cfg,
           api.WithScenarioService(scenarioSvc),
           api.WithImageGenService(imageGenSvc),
           api.WithTTSService(ttsSvc),
           api.WithCharacterService(characterSvc),
           api.WithAssemblerService(assemblerSvc),
           api.WithPluginStatus(map[string]bool{
               "llm": true, "imagegen": true, "tts": true, "output": true,
           }),
       )
       ```
     - `net.Listen("tcp", "127.0.0.1:0")` for random port
     - Starts `http.Serve(listener, srv.Router())` in goroutine — bypasses `Start()`, `Shutdown()` not available
     - Registers `t.Cleanup()` with `listener.Close()`
     - Returns `baseURL` and `st` (store returned for direct seeding in `seedProjectAtStage`)
  7. **`newPage(t *testing.T) playwright.Page`** helper:
     - Creates new `browser.NewContext()` for cookie/storage isolation
     - `context.NewPage()` for the test
     - `t.Cleanup()` closes context (which closes page)
     - Uses package-level `browser` from `TestMain`
  8. **`acceptDialogs(page playwright.Page)`** helper:
     - Registers `page.OnDialog()` to auto-accept all native `confirm()` dialogs
     - Call before any test that clicks buttons with `hx-confirm` or JS `confirm()`
  9. **`seedProject(t *testing.T, baseURL string, scpID string) string`** helper:
     - POST to `/api/v1/projects` with `{"scp_id": scpID}`
     - Returns project ID
  10. **`seedProjectAtStage(t *testing.T, baseURL string, st *store.Store, scpID string, stage string) string`** helper:
      - Calls `seedProject()` to create project
      - **Inserts test data directly into store** (not via pipeline runner):
        - Insert 3 scenes with narration, visual desc, image prompt via `st.DB().Exec()` SQL
        - For "images" stage and beyond: create temp 1x1 PNG files in workspace, update scene `image_path` in store
        - For "tts" stage and beyond: create temp WAV files in workspace, update scene `audio_path` and `subtitle_path` in store
        - For "character" stage and beyond: insert character candidate row in store, then `POST /api/v1/projects/{id}/characters/select {"candidate_num": 1}`
      - **Sets stage** via `PATCH /api/v1/projects/{id}/stage {"stage": targetStage}`
      - Returns project ID at target stage with all prerequisite data populated
- **Notes:** Fake plugins return deterministic data. No randomness, no delays. Store is returned from `StartTestServer` specifically for `seedProjectAtStage` to insert test data directly.

#### Task 3: Create pipeline flow tests (`tests/e2e/pipeline_test.go`) — PRIORITY 1

- **File:** `tests/e2e/pipeline_test.go` (new)
- **Action:** Create the highest-priority test suite:
  1. `//go:build e2e` build tag
  2. **`TestPipeline_CreateProject`**:
     - Navigate to `/dashboard/`
     - Click "New Project" button (opens `<dialog>` modal)
     - Type SCP ID in search input (`#scp-filter-input`), wait for SCP list to populate via JS `onSCPSearch()`
     - Click an SCP item to select it
     - Click "Create" button (triggers JS `createProject()` → `fetch('/api/v1/projects', ...)`)
     - Wait for JS redirect to `/dashboard/projects/{id}`
     - Verify stage = "pending" in progress bar
  3. **`TestPipeline_GenerateScenario`**:
     - Seed a project at "pending", navigate to detail page
     - Click "Generate Scenario" button (JS `runScenario()` → `fetch POST /api/v1/projects/{id}/run`)
     - Wait for HTMX polling (every 3s) to show job completion
     - Verify scenes appear in the scene list
     - Verify stage advances to "scenario"
  4. **`TestPipeline_GenerateCharacters`**:
     - Seed project at "scenario" stage (scenes pre-populated in store)
     - Navigate to detail page
     - Click "Generate Characters" button (JS `generateCharacters()`)
     - Wait for character candidates to appear via polling (every 2s)
     - Click a candidate to select (JS `selectCharacter()`)
     - Verify stage = "character" and selected character shown
  5. **`TestPipeline_GenerateImages`**:
     - Seed project at "character" stage
     - Click "Generate All" or "Generate Images" button (JS `generateImages()`)
     - Wait for completion (HTMX polling)
     - Verify scene cards show image thumbnails
     - Verify stage = "images"
  6. **`TestPipeline_GenerateTTS`**:
     - Seed project at "images" stage
     - Click "Generate TTS" button (JS `generateTTS()`)
     - Wait for completion
     - Verify scene cards show audio player
     - Verify stage = "tts"
  7. **`TestPipeline_Assemble`**:
     - Seed project at "tts" stage (all scene assets populated)
     - Click "Assemble" button (JS `runAssemble()`)
     - Wait for completion
     - Verify stage = "complete"
     - Verify output files section visible
  8. **`TestPipeline_StageBackwardTransition`**:
     - Seed project at "images" stage
     - Click "scenario" stage in progress bar (HTMX `hx-patch` + `hx-confirm`)
     - Register `page.OnDialog()` to accept the confirm dialog
     - Verify stage changes to "scenario" and UI updates
     - Note: progress bar stages are clickable via `hx-patch` when job is NOT running and stage != "pending"
- **Notes:** Each test uses `StartTestServer` + `newPage`. Fake plugins return immediately so no real wait needed — just wait for DOM update via `Locator().WaitFor()`. ALL tests that trigger `confirm()` must call `acceptDialogs(page)` first.

#### Task 4: Create dashboard tests (`tests/e2e/dashboard_test.go`)

- **File:** `tests/e2e/dashboard_test.go` (new)
- **Action:**
  1. **`TestDashboard_ListProjects`**: Seed 3 projects with different SCP IDs, verify all appear on `/dashboard/` as SCP accordion groups
  2. **`TestDashboard_FilterByStage`**: Seed projects at different stages, use stage dropdown filter (HTMX `hx-get`), verify filtered results in SCP groups
  3. **`TestDashboard_SCPSearch`**: Seed projects with different SCP IDs, type in search input (HTMX `hx-trigger="keyup changed delay:300ms"`), verify partial update shows filtered groups
  4. **`TestDashboard_NavigateToDetail`**: Expand SCP accordion group, click project link (`<a>` inside accordion), verify navigation to `/dashboard/projects/{id}`
  5. **`TestDashboard_DeleteProject`**: Navigate to project detail, register `acceptDialogs(page)`, click delete button (`hx-delete` + `hx-confirm`), verify JS redirect to `/dashboard/` via `hx-on::after-request`
- **Notes:** SCP search has 300ms debounce — use `page.Locator().Fill()` then wait for response or DOM change. Dashboard uses SCP-grouped accordion layout, NOT flat cards.

#### Task 5: Create scene tests (`tests/e2e/scenes_test.go`)

- **File:** `tests/e2e/scenes_test.go` (new)
- **Action:**
  1. **`TestScene_EditNarration`**: Seed project at "scenario" stage with scenes, edit narration text inline, verify PATCH request updates and UI reflects change
  2. **`TestScene_RegenerateImage`**: Seed at "images" stage, register `acceptDialogs(page)`, click regenerate image button on scene card (`hx-post` + `hx-confirm`), verify image updates after job completion
  3. **`TestScene_RegenerateTTS`**: Seed at "tts" stage, register `acceptDialogs(page)`, click regenerate TTS button (`hx-post` + `hx-confirm`), verify audio updates
  4. **`TestScene_InsertScene`**: Seed at "scenario" stage, click insert scene button between scene cards (JS `insertScene()` → opens insert modal dialog), fill narration in modal, click "Insert", verify new scene card appears
  5. **`TestScene_DeleteScene`**: Register `acceptDialogs(page)`, click delete on a scene card (`hx-delete` + `hx-confirm` + JS `location.reload()` on success), verify scene removed after page reload

#### Task 6: Create character tests (`tests/e2e/characters_test.go`)

- **File:** `tests/e2e/characters_test.go` (new)
- **Action:**
  1. **`TestCharacter_GenerateAndSelect`**: Seed project at "scenario" stage, click "Generate Characters" (JS `generateCharacters()`), wait for candidates to appear via 2s polling (character section transitions from "generating" skeleton to "ready" state), click a candidate (JS `selectCharacter()`), verify selected character shown with "Change Selection" and "Generate New" buttons
  2. **`TestCharacter_Deselect`**: Seed at "character" stage with selected character, click "Change Selection" → deselect button (JS `deselectCharacter()`), verify character section resets to empty/ready state
  3. **`TestCharacter_CandidatePolling`**: Generate characters, verify `hx-trigger="every 2s"` polling transitions from skeleton loading cards to actual candidate images when job completes

#### Task 7: Add `test-e2e` Makefile target

- **File:** `Makefile`
- **Action:** Add target:
  ```makefile
  test-e2e:
  	go run github.com/playwright-community/playwright-go/cmd/playwright install --with-deps chromium
  	go test -tags=e2e -timeout 300s ./tests/e2e/...
  ```
- **Notes:** First run installs Chromium (~130MB download, cached after). Timeout 300s for browser startup overhead.

#### Task 8: Add E2E job to GitHub Actions

- **File:** `.github/workflows/deploy.yml`
- **Action:** Add `e2e` job parallel with `build-and-push` (both depend on `test`):
  ```yaml
  e2e:
    runs-on: ubuntu-latest
    needs: test
    steps:
      - uses: actions/checkout@v5
      - uses: actions/setup-go@v6
        with:
          go-version: '1.25'
      - name: Install Chromium
        run: go run github.com/playwright-community/playwright-go/cmd/playwright install --with-deps chromium
      - name: Run E2E tests
        run: go test -tags=e2e -timeout 300s -v ./tests/e2e/...
  ```
- **Notes:** Uses `actions/checkout@v5` (matching existing workflow). E2E uses in-process server (not deployed instance), so `needs: test` is correct — runs parallel with build/deploy. Update `notify` job `needs` array to include `e2e`.

### Acceptance Criteria

**AC1: E2E framework runs locally**
- Given playwright-go is installed and Chromium is available
- When `go test -tags=e2e -timeout 300s ./tests/e2e/...` is executed
- Then all tests pass with Chromium browser driving the in-process server

**AC2: Pipeline stage flow end-to-end**
- Given a fresh in-memory database and fake plugins with all plugin status flags enabled
- When a project is created and pipeline stages are executed sequentially via UI (scenario → character → images → tts → complete)
- Then each stage transition is reflected in the progress bar UI and scene cards update with generated content

**AC3: HTMX polling updates UI**
- Given a project with an active job (e.g., generating scenario)
- When the job completes and HTMX polling fires (every 3s for project detail, every 2s for characters)
- Then the page content updates without manual refresh, showing new stage/data

**AC4: Dashboard filtering works in browser**
- Given multiple projects at different stages and SCP IDs organized in accordion groups
- When stage filter dropdown is changed or SCP search is typed
- Then the project list updates via HTMX partial swap showing only matching SCP groups

**AC5: Scene editing persists**
- Given a project with generated scenes
- When narration is edited, image is regenerated, or TTS is regenerated via UI buttons
- Then the changes persist in the database and are reflected when the page is reloaded

**AC6: Character selection flow works**
- Given a project at scenario stage
- When characters are generated (JS fetch), a candidate is selected (JS onclick), then deselected
- Then each action updates the character section UI correctly through its 5 states

**AC7: Stage backward transition via progress bar**
- Given a project at "images" stage with no running job
- When a user clicks an earlier stage in the progress bar (hx-patch) and accepts the native confirm dialog
- Then the stage changes and the UI updates accordingly

**AC8: Build tags isolate E2E from unit tests**
- Given the standard `go test ./...` command
- When run without `-tags=e2e`
- Then no E2E tests are compiled or executed

**AC9: CI job runs after unit tests**
- Given a push to master triggers the deploy workflow
- When the unit test job completes
- Then the e2e job runs in parallel with build-and-push, installs Chromium, runs all E2E tests, and reports status in Discord notification

**AC10: Test isolation**
- Given multiple E2E test functions
- When run with `t.Parallel()`
- Then each test uses its own server instance on a unique port with a fresh database, no cross-contamination

**AC11: Chromium auto-install on first run**
- Given a fresh environment without Chromium installed
- When `make test-e2e` is executed
- Then Chromium is automatically installed via `playwright install --with-deps chromium` and tests run successfully

**AC12: Native dialog handling**
- Given a test clicks a button with `hx-confirm` (delete project, regenerate image, stage change)
- When the native `confirm()` dialog appears
- Then the test's `page.OnDialog()` handler accepts it and the action proceeds

## Additional Context

### Dependencies

- **New:** `github.com/playwright-community/playwright-go` — Go bindings for Playwright
- **Existing (no changes):** `testify`, `chi`, `modernc.org/sqlite`, `glossary`, all internal packages
- **Runtime:** Chromium browser (installed via `playwright install`, cached in `~/.cache/ms-playwright/`)

### Testing Strategy

- **E2E tests (`//go:build e2e`)**: Browser-driven via playwright-go. Fake plugins at interface level. Validates full stack: browser → JS/HTMX → HTTP handler → service → store → response → DOM update.
- **Unit tests (existing)**: Continue covering handler logic, service logic, store queries via `httptest` + mock.
- **Integration tests (`//go:build integration`)**: Continue covering real external API calls.
- **Local execution**: `make test-e2e` or `go test -tags=e2e -timeout 300s ./tests/e2e/...`
- **CI execution**: Post-unit-test GitHub Actions job (parallel with build-and-push)
- **Debugging**: Run with `PWDEBUG=1` to open Playwright Inspector for step-through debugging locally

### Notes

- **Headless by default**: `pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{Headless: playwright.Bool(true)})`. Set `Headless: false` locally for visual debugging.
- **No flaky timer waits**: Use `page.Locator().WaitFor()` with Playwright's auto-retry instead of `time.Sleep`. HTMX polling is the only time-based mechanism, and we wait for DOM changes not HTTP responses.
- **Fake plugin latency**: Fakes return immediately (no `time.Sleep`). The job manager still wraps execution in a goroutine, so HTMX polling will pick up results on the next cycle (2-3s max).
- **Scene asset paths for assemble**: `seedProjectAtStage("tts")` and `seedProjectAtStage("complete")` must create actual temp files (1x1 PNG, minimal WAV) and set their paths in store — the assembler service validates scene assets exist before assembly.
- **Future expansion**: Review page tests, cross-browser, visual regression testing — all deferred to separate sprints.
- **Risk: Service constructor changes**: If service constructors gain new required parameters, `helpers_test.go` will need updating. Mitigated by keeping fake wiring in one place.
- **No testdata directory needed**: Fake plugins return canned responses. `seedProject` only needs API POST. Store-level seeding uses SQL inserts for scenes/assets.
