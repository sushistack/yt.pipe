# Story 19.2: Glossary Suggestion Domain Model & Storage

Status: done

## Story

As a system,
I want glossary suggestions stored in SQLite with proper state management,
so that term suggestions can be tracked through pending → approved/rejected lifecycle.

## Acceptance Criteria

1. **Given** the database is initialized
   **When** migration `014_glossary_suggestions.sql` runs
   **Then** a `glossary_suggestions` table is created with columns: id, project_id, term, pronunciation, definition, category, status, created_at, updated_at
   **And** a UNIQUE constraint on (term, project_id) exists
   **And** a CHECK constraint on status IN ('pending', 'approved', 'rejected') exists
   **And** indexes on status and project_id exist

2. **Given** a `GlossarySuggestion` domain model
   **When** CRUD operations are performed via store
   **Then** Create, Read (by project + status filter), Update (status transition), Delete all work correctly
   **And** duplicate term+project_id insertion returns a clear constraint violation error

## Tasks / Subtasks

- [x] Task 1: Create migration 014_glossary_suggestions.sql (AC: 1)
  - [x] 1.1: Write SQL migration with table, constraints, indexes
  - [x] 1.2: Update expected schema version in store_test.go to 15 (015 from Epic 20 already exists)
- [x] Task 2: Create GlossarySuggestion domain model (AC: 2)
  - [x] 2.1: Create internal/domain/glossary_suggestion.go with model, constants, validation, transitions
- [x] Task 3: Create store CRUD operations (AC: 2)
  - [x] 3.1: Create internal/store/glossary_suggestion.go with Create, Get, ListByProject, UpdateStatus, Delete
- [x] Task 4: Unit tests (AC: 1, 2)
  - [x] 4.1: Test Create success + duplicate constraint
  - [x] 4.2: Test Get + NotFound
  - [x] 4.3: Test ListByProject with status filter
  - [x] 4.4: Test UpdateStatus valid/invalid transitions
  - [x] 4.5: Test Delete

## Dev Notes

### Migration Pattern (Source: internal/store/migrations/)

- File: `internal/store/migrations/014_glossary_suggestions.sql`
- Current latest: 013_cut_decomposition.sql (schema version 13)
- New version: 14
- Embedded via `//go:embed migrations/*.sql` in store.go
- Must update `TestSchemaVersion` expected value from 13 → 14

### Domain Model Pattern (Source: internal/domain/scene_approval.go)

- Use `int` for ID (auto-increment), `string` for status
- `time.Time` for timestamps, stored as RFC3339 strings
- Status constants: `SuggestionPending`, `SuggestionApproved`, `SuggestionRejected`
- State transitions: `pending → approved`, `pending → rejected` (terminal states)
- Validation: Use `domain.ValidationError` for field validation

### Store Pattern (Source: internal/store/scene_approval.go)

- Methods on `*Store` receiver
- `time.Now().UTC().Format(time.RFC3339)` for timestamps
- `sql.ErrNoRows` → `domain.NotFoundError`
- `result.RowsAffected()` check on updates
- Helper `scanGlossarySuggestion(row)` function
- Constraint violations propagated as-is from SQLite

### Test Pattern (Source: internal/store/scene_approval_test.go)

- `setupTestStore(t)` helper already exists
- Create project first (foreign key)
- Use `require.NoError` / `assert.Equal` / `assert.IsType`

### References

- [Source: internal/store/store.go] — Migration framework, schema_version
- [Source: internal/store/migrations/007_scene_approvals.sql] — Similar migration pattern
- [Source: internal/domain/scene_approval.go] — State machine pattern
- [Source: internal/store/scene_approval.go] — Store CRUD pattern
- [Source: internal/domain/errors.go] — ValidationError, NotFoundError

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Completion Notes List

- Created migration 014_glossary_suggestions.sql with UNIQUE, CHECK, FK constraints and indexes
- Domain model with status constants, state transitions (pending→approved/rejected), validation
- Full CRUD store: Create, Get, ListByProject (with status filter), UpdateStatus (with transition validation), Delete
- 11 test cases covering all CRUD ops, constraint violations, state transitions, validation
- Schema version updated to 15 (accounting for existing 015_validation_score.sql from Epic 20)

### Change Log

- 2026-03-18: Implemented Story 19.2 — Glossary suggestion domain model & storage

### File List

- internal/store/migrations/014_glossary_suggestions.sql (new)
- internal/domain/glossary_suggestion.go (new)
- internal/store/glossary_suggestion.go (new)
- internal/store/glossary_suggestion_test.go (new)
- internal/store/store_test.go (modified — schema version 13→15)
