# Project Overview — yt.pipe

> Auto-generated project documentation (2026-03-09)

## Summary

**yt.pipe** (youtube.pipeline) is an automated YouTube content production pipeline for SCP Foundation videos. It transforms SCP data into complete video projects through an 8-stage pipeline: data loading → scenario generation → human approval → image generation → TTS narration → timing resolution → subtitle generation → CapCut project assembly.

**Philosophy**: "80% automation, 20% manual finishing" — the pipeline automates the heavy lifting while providing human review gates at critical decision points.

## Key Features

- **Automated Video Pipeline**: End-to-end SCP content production from raw data to CapCut project
- **8-Stage Sequential Pipeline**: With checkpoint/resume, incremental builds, and parallel execution
- **Human Review Gates**: Approval workflow for scenario, images, and TTS at scene level
- **Plugin Architecture**: Swappable LLM, TTS, ImageGen, and Output providers
- **Dual Interface**: CLI + REST API over the same service layer
- **Incremental Builds**: Hash-based skip detection — only regenerate changed scenes
- **Character Consistency**: Character presets with visual descriptors for consistent image generation
- **Mood-Aware TTS**: Mood presets with LLM auto-mapping for expressive narration
- **BGM Integration**: Background music library with mood-based recommendation and audio mixing

## Tech Stack Summary

| Category | Technology |
|----------|-----------|
| Language | Go 1.25.7 |
| CLI | cobra v1.10.2 |
| HTTP | chi/v5 v5.2.5 |
| Database | SQLite (modernc, pure-Go) |
| LLM | OpenAI-compatible (Gemini, Qwen, DeepSeek) |
| Image Gen | SiliconFlow FLUX |
| TTS | DashScope CosyVoice |
| Output | CapCut project format |
| Container | Docker + Compose |

## Architecture Type

**Monolith** — Single Go binary with layered architecture:

```
CLI/API Adapters → Pipeline Orchestrator → Service Layer → Plugin Interfaces → Domain Models → SQLite Store
```

## Repository Structure

- **Module**: `github.com/sushistack/yt.pipe`
- **Entry Point**: `cmd/yt-pipe/main.go`
- **Core Code**: `internal/` (14 packages, 100+ Go files)
- **Database**: 7 SQL migrations, 13+ tables
- **API**: 20 REST endpoints via chi router
- **CLI**: 20+ commands via cobra
- **Plugins**: 4 types (LLM, TTS, ImageGen, Output) with factory registry

## Pipeline Stages

| # | Stage | Description | External API |
|---|-------|-------------|--------------|
| 1 | data_load | Load SCP facts, meta, main text | None |
| 2 | scenario_generate | 4-stage LLM pipeline (research→structure→write→review) | LLM |
| 3 | scenario_approval | Human review gate (pause/resume) | None |
| 4 | image_generate | AI image per scene | ImageGen |
| 5 | tts_synthesize | Narration audio per scene | TTS |
| 6 | timing_resolve | Scene/word timing calculation | None |
| 7 | subtitle_generate | Subtitle from word timings | None |
| 8 | assemble | CapCut project assembly | None |

## Links to Detailed Docs

- [Architecture](./architecture.md) — System design, patterns, data flow
- [API Contracts](./api-contracts.md) — REST API endpoints and schemas
- [Data Models](./data-models.md) — Database schema and domain models
- [Source Tree](./source-tree-analysis.md) — Annotated directory structure
- [Development Guide](./development-guide.md) — Setup, build, test, deploy
