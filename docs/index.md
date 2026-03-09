# yt.pipe — Project Documentation Index

> Generated: 2026-03-09 | Scan Level: Deep | Mode: Initial Scan

## Project Overview

- **Type**: Monolith (single Go binary)
- **Primary Language**: Go 1.25.7
- **Architecture**: Layered / Clean Architecture with Plugin System
- **Module**: `github.com/sushistack/yt.pipe`
- **Purpose**: Automated SCP Foundation YouTube content production pipeline

## Quick Reference

- **Tech Stack**: Go + cobra CLI + chi REST API + SQLite + Docker
- **Entry Point**: `cmd/yt-pipe/main.go` → `cli.Execute()`
- **Architecture Pattern**: Layered with Plugin Registry (LLM, TTS, ImageGen, Output)
- **Pipeline**: 8-stage sequential (data → scenario → approval → image+TTS → timing → subtitle → assembly)
- **Database**: SQLite (7 migrations, 13+ tables, WAL mode)
- **API**: 20 REST endpoints with Bearer token auth
- **CLI**: 20+ commands via cobra

## Generated Documentation

- [Project Overview](./project-overview.md) — Summary, features, tech stack
- [Architecture](./architecture.md) — System design, patterns, data flow, plugin system
- [Source Tree Analysis](./source-tree-analysis.md) — Annotated directory structure
- [API Contracts](./api-contracts.md) — REST API endpoints, schemas, auth
- [Data Models](./data-models.md) — Database schema, domain models, state machines
- [Development Guide](./development-guide.md) — Setup, build, test, deploy, CLI usage

## Existing Documentation

- [README.md](../README.md) — Project introduction and overview
- [CLAUDE.md](../CLAUDE.md) — AI agent context (project overview, architecture, conventions)
- [PROJECT_SPEC.md](../PROJECT_SPEC.md) — Full project specification
- [config.example.yaml](../config.example.yaml) — Configuration reference with all options
- [.env.example](../.env.example) — Environment variable reference
- [Dockerfile](../Dockerfile) — Docker build configuration
- [docker-compose.yml](../docker-compose.yml) — Docker Compose deployment
- [Makefile](../Makefile) — Build/test/run automation

## BMAD Planning Artifacts

- [PRD](../_bmad-output/planning-artifacts/prd.md) — Product Requirements Document (44 FR + 24 NFR)
- [Architecture Design](../_bmad-output/planning-artifacts/architecture.md) — BMAD architecture decisions
- [Epics & Stories](../_bmad-output/planning-artifacts/epics.md) — 12 epics breakdown

## Getting Started

1. **Understand the project**: Start with [Project Overview](./project-overview.md)
2. **Explore the architecture**: Read [Architecture](./architecture.md) for system design
3. **Navigate the codebase**: Use [Source Tree Analysis](./source-tree-analysis.md)
4. **API integration**: Reference [API Contracts](./api-contracts.md)
5. **Data layer**: See [Data Models](./data-models.md) for schema and state machines
6. **Development setup**: Follow [Development Guide](./development-guide.md)

## AI-Assisted Development

When using this documentation with AI coding assistants:

- Point to `docs/index.md` as the primary context source
- Reference specific docs for targeted tasks (e.g., API contracts for endpoint work)
- The [Architecture](./architecture.md) doc explains patterns and conventions to follow
- The [Data Models](./data-models.md) doc provides schema context for database work
- For brownfield PRD creation, use this index as input to the BMAD PRD workflow
