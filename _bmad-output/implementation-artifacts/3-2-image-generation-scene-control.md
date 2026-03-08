# Story 3.2: Image Generation & Scene Control

Status: done

## Story
As a creator, I want to generate images for all or specific scenes and be able to edit prompts and regenerate individual scenes.

## Implementation
- `internal/service/image_gen.go`: ImageGenService with GenerateSceneImage() (single scene + retry + manifest), GenerateAllImages() (batch with scene filtering + partial failure), ReadManualPrompt() for edited prompts
- `internal/service/image_gen_test.go`: 8 tests covering success, partial failure, scene filtering, context cancellation, manual prompt read
- Retry: 3 attempts with 1s exponential backoff via retry.Do()
- Scene filtering: filterPrompts() for selective regeneration (--scene 3,5,7)
- Manifest: SQLite update with image SHA-256 hash, failed scene marking
- Atomic file writes, slog structured logging, context cancellation between scenes

## Dev Agent Record
### Agent Model Used
Claude Opus 4.6
