---
stepsCompleted:
  - step-01-validate-prerequisites
  - step-02-design-epics
  - step-03-create-stories
  - step-04-final-validation
  - step-01-validate-prerequisites-phase2
  - step-02-design-epics-phase2
  - step-03-create-stories-phase2
  - step-04-final-validation-phase2
inputDocuments:
  - _bmad-output/planning-artifacts/prd.md
  - _bmad-output/planning-artifacts/architecture.md
  - _bmad-output/planning-artifacts/prd-validation-report.md
  - /mnt/work/projects/video.pipeline/assets/prompts/ (legacy prompt templates)
  - /mnt/data/raw/ (422 SCP structured data samples)
---

# youtube.pipeline - Epic Breakdown

## Overview

This document provides the complete epic and story breakdown for youtube.pipeline, decomposing the requirements from the PRD and Architecture requirements into implementable stories.

## Requirements Inventory

### Functional Requirements

**SCP Data Management (3)**
- FR1: Creator can input an SCP ID to automatically load that SCP's structured data (facts.json, meta.json, main.txt)
- FR2: System can validate loaded data's schema version and return clear errors on mismatch
- FR3: System can isolate each SCP project in an independent directory structure

**Scenario Generation & Review (5)**
- FR4: System can auto-generate video scenarios using a frontier LLM based on SCP structured data
- FR5: System can inline-tag generated scenarios with facts.json source references for fact verification
- FR6: System can verify scenario fact coverage against a configured threshold (default 80%, configurable) and suggest supplements when below threshold
- FR7: Creator can review generated scenarios as markdown files, direct modifications to specific sections, and approve via `yt-pipe scenario approve` to proceed to next stage
- FR8: System can regenerate only specific sections of a scenario (no full regeneration required)

**Image Generation (4)**
- FR9: System can auto-generate per-scene image prompts based on an approved scenario
- FR10: System can generate per-scene images via configured image generation plugin
- FR11: Creator can selectively regenerate images for specific scenes (single/multiple scene specification)
- FR12: Creator can modify a specific scene's image prompt and regenerate

**TTS & Subtitles (4)**
- FR13: System can synthesize TTS narration based on the scenario
- FR14: System can apply TTS pronunciation overrides for all SCP terminology dictionary entries. Verified by 100% dictionary entry application
- FR15: Creator can re-synthesize narration for specific segments only
- FR16: System can auto-generate subtitles based on narration

**CapCut Project Assembly (3)**
- FR17: System can auto-assemble all generated assets (images, narration, subtitles) into a CapCut project
- FR18: System can auto-include CC-BY-SA 3.0 copyright notice in the video description
- FR19: System can display warnings when specific SCPs have additional copyright conditions

**Pipeline Control & State (13)**
- FR20: Creator can execute the full pipeline with a single command
- FR21: Creator can execute each pipeline stage individually
- FR22: System can manage project state via state machine (pending -> scenario_review -> approved -> generating -> complete)
- FR23: Creator can query a project's current state and progress
- FR24: System can perform incremental builds, regenerating only changed scenes
- FR25: System can store per-scene artifacts independently to support partial regeneration
- FR26: System can run in dry-run mode to verify pipeline flow without actual API calls
- FR27: System can record each pipeline stage's execution results as structured logs
- FR28: System can provide error information including failure point, cause, and CLI recovery command on error
- FR29: Creator can query scene-image mapping list to verify generated assets per scene
- FR30: System can send webhook notifications on project state changes. Supports event types (state_changed, error, completed), payload structure (projectId, event, state, timestamp, data), and up to 3 retries on delivery failure
- FR42: System can display current stage name, progress (%), and elapsed time in real-time on CLI during pipeline execution
- FR43: System can aggregate and query pipeline execution success rate (success/failure ratio)
- FR44: System can track and query the ratio of manual intervention steps vs total pipeline steps

**Configuration & Plugins (6)**
- FR31: Creator can set up API keys, data paths, and default profiles via initial setup wizard
- FR32: System can validate configured API key validity
- FR33: Creator can swap TTS, image generation, and LLM plugins via YAML config file
- FR34: System can support global configuration and per-project configuration overrides
- FR35: System can apply configuration priority (CLI flags > env vars > project YAML > global YAML > defaults)
- FR36: Creator can run pipeline verification with a test SCP after configuration changes

**API Interface (4)**
- FR37: System can expose each pipeline stage as an independent API endpoint
- FR38: System can perform API key-based authentication
- FR39: System can support approval wait state in async workflows. Applies a default 72-hour approval timeout with notification on expiry
- FR40: System can return consistent JSON response structure (status, data, error, timestamp, requestId)

**Total: 44 Functional Requirements**

### NonFunctional Requirements

**Performance (4)**
- NFR1: Full pipeline execution (SCP ID -> CapCut project) under 5 minutes excluding external API time, under 10 minutes including external APIs (10-scene basis). Measured by pipeline execution log total elapsed time
- NFR2: CLI command response (status queries, config validation, etc.) under 2 seconds. Measured by command start-to-response elapsed time
- NFR3: API endpoint response (request received -> job start confirmation) under 1 second. Measured by request-to-response elapsed time
- NFR4: Incremental builds skip unchanged scenes, reducing execution time proportional to (changed scenes / total scenes) ratio. Verified by pipeline log processed scene count and elapsed time

**Reliability (4)**
- NFR5: Pipeline success rate 99.9% — zero internal-error failures under normal external API conditions. Measured by success/failure ratio of last 100 executions
- NFR6: Selective automatic retry on external API errors (max 3 retries, progressive delay increase) for failed items only
- NFR7: Preserve intermediate artifacts on pipeline interruption — per-stage checkpoint saving, resume from interruption point. Verified by existence of previous stage artifacts after restart
- NFR8: Project data integrity — prevent existing project data corruption on abnormal termination. Verified by file integrity (checksum comparison) of project directory after abnormal termination

**Integration (4)**
- NFR9: Standardized plugin interfaces — LLM, TTS, image generation plugins conform to identical standard interface contracts
- NFR10: External API timeout — configurable per-API-call timeout (default 120 seconds)
- NFR11: n8n compatibility — API responses in standard JSON structure directly parseable by n8n HTTP Request nodes
- NFR12: CapCut project format compatibility — generated project files compatible with CapCut format version 360000 (new_version: 151.0.0). Generated based on verified template JSON structure from existing video.pipeline. Verified by successful CapCut project loading

**Deployment (3)**
- NFR13: Packaged as Docker image — full system startup via `docker-compose up` single command
- NFR14: API key injection via environment variables — no secret hardcoding in config files
- NFR15: Data persistence — SCP data and project output preserved across container recreation via Docker volumes

**Security (3)**
- NFR16: API keys managed only via environment variables or config files, never exposed in logs
- NFR17: Return 401 on authentication failure, do not log request contents
- NFR24: API server accessible only from localhost by default, expandable to designated networks via configuration

**Maintainability (5)**
- NFR18: Include per-project disk usage in `yt-pipe status` output, provide cleanup function for completed project intermediate artifacts
- NFR19: Structured logs output in JSON format, compatible with external log collection tools (n8n parseable)
- NFR20: Minimize inter-module coupling so individual modules (LLM/TTS/image gen/CapCut assembly) can be independently modified and tested. Verified by independent unit test execution per module
- NFR21: New plugin integration possible with only plugin implementation, no existing code changes required
- NFR22: API status query endpoint returns current stage name, progress (%), and elapsed time optimized for n8n polling

**Testing (1)**
- NFR23: Plugins provide test substitute implementations enabling full pipeline unit testing without external API calls

**Total: 24 Non-Functional Requirements**

### Additional Requirements

**From Architecture — Starter Template & Technology Stack:**
- Go language with Cobra (CLI), Chi (API Router), modernc.org/sqlite (CGO-free DB), testify + mockery (Testing), log/slog (Logging)
- Project scaffolding + `go mod init` is the first implementation story
- Makefile with build, test, generate, docker, run, lint targets

**From Architecture — Critical Design Decisions:**
- CapCut PoC validation required as pre-MVP gate — verify output generation using existing video.pipeline templates before full implementation
- SQLite Option B (aggressive) — unified storage for project state, scene manifests, execution history, and API cost logs
- Job table-based async task management for long-running operations
- Store (SQLite metadata) / Workspace (filesystem assets) separation pattern

**From Architecture — Domain Model & Patterns:**
- Scene model as shared domain model — pipe-filter pattern where each pipeline stage progressively enriches the scene
- Scenario output schema as inter-module contract (narration, visualDescription, factTags, mood) consumed by 4 downstream modules
- Timing Resolver component — separates TTS audio timing interpretation from consumers (image transitions, subtitle sync, CapCut timeline)
- Scene dependency chain — manifest tracks asset dependencies, upstream changes auto-invalidate downstream (incremental build correctness)
- Image generation and TTS can run in parallel (both depend only on scenario)

**From Architecture — Infrastructure & Operations:**
- Docker multi-stage build (golang -> scratch) for minimal image size
- 4 volume mounts: /data/raw (SCP data, read-only), /data/projects (workspace), /data/db (SQLite), /config (YAML)
- Prompt sanitization — pre-process image prompts with safety modifiers for SCP horror/violence content
- MVP concurrency constraint — single pipeline execution only; concurrent trigger queueing/rejection policy needed
- Schema migration via go:embed SQL files + schema_version table

**From Architecture — Implementation Patterns:**
- Custom error types: NotFoundError, ValidationError, PluginError, TransitionError
- Common retry helper: retry(ctx, maxAttempts, backoff, fn)
- Context propagation: all service/plugin functions take context.Context as first parameter
- State machine transitions within SQLite transactions
- Atomic file writes (temp file + rename) for data integrity
- Plugin 4 types: LLM, TTS, ImageGen, OutputAssembler

**From Epic Planning — Prompt Quality Management (MVP Addition):**
- Prompt templates externalized as config files (not hardcoded) — scenario and image prompt templates editable without code changes for prompt tuning
- Prompt version tracking — record which prompt template version produced each output for reproducibility and rollback
- Per-prompt result quality feedback loop — record satisfaction/dissatisfaction per generation result to accumulate prompt improvement evidence

**From PRD Validation Report — Addressed Issues:**
- NFR measurement methods added (10 NFRs updated with measurement criteria)
- Missing FRs added: FR42 (CLI progress display), FR43 (success rate reporting), FR44 (manual intervention tracking)
- FR6 updated with default 80% threshold
- FR7 updated with approve command specification
- FR28 updated with recovery CLI command inclusion
- FR30 updated with event types, payload structure, retry policy
- FR39 updated with 72-hour timeout
- NFR24 added (localhost binding)
- Numeric conflict resolved (70% -> 75%)

### FR Coverage Map

- FR1: Epic 2 - SCP ID input and structured data auto-loading
- FR2: Epic 2 - SCP data schema version validation
- FR3: Epic 2 - Per-SCP project directory isolation
- FR4: Epic 2 - LLM-based scenario auto-generation
- FR5: Epic 2 - Inline fact tagging with facts.json source
- FR6: Epic 2 - Fact coverage verification (default 80% threshold)
- FR7: Epic 2 - Scenario markdown review, modification, and approval
- FR8: Epic 2 - Scenario section-level partial regeneration
- FR9: Epic 3 - Per-scene image prompt auto-generation
- FR10: Epic 3 - Per-scene image generation via plugin
- FR11: Epic 3 - Selective scene image regeneration (single/multiple)
- FR12: Epic 3 - Image prompt editing and regeneration
- FR13: Epic 3 - TTS narration synthesis from scenario
- FR14: Epic 3 - SCP terminology dictionary TTS pronunciation override
- FR15: Epic 3 - Segment-level narration re-synthesis
- FR16: Epic 3 - Narration-based subtitle auto-generation
- FR17: Epic 4 - Auto-assemble all assets into CapCut project
- FR18: Epic 4 - CC-BY-SA 3.0 copyright auto-inclusion
- FR19: Epic 4 - Additional SCP copyright condition warnings
- FR20: Epic 5 - Full pipeline single-command execution
- FR21: Epic 5 - Stage-by-stage individual execution
- FR22: Epic 1 - Project state machine management
- FR23: Epic 5 - Project state and progress query
- FR24: Epic 5 - Incremental build (changed scenes only)
- FR25: Epic 5 - Per-scene independent artifact storage
- FR26: Epic 1 - Dry-run mode pipeline flow verification
- FR27: Epic 5 - Structured execution logs per stage
- FR28: Epic 5 - Error info with failure point, cause, and recovery CLI command
- FR29: Epic 5 - Scene-image mapping list query
- FR30: Epic 7 - Webhook notifications on state changes
- FR31: Epic 1 - Initial setup wizard (API keys, data paths, profiles)
- FR32: Epic 1 - API key validity validation
- FR33: Epic 1 - Plugin swap via YAML config
- FR34: Epic 1 - Global and per-project config overrides
- FR35: Epic 1 - 5-level configuration priority chain
- FR36: Epic 1 - Test pipeline run after config changes
- FR37: Epic 7 - Per-stage independent API endpoints
- FR38: Epic 7 - API key-based authentication
- FR39: Epic 7 - Async approval wait state (72h timeout)
- FR40: Epic 7 - Consistent JSON response structure
- FR42: Epic 5 - Real-time CLI progress display (stage, %, elapsed)
- FR43: Epic 6 - Pipeline success rate aggregation and query
- FR44: Epic 6 - Manual intervention ratio tracking and query

