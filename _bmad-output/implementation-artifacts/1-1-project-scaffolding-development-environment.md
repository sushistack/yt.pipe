# Story 1.1: Project Scaffolding & Development Environment

Status: done

## Story

As a developer,
I want a fully initialized Go project with the correct directory structure, domain models, SQLite store, and build tooling,
So that all subsequent stories have a solid foundation to build upon.

## Acceptance Criteria

1. **Directory Structure** (AC:1)
   - Given the repository is cloned and Go is installed
   - When `go mod init` is run and the project structure is created
   - Then the directory structure matches the Architecture document: `cmd/yt-pipe/`, `internal/cli/`, `internal/api/`, `internal/service/`, `internal/domain/`, `internal/plugin/`, `internal/config/`, `internal/store/`, `internal/workspace/`, `internal/glossary/`, `internal/retry/`, `internal/mocks/`
   - And `go build ./...` compiles without errors

2. **Domain Models** (AC:2)
   - Given the domain package is created
   - When domain models are defined
   - Then Project (with state enum: `pending`, `scenario_review`, `approved`, `generating_assets`, `assembling`, `complete`), Scene, SceneManifest, Job models exist in `internal/domain/`
   - And custom error types (NotFoundError, ValidationError, PluginError, TransitionError) are defined in `domain/errors.go`
   - And state transition map with allowed transitions is defined in `domain/project.go`

3. **SQLite Store** (AC:3)
   - Given the store package is created
   - When SQLite is initialized with modernc.org/sqlite
   - Then `store.go` creates the database, runs embedded SQL migrations via `go:embed`, and tracks schema version
   - And initial migration `001_initial.sql` creates `projects`, `jobs`, `scene_manifests`, and `execution_logs` tables
   - And all table/column names follow `snake_case` convention per Architecture

4. **Build Tooling** (AC:4)
   - Given the Makefile is created
   - When make targets are executed
   - Then `make build` produces `bin/yt-pipe`, `make test` runs all tests, `make generate` runs mockery, `make lint` runs go vet, `make docker` builds Docker image

5. **Cobra Root Command** (AC:5)
   - Given the Cobra root command is created
   - When `go run ./cmd/yt-pipe --help` is executed
   - Then the CLI displays help text with `yt-pipe` as the binary name
   - And the root help text is shown (subcommand list may be empty at this stage — subsequent stories add commands)

## Tasks / Subtasks

