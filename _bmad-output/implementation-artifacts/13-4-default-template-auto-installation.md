# Story 13.4: Default Template Auto-Installation

Status: done

## Story

As a creator,
I want default prompt templates to be automatically installed during initial setup,
So that I can start using the pipeline immediately with proven templates from video.pipeline.

## Acceptance Criteria

1. **AC1: Default Templates Installed on Init (FR61)**
   - During `yt-pipe init`, default templates are created for all 4 categories: scenario, image, tts, caption
   - Each default template has `is_default=1`
   - Template content is loaded from embedded template files

2. **AC2: Idempotent Seeding**
   - Running `yt-pipe init` again does not overwrite or duplicate existing default templates
   - A log message indicates "Default templates already installed, skipping"

3. **AC3: Init Command Integration**
   - Seeding logic calls `service/template.go` `InstallDefaults()` for each default template
   - Existing init functionality (API keys, config) is not affected

## Tasks / Subtasks

- [x] Task 1: Integrate default template installation into init command (AC: #1, #2, #3)
  - [x] 1.1 Add `installDefaultTemplates()` helper function to `internal/cli/init_cmd.go`
  - [x] 1.2 Call helper after config generation in interactive mode (line ~281-288)
  - [x] 1.3 Call helper after config generation in non-interactive mode (line ~359-366)
  - [x] 1.4 Helper opens DB, creates template service, calls `InstallDefaults()`
- [x] Task 2: Verify idempotency (AC: #2)
  - [x] 2.1 `InstallDefaults()` checks for existing defaults before creating
  - [x] 2.2 Test double-init scenario

## Dev Notes

### Integration Point

`installDefaultTemplates()` in `internal/cli/init_cmd.go` (lines 540-551):
1. Opens SQLite store via `store.New(dbPath)`
2. Creates `service.NewTemplateService(store)`
3. Calls `service.InstallDefaults(ctx)`
4. Logs success or skip message

Called from two paths in the init wizard:
- Interactive mode: after config file generation (line ~281)
- Non-interactive mode: after config file generation (line ~359)

### Note

The actual `InstallDefaults()` implementation lives in `service/template.go` (Story 13.3). This story only adds the init command integration that calls it.

## Dev Agent Record

### Completion Notes List

- Added `installDefaultTemplates()` helper to init_cmd.go
- Integrated into both interactive and non-interactive init paths
- Existing init functionality unchanged — template installation is additive
- Idempotency guaranteed by `InstallDefaults()` logic in service layer
- `make test` — all pass, zero regressions
- `make lint` — clean

### File List

- `internal/cli/init_cmd.go` (modified) — Added `installDefaultTemplates()` helper, integrated into init wizard
