# Source Tree Analysis — yt.pipe

> Auto-generated project documentation (2026-03-09)

## Project Root

```
yt.pipe/
├── cmd/yt-pipe/                  # [ENTRY] CLI entry point
│   └── main.go                   #   → cli.Execute()
│
├── internal/                     # Application core (not importable)
│   ├── api/                      # REST API layer (chi router)
│   │   ├── server.go             #   Server setup, route registration
│   │   ├── auth.go               #   Bearer token authentication
│   │   ├── middleware.go          #   Recovery, RequestID, Logging, Auth
│   │   ├── projects.go           #   CRUD endpoints
│   │   ├── pipeline.go           #   Run, status, cancel, approve
│   │   ├── scenes.go             #   Scene dashboard, approve/reject
│   │   ├── assets.go             #   Image/TTS regeneration, prompt edit
│   │   ├── config_handler.go     #   Config GET/PATCH, plugin management
│   │   ├── webhook.go            #   State change notifications
│   │   ├── health.go             #   Health + readiness checks
│   │   └── response.go           #   Standard JSON envelope
│   │
│   ├── cli/                      # CLI commands (cobra)
│   │   ├── root.go               #   Root command + global flags
│   │   ├── serve_cmd.go          #   `serve` → start API server
│   │   ├── init_cmd.go           #   `init` → project initialization
│   │   ├── run_cmd.go            #   `run` → full pipeline execution
│   │   ├── stage_cmds.go         #   Individual stage commands
│   │   ├── status_cmd.go         #   `status` → project status
│   │   ├── config_cmd.go         #   `config` → show/validate config
│   │   ├── feedback_cmd.go       #   `feedback` → submit feedback
│   │   ├── clean_cmd.go          #   `clean` → workspace cleanup
│   │   ├── logs_cmd.go           #   `logs` → execution log viewer
│   │   ├── metrics_cmd.go        #   `metrics` → cost/performance stats
│   │   ├── template_cmd.go       #   `template` → CRUD template management
│   │   ├── character_cmd.go      #   `character` → character presets
│   │   ├── mood_cmd.go           #   `mood` → mood preset management
│   │   ├── bgm_cmd.go            #   `bgm` → BGM library management
│   │   ├── scenes_cmd.go         #   `scenes` → scene approval workflow
│   │   ├── tts_cmd.go            #   `tts` → TTS generation commands
│   │   ├── assemble_cmd.go       #   `assemble` → CapCut assembly
│   │   ├── prompt.go             #   Interactive prompt utilities
│   │   ├── plugins.go            #   Plugin registration bootstrap
│   │   └── validate_api.go       #   API validation utilities
│   │
│   ├── config/                   # Configuration management
│   │   ├── config.go             #   5-level priority: CLI > env > project > global > defaults
│   │   └── types.go              #   Config struct definitions
│   │
│   ├── domain/                   # Domain models & business rules
│   │   ├── project.go            #   Project model + state machine
│   │   ├── scenario.go           #   ScenarioOutput, SceneScript
│   │   ├── scene.go              #   Scene model (image, audio, subtitle)
│   │   ├── manifest.go           #   SceneManifest (incremental build)
│   │   ├── job.go                #   Async job model
│   │   ├── execution_log.go      #   Cost/performance tracking
│   │   ├── feedback.go           #   User feedback model
│   │   ├── template.go           #   Prompt template + versioning
│   │   ├── character.go          #   Character preset model
│   │   ├── mood_preset.go        #   Mood preset + scene assignment
│   │   ├── bgm.go                #   BGM model + scene assignment
│   │   ├── scene_approval.go     #   Per-scene approval state machine
│   │   └── errors.go             #   NotFound, Validation, Plugin, Transition errors
│   │
│   ├── glossary/                 # SCP term dictionary
│   │   └── glossary.go           #   Pronunciation lookup for TTS accuracy
│   │
│   ├── logging/                  # Structured logging setup
│   │   └── logging.go            #   JSON/text format, slog configuration
│   │
│   ├── pipeline/                 # Pipeline orchestration
│   │   ├── runner.go             #   8-stage pipeline: load→scenario→approval→image+TTS→timing→subtitle→assembly
│   │   ├── checkpoint.go         #   Checkpoint save/load for resume
│   │   ├── dryrun.go             #   Dry-run mode (validation only)
│   │   └── progress.go           #   Real-time progress reporting
│   │
│   ├── plugin/                   # Plugin system
│   │   ├── base.go               #   Base plugin config & HTTP client
│   │   ├── registry.go           #   Factory-based plugin registry
│   │   ├── llm/                  #   LLM plugin interface + OpenAI-compatible impl
│   │   │   ├── interface.go      #     LLM interface (Complete, GenerateScenario, RegenerateSection)
│   │   │   └── openai.go         #     OpenAI/Gemini/Qwen/DeepSeek + FallbackChain
│   │   ├── tts/                  #   TTS plugin interface + DashScope impl
│   │   │   ├── interface.go      #     TTS interface (Synthesize, SynthesizeWithOverrides)
│   │   │   └── dashscope.go      #     DashScope CosyVoice provider
│   │   ├── imagegen/             #   Image generation plugin
│   │   │   ├── interface.go      #     ImageGen interface (Generate)
│   │   │   └── siliconflow.go    #     SiliconFlow FLUX provider
│   │   └── output/               #   Output assembly plugin
│   │       ├── interface.go      #     Assembler interface (Assemble, Validate)
│   │       └── capcut/           #     CapCut project format
│   │           ├── capcut.go     #       Draft content + meta generation
│   │           └── types.go      #       CapCut JSON schema types
│   │
│   ├── retry/                    # Retry logic
│   │   └── retry.go              #   Exponential backoff + jitter + RetryableError
│   │
│   ├── service/                  # Business service layer (30+ files)
│   │   ├── project.go            #   Project lifecycle + state transitions
│   │   ├── scenario.go           #   Scenario generation orchestration
│   │   ├── scenario_pipeline.go  #   4-stage LLM pipeline (research→structure→write→review)
│   │   ├── image_gen.go          #   Image generation per-scene
│   │   ├── image_prompt.go       #   Prompt construction + character injection
│   │   ├── tts.go                #   TTS synthesis + skip logic
│   │   ├── subtitle.go           #   Subtitle generation from word timings
│   │   ├── timing.go             #   Audio timing resolution
│   │   ├── assembler.go          #   Final output assembly + copyright
│   │   ├── template.go           #   Template CRUD + default installation
│   │   ├── character.go          #   Character management + scene matching
│   │   ├── mood.go               #   Mood preset + LLM auto-mapping
│   │   ├── bgm.go                #   BGM library + mood-based recommendation
│   │   ├── approval.go           #   Scene approval orchestration
│   │   ├── scene_dashboard.go    #   Scene overview with approval status
│   │   ├── pipeline_orchestrator.go # Pipeline flow coordination
│   │   ├── cleanup.go            #   Workspace cleanup utilities
│   │   ├── metrics.go            #   Cost/performance aggregation
│   │   ├── execution_summary.go  #   Execution report generation
│   │   ├── fact_coverage.go      #   Fact coverage validation
│   │   ├── frozen_descriptor.go  #   Character descriptor freezing
│   │   ├── pronunciation.go      #   Pronunciation override building
│   │   ├── shot_breakdown.go     #   Scene shot analysis
│   │   └── default_templates/    #   Built-in prompt templates (.tmpl)
│   │
│   ├── store/                    # SQLite persistence layer
│   │   ├── store.go              #   DB connection, migration runner
│   │   ├── migrations/           #   SQL migrations (001-007)
│   │   ├── project.go            #   Project CRUD + filtered listing
│   │   ├── job.go                #   Job CRUD
│   │   ├── manifest.go           #   Scene manifest operations
│   │   ├── execution_log.go      #   Execution log operations
│   │   ├── feedback.go           #   Feedback CRUD
│   │   ├── template.go           #   Template versioning + overrides
│   │   ├── character.go          #   Character CRUD + alias search
│   │   ├── mood_preset.go        #   Mood preset + scene assignment
│   │   ├── bgm.go                #   BGM CRUD + mood tag search
│   │   └── scene_approval.go     #   Approval state machine operations
│   │
│   ├── template/                 # Go template engine
│   │   └── template.go           #   Prompt template rendering
│   │
│   └── workspace/                # File system operations
│       └── workspace.go          #   Project dirs, atomic writes, SCP data loading
│
├── templates/                    # Prompt template files
│   ├── research.tmpl             #   Stage 1: SCP research
│   ├── structure.tmpl            #   Stage 2: Scenario structure
│   ├── writing.tmpl              #   Stage 3: Narration writing
│   └── review.tmpl               #   Stage 4: Quality review
│
├── testdata/                     # Test fixtures
│   └── SCP-173/                  #   Sample SCP data (facts, meta, main text)
│
├── tests/                        # Integration tests
│   └── integration/              #   End-to-end pipeline tests
│
├── Dockerfile                    # Multi-stage Docker build
├── docker-compose.yml            # Docker Compose configuration
├── Makefile                      # Build, test, lint, run commands
├── go.mod                        # Go module definition
├── config.example.yaml           # Configuration reference
├── .env.example                  # Environment variable reference
└── README.md                     # Project documentation
```

## Critical Directories

| Directory | Purpose | Key Files |
|-----------|---------|-----------|
| `cmd/yt-pipe/` | Entry point | `main.go` |
| `internal/api/` | REST API (20 endpoints) | `server.go`, handler files |
| `internal/cli/` | CLI (20+ commands) | `root.go`, command files |
| `internal/domain/` | Domain models (13 models) | Model + error definitions |
| `internal/pipeline/` | 8-stage orchestrator | `runner.go`, `checkpoint.go` |
| `internal/plugin/` | 4 plugin types | Interface + implementation per type |
| `internal/service/` | Business logic (30+ files) | Service implementations |
| `internal/store/` | SQLite persistence | CRUD + migrations |

## Entry Points

- **CLI**: `cmd/yt-pipe/main.go` → `cli.Execute()` → cobra command tree
- **API**: `internal/api/server.go` → `NewServer()` → chi router
- **Pipeline**: `internal/pipeline/runner.go` → `Run()` / `Resume()`