- [x] Task 1: Initialize Go module and directory structure (AC: #1)
  - [x] 1.1 Run `go mod init github.com/jay/youtube-pipeline`
  - [x] 1.2 Create all directories per architecture (see File Structure below)
  - [x] 1.3 Add `.gitkeep` files to empty directories
  - [x] 1.4 Create `.gitignore` (bin/, *.db, internal/mocks/*, vendor/)
  - [x] 1.5 Add Go dependencies: cobra, viper, chi, modernc.org/sqlite, testify
  - [x] 1.6 Install mockery: `go install github.com/vektra/mockery/v2@latest`
  - [x] 1.7 Run `go mod tidy` to clean up dependencies and ensure go.sum consistency
  - [x] 1.8 Verify `go build ./...` compiles

- [x] Task 2: Create domain models (AC: #2)
  - [x] 2.1 Create `internal/domain/project.go` — Project model with state enum + transition map
  - [x] 2.2 Create `internal/domain/scene.go` — Scene model (shared domain model)
  - [x] 2.3 Create `internal/domain/manifest.go` — SceneManifest model (incremental build tracking)
  - [x] 2.4 Create `internal/domain/job.go` — Job model
  - [x] 2.5 Create `internal/domain/scenario.go` — ScenarioOutput structure (inter-module contract)
  - [x] 2.6 Create `internal/domain/errors.go` — 4 custom error types
  - [x] 2.7 Write unit tests for state transition map in `internal/domain/project_test.go`
  - [x] 2.8 Write unit tests for custom error types in `internal/domain/errors_test.go` — verify `Error()`, `Unwrap()`, `errors.Is()`, `errors.As()` compatibility

- [x] Task 3: Create SQLite store with migrations (AC: #3)
  - [x] 3.1 Create `internal/store/store.go` — DB init, migration runner, schema version tracker
  - [x] 3.2 Add `//go:embed migrations/*.sql` directive in `internal/store/store.go` (package-level embed, no separate embed.go needed)
  - [x] 3.3 Create `internal/store/migrations/001_initial.sql` — Initial schema (4 tables)
  - [x] 3.4 Create `internal/store/project.go` — Project CRUD operations
  - [x] 3.5 Create `internal/store/job.go` — Job CRUD operations
  - [x] 3.6 Create `internal/store/manifest.go` — SceneManifest CRUD
  - [x] 3.7 Create `internal/store/execution_log.go` — Execution log + API cost tracking
  - [x] 3.8 Write integration tests using `:memory:` SQLite — test each CRUD operation (Create, GetByID, List, Update) for projects/jobs/manifests, verify migration runner applies 001_initial.sql correctly, confirm schema_version is set to 1

- [x] Task 4: Create Makefile and build tooling (AC: #4)
  - [x] 4.1 Create Makefile with targets: build, test, generate, lint, docker, run
  - [x] 4.2 Create minimal Dockerfile (multi-stage: golang → scratch)
  - [x] 4.3 Create docker-compose.yml with 4 volume mounts
  - [x] 4.4 Create config.example.yaml with documented fields

- [x] Task 5: Create Cobra root command (AC: #5)
  - [x] 5.1 Create `cmd/yt-pipe/main.go` — Entry point
  - [x] 5.2 Create `internal/cli/root.go` — Root command with global flags
  - [x] 5.3 Verify `go run ./cmd/yt-pipe --help` works

- [x] Task 6: Create placeholder files for future packages (AC: #1)
  - [x] 6.1 Create stub files in `internal/api/`, `internal/service/`, `internal/plugin/`, `internal/config/`, `internal/workspace/`, `internal/glossary/`, `internal/retry/`
  - [x] 6.2 Each stub should have correct package declaration, a brief doc comment, and import direction constraint comment (e.g., `// Package domain contains pure data structures. MUST NOT import other internal packages.`)

- [x] Task 7: Final verification
  - [x] 7.1 `go build ./...` — zero errors
  - [x] 7.2 `go test ./...` — all tests pass
  - [x] 7.3 `go vet ./...` — zero warnings
  - [x] 7.4 `make build` — produces `bin/yt-pipe`

## Dev Notes

### Critical Architecture Constraints

**Language & Runtime:**
- Go (latest stable), modules enabled
- Single binary compilation, CGO-free (required for scratch Docker image)
- Module path: `github.com/jay/youtube-pipeline`

**Required Dependencies (exact):**
```bash
go get github.com/spf13/cobra@latest
go get github.com/spf13/viper@latest
go get github.com/go-chi/chi/v5@latest
go get modernc.org/sqlite@latest
go get github.com/stretchr/testify@latest
go install github.com/vektra/mockery/v2@latest
```

**Logging:** Use `log/slog` (Go 1.21+ stdlib) — NO external logging libraries. JSON format.

### Naming Conventions (MUST FOLLOW)

**Go Code:**
- Packages: lowercase single word — `store`, `domain`, `plugin`, `glossary`
- Types/Interfaces: `PascalCase` — `SceneManifest`, `LLMPlugin`, `TransitionError`
- Functions/Methods: `PascalCase` (exported) / `camelCase` (unexported)
- Files: `snake_case.go` — `scene_manifest.go`, `pipeline_service.go`
- Constants: `PascalCase` (exported) — `StatusPending`, `StatusApproved`

**Database (SQLite):**
- Tables: `snake_case` plural — `projects`, `jobs`, `scene_manifests`, `execution_logs`
- Columns: `snake_case` — `project_id`, `created_at`, `scene_num`, `estimated_cost_usd`
- Indexes: `idx_{table}_{column}` — `idx_jobs_project_id`
- FK: `{referenced_table_singular}_id` — `project_id`, `job_id`

**Test Naming:** `Test{Function}_{Scenario}` — `TestCreateProject_InvalidSCPID`, `TestTransition_InvalidState`

### Domain Model Specifications

**Project State Enum & Transitions:**
```go
// States
const (
    StatusPending         = "pending"
    StatusScenarioReview  = "scenario_review"
    StatusApproved        = "approved"
    StatusGeneratingAssets = "generating_assets"
    StatusAssembling      = "assembling"
    StatusComplete        = "complete"
)

// Allowed transitions map
var AllowedTransitions = map[string][]string{
    StatusPending:         {StatusScenarioReview},
    StatusScenarioReview:  {StatusApproved, StatusPending},  // can reject back
    StatusApproved:        {StatusGeneratingAssets},
    StatusGeneratingAssets:{StatusAssembling},
    StatusAssembling:      {StatusComplete},
    StatusComplete:        {},  // terminal
}
```

**Custom Error Types (domain/errors.go):**
```go
type NotFoundError struct{ Resource, ID string }      // → API 404
type ValidationError struct{ Field, Message string }   // → API 400
type PluginError struct{ Plugin, Operation string; Err error }  // → API 500/502
type TransitionError struct{ Current, Requested string; Allowed []string }  // → API 409
```
Each must implement `error` interface and `Unwrap()` where wrapping an inner error.

**Scene Model (domain/scene.go) — shared domain model, pipe-filter pattern:**
```go
type Scene struct {
    SceneNum       int
    Narration      string       // from scenario
    VisualDesc     string       // from scenario
    FactTags       []string     // fact references
    Mood           string       // from scenario
    ImagePrompt    string       // generated later
    ImagePath      string       // generated later
    AudioPath      string       // generated later
    AudioDuration  float64      // seconds, from TTS
    WordTimings    []WordTiming // from TTS
    SubtitlePath   string       // generated later
}

// WordTiming represents a single word's timing within TTS audio output
type WordTiming struct {
    Word      string  // the spoken word
    StartSec  float64 // start time in seconds
    EndSec    float64 // end time in seconds
}
```

**SceneManifest (domain/manifest.go) — incremental build tracking:**
```go
type SceneManifest struct {
    ProjectID    string
    SceneNum     int
    ContentHash  string    // hash of input content for change detection
    ImageHash    string    // hash after image generation
    AudioHash    string    // hash after TTS
    SubtitleHash string    // hash after subtitle
    Status       string    // pending, image_done, audio_done, complete
    UpdatedAt    time.Time
}
```

**Job Model (domain/job.go):**
```go
type Job struct {
    ID          string
    ProjectID   string
    Type        string    // scenario, image, tts, subtitle, assemble, full_pipeline
    Status      string    // pending, running, completed, failed
    Progress    int       // 0-100
    Result      string    // JSON result on completion
    Error       string    // error message on failure
    CreatedAt   time.Time
    UpdatedAt   time.Time
}
```

**ScenarioOutput (domain/scenario.go) — inter-module contract:**
```go
type ScenarioOutput struct {
    SCPID     string
    Title     string
    Scenes    []SceneScript
    Metadata  map[string]string
}

type SceneScript struct {
    SceneNum         int
    Narration        string
    VisualDescription string
    FactTags         []FactTag
    Mood             string
}

type FactTag struct {
    Key     string  // reference to facts.json key
    Content string  // tagged content
}
```

### SQLite Schema — 001_initial.sql

```sql
CREATE TABLE IF NOT EXISTS schema_version (
    version INTEGER NOT NULL,
    applied_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS projects (
    id TEXT PRIMARY KEY,
    scp_id TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    scene_count INTEGER NOT NULL DEFAULT 0,
    workspace_path TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX idx_projects_scp_id ON projects(scp_id);
CREATE INDEX idx_projects_status ON projects(status);

CREATE TABLE IF NOT EXISTS jobs (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id),
    type TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    progress INTEGER NOT NULL DEFAULT 0,
    result TEXT,
    error TEXT,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX idx_jobs_project_id ON jobs(project_id);
CREATE INDEX idx_jobs_status ON jobs(status);

CREATE TABLE IF NOT EXISTS scene_manifests (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    project_id TEXT NOT NULL REFERENCES projects(id),
    scene_num INTEGER NOT NULL,
    content_hash TEXT NOT NULL DEFAULT '',
    image_hash TEXT NOT NULL DEFAULT '',
    audio_hash TEXT NOT NULL DEFAULT '',
    subtitle_hash TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'pending',
    updated_at TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE(project_id, scene_num)
);
CREATE INDEX idx_scene_manifests_project_id ON scene_manifests(project_id);

CREATE TABLE IF NOT EXISTS execution_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    project_id TEXT NOT NULL REFERENCES projects(id),
    job_id TEXT REFERENCES jobs(id),
    stage TEXT NOT NULL,
    action TEXT NOT NULL,
    status TEXT NOT NULL,
    duration_ms INTEGER,
    estimated_cost_usd REAL,
    details TEXT,
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX idx_execution_logs_project_id ON execution_logs(project_id);
CREATE INDEX idx_execution_logs_job_id ON execution_logs(job_id);

INSERT INTO schema_version (version) VALUES (1);
```

### Store Implementation Pattern

```go
// store/store.go pattern
type Store struct {
    db *sql.DB
}

func New(dbPath string) (*Store, error) {
    // Open with modernc.org/sqlite driver name: "sqlite"
    db, err := sql.Open("sqlite", dbPath)
    // Enable WAL mode for better concurrency
    // Run migrations
    // Return store
}

func (s *Store) Close() error { return s.db.Close() }
```

**Migration runner pattern:**
- Embed SQL files at `store/` package level: `//go:embed migrations/*.sql` in `store.go` (NOT in a separate `migrations/` package — `go:embed` only works within the same package)
- The `migrations/embed.go` file is NOT needed — place the embed directive directly in `store.go`. NOTE: The Architecture document's directory structure still lists `store/migrations/embed.go` — this is INCORRECT and superseded by this story's guidance. Do NOT create embed.go.
- Read `schema_version` table for current version
- Apply all migrations with version > current
- Each migration in a transaction
- Driver import: `import _ "modernc.org/sqlite"` — registers as "sqlite"

### Cobra Root Command Pattern

```go
// cmd/yt-pipe/main.go
func main() {
    if err := cli.Execute(); err != nil {
        os.Exit(1)
    }
}

// internal/cli/root.go
var rootCmd = &cobra.Command{
    Use:   "yt-pipe",
    Short: "SCP YouTube content pipeline",
}

func Execute() error {
    return rootCmd.Execute()
}

func init() {
    // Global flags: --config, --verbose, --json-output
}
```

### Makefile Specification

```makefile
.PHONY: build test generate lint docker run clean

BINARY := bin/yt-pipe
MODULE := github.com/jay/youtube-pipeline

build:
	go build -o $(BINARY) ./cmd/yt-pipe

test:
	go test ./...

generate:
	go generate ./...

lint:
	go vet ./...

docker:
	docker build -t yt-pipe .

run:
	go run ./cmd/yt-pipe serve

clean:
	rm -rf bin/
```

### Dockerfile Specification

```dockerfile
# Stage 1: Build
FROM golang:latest AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /yt-pipe ./cmd/yt-pipe

# Stage 2: Minimal runtime
FROM scratch
COPY --from=builder /yt-pipe /yt-pipe
ENTRYPOINT ["/yt-pipe"]
```

### docker-compose.yml Specification

```yaml
services:
  yt-pipe:
    build: .
    ports:
      - "8080:8080"
    volumes:
      - /data/raw:/data/raw:ro          # SCP data (read-only)
      - /data/projects:/data/projects    # Project workspace
      - /data/db:/data/db               # SQLite DB
      - ./config.yaml:/config/config.yaml  # YAML settings
    environment:
      - YTP_LLM_API_KEY=${YTP_LLM_API_KEY}
      - YTP_SILICONFLOW_KEY=${YTP_SILICONFLOW_KEY}
    command: ["serve"]
```

### Project Structure Notes

**Complete directory tree to create:**
```
youtube-pipeline/
├── cmd/
│   └── yt-pipe/
│       └── main.go
├── internal/
│   ├── cli/
│   │   └── root.go
│   ├── api/
│   │   └── doc.go                  # package api — placeholder
│   ├── service/
│   │   └── doc.go                  # package service — placeholder
│   ├── domain/
│   │   ├── project.go
│   │   ├── project_test.go
│   │   ├── scene.go
│   │   ├── manifest.go
│   │   ├── job.go
│   │   ├── scenario.go
│   │   ├── errors.go
│   │   └── errors_test.go
│   ├── plugin/
│   │   ├── doc.go                  # package plugin — placeholder
│   │   ├── llm/
│   │   │   └── doc.go
│   │   ├── tts/
│   │   │   └── doc.go
│   │   ├── imagegen/
│   │   │   └── doc.go
│   │   └── output/
│   │       └── doc.go
│   ├── config/
│   │   └── doc.go                  # package config — placeholder
│   ├── store/
│   │   ├── store.go
│   │   ├── store_test.go
│   │   ├── project.go
│   │   ├── job.go
│   │   ├── manifest.go
│   │   ├── execution_log.go
│   │   └── migrations/
│   │       └── 001_initial.sql
│   ├── workspace/
│   │   └── doc.go                  # package workspace — placeholder
│   ├── glossary/
│   │   └── doc.go                  # package glossary — placeholder
│   ├── retry/
│   │   └── doc.go                  # package retry — placeholder
│   └── mocks/
│       └── .gitkeep
├── tests/
│   └── integration/
│       └── .gitkeep
├── testdata/
│   └── .gitkeep
├── config.example.yaml
├── Dockerfile
├── docker-compose.yml
├── Makefile
├── .gitignore
├── go.mod
└── go.sum
```

### config.example.yaml Minimal Structure

```yaml
# youtube.pipeline configuration
# Full configuration is implemented in Story 1.2 — this is the placeholder structure

# SCP data source directory (read-only)
scp_data_path: "/data/raw"

# Project workspace directory
workspace_path: "/data/projects"

# SQLite database path
db_path: "/data/db/yt-pipe.db"

# API server settings
api:
  host: "localhost"
  port: 8080
  # api_key: set via YTP_API_KEY environment variable

# Plugin selections (configured in Story 1.2+)
llm:
  provider: "openai"
  # api_key: set via YTP_LLM_API_KEY environment variable

imagegen:
  provider: "siliconflow"
  # api_key: set via YTP_SILICONFLOW_KEY environment variable

tts:
  provider: "openai"
  # api_key: set via YTP_TTS_API_KEY environment variable

output:
  provider: "capcut"

# SCP glossary path
glossary_path: ""

# Logging
log_level: "info"    # debug, info, warn, error
log_format: "json"   # json, text
```

### Anti-Patterns (FORBIDDEN)

1. **NO** `http.Request` references in service layer
2. **NO** direct service-to-service calls (only orchestrator coordinates)
3. **NO** Option pattern, global variables, or `init()` functions (except Cobra's init for flag binding) — **Story 1.1 relevant: Cobra root.go uses init() for flag binding, this is the ONLY allowed exception**
4. **NO** external API calls in tests
5. **NO** CGO — all dependencies must be pure Go (scratch Docker requirement) — **Story 1.1 relevant: modernc.org/sqlite is CGO-free, do NOT use mattn/go-sqlite3**
6. **NO** external logging libraries — use `log/slog` only — **Story 1.1 relevant: do not add zerolog, zap, or logrus**

### Architecture Compliance Checklist

- [ ] Module path: `github.com/jay/youtube-pipeline` → Task 1.1
- [ ] All packages use lowercase single-word names → Task 2, 3, 5, 6
- [ ] `domain/` has zero external imports (pure data structures) → Task 2
- [ ] Store uses `modernc.org/sqlite` driver (registers as "sqlite") → Task 3.1
- [ ] All exported functions follow `PascalCase` → Task 2, 3, 5
- [ ] All test files follow `Test{Function}_{Scenario}` naming → Task 2.7, 2.8, 3.8
- [ ] Migration files: `{NNN}_{description}.sql` (3-digit zero-padded) → Task 3.3
- [ ] No import cycles: domain ← store, workspace, plugin ← service ← cli, api → Task 7

### Dependency Direction (Import Cycle Prevention)

```
domain/         ← (all packages reference)
retry/          ← service/, plugin/
store/          → domain/
workspace/      → domain/
plugin/         → domain/, retry/
service/        → domain/, store/, workspace/, plugin/, config/, glossary/, retry/
cli/, api/      → service/ (+ domain/ for types)
```

**CRITICAL:** `domain/` must have ZERO imports from other internal packages.

### References

- [Source: _bmad-output/planning-artifacts/architecture.md#Selected Stack]
- [Source: _bmad-output/planning-artifacts/architecture.md#Core Architectural Decisions]
- [Source: _bmad-output/planning-artifacts/architecture.md#Implementation Patterns & Consistency Rules]
- [Source: _bmad-output/planning-artifacts/architecture.md#Project Structure & Boundaries]
- [Source: _bmad-output/planning-artifacts/architecture.md#Cross-Cutting Concerns — Structured Logging (log/slog JSON)]
- [Source: _bmad-output/planning-artifacts/epics.md#Story 1.1]
- [Source: _bmad-output/planning-artifacts/prd.md#CLI Structure]
- [Source: _bmad-output/planning-artifacts/prd.md#FR22 State Machine]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (orchestrator) + executor-high (domain, store) + executor (build tooling, cobra, placeholders)

### Debug Log References

- Fixed execution_log FK constraint: empty string JobID triggered FOREIGN KEY violation; changed to pass NULL when JobID is empty and use sql.NullString for scanning

### Completion Notes List

- Task 1: Go module initialized at github.com/jay/youtube-pipeline with all dependencies (cobra, viper, chi, modernc.org/sqlite, testify). Full directory structure created per architecture spec.
- Task 2: All 5 domain models + 4 custom error types created. 10 unit tests pass covering state transitions, error messages, Unwrap(), errors.Is(), errors.As().
- Task 3: SQLite store with embedded migration runner (go:embed), 001_initial.sql creating 4 tables. CRUD operations for projects, jobs, manifests, execution_logs. 22 integration tests pass using :memory: SQLite.
- Task 4: Makefile (build/test/generate/lint/docker/run/clean), multi-stage Dockerfile (golang→scratch), docker-compose.yml with 4 volume mounts, config.example.yaml.
- Task 5: Cobra root command with global flags (--config, --verbose, --json-output). `go run ./cmd/yt-pipe --help` verified.
- Task 6: 11 placeholder doc.go files with package declarations and import direction constraints.
- Task 7: All 4 verification gates passed — go build, go test, go vet, make build.

### File List

- go.mod (new)
- go.sum (new)
- .gitignore (new)
- Makefile (new)
- Dockerfile (new)
- docker-compose.yml (new)
- config.example.yaml (new)
- cmd/yt-pipe/main.go (new)
- internal/cli/root.go (new)
- internal/domain/project.go (new)
- internal/domain/project_test.go (new)
- internal/domain/scene.go (new)
- internal/domain/manifest.go (new)
- internal/domain/job.go (new)
- internal/domain/scenario.go (new)
- internal/domain/errors.go (new)
- internal/domain/errors_test.go (new)
- internal/store/store.go (new)
- internal/store/store_test.go (new)
- internal/store/project.go (new)
- internal/store/job.go (new)
- internal/store/manifest.go (new)
- internal/domain/execution_log.go (new)
- internal/store/execution_log.go (new)
- internal/store/migrations/001_initial.sql (new)
- internal/api/doc.go (new)
- internal/service/doc.go (new)
- internal/plugin/doc.go (new)
- internal/plugin/llm/doc.go (new)
- internal/plugin/tts/doc.go (new)
- internal/plugin/imagegen/doc.go (new)
- internal/plugin/output/doc.go (new)
- internal/config/doc.go (new)
- internal/workspace/doc.go (new)
- internal/glossary/doc.go (new)
- internal/retry/doc.go (new)
- internal/mocks/.gitkeep (new)
- tests/integration/.gitkeep (new)
- testdata/.gitkeep (new)

## Change Log

- 2026-03-08: Story 1.1 implemented — full project scaffolding with Go module, domain models, SQLite store, build tooling, Cobra CLI, and placeholder packages
- 2026-03-08: Code review — 4 HIGH + 4 MEDIUM + 1 LOW issues found. Fixed: H1 (Project timestamp type string→time.Time), H2 (migration runner version tracking), H4 (ExecutionLog moved to domain/), M1 (time.Parse error handling), M3 (Transition updates UpdatedAt). H3 (Viper/Chi in go.mod) deferred to next story. M2 (datetime format) noted. M4 (test location convention) accepted. L1 (glossary doc comment) noted.
