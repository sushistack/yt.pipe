# Story 14.4: Character Service — CRUD & Scene Text Matching

Status: done

## Story

As a developer,
I want a character service that manages ID cards and matches character names in scene text,
So that the image generation pipeline can automatically inject character visual references.

## Tasks / Subtasks

- [x] Task 1: Implement CharacterService CRUD (Create, Get, List, Update, Delete)
- [x] Task 2: Implement MatchCharacters(scpID, sceneText) returning []CharacterRef
- [x] Task 3: Write comprehensive tests (15 test cases)

## Dev Agent Record

### Agent Model Used
Claude Opus 4.6

### Completion Notes List
- CharacterService: CRUD with validation (empty scp_id, name, invalid aliases)
- MatchCharacters: loads SCP-specific + all characters, deduplicates by ID, case-insensitive matching on canonical_name + aliases
- Returns []imagegen.CharacterRef for direct plugin consumption
- 15 test cases: CRUD, validation, matching (canonical, alias, multiple, dedup, case-insensitive, no match)

### File List
- `internal/service/character.go` (new)
- `internal/service/character_test.go` (new)
