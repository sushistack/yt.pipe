# Story 14.3: Character Store — CRUD & Alias Search

Status: done

## Story

As a developer,
I want a character store with CRUD operations and alias-based search capability,
So that the service layer can manage character ID cards and find characters by name/alias.

## Acceptance Criteria

1. Create(character) inserts with JSON-serialized aliases
2. Get(id) returns character with deserialized aliases
3. ListBySCPID(scpID) returns all characters for a given SCP entity
4. ListAll() returns all characters
5. Update(character) updates all fields and timestamps
6. Delete(id) removes the character
7. SearchByName(name) returns matches on canonical_name OR aliases (case-insensitive)

## Tasks / Subtasks

- [x] Task 1: Implement Character CRUD in store/character.go
- [x] Task 2: Implement SearchCharactersByName with case-insensitive alias matching
- [x] Task 3: Write comprehensive tests in store/character_test.go

## Dev Agent Record

### Agent Model Used
Claude Opus 4.6

### Completion Notes List
- Full CRUD: Create, Get, ListBySCPID, ListAll, Update, Delete
- SearchCharactersByName: case-insensitive match on canonical_name + LIKE on aliases JSON
- JSON serialization/deserialization of aliases []string
- 14 test cases covering all operations, edge cases (nil aliases, not found, case insensitive)

### File List
- `internal/store/character.go` (new)
- `internal/store/character_test.go` (new)
