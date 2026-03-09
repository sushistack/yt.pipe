# Story 14.2: Character Domain Model & Database Migration

Status: done

## Story

As a developer,
I want character domain models and SQLite tables for storing per-SCP character ID cards,
So that character visual presets can be persistently stored and queried.

## Acceptance Criteria

1. **Given** the domain package exists
   **When** `domain/character.go` is defined
   **Then** `Character` model contains: ID, SCPID, CanonicalName, Aliases ([]string), VisualDescriptor, StyleGuide, ImagePromptBase, CreatedAt, UpdatedAt
   **And** aliases are validated as non-empty when provided

2. **Given** the store package exists
   **When** migration `004_characters.sql` is created
   **Then** table `characters` is created with: id, scp_id, canonical_name, aliases (TEXT/JSON), visual_descriptor, style_guide, image_prompt_base, created_at, updated_at
   **And** index `idx_characters_scp_id` is created
   **And** existing migrations (001, 002, 003) continue to work correctly

## Tasks / Subtasks

- [x] Task 1: Create Character domain model (AC: #1)
  - [x] Define Character struct in `internal/domain/character.go`
  - [x] Add ValidateAliases helper (non-empty strings when provided)
  - [x] Write unit tests in `internal/domain/character_test.go`
- [x] Task 2: Create database migration (AC: #2)
  - [x] Create `internal/store/migrations/004_characters.sql`
  - [x] Verify migration runs with existing 001-003 migrations via store test

## Dev Notes

### Domain Model Pattern (from template.go)
- Use `time.Time` for timestamps
- String-typed enums where applicable
- Validation maps for constrained values
- Same package tests with testify

### Migration Pattern (from 003_templates.sql)
- `TEXT PRIMARY KEY` for ID
- `TEXT NOT NULL` for required strings
- `TEXT` for optional strings (aliases stored as JSON array)
- `created_at TEXT NOT NULL`, `updated_at TEXT NOT NULL` (RFC3339)
- Index on lookup fields (scp_id)

### Migration Number: 004 (NOT 003)
Architecture doc says 003 but Epic 13 already used 003_templates.sql. Next available is 004.

### References
- [Source: _bmad-output/planning-artifacts/architecture.md — characters table schema]
- [Source: internal/domain/template.go — domain model pattern]
- [Source: internal/store/migrations/003_templates.sql — migration pattern]

## Dev Agent Record

### Agent Model Used
Claude Opus 4.6

### Completion Notes List
- Created Character domain model with ID, SCPID, CanonicalName, Aliases, VisualDescriptor, StyleGuide, ImagePromptBase, timestamps
- ValidateAliases rejects empty/whitespace-only strings
- Migration 004_characters.sql creates characters table with idx_characters_scp_id index
- Updated store_test.go schema version assertion from 3 to 4

### File List
- `internal/domain/character.go` (new)
- `internal/domain/character_test.go` (new)
- `internal/store/migrations/004_characters.sql` (new)
- `internal/store/store_test.go` (modified - schema version 3→4)
