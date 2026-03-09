# Story 15.3: Mood Preset Store — CRUD & Scene Assignment

Status: done

## Story

As a developer,
I want a mood preset store with CRUD for presets and scene assignment management,
So that the service layer can manage presets and track per-scene mood configurations.

## Acceptance Criteria

1. **Given** the mood tables from Story 15.2
   **When** `store/mood_preset.go` is implemented
   **Then** `CreateMoodPreset(preset)` inserts a new mood preset with JSON-serialized params
   **And** `GetMoodPreset(id)` returns a preset by ID with deserialized params
   **And** `GetMoodPresetByName(name)` returns a preset by unique name
   **And** `ListMoodPresets()` returns all presets ordered by name
   **And** `UpdateMoodPreset(preset)` updates all fields and timestamps
   **And** `DeleteMoodPreset(id)` removes the preset (fails if scene assignments reference it)
   **And** this satisfies FR51

2. **Given** scene mood assignments are needed
   **When** assignment operations are called
   **Then** `AssignMoodToScene(projectID, sceneNum, presetID, autoMapped)` creates/updates assignment with confirmed=false
   **And** `ConfirmSceneMood(projectID, sceneNum)` sets confirmed=true
   **And** `GetSceneMoodAssignment(projectID, sceneNum)` returns the assignment
   **And** `ListSceneMoodAssignments(projectID)` returns all assignments for a project ordered by scene_num
   **And** `DeleteSceneMoodAssignment(projectID, sceneNum)` removes the assignment

3. **Given** all store operations
   **When** unit tests run
   **Then** all CRUD, assignment, and confirmation operations are covered with testify assertions

## Tasks / Subtasks

- [x] Task 1: Implement mood preset CRUD (AC: #1)
  - [x] `CreateMoodPreset` — INSERT with JSON-serialized params, nil params → `{}`
  - [x] `GetMoodPreset` — SELECT by ID with JSON deserialization
  - [x] `GetMoodPresetByName` — SELECT by unique name
  - [x] `ListMoodPresets` — SELECT all ORDER BY name
  - [x] `UpdateMoodPreset` — UPDATE all fields + timestamp, NotFoundError on miss
  - [x] `DeleteMoodPreset` — DELETE with NotFoundError on miss, FK constraint on assignments
- [x] Task 2: Implement scene mood assignment operations (AC: #2)
  - [x] `AssignMoodToScene` — INSERT or ON CONFLICT UPDATE (upsert), confirmed=0 reset
  - [x] `ConfirmSceneMood` — UPDATE confirmed=1, NotFoundError on miss
  - [x] `GetSceneMoodAssignment` — SELECT with bool conversion
  - [x] `ListSceneMoodAssignments` — SELECT by project_id ORDER BY scene_num
  - [x] `DeleteSceneMoodAssignment` — DELETE with NotFoundError on miss
- [x] Task 3: Write comprehensive tests (AC: #3)
  - [x] Preset CRUD tests (create, duplicate name, get, getByName, list, update, delete)
  - [x] Delete with FK constraint test
  - [x] Assignment tests (assign, upsert, confirm, list, delete)
  - [x] Nil params_json handling test

## Dev Notes

### Implementation Highlights

- **Upsert pattern**: `AssignMoodToScene` uses `ON CONFLICT(project_id, scene_num) DO UPDATE` for idempotent re-assignment. Confirmed flag is always reset to 0 on upsert.
- **Nil params handling**: `CreateMoodPreset` converts nil `ParamsJSON` to empty map before JSON marshaling, ensuring `{}` is stored instead of `null`.
- **Bool storage**: SQLite stores booleans as INTEGER (0/1). Scan converts to Go `bool`.

### Key Files

| File | Change |
|------|--------|
| `internal/store/mood_preset.go` | New — 11 store methods + scanMoodPresets helper |
| `internal/store/mood_preset_test.go` | New — 27 test functions covering all operations |

### Testing Standards

- In-memory SQLite (`:memory:`) for isolated tests
- `setupTestStore(t)` helper from store_test.go
- `testify` assert + require

### References

- [Source: _bmad-output/planning-artifacts/epics.md — Story 15.3 AC]
- [Source: internal/store/character.go — pattern reference for CRUD operations]

## Dev Agent Record

### Agent Model Used
Claude Opus 4.6

### Completion Notes List
- Implemented all 11 store methods for mood presets and scene assignments
- Full CRUD for presets: Create, Get, GetByName, List, Update, Delete
- Full scene assignment operations: Assign (upsert), Confirm, Get, List, Delete
- FK constraint prevents deleting presets with active assignments
- 27 test functions covering success, not-found, duplicate, upsert, confirm, nil params
- All tests pass

### File List
- `internal/store/mood_preset.go` (new)
- `internal/store/mood_preset_test.go` (new)
