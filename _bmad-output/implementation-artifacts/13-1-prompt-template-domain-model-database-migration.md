# Story 13.1: Prompt Template Domain Model & Database Migration

Status: review

## Story

As a developer,
I want prompt template domain models and SQLite tables for templates, template versions, and project overrides,
So that all subsequent template management features have a solid data foundation.

## Acceptance Criteria

1. **AC1: Domain Models Defined**
   - `internal/domain/template.go` contains:
     - `PromptTemplate` struct: id (TEXT PK), category, name, content, version (int), is_default (bool), created_at, updated_at
     - `TemplateVersion` struct: id (TEXT PK), template_id, version (int), content, created_at
     - `ProjectTemplateOverride` struct: project_id, template_id, content, created_at
     - `TemplateCategory` type with constants: `CategoryScenario`, `CategoryImage`, `CategoryTTS`, `CategoryCaption`
   - Category validation map `ValidTemplateCategories` rejects values outside the enum

2. **AC2: Migration `003_templates.sql` Created**
   - Tables `prompt_templates`, `prompt_template_versions`, `project_template_overrides` are created matching the Architecture spec
   - Indexes `idx_templates_category` and `idx_template_versions_template_id` are created
   - `go:embed` loads the migration automatically (existing `migrations/*.sql` glob in `store.go`)
   - Schema version increments to 3
   - Existing migrations (001_initial.sql, 002_feedback.sql) continue to work correctly

3. **AC3: Unit Tests Pass**
   - Domain model tests verify category validation (valid + invalid)
   - Store migration test verifies all 3 tables created successfully
   - Schema version is 3 after migration

## Tasks / Subtasks

