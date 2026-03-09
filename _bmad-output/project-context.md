---
project_name: 'youtube.pipeline'
user_name: 'Jay'
date: '2026-03-09'
sections_completed: ['technology_stack', 'language_rules', 'framework_rules', 'testing_rules', 'code_quality', 'workflow_rules', 'critical_rules']
status: 'complete'
rule_count: 62
optimized_for_llm: true
---

# Project Context for AI Agents

_This file contains critical rules and patterns that AI agents must follow when implementing code in this project. Focus on unobvious details that agents might otherwise miss._

---

## Technology Stack & Versions

- **Language:** Go 1.25.7 — module `github.com/sushistack/yt.pipe`
- **Database:** SQLite via `modernc.org/sqlite` v1.46.1 (pure Go, NO CGo, NO `mattn/go-sqlite3`)
  - PRAGMA: WAL mode + foreign_keys=ON (hardcoded in store.go)
  - Migrations: embedded via `//go:embed migrations/*.sql`, sequential versioning (001-007)
  - Tests: always use `:memory:` DB, never file-based
- **CLI:** cobra v1.10.2
- **Config:** viper v1.21.0
  - 5-level priority: CLI flags > env `YTP_*` > project YAML > global YAML > defaults
  - Config structs use `mapstructure:"..."` tags (NOT `json`)
  - Env prefix is `YTP_` with dots→underscores (`llm.api_key` → `YTP_LLM_API_KEY`)
- **HTTP Router:** go-chi/chi/v5 v5.2.5
- **Testing:** testify v1.11.1 (assert + require), build tag `-tags=integration` for integration tests
- **Mocking:** mockery v2 via `.mockery.yaml` — 4 interfaces registered (LLM, TTS, ImageGen, Assembler)
- **UUID:** google/uuid v1.6.0
- **Logging:** stdlib `log/slog` (JSON + text modes)
- **Templates:** `text/template` (NOT `html/template`) for prompt generation
- **Packaging:** Docker + docker-compose

## Critical Implementation Rules

### Go Language Rules

- **Layered dependency direction:** domain → store → service → api/cli (reverse import forbidden)
  - `domain/`: stdlib + uuid only, NO other internal package imports
  - `store/`: domain/ + database/sql + stdlib only
  - `service/`: domain/ + store/ + plugin/ + config/ allowed
  - `api/`, `cli/`: service/ + domain/ + external frameworks allowed
- **Import grouping (4-group):**
  1. stdlib
  2. third-party external
  3. internal packages (`github.com/sushistack/yt.pipe/internal/...`)
  4. (blank line separators between groups)
- **Error handling:**
  - 4 domain error types: `NotFoundError`, `ValidationError`, `TransitionError`, `PluginError` (in `domain/errors.go`)
  - Store layer: `sql.ErrNoRows` → `&domain.NotFoundError{}`
  - Service layer: wrap with `fmt.Errorf("service: <context>: %w", err)`
  - `PluginError` implements `Unwrap()` for `errors.Is/As` support
  - Tests use `assert.ErrorAs(t, err, &typedErr)` for type assertion
- **Interface compliance:** plugin implementations must include compile-time check:
  `var _ TTS = (*DashScopeProvider)(nil)`
- **Context propagation:** all plugin interface methods accept `context.Context` as first parameter
- **Options pattern:** use dedicated structs (`CompletionOptions`, `GenerateOptions`, `TTSOptions`) — not variadic functional options
- **UUID generation:** always `uuid.New().String()` for entity IDs (not auto-increment)
- **Time handling:** `time.Now().UTC()` for all timestamps, stored as RFC3339 strings in SQLite

### Framework & Architecture Rules

- **Plugin system (registry pattern):**
  - 4 plugin types: `llm`, `tts`, `imagegen`, `output`
  - Interfaces defined in `internal/plugin/{type}/interface.go`
  - Implementations registered via `Registry.Register(pluginType, provider, factory)`
  - Factory signature: `func(cfg map[string]interface{}) (interface{}, error)`
  - New plugin: define interface → implement → register factory → add to `.mockery.yaml`
- **State machine (`domain/project.go`):**
  - Transitions: `pending → scenario_review → approved → image_review → tts_review → assembling → complete`
  - `allowedTransitions` map governs valid state changes
  - Invalid transitions return `&domain.TransitionError{}`
  - All state changes go through `Project.Transition()` — never set `Status` directly
- **CLI commands (cobra):**
  - One command per file: `{feature}_cmd.go`
  - Global flags in `root.go`, command-specific flags in each `*_cmd.go`
  - Commands call service layer, never access store directly
- **API handlers (chi):**
  - Standard response envelope: `api.Response{Success, Data, Error, Timestamp, RequestID}`
  - Error responses via `WriteError(w, r, status, code, message)`
  - Middleware chain: Recovery → RequestID → Logging → Auth
  - Routes grouped under `/api/v1/`
- **Service layer pattern:**
  - Constructor: `NewXxxService(store *store.Store)` — receives store dependency
  - Methods accept `context.Context` as first parameter
  - Transactional operations use `db.Begin()` / `tx.Commit()` / `tx.Rollback()`
- **Scene as fundamental unit:**
  - Scene is a self-contained asset bundle (image, audio, subtitle, metadata)
  - All pipeline operations are scene-granular
  - Incremental builds skip unchanged scenes via content hash comparison

### Testing Rules

