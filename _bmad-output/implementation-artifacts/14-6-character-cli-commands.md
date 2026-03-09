# Story 14.6: Character CLI Commands

Status: done

## Story

As a creator,
I want CLI commands to create, list, view, update, and delete character ID cards,
So that I can manage my character visual presets from the command line.

## Tasks / Subtasks

- [x] Task 1: Implement `yt-pipe character create` with all flags
- [x] Task 2: Implement `yt-pipe character list [--scp-id]`
- [x] Task 3: Implement `yt-pipe character show <id>`
- [x] Task 4: Implement `yt-pipe character update <id>` with partial update flags
- [x] Task 5: Implement `yt-pipe character delete <id>`
- [x] Task 6: Verify build + all tests pass + lint clean

## Dev Agent Record

### Agent Model Used
Claude Opus 4.6

### Completion Notes List
- Created character_cmd.go following template_cmd.go pattern
- Commands: create, list, show, update, delete
- Flags: --scp-id, --name, --aliases (comma-separated), --visual (text or @file), --style, --prompt-base
- JSON output supported via --json-output flag
- Table output with tabwriter for list command
- readTextOrFile helper: @filepath reads from file, plain text used directly
- All existing tests pass, build succeeds, lint clean

### File List
- `internal/cli/character_cmd.go` (new)
