# Story 5.3: Incremental Build with Hash-Based Skip

Status: done

## Story
As a creator, I want the pipeline to detect what has changed and only rebuild affected scenes, so that I save time and API costs when making adjustments.

## Acceptance Criteria
- [x] System compares scene manifest hashes (prompt hash, image hash, audio hash) to detect changes
- [x] Only scenes with changed inputs are regenerated
- [x] Unchanged scenes are skipped with a log message "scene N: unchanged, skipping"
- [x] Scene assets stored independently per scene directory (`scenes/{num}/`)
- [x] When no changes detected, stage completes immediately with "0 scenes regenerated, N skipped"
- [x] Execution log records: total scenes, scenes processed, scenes skipped

## Implementation

### Incremental Build Logic
- `internal/pipeline/incremental.go`:
  - `SceneSkipChecker` struct: compares current content hashes against stored manifest hashes in SQLite
  - `FilterScenesForImageGen()`: compares sanitized prompt SHA-256 hash against manifest content_hash; returns toGenerate/toSkip scene number lists
  - `FilterScenesForTTS()`: compares narration text SHA-256 hash against manifest content_hash; returns toGenerate/toSkip lists
  - `IncrementalResult` struct: TotalScenes, Regenerated, Skipped with `Summary()` formatting
  - Uses `service.ContentHash()` (SHA-256) for consistent hash computation

### Existing Infrastructure Used
- `internal/service/pipeline_orchestrator.go`:
  - `SceneManifest.NeedsRegeneration()` — per-asset hash comparison
  - `ContentHash()` — SHA-256 hashing utility
  - `SceneManifestEntry` — per-scene hash storage (narration, prompt, image, audio, subtitle)
- `internal/store/manifest.go`:
  - `ListManifestsByProject()` — bulk manifest retrieval for incremental comparison

### Tests
- `internal/pipeline/incremental_test.go`: 5 tests
  - NoManifests: all scenes need generation
  - WithMatching: matching hash → skip, missing manifest → generate
  - ChangedPrompt: changed hash → regenerate
  - Summary: "2 scenes regenerated, 8 skipped" format
  - AllSkipped: "0 scenes regenerated, 5 skipped" format

## Dev Agent Record
### Agent Model Used
Claude Opus 4.6

### File List
- `internal/pipeline/incremental.go` — New: SceneSkipChecker, IncrementalResult
- `internal/pipeline/incremental_test.go` — New: 5 unit tests

### Change Log
- 2026-03-08: Initial implementation with all acceptance criteria met
