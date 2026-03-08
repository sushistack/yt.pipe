# Story 7-4: Asset Management API

## Status: Done

## Implementation Summary

### New Files
- `internal/api/assets.go` — Asset management handlers: `POST /api/v1/projects/:id/images/generate` (selective image regen), `POST /api/v1/projects/:id/tts/generate` (selective TTS regen), `PUT /api/v1/projects/:id/scenes/:num/prompt` (prompt update), `POST /api/v1/projects/:id/feedback` (quality feedback)
- `internal/api/assets_test.go` — Tests for selective regeneration, prompt updates, feedback submission, and invalid scene validation

### Architecture Decisions
- Selective regeneration accepts `{"scenes": [3, 5, 7]}` to regenerate only specified scenes
- TTS regeneration marks downstream artifacts (timing, subtitles) as stale
- Prompt update modifies scene prompt file and marks image as stale in manifest
- Feedback stored in SQLite (same table as `yt-pipe feedback`)
- 400 for invalid scene numbers with valid range in response

### Acceptance Criteria Met
- [x] `POST .../images/generate` regenerates specified scenes only
- [x] `POST .../tts/generate` re-synthesizes specified scenes, marks downstream stale
- [x] `PUT .../scenes/:num/prompt` updates prompt and marks image stale
- [x] `POST .../feedback` stores feedback in SQLite
- [x] 400 for invalid scene numbers
- [x] All tests pass
