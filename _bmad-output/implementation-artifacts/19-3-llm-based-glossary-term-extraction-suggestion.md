# Story 19.3: LLM-Based Glossary Term Extraction & Suggestion

Status: done

## Story

As a content creator,
I want the system to auto-extract SCP terms from scenario text and suggest pronunciations,
so that I don't have to manually identify new terms for the glossary.

## Acceptance Criteria

1. **Given** a project with an approved scenario
   **When** `yt-pipe glossary suggest <scp-id>` is executed
   **Then** the scenario text is sent to LLM with existing glossary entries as context
   **And** LLM returns JSON array of `[{term, pronunciation, definition, category}]`
   **And** results are diffed against existing glossary — only new terms are stored as `pending` suggestions
   **And** pending suggestions are displayed to the creator

2. **Given** the LLM returns invalid JSON or an error
   **When** suggestion extraction runs
   **Then** a clear error message is displayed and no partial data is persisted

3. **Given** an empty scenario or a scenario with no new terms
   **When** suggestion extraction runs
   **Then** a message "No new terms found" is displayed and no suggestions are created

## Tasks / Subtasks

- [x] Task 1: Create GlossaryService with SuggestTerms method
  - [x] 1.1: Build LLM prompt with existing glossary context
  - [x] 1.2: Parse LLM JSON response (with code block stripping)
  - [x] 1.3: Diff against existing glossary, skip duplicates
  - [x] 1.4: Store new suggestions via store.CreateGlossarySuggestion
- [x] Task 2: Create CLI command `yt-pipe glossary suggest <scp-id>`
  - [x] 2.1: Parent `glossary` command + `suggest` subcommand
  - [x] 2.2: Load scenario, glossary, LLM plugin, store
  - [x] 2.3: Display results with term, pronunciation, definition, category
- [x] Task 3: Unit tests
  - [x] 3.1: Success with mock LLM response
  - [x] 3.2: Filter existing glossary terms
  - [x] 3.3: Invalid JSON → error, no partial data
  - [x] 3.4: Empty results
  - [x] 3.5: LLM error propagation
  - [x] 3.6: Code block stripping
  - [x] 3.7: UNIQUE constraint duplicate skipping

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Completion Notes List

- GlossaryService follows existing bgm.go/mood.go LLM JSON extraction pattern
- Prompt includes existing glossary terms to avoid duplication
- Code block stripping handles ```json``` wrapped responses
- UNIQUE constraint violations silently skipped (duplicate term+project_id)
- 8 test functions with mock LLM covering all ACs

### Change Log

- 2026-03-18: Implemented Story 19.3 — LLM-based glossary term extraction

### File List

- internal/service/glossary.go (new — GlossaryService, SuggestTerms, prompt builder)
- internal/service/glossary_test.go (new — 8 tests)
- internal/cli/glossary_cmd.go (new — glossary parent + suggest subcommand)