**Phase 2 — Concrete Plugin Implementations:**
- FR4: Epic 8 - Gemini LLM scenario generation (concrete implementation)
- FR5: Epic 8 - Fact-tagged scenario generation with 4-stage pipeline
- FR6: Epic 8 - Fact coverage verification with LLM
- FR8: Epic 8 - SCP glossary-aware scenario generation
- FR9: Epic 9 - SiliconFlow FLUX image prompt generation (3-stage pipeline)
- FR10: Epic 9 - SiliconFlow FLUX image generation with Frozen Descriptor Protocol
- FR11: Epic 9 - Selective scene image regeneration via SiliconFlow
- FR12: Epic 9 - Shot breakdown and image prompt editing
- FR13: Epic 10 - DashScope CosyVoice TTS narration synthesis
- FR14: Epic 10 - Korean Hangul pronunciation XML conversion for TTS
- FR15: Epic 10 - DashScope Voice Cloning via config-level VoiceID
- FR16: Epic 10 - CosyVoice Flash model support and subtitle generation
- FR17: Epic 11 - CapCut project assembly with generated assets
- FR18: Epic 11 - Timing synchronization (narration→image→subtitle alignment)
- FR19: Epic 11 - CC-BY-SA 3.0 copyright auto-inclusion in CapCut project
- FR20: Epic 12 - Full pipeline single-command execution (SCP→CapCut)
- FR32: Epic 12 - End-to-end integration testing with checkpoint/resume

## Epic List

### Epic 1: Project Foundation & Configuration
System is installed, configured, and verified ready for use. Creator can set up API keys, configure plugins via YAML, validate the entire setup with a dry-run, and rely on a robust state machine for project lifecycle.
**FRs covered:** FR22, FR26, FR31, FR32, FR33, FR34, FR35, FR36
**NFRs addressed:** NFR13 (Docker), NFR14 (env var secrets), NFR15 (data persistence)
**Additional:** Go stack scaffolding (Cobra+Chi+SQLite+slog), Docker multi-stage build, Makefile, State Machine

### Epic 2: SCP Data & Scenario Generation
Creator inputs an SCP ID and receives an AI-generated, fact-verified scenario. They can review it as markdown, request section-level modifications, and approve it to proceed.
**FRs covered:** FR1, FR2, FR3, FR4, FR5, FR6, FR7, FR8
**Additional:** SCP glossary system, scenario prompt template externalization

### Epic 3: Visual & Audio Asset Generation
Creator can generate per-scene images and narration from the approved scenario, with fine-grained control to regenerate individual scenes, edit prompts, and correct TTS pronunciation.
**FRs covered:** FR9, FR10, FR11, FR12, FR13, FR14, FR15, FR16
**Additional:** Image prompt template externalization, prompt sanitization (NSFW safety), Timing Resolver component

### Epic 4: CapCut Project Assembly
Creator opens CapCut and finds a nearly-complete project with all assets (images, narration, subtitles) assembled and synchronized — the "it's almost done" experience.
**FRs covered:** FR17, FR18, FR19
**Additional:** CapCut PoC validation gate, CapCut template-based assembly, timing-based asset placement, CC-BY-SA auto-inclusion

### Epic 5: Pipeline Orchestration & Reliability
Creator can run the full pipeline with one command, resume from failures at the exact interruption point, and rebuild only changed scenes — with real-time progress visibility.
**FRs covered:** FR20, FR21, FR23, FR24, FR25, FR27, FR28, FR29, FR42
**NFRs addressed:** NFR1 (performance), NFR2 (CLI response), NFR4 (incremental perf), NFR5 (99.9% success), NFR6 (retry), NFR7 (checkpoint), NFR8 (data integrity), NFR10 (API timeout)
**Additional:** Checkpoint/resume, scene dependency chain, image+TTS parallel execution

### Epic 6: Quality Tracking & Prompt Engineering
Creator can manage prompt versions, track output quality metrics, and continuously improve the pipeline's output through structured feedback loops.
**FRs covered:** FR43, FR44
**NFRs addressed:** NFR18 (disk management/cleanup), NFR19 (structured JSON logs)
**Additional:** Prompt version tracking, per-result quality feedback loop, prompt template management system

### Epic 7: REST API & External Integration
n8n and external systems can orchestrate the pipeline via REST API with async job management, webhook notifications, API key authentication, and polling-optimized status endpoints.
**FRs covered:** FR30, FR37, FR38, FR39, FR40
**NFRs addressed:** NFR3 (API response time), NFR9 (plugin interface standardization), NFR11 (n8n compatibility), NFR16 (API key log protection), NFR17 (auth failure handling), NFR22 (polling-optimized status), NFR24 (localhost binding)
**Additional:** Job-based async processing, webhook delivery with retry, API key authentication middleware
**Stories:** 7.1-7.7 (7 stories)

### Epic 8: LLM Plugin — Gemini Scenario Generation
Creator can generate complete video scenarios from SCP data using Gemini LLM with the proven 4-stage pipeline (Research→Structure→Writing→Review) from video.pipeline. Includes LLM fallback chain (Gemini→Qwen→DeepSeek) as a separate enhancement and minimal E2E smoke test for early user value validation.
**FRs covered:** FR4, FR5, FR6, FR8
**Additional:** 4-stage scenario pipeline from video.pipeline prompts, Gemini OpenAI-compatible endpoint, fallback chain story, E2E smoke test story, `go test -tags=integration` build tag separation
**Reference:** `/mnt/work/projects/video.pipeline/assets/prompts/story_script/` (01_research→04_review)

### Epic 9: ImageGen Plugin — SiliconFlow FLUX Image Generation
Creator can generate per-scene images from scenario data using SiliconFlow FLUX API, with Frozen Descriptor Protocol for entity visual consistency and the 3-stage shot-breakdown-to-prompt pipeline from video.pipeline.
**FRs covered:** FR9, FR10, FR11, FR12
**Additional:** 3-stage image prompt pipeline from video.pipeline prompts, Frozen Descriptor Protocol, SiliconFlow FLUX API integration, prompt sanitization
**Reference:** `/mnt/work/projects/video.pipeline/assets/prompts/image_prompt/` (01_shot_first_breakdown→03_shot_video)

### Epic 10: TTS Plugin — DashScope CosyVoice Narration
Creator can generate narration audio from scenario scripts using DashScope CosyVoice TTS, with Korean Hangul pronunciation XML conversion. Voice Cloning supported via config-level VoiceID option (no interface change), Flash model for cost optimization.
**FRs covered:** FR13, FR14, FR15, FR16
**Additional:** DashScope non-OpenAI API (WebSocket streaming + REST), Korean TTS XML format from video.pipeline, config-level VoiceID for Voice Cloning, Flash model toggle
**Reference:** `/mnt/work/projects/video.pipeline/assets/prompts/caption/scenario_refine_ko.txt`

### Epic 11: Output Plugin — CapCut Project Assembly
Creator can assemble all generated assets (images, narration audio, subtitles) into a CapCut-compatible project format with timing synchronization and CC-BY-SA 3.0 copyright/licensing metadata auto-inclusion.
**FRs covered:** FR17, FR18, FR19
**Additional:** Timing resolver (narration→image→subtitle alignment), CapCut draft format, copyright metadata automation

### Epic 12: End-to-End Pipeline Integration
Creator can run the complete pipeline from SCP data to CapCut project in a single command with checkpoint/resume, real-time progress display, and comprehensive integration test suite using `go test -tags=integration` build tags.
**FRs covered:** FR20, FR32
**Additional:** Integration test suite with build tag separation, E2E pipeline orchestration, checkpoint/resume across all stages

## Epic 1: Project Foundation & Configuration

System is installed, configured, and verified ready for use. Creator can set up API keys, configure plugins via YAML, and validate the entire setup with a dry-run.

### Story 1.1: Project Scaffolding & Development Environment

As a developer,
I want a fully initialized Go project with the correct directory structure, domain models, SQLite store, and build tooling,
So that all subsequent stories have a solid foundation to build upon.

**Acceptance Criteria:**

**Given** the repository is cloned and Go is installed
**When** `go mod init` is run and the project structure is created
**Then** the directory structure matches the Architecture document (cmd/, internal/cli/, internal/api/, internal/service/, internal/domain/, internal/plugin/, internal/config/, internal/store/, internal/workspace/, internal/glossary/, internal/retry/, internal/mocks/)
**And** `go build ./...` compiles without errors

**Given** the domain package is created
**When** domain models are defined
**Then** Project (with state enum: pending, scenario_review, approved, generating_assets, assembling, complete), Scene, SceneManifest, Job models exist in `internal/domain/`
**And** custom error types (NotFoundError, ValidationError, PluginError, TransitionError) are defined in `domain/errors.go`
**And** state transition map with allowed transitions is defined in `domain/project.go`

**Given** the store package is created
**When** SQLite is initialized with modernc.org/sqlite
**Then** `store.go` creates the database, runs embedded SQL migrations via `go:embed`, and tracks schema version
**And** initial migration `001_initial.sql` creates projects, jobs, scene_manifests, and execution_logs tables
**And** all table/column names follow `snake_case` convention per Architecture

**Given** the Makefile is created
**When** make targets are executed
**Then** `make build` produces `bin/yt-pipe`, `make test` runs all tests, `make generate` runs mockery, `make lint` runs go vet, `make docker` builds Docker image

**Given** the Cobra root command is created
**When** `go run ./cmd/yt-pipe --help` is executed
**Then** the CLI displays help text with `yt-pipe` as the binary name and lists available subcommands

### Story 1.2: Configuration Management System

As a creator,
I want a layered configuration system that merges settings from multiple sources with clear priority,
So that I can customize the pipeline at global, project, or command level without conflicts.

**Acceptance Criteria:**

**Given** Viper is integrated with the config package
**When** configuration is loaded
**Then** the 5-level priority chain is applied: CLI flags > environment variables (YTP_ prefix) > project YAML (./config.yaml) > global YAML ($HOME/.yt-pipe/config.yaml) > built-in defaults

**Given** a global config file exists at `$HOME/.yt-pipe/config.yaml`
**When** a project-level `config.yaml` overrides specific keys
**Then** only the overridden keys use project values; all other keys fall back to global config
**And** this satisfies FR34 (global and per-project config overrides)

**Given** the config types are defined
**When** configuration is loaded
**Then** structured types exist for: LLM plugin settings, TTS plugin settings, ImageGen plugin settings, OutputAssembler settings, SCP data path, project workspace path, API server settings, glossary path
**And** environment variables like `YTP_LLM_API_KEY`, `YTP_SILICONFLOW_KEY` are mapped to corresponding config fields

**Given** a `config.example.yaml` is provided
**When** a new user copies it
**Then** all configurable fields are documented with comments explaining each option and its default value

### Story 1.3: Plugin Interface Framework

As a developer,
I want standardized plugin interfaces for all external integrations with mock implementations,
So that each pipeline module can be developed and tested independently without external API dependencies.

**Acceptance Criteria:**

**Given** the plugin package is created
**When** plugin interfaces are defined
**Then** four interfaces exist: LLM (in `plugin/llm/interface.go`), TTS (in `plugin/tts/interface.go`), ImageGen (in `plugin/imagegen/interface.go`), OutputAssembler (in `plugin/output/interface.go`)
**And** each interface's methods accept `context.Context` as the first parameter
**And** each interface uses `domain/` types for input/output (Scene, ScenarioOutput, etc.)

**Given** `plugin/base.go` defines common helpers
**When** a plugin implementation is created
**Then** it can use shared Config loading, Timeout helpers, and the common retry helper from `internal/retry/retry.go`
**And** the retry helper supports configurable max attempts, exponential backoff, and retries only on network timeout/429/5xx errors

**Given** mockery is configured
**When** `make generate` (go generate ./...) is run
**Then** mock implementations for all 4 plugin interfaces are auto-generated in `internal/mocks/`
**And** unit tests can use these mocks to test service layer without external API calls (NFR23)

**Given** a plugin registry exists in config
**When** a plugin type is specified in YAML (e.g., `llm.provider: openai`)
**Then** the corresponding implementation is selected and initialized at startup

### Story 1.4: Initial Setup Wizard

As a creator,
I want a guided setup wizard that configures API keys, data paths, and default profiles,
So that I can get the pipeline running quickly without manually editing config files.

**Acceptance Criteria:**

**Given** the creator runs `yt-pipe init`
**When** the wizard starts
**Then** it prompts step-by-step for: LLM API key, SiliconFlow API key, TTS provider selection + API key, SCP data directory path, project workspace path
**And** each input is validated before proceeding to the next step
**And** this satisfies FR31

**Given** an API key is entered during setup
**When** the wizard validates it
**Then** a lightweight validation request is sent to the corresponding API endpoint
**And** success or failure is clearly displayed with actionable error messages
**And** this satisfies FR32

**Given** setup is complete
**When** the wizard finishes
**Then** a global config file is written to `$HOME/.yt-pipe/config.yaml` with all configured values
**And** API keys are stored as references to environment variable names (not plaintext) with instructions to set them
**And** the wizard displays a summary of configured settings and suggests running a test command

**Given** a creator wants to change the image generation plugin
**When** they edit the YAML config `imagegen.provider` field
**Then** the plugin is swapped on next pipeline execution without code changes
**And** this satisfies FR33

### Story 1.5: Dry-Run Mode & Configuration Verification

As a creator,
I want to verify my pipeline configuration and flow without making real API calls,
So that I can catch configuration errors before spending API credits.

**Acceptance Criteria:**

**Given** the creator runs `yt-pipe run <scp-id> --dry-run`
**When** the pipeline executes in dry-run mode
**Then** every pipeline stage is invoked using the mock plugin implementations (from `internal/mocks/`) instead of real API calls
**And** the mock plugins return deterministic sample data (e.g., placeholder image, sample audio, fixed timing)
**And** the output shows each stage's expected inputs/outputs and timing
**And** exit code 0 indicates the pipeline flow is valid, non-zero indicates configuration or flow errors
**And** this satisfies FR26

