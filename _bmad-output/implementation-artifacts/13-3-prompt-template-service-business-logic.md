# Story 13.3: Prompt Template Service — Business Logic

Status: done

## Story

As a developer,
I want a template service that orchestrates template CRUD, version limits, and project-scoped resolution,
So that CLI and API layers have a clean interface for template management.

## Acceptance Criteria

1. **AC1: Service CRUD with Validation**
   - `CreateTemplate(category, name, content)` validates category, generates UUID, and delegates to store
   - `UpdateTemplate(id, content)` retrieves current template, delegates update to store, enforces the 10-version limit
   - `RollbackTemplate(id, version)` validates version exists before delegating to store
   - `DeleteTemplate(id)` prevents deletion of default templates (`is_default=1`)

2. **AC2: Template Resolution with Override Fallback (FR47)**
   - `ResolveTemplate(projectID, templateID)` returns the project override content if it exists
   - Falls back to the global template content if no override exists

3. **AC3: Unit Tests with Mocked Store**
   - Business rules tested: category validation, version limit enforcement, default template protection, resolution priority

## Tasks / Subtasks

- [x] Task 1: Implement service layer (AC: #1, #2)
  - [x] 1.1 `CreateTemplate()` — validate category/name/content, generate UUID, delegate to store
  - [x] 1.2 `UpdateTemplate()` — retrieve + delegate + version limit enforcement
  - [x] 1.3 `RollbackTemplate()` — validate version existence before delegating
  - [x] 1.4 `DeleteTemplate()` — check `is_default` flag, reject deletion of defaults
  - [x] 1.5 `ResolveTemplate()` — override-first resolution with global fallback
  - [x] 1.6 `ListTemplates()`, `GetTemplate()`, `GetTemplateVersion()`, `ListTemplateVersions()`
  - [x] 1.7 `SetOverride()`, `DeleteOverride()` — pass-through to store
  - [x] 1.8 `InstallDefaults()` — install 4 default templates (idempotent)
- [x] Task 2: Write tests (AC: #3)
  - [x] 2.1 Template lifecycle tests (create, update, rollback, delete)
  - [x] 2.2 Default template protection test
  - [x] 2.3 Override resolution priority test
  - [x] 2.4 Default installation idempotency test

## Dev Notes

### Default Templates

`InstallDefaults()` embeds 4 default templates via `//go:embed default_templates/*.md`:
- `scenario.md` → "SCP Research & Analysis" (category: scenario)
- `image.md` → "Cinematic Shot Breakdown" (category: image)
- `tts.md` → "Korean TTS Preprocessing" (category: tts)
- `caption.md` → "Korean Subtitle Generation" (category: caption)

Idempotent: checks if defaults already exist by listing templates and comparing names. Skips if found.

### Service Pattern

Follows existing service layer patterns:
- Constructor: `NewTemplateService(store TemplateStore)` with interface dependency
- UUID generation: `github.com/google/uuid`
- Validation before delegation to store
- Domain error types for business rule violations

## Dev Agent Record

### Completion Notes List

- Implemented 210-line service layer in `internal/service/template.go`
- Created `internal/service/default_templates/` directory with 4 embedded .md files
- Business rules: category validation, default template protection, override-first resolution
- `InstallDefaults()` for idempotent seeding of 4 default templates
- 221 lines of tests covering all business rules
- `make test` — all pass, zero regressions
- `make lint` — clean

### File List

- `internal/service/template.go` (new, 210 lines) — Template service with business logic
- `internal/service/template_test.go` (new, 221 lines) — Service unit tests
- `internal/service/default_templates/scenario.md` (new) — Default scenario template
- `internal/service/default_templates/image.md` (new) — Default image template
- `internal/service/default_templates/tts.md` (new) — Default TTS template
- `internal/service/default_templates/caption.md` (new) — Default caption template
