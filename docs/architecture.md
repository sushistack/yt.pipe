# Architecture — yt.pipe

> Auto-generated project documentation (2026-03-09)

## Executive Summary

yt.pipe is an automated YouTube content production pipeline for SCP Foundation videos. Given an SCP ID, it generates a complete video project: scenario → images → narration → subtitles → CapCut project. The system follows an "80% automation, 20% manual finishing" philosophy with human review gates at critical points.

## Technology Stack

| Category | Technology | Version | Purpose |
|----------|-----------|---------|---------|
| Language | Go | 1.25.7 | Primary language |
| CLI | cobra | v1.10.2 | CLI command structure |
| HTTP | chi/v5 | v5.2.5 | REST API router + middleware |
| Config | viper | v1.21.0 | 5-level priority config chain |
| Database | SQLite (modernc) | v1.46.1 | Pure-Go SQLite, WAL mode |
| Testing | testify | v1.11.1 | Assertions and mocking |
| Logging | log/slog | stdlib | Structured JSON logging |
| Container | Docker + Compose | - | Deployment |
| LLM | OpenAI-compatible | - | Gemini, Qwen, DeepSeek (fallback chain) |
| Image Gen | SiliconFlow FLUX | - | AI image generation |
| TTS | DashScope CosyVoice | - | Text-to-speech narration |
| Output | CapCut format | - | Video project assembly |

## Architecture Pattern

**Layered / Clean Architecture with Plugin System**

```
┌────────────────────────────────────────────┐
│              Adapters (CLI + API)           │
│  cmd/yt-pipe → cli/    api/ (chi router)   │
├────────────────────────────────────────────┤
│           Pipeline Orchestrator            │
│  pipeline/runner.go (8-stage sequential)   │
├────────────────────────────────────────────┤
│            Service Layer                   │
│  service/ (30+ files, business logic)      │
├────────────────────────────────────────────┤
│         Plugin Interfaces                  │
│  plugin/ (LLM, TTS, ImageGen, Output)      │
├────────────────────────────────────────────┤
│           Domain Models                    │
│  domain/ (13 models, state machines)       │
├────────────────────────────────────────────┤
│          Persistence (Store)               │
│  store/ (SQLite, 7 migrations, 80+ ops)    │
└────────────────────────────────────────────┘
```

### Key Architecture Principles

1. **Scene as Unit of Work** — Each scene is a self-contained asset bundle (image, audio, subtitle, metadata). All processing is per-scene.

2. **Plugin Architecture** — External services (LLM, TTS, ImageGen, Output) are behind interfaces. Implementations are registered via Factory pattern in a thread-safe Registry.

3. **Incremental Build** — Scene manifests track content hashes. Only changed scenes are regenerated. Dependency chain invalidation propagates downstream.

4. **Dual Interface** — CLI and REST API are adapters over the same service layer. No business logic in adapters.

5. **State Machine** — Projects follow a strict state machine: `pending → scenario_review → approved → image_review → tts_review → assembling → complete`. Invalid transitions are rejected with `TransitionError`.

6. **Checkpoint/Resume** — Pipeline saves checkpoints after each stage. Interrupted runs resume from the last completed stage.

## Pipeline Architecture

### 8-Stage Sequential Pipeline

```
Stage 1: data_load          → Load SCP data (facts, meta, main text)
Stage 2: scenario_generate  → 4-stage LLM pipeline (research→structure→write→review)
Stage 3: scenario_approval  → PAUSE for human review
Stage 4: image_generate     → Generate images per scene (parallel with Stage 5)
Stage 5: tts_synthesize     → Generate narration audio per scene
Stage 6: timing_resolve     → Calculate scene/word timing from audio duration
Stage 7: subtitle_generate  → Create subtitles from word timings
Stage 8: assemble           → Build CapCut project with all assets
```

### Approval Workflow

Two modes of operation:

- **Approval Path** (default): Pauses at scenario_review, image_review, and tts_review for human approval of each scene's assets.
- **Skip-Approval Path** (`--auto-approve`): Runs all stages without pause. Legacy backward-compatible mode.

### Parallel Execution

Stages 4 (images) and 5 (TTS) run in parallel using goroutines when in skip-approval mode. In approval mode, they run sequentially with review gates.

## Plugin System

### Registry Pattern

