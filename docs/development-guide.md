# Development Guide — yt.pipe

> Auto-generated project documentation (2026-03-09)

## Prerequisites

- **Go 1.25.7** or later
- **Docker** + Docker Compose (optional, for containerized deployment)
- **Make** (build automation)

## Quick Start

```bash
# Clone and build
git clone <repo-url>
cd yt.pipe
make build          # → bin/yt-pipe

# Configure
cp config.example.yaml config.yaml
cp .env.example .env
# Edit config.yaml and .env with your API keys

# Run
make run            # → go run ./cmd/yt-pipe serve
# or
./bin/yt-pipe serve
```

## Build Commands

| Command | Description |
|---------|-------------|
| `make build` | Build binary → `bin/yt-pipe` |
| `make test` | Run unit tests (`go test ./...`) |
| `make test-integration` | Run integration tests (`-tags=integration -timeout 600s`) |
| `make lint` | Run linting (`go vet ./...`) |
| `make run` | Run API server (`go run ./cmd/yt-pipe serve`) |
| `make generate` | Run code generation (`go generate ./...`) |
| `make docker` | Build Docker image (`yt-pipe:latest`) |
| `make docker-up` | Start Docker Compose |
| `make docker-down` | Stop Docker Compose |
| `make clean` | Remove build artifacts (`rm -rf bin/`) |

## Configuration

### Priority Chain (highest → lowest)

1. CLI flags (`--flag`)
2. Environment variables (`YTP_` prefix)
3. Project config (`./config.yaml`)
4. Global config (`~/.yt-pipe/config.yaml`)
5. Built-in defaults

### Key Environment Variables

| Variable | Description |
|----------|-------------|
| `YTP_LLM_API_KEY` | LLM provider API key (Gemini/OpenAI) |
| `YTP_IMAGEGEN_API_KEY` | SiliconFlow FLUX API key |
| `YTP_TTS_API_KEY` | DashScope TTS API key |
| `YTP_API_PORT` | API server port (default: 8080) |
| `YTP_DB_PATH` | SQLite database path |
| `YTP_WORKSPACE_PATH` | Project workspace directory |
| `YTP_SCP_DATA_PATH` | SCP raw data directory |
| `YTP_LOG_LEVEL` | Log level: debug, info, warn, error |

### Config Validation

```bash
./bin/yt-pipe config show      # Display current config (keys masked)
./bin/yt-pipe config validate  # Validate configuration
```

## CLI Usage

### Core Commands

```bash
# Project lifecycle
yt-pipe init <scp-id>          # Initialize project
yt-pipe run <scp-id>           # Run full pipeline
yt-pipe run <scp-id> --auto-approve  # Skip all approval gates
yt-pipe status <scp-id>        # Check project status

# Individual stages
yt-pipe run <scp-id> --stage image_generate
yt-pipe run <scp-id> --stage tts_synthesize
yt-pipe run <scp-id> --stage assemble

# Asset management
yt-pipe image generate <scp-id> --scenes 1,3,5  # Regenerate specific scenes
yt-pipe tts generate <scp-id> --scenes 2,4      # Regenerate TTS
yt-pipe assemble <scp-id>                        # Reassemble output

# Scene approval
yt-pipe scenes list <project-id>
yt-pipe scenes approve <project-id> --type image --scene 1
yt-pipe scenes reject <project-id> --type tts --scene 3

# Template management
yt-pipe template list
yt-pipe template show <id>
yt-pipe template create --category scenario --name custom

# Character presets
yt-pipe character list --scp SCP-173
yt-pipe character create --scp SCP-173 --name "The Sculpture"

# Mood presets
yt-pipe mood list
yt-pipe mood create --name tense --emotion fear --speed 1.1

# BGM management
yt-pipe bgm list
yt-pipe bgm add --name "Dark Ambient" --file /path/to/bgm.mp3

# Utilities
yt-pipe serve                   # Start API server
yt-pipe config show             # Show config
yt-pipe logs <project-id>       # View execution logs
yt-pipe metrics <project-id>    # Cost/performance metrics
yt-pipe clean <scp-id>          # Clean workspace
yt-pipe feedback <project-id>   # Submit feedback
```

## Testing

### Run Tests

```bash
make test                    # Unit tests
make test-integration        # Integration tests (600s timeout)
go test ./internal/store/... # Test specific package
go test -v -run TestName     # Run specific test
```

### Test Patterns

- Tests use `*_test.go` in the same package
- SQLite `:memory:` for test isolation
- `testify` for assertions (`assert`, `require`)
- Test data in `testdata/SCP-173/`
- Mocking configured via `.mockery.yaml`

## Code Conventions

- **Error handling**: Domain error types in `internal/domain/errors.go` (NotFoundError, ValidationError, PluginError, TransitionError)
- **External deps**: Always behind interfaces (plugin pattern)
- **Logging**: Use `log/slog` structured logger (JSON format)
- **Config**: Viper-based with 5-level priority chain
- **State machine**: Explicit transitions only, rejected via TransitionError
- **File I/O**: Atomic writes (temp file + rename) for crash safety

## Docker Deployment

```bash
# Build image
make docker

# Run with Compose
make docker-up

# View logs
make docker-logs

# Stop
make docker-down
```

### Docker Compose Volumes

The compose configuration mounts volumes for:
- Database persistence (`/data/db/`)
- Workspace storage (`/data/projects/`)
- SCP raw data (`/data/raw/`)
- Configuration (`/config/`)

## Project Layout

```
cmd/yt-pipe/     → Entry point
internal/
  api/           → REST API (chi router, 20 endpoints)
  cli/           → CLI commands (cobra, 20+ commands)
  config/        → Configuration (viper, 5-level priority)
  domain/        → Domain models (13 models, state machines)
  glossary/      → SCP terminology dictionary
  logging/       → Structured logging setup
  pipeline/      → 8-stage orchestrator with checkpoint/resume
  plugin/        → Plugin system (LLM, TTS, ImageGen, Output)
  retry/         → Exponential backoff + jitter
  service/       → Business logic (30+ files)
  store/         → SQLite persistence (7 migrations, 80+ ops)
  template/      → Go template rendering
  workspace/     → Filesystem operations + atomic writes
templates/       → Prompt template files (.tmpl)
testdata/        → Test fixtures (SCP-173 sample)
tests/           → Integration tests
```
