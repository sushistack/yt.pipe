# Story 1.6: Docker Packaging & Deployment

Status: ready-for-dev

## Story

As a creator,
I want to deploy the pipeline as a Docker container on my home server with a single command,
So that setup and updates are simple and data persists across container restarts.

## Acceptance Criteria

1. **Multi-Stage Docker Build** (AC:1)
   - Given a Dockerfile exists using multi-stage build
   - When `docker build` is executed
   - Then the first stage compiles with `golang:latest` and the final stage uses `scratch` for minimal image size
   - And the resulting image contains only the `yt-pipe` binary
   - And the binary is statically linked (CGO_ENABLED=0)
   - And this satisfies NFR13

2. **Docker Compose Service** (AC:2)
   - Given a `docker-compose.yml` is configured
   - When `docker-compose up` is executed
   - Then the service starts with 4 volume mounts: `/data/raw` (SCP data, read-only), `/data/projects` (workspace), `/data/db` (SQLite), `/config` (YAML settings)
   - And the API server starts on localhost:8080 by default
   - And this satisfies NFR15

3. **Secret Injection via Environment** (AC:3)
   - Given API keys are configured via environment variables
   - When the container starts
   - Then `YTP_LLM_API_KEY`, `YTP_IMAGEGEN_API_KEY`, `YTP_TTS_API_KEY` are injected from environment
   - And no secrets appear in the Docker image, config files, or logs
   - And this satisfies NFR14

4. **Makefile Docker Targets** (AC:4)
   - Given the Makefile has docker targets
   - When `make docker` is run
   - Then the image is built and tagged as `yt-pipe:latest`
   - When `make docker-up` is run
   - Then docker-compose up is executed
   - When `make docker-down` is run
   - Then docker-compose down is executed

## Tasks / Subtasks

- [ ] Task 1: Enhance Dockerfile (AC: #1)
  - [ ] 1.1 Pin Go version (golang:1.24 instead of latest) for reproducible builds
  - [ ] 1.2 Add LABEL metadata (maintainer, version, description)
  - [ ] 1.3 Add `-ldflags="-s -w"` for smaller binary (strip debug info)
  - [ ] 1.4 Add `-trimpath` for reproducible builds
  - [ ] 1.5 Copy ca-certificates from builder for HTTPS API calls
  - [ ] 1.6 Set non-root user (nobody) for security
  - [ ] 1.7 Add EXPOSE 8080 documentation

- [ ] Task 2: Enhance docker-compose.yml (AC: #2, #3)
  - [ ] 2.1 Add all env vars: YTP_LLM_API_KEY, YTP_IMAGEGEN_API_KEY, YTP_TTS_API_KEY, YTP_API_KEY
  - [ ] 2.2 Add health check using CLI command
  - [ ] 2.3 Add restart policy (unless-stopped)
  - [ ] 2.4 Add container_name for easy identification
  - [ ] 2.5 Ensure /config volume mount is correct

- [ ] Task 3: Create .env.example file (AC: #3)
  - [ ] 3.1 Document all supported environment variables
  - [ ] 3.2 Add comments explaining each variable

- [ ] Task 4: Enhance Makefile (AC: #4)
  - [ ] 4.1 Add `docker-up` target
  - [ ] 4.2 Add `docker-down` target
  - [ ] 4.3 Add `docker-logs` target
  - [ ] 4.4 Add version/tag support for docker build

- [ ] Task 5: Verification
  - [ ] 5.1 `docker build -t yt-pipe .` succeeds
  - [ ] 5.2 Image size is minimal (under 20MB)
  - [ ] 5.3 Binary runs from scratch image
  - [ ] 5.4 All tests still pass: `go test ./...`

## Dev Notes

### Critical Architecture Constraints

**Docker Build (Architecture-mandated):**
- Multi-stage build: golang → scratch
- CGO_ENABLED=0 mandatory (modernc.org/sqlite is CGO-free)
- Single binary only in final image
- ca-certificates needed for HTTPS calls to external APIs

**Volume Mounts (Architecture-mandated):**
- `/data/raw` — SCP source data (READ-ONLY)
- `/data/projects` — generated project workspace
- `/data/db` — SQLite database file
- `/config` — YAML configuration

**Environment Variables (Config system):**
- YTP_LLM_API_KEY — LLM provider API key
- YTP_IMAGEGEN_API_KEY — Image generation API key
- YTP_TTS_API_KEY — TTS provider API key
- YTP_API_KEY — REST API authentication key

**Anti-Patterns (FORBIDDEN):**
1. **NO** secrets in Docker image layers
2. **NO** CGO dependencies (breaks scratch base)
3. **NO** `latest` tag for Go version in production
4. **NO** running as root in container

### Previous Story Intelligence (Stories 1.1-1.5)

**From Story 1.1 (Scaffolding):**
- Basic Dockerfile, docker-compose.yml, Makefile already exist
- Entry point: `cmd/yt-pipe/main.go` → `cli.Execute()`
- Module: `github.com/jay/youtube-pipeline`

**From Story 1.2 (Config):**
- Config reads from env vars with YTP_ prefix
- Viper handles env var binding automatically
- `config.MaskSecrets()` ensures no key leakage in logs

**Files this story modifies:**
- `Dockerfile` — enhance from scaffold to production
- `docker-compose.yml` — enhance with all volumes and env vars
- `Makefile` — add docker-up, docker-down targets

**Files this story creates:**
- `.env.example` — environment variable documentation

**Files that MUST NOT be modified:**
- All `internal/` code — this is infra-only story

### References

- [Source: _bmad-output/planning-artifacts/architecture.md#Infrastructure & Deployment]
- [Source: _bmad-output/planning-artifacts/epics.md#Story 1.6]
- [Source: _bmad-output/planning-artifacts/prd.md#NFR13 — Docker packaging]
- [Source: _bmad-output/planning-artifacts/prd.md#NFR14 — Env var secrets]
- [Source: _bmad-output/planning-artifacts/prd.md#NFR15 — Volume persistence]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6

### Completion Notes List

### File List
