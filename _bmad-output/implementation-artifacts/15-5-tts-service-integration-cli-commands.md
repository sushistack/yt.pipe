# Story 15.5: TTS Service Integration & CLI Commands

Status: done

## Story

As a creator,
I want the TTS generation to apply mood presets automatically and CLI commands to manage presets and review mood mappings,
So that narration tone matches each scene's atmosphere with my approval.

## Acceptance Criteria

1. **Given** `service/tts.go` exists with TTS generation logic
   **When** mood preset integration is added
   **Then** before calling `tts.Generate()`, the service retrieves the confirmed mood assignment for the scene
   **And** the assignment's preset is converted to `plugin/tts.MoodPreset` and passed via `TTSOptions`
   **And** if no mood is assigned or not confirmed, `opts` is passed with nil MoodPreset (default tone)

2. **Given** the mood service from Story 15.4
   **When** `yt-pipe mood list` is executed
   **Then** all mood presets are listed showing: id, name, description, speed, emotion, pitch

3. **Given** the creator wants to create a preset
   **When** `yt-pipe mood create --name <name> --speed <f> --emotion <str> --pitch <f> [--description <text>]` is executed
   **Then** the preset is created and its ID is displayed
   **And** this satisfies FR51

4. **Given** the creator wants to update or delete a preset
   **When** `yt-pipe mood update <id> [--name] [--speed] [--emotion] [--pitch]` or `yt-pipe mood delete <id>` is executed
   **Then** the preset is updated or deleted accordingly

5. **Given** a project has scenes with auto-mapped moods pending confirmation
   **When** `yt-pipe mood review <project-id>` is executed
   **Then** each scene is displayed with: scene number, preset ID, auto_mapped status, confirmed status
   **And** the creator can confirm all (`--confirm-all`), confirm individual (`--confirm <scene-num>`), or reassign (`--reassign <scene-num> --preset <id>`)
   **And** this satisfies FR52

## Tasks / Subtasks

- [x] Task 1: TTS Service mood integration (AC: #1)
  - [x] Add `buildTTSOptions(projectID, sceneNum)` method to TTSService
  - [x] Look up `GetSceneMoodAssignment` — return nil if not found or not confirmed
  - [x] Look up `GetMoodPreset` by assignment's preset_id
  - [x] Convert domain MoodPreset to `tts.MoodPreset` and wrap in `tts.TTSOptions`
  - [x] Call `Synthesize`/`SynthesizeWithOverrides` with opts instead of nil
- [x] Task 2: CLI mood list command (AC: #2)
  - [x] `yt-pipe mood list` — tabwriter output with ID, NAME, DESCRIPTION, SPEED, EMOTION, PITCH
  - [x] JSON output support via `--json-output` flag
- [x] Task 3: CLI mood create command (AC: #3)
  - [x] `yt-pipe mood create --name --emotion [--speed] [--pitch] [--description]`
  - [x] Required flags: --name, --emotion
  - [x] Display created preset name and ID
- [x] Task 4: CLI mood update/delete commands (AC: #4)
  - [x] `yt-pipe mood update <id> [--name] [--speed] [--emotion] [--pitch] [--description]`
  - [x] `yt-pipe mood delete <id>`
  - [x] `yt-pipe mood show <id>` — detailed view with params
- [x] Task 5: CLI mood review command (AC: #5)
  - [x] `yt-pipe mood review <project-id>` — show pending confirmations table
  - [x] `--confirm-all` — confirm all pending
  - [x] `--confirm <scene-num>` — confirm individual
  - [x] `--reassign <scene-num> --preset <id>` — reassign with validation

## Dev Notes

### TTS Integration Design

The `buildTTSOptions` method in TTSService follows a three-step lookup:
1. `GetSceneMoodAssignment(projectID, sceneNum)` — check if scene has a mood assignment
2. Check `assignment.Confirmed` — only confirmed moods are applied
3. `GetMoodPreset(assignment.PresetID)` — load the full preset

If any step fails or returns unconfirmed, `nil` opts are returned (default TTS tone).

### CLI Command Structure

```
yt-pipe mood
├── list                    # List all presets
├── create                  # Create new preset (--name, --emotion required)
├── show <id>               # Show preset detail
├── update <id>             # Update preset fields
├── delete <id>             # Delete preset
└── review <project-id>     # Review/confirm mood assignments
    ├── --confirm-all       # Confirm all pending
    ├── --confirm <n>       # Confirm specific scene
    └── --reassign <n> --preset <id>  # Reassign scene
```

### Key Files

| File | Change |
|------|--------|
| `internal/service/tts.go` | Modified — added `buildTTSOptions()`, updated `SynthesizeScene` to use it |
| `internal/cli/mood_cmd.go` | New — mood command group with 6 subcommands |

### Pattern Reference

CLI structure follows the `character_cmd.go` and `template_cmd.go` patterns:
- `openMoodService()` helper for database/service setup
- `cmd.SilenceUsage = true` in all RunE functions
- `tabwriter` for tabular output
- `--json-output` flag support

### References

- [Source: _bmad-output/planning-artifacts/epics.md — Story 15.5 AC]
- [Source: internal/cli/character_cmd.go — CLI pattern reference]
- [Source: internal/cli/template_cmd.go — CLI pattern reference]

## Dev Agent Record

### Agent Model Used
Claude Opus 4.6

### Completion Notes List
- TTSService now looks up confirmed mood assignments before synthesis
- `buildTTSOptions` converts domain MoodPreset to tts.MoodPreset via tts.TTSOptions
- Unconfirmed or missing assignments result in nil opts (default TTS tone)
- CLI `mood` command with 6 subcommands: list, create, show, update, delete, review
- Review command supports --confirm-all, --confirm <n>, --reassign <n> --preset <id>
- All tests pass including updated TTS service tests

### File List
- `internal/service/tts.go` (modified)
- `internal/cli/mood_cmd.go` (new)
