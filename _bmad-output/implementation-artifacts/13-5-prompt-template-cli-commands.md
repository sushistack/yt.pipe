# Story 13.5: Prompt Template CLI Commands

Status: done

## Story

As a creator,
I want CLI commands to list, view, create, update, rollback, and override prompt templates,
So that I can manage my prompt library from the command line.

## Acceptance Criteria

1. **AC1: List Templates (FR45)**
   - `yt-pipe prompt list [--category scenario|image|tts|caption]` lists all templates filtered by category
   - Shows: id, name, category, version, is_default

2. **AC2: Show Template**
   - `yt-pipe prompt show <template-id> [--version N]` displays template content
   - Shows specific version content if `--version` is specified

3. **AC3: Create Template (FR45)**
   - `yt-pipe prompt create --category <cat> --name <name> --file <path>` creates template from file content
   - Version 1 is recorded

4. **AC4: Update Template (FR45)**
   - `yt-pipe prompt update <template-id> --file <path>` updates template content and creates new version

5. **AC5: Rollback (FR46)**
   - `yt-pipe prompt rollback <template-id> --version <N>` rolls back to specified version
   - Confirmation message shows rollback details

6. **AC6: Project Override (FR47)**
   - `yt-pipe prompt override <template-id> --project <project-id> --file <path>` saves project override
   - `yt-pipe prompt override <template-id> --project <project-id> --delete` removes override

7. **AC7: Delete Template**
   - `yt-pipe prompt delete <template-id>` deletes template (protected for defaults)

## Tasks / Subtasks

- [x] Task 1: Implement CLI subcommands (AC: #1-#7)
  - [x] 1.1 `prompt list` — tabular output with optional `--category` filter
  - [x] 1.2 `prompt show` — full template content, optional `--version` flag
  - [x] 1.3 `prompt create` — reads file, calls service.CreateTemplate
  - [x] 1.4 `prompt update` — reads file, calls service.UpdateTemplate
  - [x] 1.5 `prompt rollback` — calls service.RollbackTemplate
  - [x] 1.6 `prompt delete` — calls service.DeleteTemplate (rejects defaults)
  - [x] 1.7 `prompt override` — set (with `--file`) or delete (with `--delete`)
- [x] Task 2: Write CLI tests (AC: all)
  - [x] 2.1 List, create, update, rollback, delete operation tests
  - [x] 2.2 Default template protection test
  - [x] 2.3 Project override workflow test

## Dev Notes

### CLI Structure

`internal/cli/template_cmd.go` (311 lines) registers `prompt` as a root subcommand with 7 sub-subcommands:

```
yt-pipe prompt list [--category CAT]
yt-pipe prompt show <id> [--version N]
yt-pipe prompt create --category CAT --name NAME --file FILE
yt-pipe prompt update <id> --file FILE
yt-pipe prompt rollback <id> --version N
yt-pipe prompt delete <id>
yt-pipe prompt override <id> --project PID [--file FILE | --delete]
```

### Output Format

- `list`: Tabular format — ID (truncated 8 chars), Category, Name, Version, Default flag
- `show`: Full content with header metadata
- Mutating commands: Success/error messages with relevant IDs

### Pattern

Follows existing CLI patterns from `config_cmd.go`:
- cobra.Command with RunE
- Flag validation in PreRunE
- Service initialization via store.New() + service.New()

## Dev Agent Record

### Completion Notes List

- Implemented 311-line CLI module in `internal/cli/template_cmd.go`
- 7 subcommands covering all FR45/FR46/FR47 requirements
- Tabular output for list, full content display for show
- 155 lines of CLI tests covering all operations
- `make test` — all pass, zero regressions
- `make lint` — clean

### File List

- `internal/cli/template_cmd.go` (new, 311 lines) — Prompt template CLI commands
- `internal/cli/template_cmd_test.go` (new, 155 lines) — CLI command tests
