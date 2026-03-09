# Story 13.2: Prompt Template Store — CRUD & Version Management

Status: done

## Story

As a developer,
I want a template store with CRUD operations, version history tracking, and rollback capability,
So that the service layer can manage templates with full version control.

## Acceptance Criteria

1. **AC1: CRUD Operations**
   - `CreateTemplate(template)` inserts a new template and creates version 1 in `prompt_template_versions`
   - `GetTemplate(id)` returns a template by ID
   - `ListTemplates(category)` returns all templates filtered by optional category
   - `UpdateTemplate(id, content)` increments the version, saves the new content, and creates a new version record
   - `DeleteTemplate(id)` removes the template and all its version records and project overrides

2. **AC2: Version History with 10-Version Limit (FR46)**
   - When version history exceeds 10 entries, the oldest version beyond 10 is automatically deleted on the next update

3. **AC3: Rollback (FR46)**
   - `RollbackTemplate(id, version)` restores the template content to the specified version
   - A new version record is created (version number increments, not reverts)

4. **AC4: Project Overrides (FR47)**
   - `SetOverride(projectID, templateID, content)` stores the override in `project_template_overrides`
   - `GetOverride(projectID, templateID)` returns the project-specific content
   - `DeleteOverride(projectID, templateID)` removes the override

5. **AC5: Unit Tests**
   - All CRUD, version management, rollback, and override operations are covered with testify assertions

## Tasks / Subtasks

- [x] Task 1: Implement CRUD operations (AC: #1)
  - [x] 1.1 `CreateTemplate()` — inserts template + creates v1 version record in transaction
  - [x] 1.2 `GetTemplate()` — query by ID
  - [x] 1.3 `ListTemplates()` — optional category filter
  - [x] 1.4 `UpdateTemplate()` — increment version, save new content, create version record
  - [x] 1.5 `DeleteTemplate()` — cascade delete versions + overrides
- [x] Task 2: Implement version management (AC: #2, #3)
  - [x] 2.1 Auto-prune versions beyond 10 on update
  - [x] 2.2 `RollbackTemplate()` — restore content, create new version record
  - [x] 2.3 `GetTemplateVersion()`, `ListTemplateVersions()` — version history queries
- [x] Task 3: Implement project overrides (AC: #4)
  - [x] 3.1 `SetOverride()` — upsert project override
  - [x] 3.2 `GetOverride()` — retrieve override
  - [x] 3.3 `DeleteOverride()` — remove override
- [x] Task 4: Write tests (AC: #5)
  - [x] 4.1 CRUD operation tests (22 test cases)
  - [x] 4.2 Version pruning test (10-version limit)
  - [x] 4.3 Rollback tests
  - [x] 4.4 Override upsert/cascade tests

## Dev Notes

### Implementation Details

- All mutating operations (Create, Update, Delete, Rollback) use transactions for atomicity
- `UpdateTemplate()` auto-prunes old versions after creating the new one — deletes versions with the lowest `created_at` when count exceeds 10
- `RollbackTemplate()` reads the target version's content, then calls the same update path (increment version + create version record)
- `DeleteTemplate()` cascades: deletes overrides → versions → template (order matters for FK constraints)
- `SetOverride()` uses `INSERT OR REPLACE` for upsert behavior

### Store Pattern (from existing code)

Follows `store/project.go` and `store/job.go` patterns:
- Methods on `*Store` receiver
- `context.Context` as first parameter (reserved for future use)
- `time.Now().UTC().Format(time.RFC3339)` for timestamp serialization
- `sql.ErrNoRows` → `domain.NotFoundError` mapping

## Dev Agent Record

### Completion Notes List

- Implemented 323-line store layer in `internal/store/template.go`
- Full CRUD: Create, Get, List, Update, Delete with transactional integrity
- Version history: auto-creates version records, 10-version pruning, rollback
- Project overrides: Set (upsert), Get, Delete
- 288 lines of tests covering all operations
- `make test` — all pass, zero regressions
- `make lint` — clean

### File List

- `internal/store/template.go` (new, 323 lines) — Template store with CRUD, versioning, rollback, overrides
- `internal/store/template_test.go` (new, 288 lines) — Comprehensive test suite (22+ test cases)