**Given** the creator has changed configuration settings
**When** they run `yt-pipe run <scp-id> --dry-run` to verify
**Then** the new config values are loaded and applied throughout the dry-run
**And** any invalid config values (missing API keys, unreachable paths) are reported with specific error messages
**And** this satisfies FR36

**Given** a dry-run completes successfully
**When** results are displayed
**Then** JSON output on stdout includes: stages executed, config values used (keys masked), plugin selections, data paths verified
**And** exit code follows the convention: 0=success, 2=config error

### Story 1.6: Docker Packaging & Deployment

As a creator,
I want to deploy the pipeline as a Docker container on my home server with a single command,
So that setup and updates are simple and data persists across container restarts.

**Acceptance Criteria:**

**Given** a Dockerfile exists using multi-stage build
**When** `docker build` is executed
**Then** the first stage compiles with `golang:latest` and the final stage uses `scratch` for minimal image size
**And** the resulting image contains only the `yt-pipe` binary
**And** this satisfies NFR13

**Given** a `docker-compose.yml` is configured
**When** `docker-compose up` is executed
**Then** the service starts with 4 volume mounts: `/data/raw` (SCP data, read-only), `/data/projects` (workspace), `/data/db` (SQLite), `/config` (YAML settings)
**And** the API server starts on localhost:8080 by default
**And** this satisfies NFR15

**Given** API keys are configured via environment variables
**When** the container starts
**Then** `YTP_LLM_API_KEY`, `YTP_SILICONFLOW_KEY`, and other secrets are injected from environment
**And** no secrets appear in the Docker image, config files, or logs
**And** this satisfies NFR14

### Story 1.7: Project State Machine & Transitions

As a developer,
I want a robust state machine that governs project lifecycle transitions within SQLite transactions,
So that the project always has a consistent, valid state even during failures.

**Acceptance Criteria:**

**Given** the state machine is implemented in `service/project.go`
**When** a state transition is requested
**Then** the system validates the transition against the allowed transition map (pending -> scenario_review -> approved -> generating_assets -> assembling -> complete)
**And** invalid transitions return a TransitionError with current state, requested state, and allowed transitions
**And** this satisfies FR22

**Given** a state transition is valid
**When** it is executed
**Then** the state update runs within a SQLite transaction
**And** the transition timestamp is recorded in the project record
**And** the previous state is preserved in the execution log for audit

**Given** a concurrent state change is attempted (e.g., two CLI commands for the same project)
**When** both try to update the state simultaneously
**Then** SQLite's serialized writes ensure only one succeeds
**And** the other receives a TransitionError

**Given** the system restarts after a crash
**When** the project state is loaded
**Then** the last committed state in SQLite is the authoritative state
**And** no intermediate/corrupted states exist

## Epic 2: SCP Data & Scenario Generation

Creator inputs an SCP ID and receives an AI-generated, fact-verified scenario. They can review it as markdown, request section-level modifications, and approve it to proceed.

### Story 2.1: SCP Data Loading & Schema Validation

As a creator,
I want to input an SCP ID and have the system automatically load and validate its structured data,
So that I can start the content pipeline with confidence that the source data is correct.

**Acceptance Criteria:**

**Given** a valid SCP ID (e.g., SCP-173) is provided
**When** `yt-pipe scenario generate SCP-173` is executed
**Then** the system locates the SCP data directory under the configured SCP data path (e.g., `/data/raw/SCP-173/`)
**And** loads facts.json, meta.json, and main.txt files
**And** returns the parsed data as structured Go types
**And** this satisfies FR1

**Given** SCP data files are loaded
**When** schema validation runs
**Then** the system checks the schema version field in facts.json and meta.json against the expected version
**And** on mismatch, returns a ValidationError with expected vs actual version details
**And** on missing files, returns a clear error specifying which file is missing
**And** this satisfies FR2

**Given** an SCP ID that does not exist in the data directory
**When** loading is attempted
**Then** the system returns a NotFoundError with the message "SCP data not found: SCP-XXX" and exit code 1

**Given** SCP data is successfully loaded
**When** the data is returned
**Then** the `workspace/scp_data.go` module handles all file I/O
**And** the loaded data is read-only (never modified by the pipeline)

### Story 2.2: Project Workspace Initialization

As a creator,
I want each SCP project to be isolated in its own directory with a structured scene layout,
So that projects don't interfere with each other and I can manage them independently.

**Acceptance Criteria:**

**Given** a new pipeline run is started for an SCP ID
**When** the project is initialized
**Then** a project directory is created at `{workspace}/{scp-id}-{timestamp}/`
**And** a `scenes/` subdirectory is prepared for per-scene artifact storage
**And** a project record is created in SQLite with state `pending`
**And** this satisfies FR3

**Given** a project directory is created
**When** the directory structure is inspected
**Then** the layout follows: `{scp-id}-{timestamp}/scenes/{scene-num}/` with subdirectories for each scene's assets (image, audio, subtitle, metadata)
**And** each scene directory is self-contained with all its artifacts

**Given** multiple projects exist for the same SCP ID
**When** `yt-pipe status SCP-173` is queried
**Then** all projects for that SCP ID are listed with their timestamps and current states

**Given** a project is initialized
**When** the workspace module creates directories
**Then** all file writes use atomic operations (temp file + rename) to prevent corruption on interruption

### Story 2.3: SCP Glossary System

As a creator,
I want an SCP terminology dictionary that provides accurate terms across the entire pipeline,
So that TTS pronunciation, subtitles, and scenarios consistently use correct SCP terminology.

**Acceptance Criteria:**

**Given** a glossary JSON file exists at the configured glossary path
**When** the system starts
**Then** `glossary/glossary.go` loads the external JSON file at runtime
**And** the glossary contains entries with: term, pronunciation override, definition, and category (containment class, organization, entity, etc.)

**Given** the glossary is loaded
**When** any module queries a term
**Then** the glossary provides lookup by term name, returning pronunciation override and metadata
**And** the glossary is read-only and thread-safe (can be used across goroutines)

**Given** the glossary file is missing or malformed
**When** loading is attempted
**Then** the system logs a warning and continues with an empty glossary (non-blocking)
**And** a warning is displayed to the creator suggesting to configure the glossary path

**Given** the glossary is available
**When** used across modules
**Then** scenario generation uses it for term accuracy, TTS uses it for pronunciation overrides, and subtitle generation uses it for spelling consistency

### Story 2.4: Scenario Generation with Fact Tagging

As a creator,
I want the system to generate a structured video scenario from SCP data with inline fact references,
So that I can verify the scenario's factual accuracy against the source data.

**Acceptance Criteria:**

**Given** SCP data is loaded and validated
**When** `yt-pipe scenario generate <scp-id>` is executed
**Then** the system sends the SCP structured data (facts.json, meta.json, main.txt) to the configured LLM plugin
**And** the LLM generates a scenario with structured sections: intro, containment procedures, description, incident logs, conclusion
**And** each section contains narration text and visual description fields per the ScenarioOutput domain model
**And** this satisfies FR4

**Given** the scenario is generated
**When** the output is processed
**Then** fact references are inline-tagged as `[FACT:key]content[/FACT]` linking to facts.json entries
**And** each tagged fact can be traced back to a specific key in facts.json
**And** this satisfies FR5

**Given** the scenario prompt template is externalized
**When** the creator wants to tune the prompt
**Then** the template file can be edited without code changes
**And** the template path is configurable in YAML
**And** the template version is recorded in the scenario output metadata

**Given** scenario generation completes
**When** the output is saved
**Then** the scenario is written as a structured JSON file in the project workspace
**And** a markdown rendering is also saved for human review
**And** the project state transitions to `scenario_review` in SQLite

### Story 2.5: Fact Coverage Verification

As a creator,
I want the system to verify that the scenario covers sufficient facts from the source data,
So that I can be confident the video will be factually comprehensive.

**Acceptance Criteria:**

**Given** a scenario with inline fact tags exists
**When** fact coverage verification runs
**Then** the system compares tagged facts against all key entries in facts.json
**And** calculates a coverage percentage (tagged facts / total key facts * 100)
**And** this satisfies FR6

**Given** the coverage is at or above the configured threshold (default 80%)
**When** verification completes
**Then** the result is PASS with the coverage percentage displayed
**And** a detailed report shows which facts were covered and which were missed

**Given** the coverage is below the threshold
**When** verification completes
**Then** the result is WARN with the coverage percentage
**And** the system lists uncovered facts and suggests specific additions to improve coverage
**And** the creator can choose to proceed anyway or regenerate sections

**Given** the threshold is configurable
**When** the creator sets `scenario.fact_coverage_threshold: 90` in config
**Then** 90% is used instead of the default 80%

### Story 2.6: Scenario Review, Edit & Approval

As a creator,
I want to review the generated scenario as markdown, request modifications to specific sections, and formally approve it,
So that I maintain creative control over the content before proceeding to asset generation.

**Acceptance Criteria:**

**Given** a scenario is generated and state is `scenario_review`
**When** the creator opens the scenario markdown file
**Then** the file is human-readable with clear section headers, narration text, visual descriptions, and fact coverage summary
**And** the file path is displayed in CLI output for easy access

**Given** the creator wants to modify a specific section
**When** they run `yt-pipe scenario generate <scp-id> --section intro --instruction "make it more suspenseful"`
**Then** only the specified section is regenerated via LLM with the given instruction
**And** all other sections remain unchanged
**And** fact tags in the regenerated section are updated
**And** this satisfies FR8

**Given** the creator is satisfied with the scenario
**When** they run `yt-pipe scenario approve <scp-id>`
**Then** the project state transitions from `scenario_review` to `approved` in SQLite
**And** the approved scenario is locked (marked as final version)
**And** a confirmation message is displayed with next steps
**And** this satisfies FR7

**Given** the creator tries to approve without a generated scenario
**When** `yt-pipe scenario approve` is run
**Then** the system returns a TransitionError explaining the current state doesn't allow approval

**Given** the creator tries to generate images before approval
**When** `yt-pipe image generate <scp-id>` is run in `scenario_review` state
**Then** the system returns a TransitionError: "Scenario must be approved before generating images"

## Epic 3: Visual & Audio Asset Generation

Creator can generate per-scene images and narration from the approved scenario, with fine-grained control to regenerate individual scenes, edit prompts, and correct TTS pronunciation.

### Story 3.1: Image Prompt Generation & Sanitization

As a creator,
I want the system to auto-generate image prompts from the scenario's visual descriptions with safety processing,
So that each scene gets a high-quality, API-safe image prompt without manual prompt engineering.

**Acceptance Criteria:**

**Given** a scenario is approved (state: `approved`)
**When** image prompt generation is triggered
**Then** the system reads each scene's `visualDescription` from the ScenarioOutput
**And** generates a detailed image prompt for each scene using the externalized image prompt template
**And** the prompt template version is recorded in the scene metadata
**And** this satisfies FR9

**Given** image prompts are generated
**When** safety sanitization runs
**Then** each prompt is preprocessed with safety modifiers to avoid NSFW filter triggers for SCP horror/violence content
**And** sanitization rules are configurable (add/remove modifier terms)
**And** the original prompt and sanitized prompt are both stored in the scene directory

**Given** the image prompt template is externalized
**When** the creator edits the template file
**Then** subsequent prompt generations use the updated template without code changes
**And** the template path is configurable in YAML

### Story 3.2: Image Generation & Scene Control

As a creator,
I want to generate images for all or specific scenes and be able to edit prompts and regenerate individual scenes,
So that I can achieve the desired visual quality with minimal effort and API cost.

**Acceptance Criteria:**

**Given** image prompts exist for all scenes
**When** `yt-pipe image generate <scp-id>` is executed
**Then** the system sends each scene's prompt to the configured ImageGen plugin
**And** generated images are saved to each scene's directory (`scenes/{num}/image.png`)
**And** the scene manifest in SQLite is updated with image hash and generation timestamp
**And** this satisfies FR10

**Given** the creator wants to regenerate specific scenes only
**When** `yt-pipe image generate <scp-id> --scene 3,5,7` or `--scene 3-7` is executed
**Then** only the specified scenes' images are regenerated
**And** all other scenes' images remain unchanged
**And** this satisfies FR11

**Given** the creator is unsatisfied with a scene's image prompt
**When** they edit the prompt file in `scenes/{num}/prompt.txt` and run `yt-pipe image generate <scp-id> --scene {num}`
**Then** the image is regenerated using the manually edited prompt
**And** the manifest records that the prompt was manually modified
**And** this satisfies FR12

**Given** an image generation fails for a specific scene
**When** the retry helper exhausts max attempts (3 retries with exponential backoff)
**Then** the error is logged with scene number and failure reason
**And** other scenes continue generating (partial failure does not abort all)
**And** the failed scene is marked in the manifest for easy identification

### Story 3.3: TTS Narration & Pronunciation

As a creator,
I want TTS narration synthesized from the scenario with correct SCP terminology pronunciation,
So that the narration sounds natural and uses accurate domain-specific pronunciation.

**Acceptance Criteria:**

**Given** a scenario is approved
**When** `yt-pipe tts generate <scp-id>` is executed
**Then** the system sends each scene's narration text to the configured TTS plugin
**And** generates audio files saved to each scene's directory (`scenes/{num}/audio.mp3`)
**And** this satisfies FR13

**Given** the SCP glossary is loaded with pronunciation overrides
**When** TTS synthesis processes the narration text
**Then** all glossary terms in the narration are replaced with their pronunciation overrides before sending to the TTS API
**And** 100% of glossary entries present in the text have overrides applied
**And** this satisfies FR14

