# Story 17.2: BGM Domain Model & Database Migration

Status: done

## Story

As a developer,
I want BGM domain models and SQLite tables for storing BGM files, mood tags, license metadata, and scene assignments,
So that BGM library data can be persistently managed.

## Acceptance Criteria

1. **Given** the domain package exists
   **When** `domain/bgm.go` is defined
   **Then** `BGM` model contains: ID, Name, FilePath, MoodTags ([]string), DurationMs, LicenseType, LicenseSource, CreditText, CreatedAt
   **And** `SceneBGMAssignment` model contains: ProjectID, SceneNum, BGMID, VolumeDB, FadeInMs, FadeOutMs, DuckingDB, AutoRecommended (bool), Confirmed (bool)
   **And** LicenseType is validated against allowed values: "royalty_free", "cc_by", "cc_by_sa", "cc_by_nc", "custom"

2. **Given** the store package exists
   **When** migration `006_bgms.sql` is created
   **Then** tables `bgms` and `scene_bgm_assignments` are created
   **And** index `idx_bgms_mood_tags` is created
   **And** existing migrations (001-005) continue to work correctly

## Tasks / Subtasks

- [ ] Task 1: Create BGM domain model (AC: #1)
  - [ ] 1.1: Define `BGM` struct in `internal/domain/bgm.go`
  - [ ] 1.2: Define `SceneBGMAssignment` struct
  - [ ] 1.3: Define `ValidLicenseTypes` map and `ValidateLicenseType` function
  - [ ] 1.4: Write domain model tests in `internal/domain/bgm_test.go`
- [ ] Task 2: Create database migration (AC: #2)
  - [ ] 2.1: Create `internal/store/migrations/006_bgms.sql`
  - [ ] 2.2: Update `store_test.go` schema version assertion to 6
  - [ ] 2.3: Add migration test for BGM tables
- [ ] Task 3: Run full test suite
  - [ ] 3.1: `make test` passes
  - [ ] 3.2: `make lint` passes

## Dev Notes

- Follow Epic 14 (character.go) pattern: simple flat struct, package-level validation function
- Migration number: 006 (after 005_mood_presets.sql)
- JSON in TEXT column for mood_tags (like aliases in characters)
- Booleans as INTEGER (0/1) in SQLite
- Composite PK for scene_bgm_assignments (project_id, scene_num)
- CHECK constraint for license_type enum at DB level

## Dev Agent Record

### Agent Model Used
Claude Opus 4.6

### Completion Notes List

### File List
