# Story 14.5: Image Service Integration — Character Auto-Reference

Status: done

## Story

As a developer,
I want the image generation service to automatically inject matched character references into ImageGen plugin calls,
So that generated images maintain character visual consistency without manual intervention.

## Tasks / Subtasks

- [x] Task 1: Add optional CharacterService dependency to ImageGenService
- [x] Task 2: Add SCPID and SceneText fields to ImagePromptResult
- [x] Task 3: Integrate character matching in GenerateSceneImage before calling Generate
- [x] Task 4: Verify all existing tests pass (backward compatible — nil characterSvc skips matching)

## Dev Agent Record

### Agent Model Used
Claude Opus 4.6

### Completion Notes List
- Added `characterSvc *CharacterService` optional field to ImageGenService
- Added `SetCharacterService(cs)` method for dependency injection
- Added `SCPID` and `SceneText` fields to ImagePromptResult (omitempty for backward compat)
- In GenerateSceneImage: if characterSvc + SCPID + SceneText are set, calls MatchCharacters and injects refs into opts.CharacterRefs
- On match failure: logs warning and proceeds without refs (no degradation)
- All 18+ existing tests pass without modification

### File List
- `internal/service/image_gen.go` (modified — added characterSvc, SetCharacterService, auto-reference logic)
- `internal/service/image_prompt.go` (modified — added SCPID, SceneText to ImagePromptResult)
