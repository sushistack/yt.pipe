# Story 19.4: Glossary Suggestion Approval & Integration

Status: done

## Story

As a content creator,
I want to approve or reject suggested terms and have approved terms auto-added to glossary.json,
so that approved terms immediately improve TTS pronunciation accuracy.

## Acceptance Criteria

1. **Given** pending glossary suggestions exist for a project
   **When** `yt-pipe glossary approve <scp-id>` is executed
   **Then** all pending suggestions are listed with index numbers
   **And** the creator can select which suggestions to approve (--select or --all)
   **And** approved suggestions transition to `approved` status
   **And** approved terms are written to the project's `glossary.json` file
   **And** rejected suggestions transition to `rejected` status

2. **Given** a suggestion is approved
   **When** the glossary file is written
   **Then** the existing glossary entries are preserved
   **And** the new term is appended via `Glossary.AddEntry()` method

3. **Given** no pending suggestions exist
   **When** `yt-pipe glossary approve <scp-id>` is executed
   **Then** a message "No pending suggestions" is displayed

## Tasks / Subtasks

- [x] Task 1: Add AddEntry() method to Glossary
  - [x] 1.1: Implement Glossary.AddEntry(Entry) with thread-safe mutex
- [x] Task 2: Add ApproveSuggestion, RejectSuggestion, ListPendingSuggestions to GlossaryService
  - [x] 2.1: ApproveSuggestion — update status + add to glossary via AddEntry
  - [x] 2.2: RejectSuggestion — update status to rejected
  - [x] 2.3: ListPendingSuggestions — delegate to store with pending filter
- [x] Task 3: Create CLI command `yt-pipe glossary approve <scp-id>`
  - [x] 3.1: List pending suggestions with indices
  - [x] 3.2: --all flag to approve all
  - [x] 3.3: --select flag for comma-separated indices
  - [x] 3.4: Save glossary.json preserving existing entries
- [x] Task 4: Unit tests
  - [x] 4.1: ApproveSuggestion — status change + glossary integration
  - [x] 4.2: RejectSuggestion — status change
  - [x] 4.3: ListPendingSuggestions — filter verification

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Completion Notes List

- Added Glossary.AddEntry() method with thread-safe mutex locking
- GlossaryService.ApproveSuggestion updates store status + adds entry to in-memory glossary
- CLI approve command: --all for bulk approve, --select '1,3' for selective, auto-reject unselected
- glossary.json saved via WriteToFile with all entries (existing + approved)
- 3 new test functions for approve/reject/list

### Change Log

- 2026-03-18: Implemented Story 19.4 — Glossary suggestion approval & integration

### File List

- internal/glossary/glossary.go (modified — added AddEntry method)
- internal/service/glossary.go (modified — added ApproveSuggestion, RejectSuggestion, ListPendingSuggestions)
- internal/service/glossary_test.go (modified — added 3 approval tests)
- internal/cli/glossary_cmd.go (modified — added approve subcommand with --all/--select flags)