- [x] Task 1: Create domain models (AC: #1)
  - [x] 1.1 Create `internal/domain/template.go` with PromptTemplate, TemplateVersion, ProjectTemplateOverride structs
  - [x] 1.2 Define TemplateCategory type and constants (scenario, image, tts, caption)
  - [x] 1.3 Define ValidTemplateCategories validation map
- [x] Task 2: Create migration SQL (AC: #2)
  - [x] 2.1 Create `internal/store/migrations/003_templates.sql` with 3 tables + 2 indexes
- [x] Task 3: Write tests (AC: #3)
  - [x] 3.1 Create `internal/domain/template_test.go` — category validation tests
  - [x] 3.2 Add migration verification in store test — verify tables exist, schema version = 3

## Dev Notes

### CRITICAL: Migration Numbering Conflict

The Architecture doc specifies `002_templates.sql`, but the codebase already has:
- `001_initial.sql` — projects, jobs, scene_manifests, execution_logs
- `002_feedback.sql` — feedback table

**The correct migration file MUST be `003_templates.sql`** (version 3). The migration system in `store.go:98` parses `%03d_` from filename and skips versions <= current. Using `002` would collide with the existing feedback migration.

Downstream epics must also shift: characters → `004`, mood_presets → `005`, bgms → `006`, scene_approvals → `007`.

### Domain Model Conventions (from existing code)

Follow exact patterns from existing domain models:

```go
// Struct pattern (see domain/project.go, domain/feedback.go):
type PromptTemplate struct {
    ID        string
    Category  TemplateCategory  // typed enum, not raw string
    Name      string
    Content   string
    Version   int
    IsDefault bool
    CreatedAt time.Time
    UpdatedAt time.Time
}
```

- **No JSON tags** on domain structs (existing models don't use them)
- **time.Time** for timestamps (not string)
- **Validation via map** pattern (see `domain/feedback.go` lines 17-29: `ValidAssetTypes`, `ValidRatings`)
- **String-based type** for category enum: `type TemplateCategory string` with `const` block

### SQL Schema (from Architecture spec)

```sql
CREATE TABLE prompt_templates (
    id          TEXT PRIMARY KEY,
    category    TEXT NOT NULL CHECK(category IN ('scenario','image','tts','caption')),
    name        TEXT NOT NULL,
    content     TEXT NOT NULL,
    version     INTEGER NOT NULL DEFAULT 1,
    is_default  INTEGER NOT NULL DEFAULT 0,
    created_at  TEXT NOT NULL,
    updated_at  TEXT NOT NULL
);

CREATE TABLE prompt_template_versions (
    id          TEXT PRIMARY KEY,
    template_id TEXT NOT NULL REFERENCES prompt_templates(id),
    version     INTEGER NOT NULL,
    content     TEXT NOT NULL,
    created_at  TEXT NOT NULL
);

CREATE TABLE project_template_overrides (
    project_id  TEXT NOT NULL,
    template_id TEXT NOT NULL,
    content     TEXT NOT NULL,
    created_at  TEXT NOT NULL,
    PRIMARY KEY (project_id, template_id)
);

CREATE INDEX idx_templates_category ON prompt_templates(category);
CREATE INDEX idx_template_versions_template_id ON prompt_template_versions(template_id);
```

### Migration System (store/store.go)

- `go:embed migrations/*.sql` auto-includes any new `.sql` file in the directory
- No changes to `store.go` needed — the existing glob and migration loop handles new files automatically
- Migrations run in a transaction with rollback on error
- Version extracted via `fmt.Sscanf(name, "%03d_", &version)`
- PRAGMAs: `journal_mode=WAL`, `foreign_keys=ON`

### Store Testing Pattern

```go
func setupTestStore(t *testing.T) *Store {
    t.Helper()
    s, err := New(":memory:")
    require.NoError(t, err)
    t.Cleanup(func() { s.Close() })
    return s
}
```

Use `:memory:` SQLite for all tests. The `New()` function runs migrations automatically, so just creating a store validates that all migrations succeed.

### Existing Template Infrastructure (DO NOT conflict)

`internal/template/manager.go` is the **file-based** template manager from Epic 6. It loads `.tmpl` files from the `templates/` directory. Story 13.1 creates the **database-backed** template system that will eventually replace/complement this file-based system. The two systems coexist — do NOT modify or remove the existing `internal/template/` package in this story.

### Error Handling Pattern

Use existing domain error types from `internal/domain/errors.go`:
- `ValidationError{Field, Message}` for invalid category
- `NotFoundError{Resource, ID}` for missing template (in later stories)

### Project Structure Notes

- New files: `internal/domain/template.go`, `internal/domain/template_test.go`, `internal/store/migrations/003_templates.sql`
- NO modifications to existing files needed (migration glob auto-discovers)
- Alignment with project structure: domain models in `internal/domain/`, migrations in `internal/store/migrations/`

### References

- [Source: _bmad-output/planning-artifacts/architecture.md#807-843] — New SQLite Tables spec
- [Source: _bmad-output/planning-artifacts/architecture.md#931-957] — New & Modified Files spec
- [Source: _bmad-output/planning-artifacts/epics.md#2338-2357] — Story 13.1 AC
- [Source: internal/store/store.go] — Migration system implementation
- [Source: internal/store/migrations/001_initial.sql] — Existing migration 001
- [Source: internal/store/migrations/002_feedback.sql] — Existing migration 002
- [Source: internal/domain/feedback.go] — Validation map pattern reference
- [Source: internal/domain/project.go] — Domain struct pattern reference
- [Source: internal/domain/errors.go] — Error type pattern reference

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6

### Debug Log References

None — clean implementation, no debug issues encountered.

### Completion Notes List

- Created `PromptTemplate`, `TemplateVersion`, `ProjectTemplateOverride` domain models following existing codebase patterns (no JSON tags, `time.Time` timestamps, typed `TemplateCategory` enum)
- Created `ValidTemplateCategories` map for category validation, matching `feedback.go` pattern
- Created `003_templates.sql` migration (not `002` as architecture doc stated — `002_feedback.sql` already exists)
- Migration creates 3 tables + 2 indexes matching architecture spec exactly
- Updated existing `TestNew_MigrationApplied` to expect schema version 3
- Added `TestNew_TemplateTablesCreated` verifying all 3 tables accept valid inserts
- Added `TestNew_TemplateCategoryConstraint` verifying SQL CHECK constraint rejects invalid categories
- Added `TestValidTemplateCategories_AcceptsValidCategories` and `TestValidTemplateCategories_RejectsInvalidCategories` for domain validation
- `make test` — all packages pass, zero regressions
- `make lint` — clean, no issues

### File List

- `internal/domain/template.go` (new) — Domain models: PromptTemplate, TemplateVersion, ProjectTemplateOverride, TemplateCategory
- `internal/domain/template_test.go` (new) — Category validation unit tests
- `internal/store/migrations/003_templates.sql` (new) — Migration: 3 tables + 2 indexes
- `internal/store/store_test.go` (modified) — Updated schema version assertion to 3, added template table and constraint tests
