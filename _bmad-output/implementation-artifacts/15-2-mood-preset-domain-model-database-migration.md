# Story 15.2: Mood Preset Domain Model & Database Migration

Status: done

## Story

As a developer,
I want mood preset domain models and SQLite tables for storing TTS mood presets and scene assignments,
So that mood configurations can be persistently managed and assigned to scenes.

## Acceptance Criteria

1. **Given** the domain package exists
   **When** `domain/mood_preset.go` is defined
   **Then** `MoodPreset` model contains: id, name (unique), description, speed (float64), emotion (string), pitch (float64), params_json (map[string]any), timestamps
   **And** `SceneMoodAssignment` model contains: project_id, scene_num, preset_id, auto_mapped (bool), confirmed (bool)
   **And** `ValidateMoodPreset` validates name, emotion non-empty and speed, pitch positive

2. **Given** the store package exists
   **When** migration `005_mood_presets.sql` is created
   **Then** tables `mood_presets` and `scene_mood_assignments` are created
   **And** `mood_presets.name` has UNIQUE constraint
   **And** `scene_mood_assignments` has composite PK (project_id, scene_num) and FK to mood_presets
   **And** existing migrations (001-004) continue to work correctly
   **And** schema version advances to 5

## Tasks / Subtasks

- [x] Task 1: Create domain models (AC: #1)
  - [x] Define `MoodPreset` struct with all fields
  - [x] Define `SceneMoodAssignment` struct
  - [x] Implement `ValidateMoodPreset` validation function
  - [x] Write domain validation tests
- [x] Task 2: Create database migration (AC: #2)
  - [x] Create `internal/store/migrations/005_mood_presets.sql`
  - [x] `mood_presets` table with UNIQUE name constraint
  - [x] `scene_mood_assignments` table with composite PK and FK
  - [x] Index on `scene_mood_assignments.preset_id`
  - [x] Update store_test.go schema version assertion (4 → 5)

## Dev Notes

### Migration Numbering

Migration is `005` (not `004` as originally specified in the epic) because Epic 14 already used `004_characters.sql`. The migration ordering is:
- 001_initial.sql → projects, jobs, scene_manifests, execution_logs
- 002_feedback.sql → feedback table
- 003_templates.sql → prompt_templates, versions, overrides
- 004_characters.sql → characters table
- **005_mood_presets.sql → mood_presets, scene_mood_assignments**

### Key Files

| File | Change |
|------|--------|
| `internal/domain/mood_preset.go` | New — MoodPreset + SceneMoodAssignment models + validation |
| `internal/domain/mood_preset_test.go` | New — 5 validation tests |
| `internal/store/migrations/005_mood_presets.sql` | New — DDL for mood_presets + scene_mood_assignments |
| `internal/store/store_test.go` | Modified — schema version 4 → 5 |

### References

- [Source: _bmad-output/planning-artifacts/epics.md — Story 15.2 AC]
- [Source: _bmad-output/planning-artifacts/architecture.md — Database Schema section]

## Dev Agent Record

### Agent Model Used
Claude Opus 4.6

### Completion Notes List
- Created `MoodPreset` domain model with ID, Name, Description, Speed, Emotion, Pitch, ParamsJSON, timestamps
- Created `SceneMoodAssignment` with ProjectID, SceneNum, PresetID, AutoMapped, Confirmed
- `ValidateMoodPreset` checks name/emotion non-empty, speed/pitch positive
- Migration `005_mood_presets.sql` creates both tables with proper constraints
- FK from `scene_mood_assignments.preset_id` → `mood_presets.id`
- Index on `scene_mood_assignments.preset_id` for efficient joins
- Schema version test updated to 5
- All tests pass

### File List
- `internal/domain/mood_preset.go` (new)
- `internal/domain/mood_preset_test.go` (new)
- `internal/store/migrations/005_mood_presets.sql` (new)
- `internal/store/store_test.go` (modified)