**Given** the creator wants to re-synthesize a specific narration segment
**When** `yt-pipe tts generate <scp-id> --scene 5` is executed
**Then** only scene 5's narration is re-synthesized
**And** the previous audio file is preserved as backup until the new one is confirmed
**And** this satisfies FR15

**Given** TTS synthesis completes for a scene
**When** the audio file is saved
**Then** the audio duration (milliseconds) and word-level timing data are extracted and stored in the scene metadata
**And** the scene manifest is updated with audio hash, duration, and generation timestamp

### Story 3.4: Timing Resolver

As a developer,
I want a timing resolver that interprets TTS audio timing into image transitions and subtitle synchronization data,
So that downstream modules (subtitles, CapCut assembly) have accurate timing without depending on TTS plugin specifics.

**Acceptance Criteria:**

**Given** TTS audio has been generated for all scenes with word-level timing data
**When** the Timing Resolver processes the timing data
**Then** it produces per-scene timing metadata: scene start time, scene end time, scene duration, word timestamps
**And** it calculates image transition points (when to switch from one scene's image to the next)
**And** it generates subtitle segment timing (start/end for each subtitle chunk)

**Given** a TTS plugin is swapped (e.g., OpenAI TTS to Edge TTS)
**When** the new plugin returns timing data in a different format
**Then** the Timing Resolver normalizes it to the same internal format
**And** all downstream consumers (subtitle generator, CapCut assembler) work without changes

**Given** timing data is resolved
**When** saved to the project workspace
**Then** a `timing.json` file per scene contains: scene duration, word timestamps, subtitle segments, transition points
**And** a project-level `timeline.json` contains the full video timeline with all scenes' timing concatenated

**Given** a scene's TTS is re-synthesized
**When** timing is recalculated
**Then** only the affected scene's timing is updated
**And** the project timeline is regenerated to reflect the change

### Story 3.5: Subtitle Generation

As a creator,
I want subtitles automatically generated from narration timing data with accurate SCP terminology,
So that the video has synchronized subtitles ready for CapCut assembly.

**Acceptance Criteria:**

**Given** TTS audio and timing data exist for all scenes
**When** subtitle generation is triggered
**Then** the system generates subtitle segments based on the Timing Resolver's word timestamps
**And** each subtitle segment has start time, end time, and text content
**And** subtitles are saved to each scene's directory (`scenes/{num}/subtitle.json`)
**And** this satisfies FR16

**Given** the SCP glossary is available
**When** subtitle text is generated
**Then** all SCP terminology uses the glossary's canonical spelling
**And** subtitle text matches the narration text exactly (no paraphrasing)

**Given** subtitles are generated for the full project
**When** the output is reviewed
**Then** a combined subtitle file is also generated at project level for preview purposes
**And** subtitle segment boundaries align with natural sentence/clause breaks (not mid-word)

**Given** a scene's TTS is re-synthesized
**When** subtitle regeneration runs for that scene
**Then** only the affected scene's subtitles are regenerated
**And** the combined project subtitle file is updated accordingly

## Epic 4: CapCut Project Assembly

Creator opens CapCut and finds a nearly-complete project with all assets (images, narration, subtitles) assembled and synchronized — the "it's almost done" experience.

### Story 4.1: CapCut Format PoC Validation

As a developer,
I want to validate that we can programmatically generate a valid CapCut project from the existing video.pipeline templates,
So that we confirm the core value proposition is technically feasible before building the full assembler.

**Acceptance Criteria:**

**Given** the existing video.pipeline CapCut templates (draft.template.json, draft.meta.info.json) are available
**When** a minimal PoC program generates a CapCut project file using these templates with sample assets (1 image, 1 audio, 1 subtitle track)
**Then** the generated project opens successfully in CapCut without errors
**And** the image is displayed on the video track at the correct position
**And** the audio plays on the audio track synchronized with the image duration
**And** the subtitle text appears at the correct timestamps

**Given** the PoC validates CapCut format version 360000 (new_version: 151.0.0) compatibility
**When** the project is loaded in CapCut
**Then** all tracks (video, audio, text) are recognized and editable
**And** this confirms NFR12 (CapCut format compatibility)

**Given** the generated CapCut project file exists
**When** automated validation runs
**Then** the JSON structure is validated against the known CapCut schema (required fields: tracks, segments, materials, canvas_config)
**And** track counts match expected (1 video, 1 audio, 1 text minimum)
**And** segment timing values are non-negative and sequential
**And** this provides automated regression testing for CapCut format changes

**Given** the PoC fails to produce a valid CapCut project
**When** the failure is analyzed
**Then** the team evaluates the fallback strategy: JSON timeline + FFmpeg assembly as alternative output
**And** the decision is documented before proceeding with Epic 4 remaining stories

### Story 4.2: CapCut Project Assembly

As a creator,
I want all generated assets automatically assembled into a CapCut project with proper timing,
So that I can open CapCut and find a nearly-complete video ready for final touches.

**Acceptance Criteria:**

**Given** all scene assets exist (images, audio, subtitles) and timing data is resolved
**When** `yt-pipe assemble <scp-id>` is executed
**Then** the OutputAssembler plugin creates a CapCut project file based on the validated template structure
**And** each scene's image is placed on the video track at the timing determined by the Timing Resolver
**And** each scene's audio is placed on the audio track synchronized with the corresponding image
**And** each subtitle segment is placed on the text track at the word-level timing positions
**And** the project file is saved to the project workspace (`output/draft_content.json`, `draft_meta_info.json`)
**And** this satisfies FR17

**Given** the assembly completes
**When** the project state is updated
**Then** the state transitions to `complete` in SQLite
**And** CLI output shows the CapCut project file path and total video duration
**And** a summary displays: number of scenes, total images, total audio duration, subtitle count

**Given** a scene's assets are regenerated after initial assembly
**When** `yt-pipe assemble <scp-id>` is re-run
**Then** the CapCut project is regenerated with the updated assets
**And** only the changed scenes' tracks are updated in the project

**Given** the CapCut assembler encounters missing assets for a scene
**When** assembly is attempted
**Then** a ValidationError lists all scenes with missing assets (image, audio, or subtitle)
**And** assembly does not proceed with incomplete data

### Story 4.3: Copyright & Licensing Automation

As a creator,
I want copyright notices automatically included in the output and warnings for special licensing conditions,
So that I comply with SCP Foundation licensing without manual tracking.

**Acceptance Criteria:**

**Given** a CapCut project is assembled
**When** the output is generated
**Then** a `description.txt` file is created in the project output directory containing the CC-BY-SA 3.0 attribution text
**And** the attribution includes: SCP Foundation credit, original author(s) from meta.json, CC-BY-SA 3.0 license link, and AI-generated content notice placeholder
**And** this satisfies FR18

**Given** an SCP entry has additional copyright conditions in its meta.json
**When** the project is assembled
**Then** the system displays a prominent CLI warning: "SCP-XXX has additional copyright conditions: [details]"
**And** the warning is also written to the project's metadata file for reference
**And** the warning is logged in structured JSON format
**And** this satisfies FR19

**Given** an SCP entry has no special copyright conditions
**When** the project is assembled
**Then** only the standard CC-BY-SA 3.0 attribution is generated
**And** no additional warnings are displayed

## Epic 5: Pipeline Orchestration & Reliability

Creator can run the full pipeline with one command, resume from failures at the exact interruption point, and rebuild only changed scenes — with real-time progress visibility.

### Story 5.1: Full Pipeline Orchestration

As a creator,
I want to run the entire pipeline from SCP data to CapCut project with a single command,
So that I can produce a complete video project with minimal manual steps.

**Acceptance Criteria:**

**Given** SCP data exists and configuration is valid
**When** `yt-pipe run <scp-id>` is executed
**Then** the pipeline orchestrator (`service/pipeline.go`) executes all stages in sequence: data loading -> scenario generation -> (pause for approval) -> image generation + TTS generation (parallel) -> subtitle generation -> assembly
**And** each stage's start/end is logged with slog
**And** this satisfies FR20

**Given** the pipeline reaches the scenario approval stage
**When** the scenario is generated
**Then** the pipeline pauses and prompts the creator to review and approve
**And** the creator runs `yt-pipe scenario approve <scp-id>` to resume
**And** the pipeline continues from the approved state

**Given** image generation and TTS generation are independent
**When** the pipeline enters `generating_assets` state
**Then** image generation and TTS synthesis run in parallel using goroutines
**And** subtitle generation waits for TTS completion (depends on timing data)
**And** assembly waits for all assets to complete

**Given** each pipeline stage is also available individually
**When** `yt-pipe scenario generate`, `yt-pipe image generate`, `yt-pipe tts generate`, `yt-pipe assemble` are run separately
**Then** each command executes only its specific stage and validates the required project state
**And** this satisfies FR21

### Story 5.2: Real-Time Progress & Status Display

As a creator,
I want real-time progress updates during pipeline execution and the ability to query project status at any time,
So that I know exactly what's happening and how far along the pipeline is.

**Acceptance Criteria:**

**Given** the pipeline is running
**When** a stage is executing
**Then** the CLI displays on stderr: current stage name, progress percentage (scenes completed / total scenes), and elapsed time
**And** progress updates at least once per scene completion
**And** this satisfies FR42

**Given** the creator runs `yt-pipe status <scp-id>`
**When** a project exists
**Then** JSON output on stdout includes: project state, current/last stage, progress percentage, elapsed time, scene count, per-scene asset status (image: yes/no, audio: yes/no, subtitle: yes/no)
**And** response time is under 2 seconds (NFR2)
**And** this satisfies FR23

**Given** the creator wants to see the scene-image mapping
**When** `yt-pipe status <scp-id> --scenes` is executed
**Then** a table displays: scene number, image file path, image status (generated/failed/pending), prompt (truncated), generation timestamp
**And** this satisfies FR29

### Story 5.3: Incremental Build with Hash-Based Skip

As a creator,
I want the pipeline to detect what has changed and only rebuild affected scenes,
So that I save time and API costs when making adjustments.

**Acceptance Criteria:**

**Given** a project has been fully generated once
**When** the creator modifies a scene's image prompt and runs `yt-pipe image generate <scp-id>`
**Then** the system compares scene manifest hashes (prompt hash, image hash, audio hash) to detect changes
**And** only scenes with changed inputs are regenerated
**And** unchanged scenes are skipped with a log message "scene N: unchanged, skipping"
**And** this satisfies FR24

**Given** scene assets are stored independently
**When** a scene is regenerated
**Then** only that scene's directory (`scenes/{num}/`) is modified
**And** other scenes' directories are untouched
**And** this satisfies FR25

**Given** the hash comparison detects no changes across all scenes
**When** the pipeline stage runs
**Then** all scenes are skipped and the stage completes immediately
**And** a summary message shows "0 scenes regenerated, N scenes skipped"

**Given** incremental build runs
**When** execution completes
**Then** the execution log records: total scenes, scenes processed, scenes skipped, time saved estimate

### Story 5.4: Scene Dependency Chain & Stale Invalidation

As a creator,
I want upstream changes to automatically invalidate downstream artifacts so the pipeline rebuilds only what's needed,
So that I never have stale outputs after modifying a scene's scenario, prompt, or audio.

**Acceptance Criteria:**

**Given** the scene dependency chain is defined: scenario section -> image prompt -> image, scenario section -> narration -> TTS audio -> timing -> subtitle
**When** a scene's scenario section is modified (upstream change)
**Then** the scene manifest invalidates all downstream artifacts: image prompt (depends on visual description), image (depends on prompt), TTS audio (depends on narration), subtitle (depends on audio timing)
**And** all invalidated artifacts are marked as `stale` in the manifest

**Given** artifacts are marked as stale
**When** the next pipeline run executes
**Then** only stale artifacts are regenerated
**And** non-stale artifacts are skipped
**And** the execution log records which artifacts were invalidated and why

**Given** a scene's image prompt is manually edited (not upstream-triggered)
**When** the manifest detects the prompt file hash changed
**Then** only the image is marked as stale (not TTS or subtitle, since narration didn't change)
**And** this demonstrates targeted invalidation based on the dependency graph

**Given** incremental build with dependency chain runs
**When** execution completes
**Then** the execution log records: total scenes, artifacts regenerated, artifacts skipped, dependency chain triggers
**And** this verifies NFR4 (proportional time reduction)

### Story 5.5: Checkpoint, Resume & Error Recovery

As a creator,
I want the pipeline to preserve progress on failure and provide clear recovery instructions,
So that I never lose completed work and can quickly fix and resume.

**Acceptance Criteria:**

**Given** the pipeline completes a stage successfully
**When** progress is saved
**Then** a checkpoint is recorded in SQLite: completed stage, scene-level progress, timestamp
**And** all generated artifacts are persisted to disk via atomic writes
**And** this satisfies NFR7

**Given** the pipeline fails mid-execution (e.g., scene 5 image generation fails)
**When** the creator checks the project state
**Then** scenes 1-4 artifacts are fully preserved on disk
**And** the project state reflects the last successful checkpoint
**And** this satisfies NFR8

**Given** a pipeline stage fails
**When** error information is returned
**Then** the error includes: failed stage name, scene number (if applicable), error cause, and a specific CLI recovery command (e.g., `yt-pipe image generate SCP-173 --scene 5`)
**And** the error is logged as structured JSON with all fields
**And** this satisfies FR27 and FR28

**Given** the creator runs the recovery command
**When** the pipeline resumes
**Then** it starts from the failed point, not from the beginning
**And** previously completed scenes are not re-processed

**Given** an abnormal termination occurs (kill signal, power loss)
**When** the system restarts
**Then** no existing project data is corrupted (verified by file integrity)
**And** the project can be resumed from the last checkpoint

### Story 5.6: Retry & Reliability Hardening

As a creator,
I want external API failures to be automatically retried with smart backoff,
So that transient errors don't require manual intervention.

**Acceptance Criteria:**

**Given** an external API call (LLM, TTS, ImageGen) fails with a retryable error (network timeout, 429, 5xx)
**When** the retry helper processes the failure
**Then** the call is retried up to 3 times with exponential backoff (e.g., 1s, 2s, 4s)
**And** each retry attempt is logged with attempt number, error type, and wait duration
**And** this satisfies NFR6

**Given** a non-retryable error occurs (400, 401, 403)
**When** the retry helper evaluates the error
**Then** no retry is attempted
**And** the error is immediately propagated with a clear message

**Given** all external API calls have configurable timeouts
**When** a call exceeds the timeout (default 120 seconds per NFR10)
**Then** the call is cancelled via context cancellation
**And** the timeout is treated as a retryable error

**Given** the creator presses Ctrl+C during pipeline execution
**When** the cancellation signal is received
**Then** context.Cancel propagates to all in-flight API calls
**And** the current stage's progress is checkpointed before exit
**And** a message displays: "Pipeline interrupted. Resume with: yt-pipe run <scp-id>"

**Given** the pipeline runs over many executions
**When** success/failure data accumulates in execution_logs
**Then** the success rate can be calculated from the last 100 executions
**And** this provides the measurement basis for NFR5 (99.9% success rate)

## Epic 6: Observability, Quality & Operational Excellence

Creator can track execution costs, manage prompt templates with version control, collect quality feedback on outputs, and clean up completed projects — ensuring the pipeline operates efficiently and improves over time.

### Story 6.1: Structured Logging & Execution History

As a creator,
I want all pipeline operations logged in structured JSON format with execution history and API cost tracking,
So that I can diagnose issues, understand costs, and audit pipeline behavior.

**Acceptance Criteria:**

**Given** any pipeline operation is executed (scenario generation, image generation, TTS synthesis, assembly)
**When** the operation starts, progresses, and completes
**Then** structured JSON log entries are emitted via slog with fields: timestamp, operation, scp_id, scene_number (if applicable), duration_ms, status (success/failure), error (if any)
**And** log level is configurable (debug/info/warn/error) via YAML config
**And** this satisfies NFR19

**Given** an external API call is made (LLM, TTS, ImageGen)
**When** the call completes
**Then** the execution log records: plugin name, model/service identifier, input token count (for LLM), estimated cost (if available from plugin), response time, and retry count
**And** logs are written to both stderr (for CLI visibility) and an execution_logs table in SQLite

**Given** a pipeline run completes (success or failure)
**When** the execution summary is generated
**Then** a summary log entry includes: total duration, stages completed, scenes processed, total API calls, total estimated cost, and final status
**And** the summary is stored in the execution_logs table with the project ID and run timestamp

**Given** the creator wants to review execution history
**When** `yt-pipe logs <scp-id>` is executed
**Then** the last N executions are displayed in reverse chronological order (default N=10, configurable)
**And** each entry shows: timestamp, operation, duration, status, cost estimate
**And** `--format json` outputs machine-readable JSON for external analysis

### Story 6.2: Prompt Template Management & Versioning

As a creator,
I want all LLM/image prompt templates externalized, versioned, and tracked per-output,
So that I can iterate on prompt quality, reproduce any previous result, and switch templates without code changes.

**Acceptance Criteria:**

**Given** the system uses prompt templates for scenario generation, image prompts, and fact verification
**When** templates are loaded
**Then** each template is read from a configurable filesystem path (default: `templates/` directory)
**And** templates use Go text/template syntax with named variables
**And** the template path for each purpose is configurable in YAML config

**Given** a prompt template file exists
**When** the system loads it for use
**Then** a SHA-256 hash of the template content is computed and stored as the template version
**And** the version hash is recorded in the output metadata (scenario output, image prompt metadata)
**And** any output can be traced back to the exact template version that produced it

**Given** the creator modifies a template
**When** the next pipeline operation uses that template
**Then** the new version hash is computed and recorded
**And** previous outputs retain their original version hash (immutable)
**And** a log entry records the template version change

**Given** the creator wants to reproduce a previous result
**When** they check the output metadata for a scene or scenario
**Then** the metadata contains: template path, template version hash, input variables used, and LLM model identifier
**And** using the same template version + inputs + model should produce similar results

**Given** a template file is missing or has syntax errors
**When** the system attempts to load it
**Then** a clear error message identifies: which template, expected path, and the parse error (if syntax)
**And** the operation fails fast before making any API calls

### Story 6.3: Quality Feedback & Pipeline Metrics

As a creator,
I want to record satisfaction ratings on generated outputs and view pipeline success metrics,
So that I can track quality trends and identify which scenes/prompts need improvement.

**Acceptance Criteria:**

**Given** a generated output exists (scenario, image, TTS audio)
**When** the creator runs `yt-pipe feedback <scp-id> --scene 3 --type image --rating good`
**Then** the feedback is stored in the feedback table in SQLite with: project ID, scene number, asset type, rating (good/bad), optional comment (--comment "too dark"), timestamp
**And** this satisfies FR43

**Given** the creator provides negative feedback
**When** `--rating bad` is specified
**Then** the system prompts for or accepts an optional `--comment` describing the issue
**And** the feedback record links to the specific asset version (hash) that was rated
**And** this enables tracking whether regeneration improved quality

**Given** feedback data has accumulated over multiple projects
**When** `yt-pipe metrics` is executed
**Then** the output displays: total projects, average scenes per project, success rate (completed / total runs), average pipeline duration, feedback summary (good/bad counts by asset type)
**And** `--format json` outputs machine-readable JSON
**And** this satisfies FR44

**Given** the creator wants per-project metrics
**When** `yt-pipe metrics <scp-id>` is executed
**Then** the output displays: project-specific stats including run count, scene count, regeneration count per asset type, feedback ratings, total estimated cost, and time from start to completion

### Story 6.4: Project Cleanup & Disk Management

As a creator,
I want to clean up intermediate artifacts from completed projects and monitor disk usage,
So that I can manage storage efficiently without losing final outputs.

**Acceptance Criteria:**

**Given** a project is in `complete` state
**When** `yt-pipe clean <scp-id>` is executed
**Then** intermediate artifacts are removed: individual scene working files (raw prompts, intermediate timing data, backup files)
**And** final outputs are preserved: CapCut project files, combined subtitle file, description.txt, scenario markdown, final images, final audio
**And** the cleanup operation logs which files were removed and disk space recovered
**And** this satisfies NFR18

**Given** the creator wants to see disk usage
**When** `yt-pipe clean --status` is executed
**Then** the output displays per-project: project ID, state, total disk usage, intermediate artifacts size, final output size
**And** a summary shows total disk usage across all projects

**Given** the creator wants to force-delete all project data
**When** `yt-pipe clean <scp-id> --all` is executed with confirmation prompt
**Then** the entire project directory is removed from the workspace
**And** the project record in SQLite is marked as `archived` (not deleted, for history)
**And** the creator must type the SCP ID to confirm (safety measure)

**Given** the creator wants a dry-run before cleanup
**When** `yt-pipe clean <scp-id> --dry-run` is executed
**Then** the system lists all files that would be removed and space that would be recovered
**And** no files are actually deleted

## Epic 7: REST API & External Integration

Creator can control the entire pipeline through a REST API in addition to the CLI, enabling future web UI integration and external system automation.

### Story 7.1: REST API Server & Health Endpoint

As a creator,
I want an HTTP server with health and readiness endpoints that shares the same service layer as the CLI,
So that I can integrate the pipeline with external tools and monitor server availability.

**Acceptance Criteria:**

**Given** the REST API server is configured
**When** `yt-pipe serve` is executed
**Then** an HTTP server starts on the configured port (default 8080, configurable via YAML and `--port` flag)
**And** the server uses Chi router with structured logging middleware (request ID, method, path, status, duration)
**And** the server reuses the same service layer instances as the CLI (no code duplication)
**And** this satisfies FR30

**Given** the server is running
**When** `GET /health` is called
**Then** a 200 response is returned with `{"status": "ok", "version": "<build-version>"}`
**And** response time is under 50ms

**Given** the server is running
**When** `GET /ready` is called
**Then** the system checks SQLite connectivity and workspace directory accessibility
**And** returns 200 with `{"ready": true}` if all checks pass
**And** returns 503 with `{"ready": false, "checks": {...}}` if any check fails

**Given** the server receives a request
**When** the request is processed
**Then** a unique request ID is generated and included in all log entries and the response header (`X-Request-ID`)
**And** all responses use `Content-Type: application/json`

**Given** the server is running and receives SIGTERM/SIGINT
**When** the shutdown signal is received
**Then** the server performs graceful shutdown: stops accepting new requests, waits for in-flight requests (max 30s), then exits
**And** a shutdown log entry is emitted

### Story 7.2: Project CRUD API

As a creator,
I want REST endpoints to create, retrieve, list, and delete projects,
So that I can manage projects programmatically without the CLI.

**Acceptance Criteria:**

**Given** the API server is running
**When** `POST /api/v1/projects` is called with `{"scp_id": "SCP-173"}`
**Then** a new project is created (same logic as `yt-pipe create`)
**And** the response is 201 with the full project JSON (id, scp_id, state, created_at, workspace_path)
**And** this satisfies FR31

**Given** a project exists
**When** `GET /api/v1/projects/:id` is called
**Then** the response is 200 with the full project JSON including current state, scene count, and asset status summary
**And** this satisfies FR32

**Given** multiple projects exist
**When** `GET /api/v1/projects` is called
**Then** the response is 200 with an array of project summaries
**And** query parameters support filtering: `?state=approved`, `?scp_id=SCP-173`
**And** results are paginated with `?limit=20&offset=0` (default limit 20)

**Given** a project exists in `complete` or `pending` state
**When** `DELETE /api/v1/projects/:id` is called
**Then** the project is archived (same as `yt-pipe clean --all`)
**And** the response is 200 with confirmation

**Given** a project does not exist
**When** any project-specific endpoint is called
**Then** the response is 404 with `{"error": "project not found", "project_id": "..."}`

**Given** invalid input is provided
**When** a request fails validation
**Then** the response is 400 with `{"error": "validation error", "details": [...]}`
**And** details include field-level error messages

### Story 7.3: Pipeline Control API

As a creator,
I want REST endpoints to trigger pipeline execution, query real-time status, and cancel running pipelines,
So that I can automate and monitor pipeline runs from external systems.

**Acceptance Criteria:**

**Given** a project exists with valid state for pipeline execution
**When** `POST /api/v1/projects/:id/run` is called
**Then** the pipeline starts executing asynchronously in a background goroutine
**And** the response is 202 with `{"status": "started", "project_id": "...", "run_id": "..."}`
**And** the pipeline uses the same orchestrator as `yt-pipe run`
**And** this satisfies FR33

**Given** a pipeline is running for a project
**When** `GET /api/v1/projects/:id/status` is called
**Then** the response is 200 with real-time status: current stage, progress percentage, scenes completed, elapsed time, per-scene asset status
**And** this satisfies FR34

**Given** a pipeline is running
**When** `POST /api/v1/projects/:id/cancel` is called
**Then** the pipeline's context is cancelled
**And** the current stage checkpoints progress before stopping
**And** the response is 200 with `{"status": "cancelled", "checkpoint": "..."}`

**Given** `POST /api/v1/projects/:id/run` is called for a project already running
**When** the server checks for an active run
**Then** the response is 409 with `{"error": "pipeline already running", "run_id": "..."}`

**Given** the pipeline requires scenario approval (human-in-the-loop)
**When** the pipeline reaches the approval gate
**Then** the status response includes `{"awaiting_action": "scenario_approval"}`
**And** `POST /api/v1/projects/:id/approve` triggers scenario approval and resumes the pipeline

### Story 7.4: Asset Management API

As a creator,
I want REST endpoints to regenerate specific assets and submit quality feedback,
So that I can fine-tune outputs without using the CLI.

**Acceptance Criteria:**

**Given** a project has generated assets
**When** `POST /api/v1/projects/:id/images/generate` is called with `{"scenes": [3, 5, 7]}`
**Then** only the specified scenes' images are regenerated
**And** the response is 202 with the regeneration job status
**And** this satisfies FR35

**Given** a project has generated assets
**When** `POST /api/v1/projects/:id/tts/generate` is called with `{"scenes": [5]}`
**Then** only the specified scene's TTS is re-synthesized
**And** downstream artifacts (timing, subtitles) are marked as stale
**And** this satisfies FR36

**Given** a creator wants to modify a scene's image prompt
**When** `PUT /api/v1/projects/:id/scenes/:num/prompt` is called with `{"prompt": "updated prompt text"}`
**Then** the scene's prompt file is updated
**And** the scene's image is marked as stale in the manifest
**And** the response is 200 with the updated scene metadata

**Given** a creator wants to submit feedback
**When** `POST /api/v1/projects/:id/feedback` is called with `{"scene": 3, "type": "image", "rating": "good", "comment": "perfect atmosphere"}`
**Then** the feedback is stored in SQLite (same as `yt-pipe feedback`)
**And** the response is 201 with the feedback record
**And** this satisfies FR37

**Given** asset regeneration is requested for a scene that doesn't exist
**When** the request is processed
**Then** the response is 400 with `{"error": "invalid scene number", "valid_range": "1-N"}`

### Story 7.5: Configuration & Plugin Management API

As a creator,
I want REST endpoints to view and modify configuration settings and manage plugins,
So that I can dynamically adjust pipeline behavior without editing config files.

**Acceptance Criteria:**

**Given** the API server is running
**When** `GET /api/v1/config` is called
**Then** the response is 200 with the current configuration as JSON (with sensitive values like API keys masked)
**And** this satisfies FR38

**Given** the creator wants to change a setting
**When** `PATCH /api/v1/config` is called with `{"scenario": {"fact_coverage_threshold": 90}}`
**Then** the specified setting is updated in the runtime configuration
**And** the response is 200 with the updated configuration
**And** changes persist to the YAML config file
**And** this satisfies FR39

**Given** the creator wants to see available plugins
**When** `GET /api/v1/plugins` is called
**Then** the response is 200 with an array of registered plugins: name, type (LLM/TTS/ImageGen/OutputAssembler), status (active/available), configuration
**And** this satisfies FR40

**Given** the creator wants to switch the active plugin for a type
**When** `PUT /api/v1/plugins/:type/active` is called with `{"plugin": "edge-tts"}`
**Then** the active plugin for that type is switched
**And** the response is 200 with confirmation and the new active plugin details
**And** subsequent pipeline operations use the new plugin

**Given** an invalid plugin name is specified
**When** the switch request is processed
**Then** the response is 400 with `{"error": "unknown plugin", "available": ["openai-tts", "edge-tts"]}`

**Given** a config change would make the system invalid (e.g., removing required fields)
**When** the PATCH request is processed
**Then** validation runs before applying changes
**And** the response is 400 with `{"error": "validation failed", "details": [...]}`
**And** no changes are applied

### Story 7.6: Webhook Notifications

As an automation system (e.g., n8n),
I want to receive webhook notifications when pipeline state changes occur,
So that I can trigger downstream workflows without polling.

**Acceptance Criteria:**

**Given** a webhook URL is configured in YAML (`webhooks.urls: ["https://n8n.local/webhook/yt-pipe"]`)
**When** a project state transition occurs (e.g., pending -> scenario_review, approved -> generating_assets, assembling -> complete)
**Then** an HTTP POST is sent to each configured webhook URL with payload: `{"event": "state_change", "project_id": "...", "scp_id": "...", "previous_state": "...", "new_state": "...", "timestamp": "..."}`
**And** this satisfies FR30

**Given** a webhook delivery fails (network error, non-2xx response)
**When** the retry policy applies
**Then** up to 3 retries are attempted with exponential backoff (1s, 2s, 4s)
**And** all delivery attempts are logged with status code and response time
**And** webhook failures do not block pipeline execution

**Given** multiple webhook URLs are configured
**When** a state change occurs
**Then** all URLs receive the notification independently (fan-out)
**And** failure of one URL does not affect delivery to others

**Given** no webhook URLs are configured
**When** a state change occurs
**Then** no webhook delivery is attempted and no errors are logged

### Story 7.7: API Authentication Middleware

As a creator,
I want the REST API protected by API key authentication,
So that only authorized clients can control the pipeline.

**Acceptance Criteria:**

**Given** the API server is configured with authentication enabled (`api.auth.enabled: true`)
**When** a request is made to any `/api/v1/*` endpoint without an `Authorization` header
**Then** the response is 401 with `{"error": "authentication required"}`
**And** this satisfies FR38

**Given** an API key is configured (`api.auth.key` in YAML or `YTP_API_KEY` environment variable)
**When** a request includes `Authorization: Bearer <valid-key>`
**Then** the request is processed normally
**And** the authenticated request is logged (without the key value, only key prefix for identification)

**Given** an invalid API key is provided
**When** the authentication middleware checks the key
**Then** the response is 401 with `{"error": "invalid API key"}`
**And** the failed attempt is logged with client IP and timestamp for security auditing
**And** this satisfies NFR17

**Given** health and readiness endpoints (`/health`, `/ready`)
**When** requests are made without authentication
**Then** these endpoints are accessible without API key (excluded from auth middleware)
**And** this allows load balancers and monitoring to function without credentials

**Given** API key authentication is disabled (`api.auth.enabled: false`)
**When** requests are made without authentication
**Then** all endpoints are accessible without API key
**And** a startup warning is logged: "API authentication is disabled"
**And** this satisfies NFR24 (safe for localhost binding without auth)

## Epic 8: LLM Plugin — Gemini Scenario Generation

Creator can generate complete video scenarios from SCP data using Gemini LLM with the proven 4-stage pipeline (Research→Structure→Writing→Review). Includes LLM fallback chain as a separate enhancement and minimal E2E smoke test for early user value.

### Story 8.1: Gemini LLM Provider Implementation

As a creator,
I want the system to connect to Gemini's OpenAI-compatible API as the LLM provider,
So that I can use Gemini for high-quality scenario generation.

**Acceptance Criteria:**

**Given** a Gemini API key is configured in `llm.api_key` and endpoint set to `generativelanguage.googleapis.com/v1beta/openai/`
**When** the LLM plugin is initialized
**Then** an `OpenAICompatibleProvider` struct in `plugin/llm/openai.go` is instantiated with Gemini-specific config (endpoint, API key, model)
**And** no separate `GeminiProvider` struct exists — all OpenAI-compatible providers (Gemini, Qwen, DeepSeek) reuse the same `OpenAICompatibleProvider` with different config injection
**And** the provider instance is registered in the plugin registry under the name `"gemini"`

**Given** a `GenerateScenario` call is made
**When** the Gemini API returns a successful response
**Then** the response is parsed into `domain.ScenarioOutput` with scenes, narration text, and fact tags
**And** token usage (input/output) is logged at INFO level for cost tracking

**Given** a Gemini API call fails with a retryable error (429, 500, 503)
**When** the retry mechanism activates
**Then** the call is retried with exponential backoff (max 3 retries) using the existing `retry` package
**And** non-retryable errors (400, 401, 403) are returned immediately with descriptive error messages

**Given** the API key is invalid or missing
**When** any LLM method is called
**Then** the error message includes "Gemini API authentication failed" with guidance to check `llm.api_key` config
**And** this satisfies FR4

### Story 8.2: 4-Stage Scenario Prompt Template System

As a creator,
I want the scenario generation to follow a proven 4-stage prompt pipeline (Research→Structure→Writing→Review),
So that generated scenarios are consistently high-quality with proper SCP accuracy and narrative flow.

**Acceptance Criteria:**

**Given** prompt templates are stored in `templates/scenario/` directory
**When** the template system is initialized
**Then** 4 template files are loaded: `01_research.md`, `02_structure.md`, `03_writing.md`, `04_review.md`
**And** templates are managed by the existing `internal/template` package with version tracking
**And** each template supports variable substitution: `{scp_fact_sheet}`, `{topic}`, `{context}`, `{research_packet}`, `{scp_visual_reference}`, `{target_duration}`, `{scene_structure}`, `{narration_script}`

**Given** the Research template (Stage 1) is executed
**When** SCP facts.json and main.txt are injected as `{scp_fact_sheet}`
**Then** the LLM produces a structured research packet with: Core Identity Summary, Visual Identity Profile (Frozen Descriptor), Key Dramatic Beats, Environment & Atmosphere Notes, and Narrative Hooks
**And** the Visual Identity Profile follows the exact format from video.pipeline: Silhouette & Build, Head/Face, Body Covering, Hands & Limbs, Carried Items, Organic Integration Note

**Given** the Structure template (Stage 2) is executed
**When** the research packet from Stage 1 is injected
**Then** the LLM produces a scene structure following the 4-act format: Hook & Introduction (~15%), Properties & Background (~30%), Incidents & Evidence (~40%), Resolution & Mystery (~15%)
**And** each scene includes `key_points` that reference the Visual Identity Profile verbatim (Frozen Descriptor Protocol)

**Given** the Writing template (Stage 3) is executed
**When** the scene structure from Stage 2 is injected
**Then** the LLM produces Korean narration in documentary style with natural conjunctions (시간/순서, 대비/반전, 누적/추가)
**And** sentences do not exceed 20 characters for TTS readability
**And** the tone uses polite-formal register (~합니다) without excessive formality

**Given** the Review template (Stage 4) is executed
**When** the narration from Stage 3 is injected
**Then** the LLM performs fact-check validation: SCP class accuracy, anomalous properties accuracy, containment procedure correctness, Visual Identity consistency
**And** issues are returned as patch-based corrections (not full rewrites) with specific line references
**And** this satisfies FR5

### Story 8.3: SCP Data to Scenario Generation Pipeline

As a creator,
I want to run `yt-pipe scenario generate SCP-173` and get a complete scenario,
So that the full 4-stage pipeline executes automatically from raw SCP data to finished script.

**Acceptance Criteria:**

**Given** SCP data is loaded (facts.json, meta.json, main.txt) for a valid SCP ID
**When** `yt-pipe scenario generate <SCP-ID>` is executed
**Then** the system executes all 4 stages sequentially: Research → Structure → Writing → Review
**And** each stage's output is saved as an intermediate artifact in the project workspace (`{project}/stages/01_research.json`, `02_structure.json`, `03_writing.json`, `04_review.json`)
**And** the final reviewed narration is saved as `{project}/scenario.md`

**Given** Stage 1 (Research) completes
**When** Stage 2 (Structure) begins
**Then** the research packet output is automatically injected into the Structure template
**And** the Visual Identity Profile from Stage 1 is passed through as `{scp_visual_reference}` (Frozen Descriptor)
**And** the target duration is read from project config (default: 10 minutes)

**Given** Stage 4 (Review) identifies corrections
**When** corrections are applied
**Then** the system applies patch-based corrections to the narration automatically
**And** a diff summary is logged showing what was changed and why
**And** the final scenario includes fact tags linking narration segments to source facts

**Given** any stage fails during execution
**When** the error is caught
**Then** the system saves a checkpoint at the last completed stage
**And** re-running the command resumes from the failed stage (not from the beginning)
**And** the error message includes which stage failed and a suggested fix
**And** this satisfies FR4, FR5

### Story 8.4: Fact Coverage Verification

As a creator,
I want the system to verify that the generated scenario covers at least 80% of source facts,
So that I can be confident the video accurately represents the SCP content.

**Acceptance Criteria:**

**Given** a scenario has been generated with fact tags
**When** fact coverage verification runs (automatically after generation or via `yt-pipe scenario verify <SCP-ID>`)
**Then** the system compares tagged facts in the scenario against all facts in facts.json
**And** calculates a coverage percentage (tagged facts / total facts × 100)
**And** displays a coverage report: total facts, covered facts, missing facts list, coverage percentage

**Given** the coverage percentage meets or exceeds the threshold (default 80%, configurable via `scenario.fact_coverage_threshold`)
**When** the verification result is returned
**Then** the scenario status is set to `verified` in the project state
**And** a success message displays the coverage percentage

**Given** the coverage percentage is below the threshold
**When** the verification result is returned
**Then** the system lists specific missing facts with their categories (physical_description, anomalous_properties, containment_procedures, etc.)
**And** suggests which scene(s) could incorporate the missing facts
**And** the scenario status remains `draft` until coverage is met
**And** this satisfies FR6

**Given** the creator runs `yt-pipe scenario verify --detail <SCP-ID>`
**When** the detailed report is generated
**Then** each fact is listed with: fact text, covered/missing status, and if covered, the scene number and narration line where it appears

### Story 8.5: SCP Glossary-Aware Generation

As a creator,
I want the LLM to use consistent SCP terminology from the glossary during generation,
So that terms like object classes, containment levels, and SCP-specific jargon are used correctly and consistently.

**Acceptance Criteria:**

**Given** a glossary is loaded from the existing `internal/glossary` package
**When** the scenario generation pipeline starts
**Then** glossary terms relevant to the target SCP are injected into each prompt template as a terminology reference section
**And** the injected glossary includes: term, definition, preferred usage, and Korean translation where applicable

**Given** the LLM generates narration text
**When** the output is post-processed
**Then** the system validates that SCP object classes (Safe, Euclid, Keter, Thaumiel, Apollyon) are used correctly per the source data
**And** any misused terms are flagged in the review stage output

**Given** a new SCP entry uses domain-specific terms not in the default glossary
**When** the creator adds custom terms via `yt-pipe glossary add <term> <definition>`
**Then** the custom terms are included in subsequent generation prompts
**And** glossary data persists in the project's SQLite database
**And** this satisfies FR8

### Story 8.6: LLM Fallback Chain (Gemini → Qwen → DeepSeek)

As a creator,
I want the system to automatically try alternative LLM providers when Gemini is unavailable,
So that scenario generation continues without manual intervention during provider outages.

**Acceptance Criteria:**

**Given** fallback providers are configured in YAML: `llm.fallback: [{provider: "qwen", model: "qwen-max"}, {provider: "deepseek", model: "deepseek-chat"}]`
**When** the LLM plugin is initialized
**Then** a `FallbackChain` wrapper implements the `LLM` interface
**And** it holds an ordered list of providers: primary (Gemini) → fallback 1 (Qwen) → fallback 2 (DeepSeek)
**And** each fallback provider is registered in the plugin registry with its own API key and endpoint config

**Given** the primary Gemini provider fails with a non-retryable error (after exhausting retries)
**When** the fallback chain activates
**Then** the next provider in the chain is attempted with the same prompt and parameters
**And** a warning log is emitted: "Primary LLM failed, falling back to {provider_name}"
**And** the execution log records which provider was ultimately used

**Given** all providers in the chain fail
**When** the final fallback is exhausted
**Then** the error message lists all attempted providers with their individual error messages
**And** the pipeline halts with a clear "All LLM providers failed" error

**Given** Qwen and DeepSeek are also OpenAI-compatible endpoints
**When** their providers are initialized
**Then** they reuse the same `openai-compatible` client code as Gemini with different endpoint/key configs
**And** no code duplication exists between providers — only config differs

### Story 8.7: Minimal E2E Smoke Test

As a creator,
I want to run a single command that proves the entire scenario generation pipeline works end-to-end,
So that I can validate my setup is correct before processing all 422 SCP entries.

**Acceptance Criteria:**

**Given** the system is configured with a valid Gemini API key and SCP data path
**When** `yt-pipe scenario generate SCP-173 --verbose` is executed
**Then** the full 4-stage pipeline runs: Research → Structure → Writing → Review
**And** a `scenario.md` file is produced in the project workspace
**And** the CLI displays stage-by-stage progress with elapsed time per stage
**And** total execution time and token usage summary are displayed at completion

**Given** the smoke test completes successfully
**When** the output is inspected
**Then** the scenario contains: Korean narration text, scene breakdowns with key_points, fact tags referencing facts.json entries, and a coverage percentage ≥ 80%

**Given** a unit test file `service/scenario_test.go` exists (no build tag required)
**When** `go test ./internal/service/ -run TestScenarioPipeline` is executed
**Then** the test uses mock LLM provider (from `internal/mocks`) to validate the 4-stage pipeline orchestration logic: correct stage ordering, inter-stage data passing, checkpoint creation, and error handling
**And** no real API calls are made — real API integration tests are deferred to Story 12.4
**And** the test completes in under 5 seconds

## Epic 9: ImageGen Plugin — SiliconFlow FLUX Image Generation

Creator can generate per-scene images from scenario data using SiliconFlow FLUX API, with Frozen Descriptor Protocol for entity visual consistency and the 3-stage shot-breakdown-to-prompt pipeline from video.pipeline.

### Story 9.1: SiliconFlow FLUX Provider Implementation

As a creator,
I want the system to connect to SiliconFlow's FLUX API for image generation,
So that I can generate high-quality scene images from my scenarios.

**Acceptance Criteria:**

**Given** a SiliconFlow API key is configured in `imagegen.api_key` and endpoint configured
**When** the ImageGen plugin is initialized
**Then** a `SiliconFlowProvider` struct in `plugin/imagegen/siliconflow.go` implements the `ImageGen` interface
**And** the provider supports configurable model selection (default: `FLUX.1-schnell`)
**And** the provider is registered in the plugin registry under the name `"siliconflow"`

**Given** a `Generate` call is made with a prompt and `GenerateOptions`
**When** the SiliconFlow API returns a successful response
**Then** the image data is returned as `ImageResult` with format, width, and height populated
**And** the default output dimensions are 1920×1080 (16:9) matching CapCut canvas config
**And** API response time and image size are logged at INFO level

**Given** a SiliconFlow API call fails with a retryable error (429, 500, 503)
**When** the retry mechanism activates
**Then** the call is retried with exponential backoff (max 3 retries) using the existing `retry` package
**And** rate limit errors (429) respect the `Retry-After` header if present
**And** this satisfies FR10

### Story 9.2: Shot Breakdown Prompt Template System

As a creator,
I want each scene's narration to be decomposed into cinematographic shots using the proven shot breakdown system,
So that generated images have professional composition with correct camera angles and entity positioning.

**Acceptance Criteria:**

**Given** prompt templates are stored in `templates/image/` directory
**When** the image prompt template system is initialized
**Then** 2 template files are loaded: `01_shot_breakdown.md` (scene→shot decomposition) and `02_shot_to_prompt.md` (shot→image prompt)
**And** templates support variable substitution: `{entity_visual_identity}`, `{frozen_descriptor}`, `{scene_number}`, `{synopsis}`, `{emotional_beat}`, `{previous_last_shot_context}`, `{shot_json}`

**Given** the Shot Breakdown template (Stage 1) is executed for a scene
**When** the scene's synopsis, emotional beat, and entity Visual Identity Profile are injected
**Then** the LLM produces a JSON shot description with: shot_number, role, camera_type, entity_visible, subject, lighting, mood, motion
**And** camera_type is one of: wide, medium, close-up, extreme close-up, POV, over-the-shoulder, bird's eye, low angle
**And** if entity_visible is true, the subject field starts with the FROZEN DESCRIPTOR verbatim

**Given** the Shot-to-Prompt template (Stage 2) is executed
**When** the shot JSON from Stage 1 is injected
**Then** the LLM produces an image generation prompt with: main subject (preserving frozen descriptor), camera angle, lighting, mood/style, and technical quality suffix
**And** a negative prompt is generated including entity-specific negatives when entity is visible
**And** the common suffix "cinematic still, dark horror photography, highly detailed, 8k, sharp focus, volumetric lighting, film grain, 16:9 aspect ratio" is appended
**And** this satisfies FR9

### Story 9.3: Frozen Descriptor Protocol Implementation

As a creator,
I want entity visual descriptions to be locked and reused verbatim across all scene images,
So that the SCP entity looks consistent throughout the entire video.

**Acceptance Criteria:**

**Given** the Research stage (Epic 8, Stage 1) has produced a Visual Identity Profile
**When** the image generation pipeline starts
**Then** a Frozen Descriptor is extracted from the Visual Identity Profile: a single dense text block containing all physical attributes (silhouette, head/face, body covering, hands/limbs, carried items)
**And** the descriptor is stored in the project workspace as `{project}/frozen_descriptor.txt`

**Given** a Frozen Descriptor exists for the project
**When** any shot breakdown is generated (Story 9.2, Stage 1)
**Then** the descriptor is injected verbatim into the `{frozen_descriptor}` template variable
**And** the LLM is instructed to use it verbatim in the `subject` field when `entity_visible: true`
**And** the shot-to-prompt stage (Stage 2) preserves it exactly — no paraphrasing, abbreviation, or modification

**Given** a generated image prompt is post-processed
**When** entity_visible is true for the shot
**Then** the system validates the frozen descriptor in the final prompt using a 2-tier check: first strict verbatim match, then fuzzy similarity (≥95% threshold) if verbatim fails
**And** if verbatim match passes, no action needed; if fuzzy match passes, a warning is logged with the diff; if fuzzy match also fails, the prompt is auto-corrected by re-inserting the descriptor verbatim
**And** entity-specific negative prompts are generated to prevent visual inconsistency (e.g., "human face visible" if entity wears a mask)
**And** this satisfies FR11

### Story 9.4: Scene Image Generation Pipeline

As a creator,
I want to run `yt-pipe image generate <SCP-ID>` to generate images for all scenes in my scenario,
So that each scene has a matching visual that I can use in the final video.

**Acceptance Criteria:**

**Given** an approved scenario exists with scene breakdowns
**When** `yt-pipe image generate <SCP-ID>` is executed
**Then** for each scene: the 2-stage prompt pipeline runs (shot breakdown → image prompt), then the SiliconFlow API generates the image
**And** generated images are saved as `{project}/scenes/{scene_num}/image.png`
**And** the corresponding prompt and shot metadata are saved as `{project}/scenes/{scene_num}/image_prompt.json`
**And** CLI displays per-scene progress: "Scene 3/10: generating shot breakdown... generating image... saved (2.3s)"

**Given** multiple scenes need image generation
**When** the pipeline processes them
**Then** scenes are generated sequentially by default (to maintain narrative context for shot continuity)
**And** the `{previous_last_shot_context}` variable carries the previous scene's last shot for visual flow
**And** a `--parallel` flag enables concurrent generation (disabling shot continuity context) for speed

**Given** image generation fails for a specific scene
**When** the error is caught
**Then** the failed scene is logged with the error, and generation continues with remaining scenes
**And** a summary at the end lists: total scenes, succeeded, failed, with scene numbers for each
**And** re-running the command skips already-generated scenes (unless `--force` is specified)
**And** this satisfies FR10, FR12

### Story 9.5: Selective Scene Image Regeneration

As a creator,
I want to regenerate images for specific scenes without re-running the entire pipeline,
So that I can fix individual images that don't meet my quality standards.

**Acceptance Criteria:**

**Given** images have been generated for a project
**When** `yt-pipe image regenerate <SCP-ID> --scenes 3,5,7` is executed
**Then** only the specified scenes have their images regenerated through the full 2-stage prompt pipeline
**And** the previous images are backed up as `image.prev.png` before overwriting
**And** the new prompt metadata is saved alongside the new image

**Given** the creator wants to edit a prompt before regeneration
**When** `yt-pipe image regenerate <SCP-ID> --scene 3 --edit-prompt` is executed
**Then** the current image prompt JSON for scene 3 is displayed
**And** the creator can provide a modified prompt or instruction (e.g., "make the lighting warmer")
**And** the LLM re-generates only the image prompt incorporating the edit, then generates a new image
**And** this satisfies FR11, FR12

**Given** a regenerated image uses a different seed or prompt
**When** the image is saved
**Then** the image_prompt.json records the generation history: original prompt, edit instruction (if any), and seed used
**And** the project execution log records the regeneration event with before/after metadata

## Epic 10: TTS Plugin — DashScope CosyVoice Narration

Creator can generate narration audio from scenario scripts using DashScope CosyVoice TTS, with Korean Hangul pronunciation XML conversion. Voice Cloning supported via config-level VoiceID option, Flash model for cost optimization.

### Story 10.1: DashScope CosyVoice Provider Implementation

As a creator,
I want the system to connect to DashScope's CosyVoice API for TTS narration,
So that I can generate professional Korean narration audio from my scenarios.

**Acceptance Criteria:**

**Given** a DashScope API key is configured in `tts.api_key` and model set (default: `cosyvoice-v1`)
**When** the TTS plugin is initialized
**Then** a `DashScopeProvider` struct in `plugin/tts/dashscope.go` implements the `TTS` interface
**And** the provider uses REST mode only in this story (WebSocket streaming is a future optimization, not in scope)
**And** the provider is registered in the plugin registry under the name `"dashscope"`

**Given** a `Synthesize` call is made with Korean text and a voice ID
**When** the DashScope API returns a successful response
**Then** the audio data is returned as `SynthesisResult` with PCM/MP3 audio bytes, word-level timings, and total duration
**And** word timings are parsed from the DashScope response for subtitle alignment
**And** the audio is saved in the configured format (default: MP3, configurable via `tts.format`)

**Given** DashScope uses a non-OpenAI-compatible API format
**When** the provider constructs API requests
**Then** it uses DashScope's native REST endpoint (`POST /api/v1/services/aigc/text2audio/generation`) with the correct header format (`Authorization: Bearer {api_key}`)
**And** API errors are mapped to standard Go errors with descriptive messages

**Given** a DashScope API call fails with a retryable error
**When** the retry mechanism activates
**Then** the call is retried with exponential backoff (max 3 retries) using the existing `retry` package
**And** this satisfies FR13

### Story 10.2: Korean Hangul Pronunciation XML Conversion

As a creator,
I want English terms and numbers in the narration to be converted to Korean pronunciation before TTS,
So that the narration sounds natural without awkward English pronunciation breaks.

**Acceptance Criteria:**

**Given** a scenario narration contains English terms (e.g., "SCP-173", "Keter", "Level 1")
**When** the TTS preprocessing stage runs
**Then** a 2-tier conversion executes: first, deterministic glossary-based substitution replaces known terms ("SCP" → "에스씨피", "Keter" → "케테르", "API" → "에이피아이"); then, the LLM handles remaining context-dependent conversions (numbers, novel English terms) using the `scenario_refine` prompt template
**And** glossary substitutions are applied before LLM invocation to reduce token cost
**And** numbers are converted contextually by the LLM: "2시" → "두 시", "2025년" → "이천이십오 년", "3개" → "세 개"

**Given** the conversion prompt template is stored in `templates/tts/scenario_refine.md` (Korean-only, no multi-language support in this phase)
**When** the template is applied
**Then** the output is valid XML in `<script>` format with speaker tags on separate lines
**And** the meaning and structure of the original narration are preserved exactly — no summarization or content changes
**And** the converted text is saved as `{project}/scenes/{scene_num}/narration_refined.xml` for audit

**Given** the glossary contains SCP-specific pronunciation overrides
**When** `SynthesizeWithOverrides` is called
**Then** glossary overrides take precedence over LLM-generated pronunciations
**And** this satisfies FR14

### Story 10.3: Voice Cloning via Config-Level VoiceID

As a creator,
I want to use a cloned voice for narration by specifying a VoiceID in config,
So that my videos have a distinctive, consistent narrator voice without changing any code.

**Acceptance Criteria:**

**Given** DashScope Voice Cloning requires a 2-step process: register voice → receive VoiceID
**When** the creator has already registered a voice via DashScope console or API
**Then** they configure the VoiceID in YAML: `tts.voice: "cosyvoice-clone-{voice_id}"`
**And** no interface change is needed — the existing `voice` parameter in `Synthesize(ctx, text, voice)` carries the VoiceID

**Given** `tts.voice` is set to a clone VoiceID (prefix: `cosyvoice-clone-`)
**When** the DashScope provider constructs the API request
**Then** it uses the VoiceID in the `voice` parameter of the DashScope request body
**And** the request includes `"voice_clone": true` in the API payload if the voice ID indicates a cloned voice

**Given** `tts.voice` is set to a standard preset voice (e.g., `longxiaochun`)
**When** the DashScope provider constructs the API request
**Then** it uses the standard voice without the clone flag
**And** no additional configuration is required
**And** this satisfies FR15

**Given** the creator wants to register a new voice clone
**When** `yt-pipe tts register-voice --audio <sample.wav> --name "my-narrator"` is executed
**Then** the system calls DashScope's voice registration API with the audio sample
**And** the returned VoiceID is displayed and optionally written to config: `tts.voice: "cosyvoice-clone-{returned_id}"`

### Story 10.4: CosyVoice Flash Model & Subtitle Generation

As a creator,
I want to optionally use the Flash model for faster/cheaper TTS and auto-generate subtitles from word timings,
So that I can balance cost vs quality and have synchronized subtitles for my videos.

**Acceptance Criteria:**

**Given** `tts.model` is set to `cosyvoice-v1-flash` in config
**When** the DashScope provider is initialized
**Then** it uses the Flash model endpoint for all synthesis calls
**And** the Flash model is documented as faster but potentially lower quality than the standard model
**And** switching between standard and Flash requires only a config change, no code modification

**Given** a `Synthesize` call returns word-level timings in `SynthesisResult.WordTimings`
**When** subtitle generation runs
**Then** the system produces an SRT subtitle file at `{project}/scenes/{scene_num}/subtitle.srt`
**And** each subtitle entry aligns with word timing boundaries from the TTS response
**And** subtitle segments are grouped into readable chunks (max 2 lines, max 40 characters per line)

**Given** word timings from DashScope have millisecond precision
**When** the subtitles are generated
**Then** timing format follows SRT standard: `HH:MM:SS,mmm --> HH:MM:SS,mmm`
**And** a project-wide subtitle file `{project}/subtitles.srt` concatenates all scene subtitles with correct time offsets
**And** this satisfies FR16

### Story 10.5: Narration Generation Pipeline

As a creator,
I want to run `yt-pipe tts generate <SCP-ID>` to generate narration audio for all scenes,
So that each scene has a matching audio file ready for CapCut assembly.

**Acceptance Criteria:**

**Given** an approved scenario exists with scene narration text
**When** `yt-pipe tts generate <SCP-ID>` is executed
**Then** for each scene: Korean pronunciation conversion runs, then DashScope TTS synthesizes the audio
**And** generated audio is saved as `{project}/scenes/{scene_num}/narration.mp3`
**And** word timings are saved as `{project}/scenes/{scene_num}/timing.json`
**And** subtitles are saved as `{project}/scenes/{scene_num}/subtitle.srt`
**And** CLI displays per-scene progress: "Scene 3/10: converting pronunciation... synthesizing... saved (1.8s, 45.2s audio)"

**Given** multiple scenes need narration generation
**When** the pipeline processes them
**Then** scenes are generated sequentially to respect API rate limits
**And** total audio duration and estimated API cost are displayed at completion

**Given** narration generation fails for a specific scene
**When** the error is caught
**Then** the failed scene is logged with the error, and generation continues with remaining scenes
**And** re-running the command skips already-generated scenes (unless `--force` is specified)
**And** a `--scenes 3,5` flag enables regenerating specific scenes only (same as image regeneration UX)
**And** this satisfies FR13, FR15

## Epic 11: Output Plugin — CapCut Project Assembly

Creator can assemble all generated assets (images, narration audio, subtitles) into a CapCut-compatible project with timing synchronization and CC-BY-SA 3.0 copyright metadata. Builds on the CapCut format validated in Epic 4 with concrete asset integration from Epics 8-10.

### Story 11.1: CapCut Assembler Concrete Implementation

As a creator,
I want the CapCut assembler to work with real generated assets from the scenario, image, and TTS pipelines,
So that I can open a fully assembled project in CapCut immediately after pipeline completion.

**Acceptance Criteria:**

**Given** a project has completed scenario generation (Epic 8), image generation (Epic 9), and TTS generation (Epic 10)
**When** `yt-pipe assemble <SCP-ID>` is executed
**Then** the `Assembler` implementation in `plugin/output/capcut/assembler.go` loads all scene assets from the project workspace
**And** for each scene, it reads: `scenes/{num}/image.png`, `scenes/{num}/narration.mp3`, `scenes/{num}/timing.json`, `scenes/{num}/subtitle.srt`
**And** the CapCut draft JSON is generated with correct material references for each asset file

**Given** assets are loaded for assembly
**When** the CapCut project structure is built
**Then** the video track contains one segment per scene with the image file as the material
**And** the audio track contains one segment per scene with the narration MP3 as the material
**And** the text track contains subtitle segments from the SRT file mapped to CapCut text segment format
**And** all track segments are ordered sequentially matching the scene order

**Given** the assembly completes
**When** the output files are saved
**Then** `draft_content.json` and `draft_meta_info.json` are written to `{project}/output/`
**And** all referenced asset files are copied or symlinked into the CapCut project's resource directory
**And** the output validates against the CapCut schema from Story 4.1
**And** this satisfies FR17

**Given** a scene has missing assets (e.g., image generation failed for scene 3 in Epic 9)
**When** the assembler loads scene assets
**Then** a pre-assembly validation runs and lists all scenes with missing assets: which files are missing per scene (image/narration/timing/subtitle)
**And** assembly halts with a clear error: "Cannot assemble: scenes 3, 7 have missing assets. Run `yt-pipe status <SCP-ID> --scenes` to see details."
**And** the creator can fix the missing assets and re-run assembly without affecting completed scenes

### Story 11.2: Timing Resolver — Narration-Driven Scene Synchronization

As a creator,
I want scene images to be displayed for exactly the duration of their narration audio,
So that visuals and audio are perfectly synchronized in the final video.

**Acceptance Criteria:**

**Given** each scene has `timing.json` with word-level timings and total audio duration
**When** the Timing Resolver processes all scenes
**Then** each scene's image display duration is set to match its narration audio duration exactly
**And** scene transitions are placed at the boundary between consecutive narration segments (no overlap, no gap)
**And** the total video duration equals the sum of all scene narration durations

**Given** subtitle segments exist for each scene
**When** timing is resolved
**Then** subtitle start/end times are offset by the cumulative duration of all preceding scenes
**And** the project-level `timeline.json` contains: per-scene start time, end time, image path, audio path, and subtitle segments with absolute timestamps

**Given** a scene has no narration (e.g., title card or transition)
**When** the Timing Resolver encounters it
**Then** a default duration is used (configurable via `assembly.default_scene_duration`, default: 3 seconds)
**And** the scene is still included in the timeline with its image but no audio track segment

**Given** timing data is recalculated after a scene's TTS is regenerated
**When** `yt-pipe assemble <SCP-ID>` is re-run
**Then** only the affected scene's timing is updated and all subsequent scene offsets are recalculated
**And** this satisfies FR18

### Story 11.3: Copyright & Licensing Metadata Integration

As a creator,
I want CC-BY-SA 3.0 copyright attribution and SCP-specific licensing warnings included automatically,
So that every assembled project is legally compliant without manual effort.

**Acceptance Criteria:**

**Given** a project is assembled for an SCP entry
**When** the assembler runs
**Then** a `description.txt` is generated in `{project}/output/` containing: SCP Foundation credit, original author(s) from `meta.json`, CC-BY-SA 3.0 license URL, and AI-generated content disclosure
**And** this text is formatted ready for YouTube video description paste

**Given** `meta.json` contains additional copyright conditions (e.g., image usage restrictions, derivative work conditions)
**When** the assembler processes the metadata
**Then** a CLI warning is displayed: "⚠ SCP-XXX has additional copyright conditions: {details}"
**And** the conditions are appended to `description.txt` with clear labeling
**And** the warning is recorded in the execution log
**And** this satisfies FR19

**Given** the creator runs `yt-pipe assemble <SCP-ID> --check-license`
**When** the license check runs
**Then** the system validates that all required attribution fields are present in meta.json
**And** missing fields are reported as warnings (assembly still proceeds)
**And** the license check result is included in the assembly summary output

## Epic 12: End-to-End Pipeline Integration

Creator can run the complete pipeline from SCP data to CapCut project in a single command with real providers (Gemini, SiliconFlow, DashScope), checkpoint/resume across all stages, real-time progress display, and comprehensive integration test suite.

### Story 12.1: Full Pipeline with Real Providers

As a creator,
I want `yt-pipe run <SCP-ID>` to execute the complete pipeline using real API providers end-to-end,
So that I get a finished CapCut project from a single command with no manual steps between stages.

**Acceptance Criteria:**

**Given** configuration has valid API keys for Gemini (LLM), SiliconFlow (ImageGen), and DashScope (TTS)
**When** `yt-pipe run <SCP-ID>` is executed
**Then** the pipeline orchestrator executes all stages in sequence with the concrete providers:
1. SCP data loading and validation
2. Scenario generation (4-stage Gemini pipeline)
3. Pause for scenario approval (`yt-pipe scenario approve <SCP-ID>` to resume)
4. Image generation (SiliconFlow FLUX) + TTS generation (DashScope CosyVoice) in parallel
5. CapCut project assembly with timing synchronization
**And** each stage uses the provider registered in the plugin registry based on config
**And** this satisfies FR20

**Given** the pipeline completes all stages
**When** the final output is produced
**Then** the `{project}/output/` directory contains: `draft_content.json`, `draft_meta_info.json`, `description.txt`, and all referenced assets
**And** CLI displays a completion summary: total time, per-stage time breakdown, total API calls, estimated cost (tokens + images + audio duration)

**Given** the creator wants to skip the approval pause for batch processing
**When** `yt-pipe run <SCP-ID> --auto-approve` is executed
**Then** the scenario approval step is skipped and the pipeline continues immediately after generation
**And** a warning is logged: "Auto-approve enabled: scenario review skipped"

### Story 12.2: Cross-Stage Checkpoint & Resume

As a creator,
I want to resume the pipeline from the exact stage that failed when using real providers,
So that I don't waste API calls and time re-running completed stages after fixing a config issue.

**Acceptance Criteria:**

**Given** the pipeline fails at image generation (e.g., SiliconFlow API key expired)
**When** the creator fixes the API key and runs `yt-pipe run <SCP-ID>` again
**Then** the orchestrator detects completed checkpoints: data loading (done), scenario generation (done), scenario approved (done)
**And** resumes from image generation stage without re-running prior stages
**And** CLI displays: "Resuming from stage: image_generation (3 stages already completed)"

**Given** image generation partially completed (scenes 1-5 done, scene 6 failed)
**When** the pipeline resumes
**Then** scenes 1-5 are skipped (already have artifacts), scene 6 onwards are processed
**And** TTS generation resumes from its own independent per-scene checkpoint (image and TTS maintain separate checkpoint state since they run in parallel)
**And** a failure in image generation does not affect TTS checkpoint progress, and vice versa
**And** this satisfies FR32

**Given** the creator wants to force a full re-run from scratch
**When** `yt-pipe run <SCP-ID> --force` is executed
**Then** all checkpoints are cleared and the pipeline starts from stage 1
**And** existing artifacts are backed up to `{project}/backup/{timestamp}/` before overwriting

### Story 12.3: Multi-Stage Pipeline Progress Dashboard

As a creator,
I want real-time progress visibility across **multiple parallel stages** when running the full pipeline,
So that I can monitor long-running batch jobs and estimate remaining time.

**Scope distinction from Story 5.2:** Story 5.2 provides single-stage progress (one active stage at a time). This story extends it to show **concurrent multi-stage progress** (image + TTS running in parallel) with a unified dashboard view.

**Acceptance Criteria:**

**Given** the full pipeline is running via `yt-pipe run <SCP-ID>`
**When** stages are executing
**Then** the CLI displays a live progress view on stderr:
```
[scenario]  ████████████████████ 100% (4/4 stages, 45s)
[image]     ████████░░░░░░░░░░░░  40% (4/10 scenes, 32s)
[tts]       ██████░░░░░░░░░░░░░░  30% (3/10 scenes, 28s)
[assembly]  waiting...
```
**And** each line updates in-place (using `\r` or ANSI escape codes)
**And** the display degrades gracefully to simple line-by-line output when stderr is not a TTY (e.g., piped to file)

**Given** parallel stages (image + TTS) are running
**When** one stage completes before the other
**Then** the completed stage shows 100% and the remaining stage continues updating
**And** assembly begins only when both parallel stages show 100%

**Given** the creator queries status from another terminal
**When** `yt-pipe status <SCP-ID>` is executed during a run
**Then** the response includes: current running stages, per-stage progress, elapsed time, and estimated remaining time based on average scene processing speed

### Story 12.4: Integration Test Suite

As a creator,
I want a comprehensive integration test suite that validates the full pipeline with real APIs,
So that regressions are caught before they affect my production workflow.

**Acceptance Criteria:**

**Given** integration test files exist with `//go:build integration` build tag
**When** `go test -tags=integration ./...` is executed
**Then** the following test cases run against real APIs:
- `TestGeminiScenarioGeneration`: Generates scenario for SCP-173 fixture, validates 4-stage output structure
- `TestSiliconFlowImageGeneration`: Generates one image from a test prompt, validates ImageResult fields
- `TestDashScopeTTSSynthesis`: Synthesizes one Korean sentence, validates audio bytes and word timings
- `TestFullPipelineE2E`: Runs the complete pipeline for SCP-173 with all real providers, validates final output
- `TestFallbackChainActivation`: Configures primary LLM with an invalid key to force fallback, validates that the fallback provider (Qwen or DeepSeek) handles the request successfully
**And** all tests are skipped in regular `go test ./...` (no build tag)

**Given** integration tests require API keys
**When** keys are not configured
**Then** each test skips with `t.Skip("GEMINI_API_KEY not set")` (or equivalent for each provider)
**And** the test output clearly indicates which tests were skipped and why

**Given** integration tests run against real APIs with latency
**When** individual test timeouts are set
**Then** scenario generation test: 120s timeout, image generation test: 60s timeout, TTS test: 30s timeout, full E2E test: 300s timeout
**And** each test cleans up its project workspace after completion (unless `--keep-artifacts` flag is set)

**Given** a CI environment is configured with API keys
**When** the integration test suite runs
**Then** test results are output in standard Go test format compatible with CI reporting
**And** a `Makefile` target `make test-integration` wraps the command: `go test -tags=integration -timeout 600s ./...`
**And** a separate `make test` target runs only unit tests (no build tag, no API calls)
