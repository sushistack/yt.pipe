# Story 9.4: Scene Image Generation Pipeline

Status: done

## Story

As a creator,
I want the image generation pipeline to run per-scene with progress tracking and manifest updates,
So that I can monitor generation progress and resume from failures.

## Acceptance Criteria

1. `ImageGenService.GenerateSceneImage` generates a single scene image with retry
2. Saves image to scene directory as `image.{ext}` and prompt as `prompt.txt`
3. Updates scene manifest in SQLite with image hash and status
4. `GenerateAllImages` supports filtering by scene numbers for selective generation
5. Partial failure tolerance — continues remaining scenes, returns all successes
6. Per-scene progress logging during standalone `image generate` command
7. Image dimensions configurable via `imagegen.width`/`imagegen.height`

## Implementation Summary

### Files Modified
- `internal/service/image_gen.go` — Added BackupSceneImage function
- `internal/cli/stage_cmds.go` — Added imageGenerateCmd flags (--parallel, --force), updated buildRunner with ImageOpts
- `internal/pipeline/runner.go` — Added per-scene progress logging in runImageGenerateStage, TemplatesPath in RunnerConfig

### Key Features
- SHA-256 image hash stored in manifest for incremental build support
- `filterPrompts` enables selective scene generation
- `markSceneFailed` records failures in manifest for recovery
- `ReadManualPrompt` supports prompt.txt editing workflow

## References
- [Source: _bmad-output/planning-artifacts/epics.md#Story 9.4]
- [Source: internal/service/image_gen.go] — existing service
- [Source: internal/pipeline/runner.go] — pipeline integration
