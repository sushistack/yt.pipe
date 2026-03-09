# Story 15.4: Mood Service — Preset Management & LLM Auto-Mapping

Status: done

## Story

As a developer,
I want a mood service that manages presets and auto-maps moods to scenes via LLM analysis,
So that creators get intelligent mood suggestions while retaining full control.

## Acceptance Criteria

1. **Given** the mood preset store from Story 15.3
   **When** `service/mood.go` is implemented
   **Then** `CreatePreset(name, description, speed, emotion, pitch, params)` validates uniqueness and delegates to store
   **And** `UpdatePreset`, `DeletePreset`, `GetPreset`, `ListPresets` delegate to store with appropriate validation
   **And** this satisfies FR51

2. **Given** a project with an approved scenario containing multiple scenes
   **When** `AutoMapMoods(projectID, scenes []SceneScript)` is called
   **Then** the service sends each scene's text to the LLM plugin with a mood analysis prompt
   **And** the LLM returns a recommended mood category per scene
   **And** the service matches each recommendation to existing presets by name similarity (case-insensitive)
   **And** matched presets are assigned to scenes with `auto_mapped=true, confirmed=false`
   **And** unmatched recommendations are logged with a warning (scene left unassigned)
   **And** this satisfies FR52

3. **Given** auto-mapped moods are pending confirmation
   **When** `GetPendingConfirmations(projectID)` is called
   **Then** all scene assignments with `confirmed=false` are returned

4. **Given** the service supports confirmation operations
   **When** `ConfirmScene`, `ConfirmAll`, `ReassignScene` are called
   **Then** assignments are confirmed or reassigned accordingly

## Tasks / Subtasks

- [x] Task 1: Implement preset CRUD service methods (AC: #1)
  - [x] `CreatePreset` — validate via `domain.ValidateMoodPreset`, generate UUID, delegate to store
  - [x] `UpdatePreset` — get existing, apply partial updates, re-validate, save
  - [x] `DeletePreset`, `GetPreset`, `ListPresets` — delegate to store
- [x] Task 2: Implement LLM auto-mapping (AC: #2)
  - [x] `AutoMapMoods` — check LLM not nil, list presets, iterate scenes
  - [x] `analyzeSceneMood` — send prompt with preset name list, parse LLM response
  - [x] Match LLM response to presets by case-insensitive name
  - [x] Log warnings for unmatched moods
- [x] Task 3: Implement confirmation operations (AC: #3, #4)
  - [x] `GetPendingConfirmations` — filter by `confirmed=false`
  - [x] `ConfirmScene`, `ConfirmAll`, `ReassignScene`
  - [x] `GetSceneAssignment` — delegate to store
- [x] Task 4: Write comprehensive tests
  - [x] Preset CRUD tests (create, validation error, duplicate, get, list, update, delete)
  - [x] AutoMapMoods tests (success, no presets, no LLM, unmatched mood)
  - [x] Confirmation tests (pending, confirm all, reassign)

## Dev Notes

### LLM Integration Design

The auto-mapping uses a **per-scene approach** (one LLM call per scene) rather than batch:
- Simpler error handling — partial failures don't affect other scenes
- Prompt is straightforward: "analyze this narration, pick from [preset names]"
- LLM returns just the mood name string — minimal parsing needed
- Future optimization: batch all scenes in one LLM call for fewer API calls

### Nil LLM Interface Gotcha

Go's nil interface vs nil pointer distinction required special handling in tests:
- Passing `(*mocks.MockLLM)(nil)` to `NewMoodService` creates a non-nil `llm.LLM` interface (typed nil)
- The `ms.llm == nil` check fails for typed nil
- Test helper uses explicit `if mockLLM == nil { return NewMoodService(s, nil, logger), s }`

### Key Files

| File | Change |
|------|--------|
| `internal/service/mood.go` | New — MoodService with CRUD + AutoMapMoods + confirmations |
| `internal/service/mood_test.go` | New — 12 test functions with mock LLM |

### Dependencies

- `internal/store` — mood preset store from Story 15.3
- `internal/plugin/llm` — LLM interface for mood analysis
- `internal/mocks` — MockLLM for testing
- `github.com/google/uuid` — UUID generation for preset IDs

### References

- [Source: _bmad-output/planning-artifacts/epics.md — Story 15.4 AC]
- [Source: internal/service/character.go — service pattern reference]

## Dev Agent Record

### Agent Model Used
Claude Opus 4.6

### Completion Notes List
- MoodService with full preset CRUD, validation, and UUID generation
- LLM auto-mapping: per-scene analysis with case-insensitive preset matching
- Confirmation workflow: GetPendingConfirmations, ConfirmScene, ConfirmAll, ReassignScene
- 12 test functions covering all service operations
- Nil LLM interface handled correctly in tests
- All tests pass

### File List
- `internal/service/mood.go` (new)
- `internal/service/mood_test.go` (new)
