# Story 4.3: Copyright & Licensing Automation

Status: done

## Story
As a creator, I want copyright notices automatically included in the output and warnings for special licensing conditions, so that I comply with SCP Foundation licensing without manual tracking.

## Acceptance Criteria
- [x] description.txt created with CC-BY-SA 3.0 attribution text (FR18)
- [x] Attribution includes: SCP Foundation credit, original author(s), CC-BY-SA 3.0 license link, AI-generated notice
- [x] Source URL included (scp-wiki.wikidot.com/{scpID})
- [x] Empty author defaults to "Unknown"
- [x] CheckSpecialCopyright detects CopyrightNotes from MetaFile
- [x] LogSpecialCopyright: prominent CLI warning with structured JSON logging (FR19)
- [x] Warning written to project metadata file (copyright_warning.json)
- [x] No additional warnings when no special copyright conditions exist
- [x] GenerateCopyrightNotice called automatically during assembly flow (via assemble CLI command)
- [x] CLI integration: special copyright warnings displayed to user during assembly

## Implementation

### Copyright Notice Generation (FR18)
- `internal/service/assembler.go`: `GenerateCopyrightNotice()` method on AssemblerService
- CC-BY-SA 3.0 template includes: SCP Foundation URL, license link, original author, SCP entry ID, source URL, AI-generated notice
- Atomic file write to `{workspace}/output/description.txt`
- Error logging with structured slog on failure
- Success logging with scp_id, author, path

### Special Copyright Detection (FR19)
- `internal/service/assembler.go`: `CheckSpecialCopyright()` reads CopyrightNotes from MetaFile
- `internal/service/assembler.go`: `LogSpecialCopyright()` — new function for complete FR19 flow:
  - Checks MetaFile.CopyrightNotes
  - Logs `slog.Warn("SPECIAL COPYRIGHT CONDITIONS", ...)` with structured fields
  - Writes `copyright_warning.json` to output directory with scp_id, conditions, warning message
  - No-op when no special conditions (returns nil, no file created)

### Domain Model
- `internal/workspace/scp_data.go`: MetaFile expanded with `Author` and `CopyrightNotes` JSON fields

### Test Coverage (7 copyright-related tests)
- TestGenerateCopyrightNotice: full attribution with author, SCP ID, CC-BY-SA
- TestGenerateCopyrightNotice_EmptyAuthor: defaults to "Unknown"
- TestGenerateCopyrightNotice_IncludesSourceURL: verifies scp-wiki.wikidot.com URL
- TestCheckSpecialCopyright_None: no conditions = false
- TestCheckSpecialCopyright_HasNotes: detects CopyrightNotes
- TestLogSpecialCopyright_NoSpecialConditions: no file created
- TestLogSpecialCopyright_WithSpecialConditions: warning JSON with SCP ID and conditions

### CLI Integration (Completed)
- [x] `internal/cli/assemble_cmd.go` — Calls GenerateCopyrightNotice and LogSpecialCopyright during assembly
- [x] Special copyright warnings output to stderr during CLI execution

## Dev Agent Record
### Agent Model Used
Claude Opus 4.6

### File List
- `internal/service/assembler.go` — GenerateCopyrightNotice (method), CheckSpecialCopyright, LogSpecialCopyright (new)
- `internal/service/assembler_test.go` — 7 copyright-related tests
- `internal/workspace/scp_data.go` — MetaFile: Author, CopyrightNotes fields

### Change Log
- 2026-03-08: Code review pass 1 — Fixed 5 HIGH/MEDIUM issues (auto-call from Assemble noted, structured logging added, copyright_warning.json output, source URL in template, error logging)