- **Test file location:** `*_test.go` in same package (not separate `_test` package)
- **Setup helper pattern:**
  - Every test file has `setupTestXxx(t *testing.T)` helper
  - Must call `t.Helper()` at the start
  - Must register cleanup with `t.Cleanup(func() { ... })`
  - Store tests: `store.New(":memory:")` for in-memory SQLite
  - Service tests: `NewXxxService(setupTestStore(t))`
- **Assertion conventions:**
  - `require.NoError(t, err)` for setup/preconditions (stops test on failure)
  - `assert.Equal(t, expected, actual)` for value comparisons
  - `assert.ErrorAs(t, err, &typedErr)` for domain error type checks
  - Never use `require` for the actual assertion under test — use `assert`
- **Test naming:** `TestMethodName_Scenario` (e.g., `TestCreateProject_EmptySCPID`)
- **Table-driven tests:** use `for _, tc := range cases { t.Run(tc.name, ...) }` for parameterized tests
- **API handler tests:** `httptest.NewRequest` + `httptest.NewRecorder` + `srv.Router().ServeHTTP(w, req)`
- **Integration tests:** gated by `-tags=integration` build tag, timeout `600s`
- **Mock generation:** mockery v2 generates mocks to `internal/mocks/` — do NOT hand-write mocks
- **Known issue:** `internal/service/assembler_test.go` references `internal/mocks` package that doesn't exist yet

### Code Quality & Style Rules

- **File naming:**
  - Domain models: `{entity}.go` (snake_case, e.g., `mood_preset.go`, `scene_approval.go`)
  - CLI commands: `{feature}_cmd.go`
  - API handlers: `{resource}.go` (e.g., `projects.go`, `pipeline.go`)
  - Plugin interfaces: always `interface.go`
  - Migrations: `{NNN}_{feature}.sql` (zero-padded 3-digit sequence)
  - Package docs: `doc.go` per package
- **One aggregate per file:** each domain entity gets its own file in `domain/`, `store/`, `service/`
- **Linting:** `go vet ./...` only (no golangci-lint configured)
- **No comments on obvious code:** only add comments where logic isn't self-evident
- **Struct tags:** `mapstructure` for config, no `json` tags on config structs
- **JSON in SQLite:** flexible arrays stored as JSON strings (`aliases`, `mood_tags`, `params_json`), parsed in Go
- **SQL conventions:**
  - `CREATE TABLE IF NOT EXISTS` for idempotency
  - `CHECK(...)` constraints for enum columns
  - Composite primary keys for junction/assignment tables
  - `datetime('now')` for SQLite default timestamps
  - Indexes on foreign keys and frequently queried columns

### Development Workflow Rules

- **Build commands:**
  - `make build` — `bin/yt-pipe` binary
  - `make test` — `go test ./...` (unit tests only)
  - `make test-integration` — `-tags=integration -timeout 600s`
  - `make lint` — `go vet ./...`
  - `make run` — `go run ./cmd/yt-pipe serve`
- **Entry point:** `cmd/yt-pipe/main.go` — minimal, delegates to CLI root
- **Dual interface:** CLI and REST API share the same service layer
  - CLI: `internal/cli/` → `internal/service/`
  - API: `internal/api/` → `internal/service/`
  - Never duplicate business logic in both adapters
- **Configuration files:**
  - `config.example.yaml` — reference template with all options documented
  - API keys: NEVER in config files, always via `YTP_*` env vars
- **Pipeline execution flow:**
  - `init` → creates project + workspace directory
  - `run` → executes pipeline stages (scenario → image → tts → assembly)
  - Checkpoint persistence allows resume after failure
  - Dry-run mode validates without making API calls

### Critical Don't-Miss Rules

- **NEVER import upward in layers:** `domain/` must never import `store/`, `service/`, or `api/`
- **NEVER set `Project.Status` directly:** always use `Project.Transition()` which validates against `allowedTransitions`
- **NEVER use `mattn/go-sqlite3`:** this project uses pure Go `modernc.org/sqlite` — CGo is not allowed
- **NEVER use `html/template`:** prompt templates use `text/template` (no HTML escaping needed)
- **NEVER hardcode entity IDs:** always generate with `uuid.New().String()`
- **NEVER create file-based SQLite in tests:** always `:memory:`
- **NEVER hand-write mocks:** use mockery v2 generation, update `.mockery.yaml` for new interfaces
- **NEVER put API keys in config files:** security rule — use `YTP_*` environment variables only
- **NEVER skip `context.Context`:** all service and plugin methods must accept it as first parameter
- **Anti-patterns:**
  - Don't add new domain error types — use the existing 4 (`NotFound`, `Validation`, `Transition`, `Plugin`)
  - Don't use `interface{}` in plugin interfaces — use typed option structs
  - Don't bypass the service layer from CLI/API — always go through service
  - Don't add migrations out of sequence — always increment from the latest `{NNN}` number
- **Security:**
  - API authentication via `X-API-Key` header (middleware enforced)
  - Config struct `APIConfig.APIKey` exists but should be set via env only
  - No user-supplied input directly in SQL — always parameterized queries
- **SCP domain context:**
  - SCP IDs follow pattern `SCP-{number}` (e.g., `SCP-173`)
  - Glossary system handles pronunciation corrections for TTS
  - Scene numbering is 1-based, not 0-based

---

## Usage Guidelines

**For AI Agents:**

- Read this file before implementing any code
- Follow ALL rules exactly as documented
- When in doubt, prefer the more restrictive option
- Update this file if new patterns emerge

**For Humans:**

- Keep this file lean and focused on agent needs
- Update when technology stack changes
- Review quarterly for outdated rules
- Remove rules that become obvious over time

Last Updated: 2026-03-09
