# Story 16-5: Scene Asset Mapping Dashboard

## Overview
CLI command and API endpoint showing per-scene text, image, TTS status mapping.

## Changes

### `internal/cli/scenes_cmd.go` — CLI Command
- `yt-pipe scenes <project-id>` — list all scenes with status
- `yt-pipe scenes <project-id> --scene <num>` — detail for a specific scene
- `yt-pipe scenes approve <project-id> --type <image|tts> --scene <num>` — approve a scene
- `yt-pipe scenes reject <project-id> --type <image|tts> --scene <num>` — reject a scene

### `internal/service/scene_dashboard.go` — Dashboard Service
- `SceneDashboardEntry` struct with text excerpt, image/TTS status, paths, mood, BGM
- `GetSceneDashboard(projectID)` — returns all scenes with full status
- `GetSceneDetail(projectID, sceneNum)` — single scene full detail

### `internal/api/scenes_handler.go` — API Handler
- `GET /api/projects/{id}/scenes` — JSON response with scene mappings

## Acceptance Criteria
- [x] CLI displays scene table with approval status
- [x] --scene flag shows full detail
- [x] approve/reject subcommands work
- [x] API endpoint returns JSON
- [x] Tests pass