```go
// Registration (at bootstrap)
registry.Register(PluginTypeLLM, "gemini", GeminiFactory)
registry.Register(PluginTypeTTS, "dashscope", DashScopeFactory)

// Instantiation (at runtime)
llmPlugin, err := registry.Create(PluginTypeLLM, "gemini", configMap)
```

### Plugin Interfaces

| Plugin Type | Interface | Methods | Current Implementation |
|-------------|-----------|---------|----------------------|
| LLM | `llm.LLM` | Complete, GenerateScenario, RegenerateSection | OpenAI-compatible (Gemini, Qwen, DeepSeek) + FallbackChain |
| TTS | `tts.TTS` | Synthesize, SynthesizeWithOverrides | DashScope CosyVoice |
| ImageGen | `imagegen.ImageGen` | Generate | SiliconFlow FLUX |
| Output | `output.Assembler` | Assemble, Validate | CapCut format |

### Error Handling & Retry

All plugins use the `retry.Do()` utility with exponential backoff + jitter:
- Retryable: HTTP 429 (rate limit), 5xx (server errors)
- Non-retryable: 400, 401, 403 (fail fast)
- Max attempts: 3 (configurable)
- Backoff cap: 60 seconds

## Data Architecture

### SQLite Database

- **7 migrations** (001-007), auto-run on startup
- **13+ tables**: projects, jobs, scene_manifests, execution_logs, feedback, prompt_templates, versions, overrides, characters, mood_presets, scene_moods, bgms, scene_bgms, scene_approvals
- **WAL mode** for concurrent read/write
- **Foreign keys** enforced
- **Embedded driver** (no external dependency)

### Workspace File System

```
{workspace}/{SCPID}-{timestamp}/
├── scenes/
│   ├── 1/
│   │   ├── image.png       # Generated image
│   │   ├── audio.mp3       # TTS narration
│   │   ├── timing.json     # Word-level timings
│   │   ├── prompt.txt      # Image prompt (editable)
│   │   ├── subtitle.json   # Generated subtitles
│   │   └── manifest.json   # Scene metadata
│   ├── 2/ ...
├── scenario.json            # Full scenario output
├── scenario.md              # Human-readable scenario
├── checkpoint.json          # Pipeline resume state
└── output/
    ├── draft_content.json   # CapCut project
    ├── draft_meta_info.json # CapCut metadata
    └── description.txt      # Video description + copyright
```

### Atomic File Operations

All file writes use atomic write (temp file + rename) to prevent corruption on crash or interruption.

## Configuration Architecture

### 5-Level Priority Chain

```
CLI flags (--flag)          ← highest priority
Environment vars (YTP_*)
Project config (./config.yaml)
Global config (~/.yt-pipe/config.yaml)
Built-in defaults           ← lowest priority
```

All config sources are tracked with `configSource` for debugging which source provided each value.

## Service Layer

### Core Services (30+ files)

| Service | Responsibility |
|---------|---------------|
| ProjectService | CRUD + state transitions (atomic via DB transaction) |
| ScenarioService | 4-stage LLM scenario pipeline + approval |
| ImageGenService | Per-scene image generation + character auto-reference |
| TTSService | Audio synthesis + word timing + pronunciation glossary |
| SubtitleService | Subtitle generation from word timings |
| TimingResolver | Scene/word timing calculation |
| AssemblerService | CapCut output assembly + copyright generation |
| TemplateService | Prompt template CRUD + version management + defaults |
| CharacterService | Character presets + alias matching in scene text |
| MoodService | Mood presets + LLM auto-mapping for TTS parameters |
| BGMService | BGM library + mood-based recommendation + credits |
| ApprovalService | Per-scene asset approval workflow |
| SceneDashboardService | Scene overview with asset statuses |

## Testing Strategy

- **Unit tests**: `*_test.go` in same package, using `testify`
- **Integration tests**: `tests/integration/` with `-tags=integration`
- **In-memory DB**: `:memory:` SQLite for test isolation
- **Mocking**: `.mockery.yaml` configured (mock generation)
- **Test data**: `testdata/SCP-173/` sample fixtures

## Deployment Architecture

- **Dockerfile**: Multi-stage build (Go builder → scratch/alpine runtime)
- **docker-compose.yml**: Single container with volume mounts for data/workspace
- **No external DB**: SQLite is embedded, no separate database service needed
- **Stateful**: Requires persistent volumes for workspace and database
