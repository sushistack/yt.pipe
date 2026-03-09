# Story 17.3: BGM Store — CRUD, Tag Search & Scene Assignment

Status: done

## Story

As a developer,
I want a store layer with CRUD operations for BGMs and scene assignments, including mood tag search with relevance ranking,
So that the BGM library can be persistently managed and queried.

## Acceptance Criteria

1. **Given** the store package and migration `006_bgms.sql`
   **When** `CreateBGM()` is called with a valid `*domain.BGM`
   **Then** the BGM is inserted into the `bgms` table with JSON-encoded `mood_tags`
   **And** `CreatedAt` is set to the current UTC time

2. **Given** a BGM exists in the database
   **When** `GetBGM(id)` is called
   **Then** the BGM is returned with `MoodTags` deserialized from JSON
   **And** `LicenseType` is correctly cast from string
   **And** `NotFoundError` is returned for non-existent IDs

3. **Given** multiple BGMs exist
   **When** `ListBGMs()` is called
   **Then** all BGMs are returned ordered by name

4. **Given** a BGM exists
   **When** `UpdateBGM()` is called with modified fields
   **Then** all fields are updated in the database
   **And** `NotFoundError` is returned for non-existent IDs

5. **Given** a BGM exists
   **When** `DeleteBGM(id)` is called
   **Then** the BGM is removed if no scene assignments reference it
   **And** an error is returned if scene assignments exist (delete protection)
   **And** `NotFoundError` is returned for non-existent IDs

6. **Given** BGMs with mood tags `["tense","dark"]` and `["calm","peaceful"]`
   **When** `SearchByMoodTags(["tense","mysterious"])` is called
   **Then** results are ranked by match count (CASE/SUM pattern)
   **And** BGMs with more matching tags appear first

7. **Given** a project and scene
   **When** `AssignBGMToScene()` is called
   **Then** the assignment is created with `ON CONFLICT DO UPDATE` (upsert)
   **And** subsequent calls replace the existing assignment

8. **Given** scene BGM assignments exist
   **When** `ConfirmSceneBGM()`, `GetSceneBGMAssignment()`, `ListSceneBGMAssignments()` are called
   **Then** they correctly read/update boolean fields (INTEGER 0/1 in SQLite)

## Tasks / Subtasks

- [x] Task 1: Implement BGM CRUD operations (AC: #1-#5)
  - [x] 1.1: `CreateBGM()` — INSERT with JSON-encoded mood_tags
  - [x] 1.2: `GetBGM()` — SELECT with JSON deserialization and NotFoundError
  - [x] 1.3: `ListBGMs()` — SELECT all ordered by name
  - [x] 1.4: `UpdateBGM()` — UPDATE all fields with NotFoundError check
  - [x] 1.5: `DeleteBGM()` — DELETE with assignment protection check
  - [x] 1.6: `scanBGMs()` helper for row scanning
- [x] Task 2: Implement tag search (AC: #6)
  - [x] 2.1: `SearchByMoodTags()` — SQL LIKE with CASE/SUM ranking
- [x] Task 3: Implement scene assignment operations (AC: #7-#8)
  - [x] 3.1: `AssignBGMToScene()` — INSERT ON CONFLICT DO UPDATE
  - [x] 3.2: `ConfirmSceneBGM()` — SET confirmed=1
  - [x] 3.3: `GetSceneBGMAssignment()` — single scene lookup
  - [x] 3.4: `ListSceneBGMAssignments()` — all assignments for project
- [x] Task 4: Write comprehensive tests (AC: all)
  - [x] 4.1: CRUD tests (create, get, list, update, delete)
  - [x] 4.2: Tag search ranking tests
  - [x] 4.3: Scene assignment upsert/confirm tests
  - [x] 4.4: Delete protection test
- [x] Task 5: Run full test suite
  - [x] 5.1: `make test` passes
  - [x] 5.2: `make lint` passes

## Dev Notes

### Key Implementation Patterns

**JSON in TEXT column for mood_tags:**
```go
tagsJSON, err := json.Marshal(b.MoodTags)
// stored as TEXT: '["tense","dark"]'
```

**CASE/SUM ranking for SearchByMoodTags:**
```go
// Each tag generates: CASE WHEN mood_tags LIKE '%"tag"%' THEN 1 ELSE 0 END
// SUM of all CASE expressions gives relevance rank
// Results ordered by rank DESC, then name
```

**Upsert pattern for AssignBGMToScene:**
```go
INSERT INTO scene_bgm_assignments (...) VALUES (...)
ON CONFLICT(project_id, scene_num) DO UPDATE SET
  bgm_id=excluded.bgm_id, volume_db=excluded.volume_db, ...
```

**Delete protection:**
```go
// Check COUNT(*) FROM scene_bgm_assignments WHERE bgm_id = ?
// Fail with descriptive error if count > 0
```

**Boolean handling in SQLite:**
```go
boolToInt(a.AutoRecommended) // true→1, false→0
a.AutoRecommended = autoRec == 1 // 1→true, 0→false
```

### Note on `boolToInt`

The `boolToInt` helper already exists in `template.go` within the same `store` package. Initially a duplicate was introduced in `bgm.go` but was caught at compile time and removed. Shared helpers should be checked before adding new ones.

### Files Touched

| File | Change |
|------|--------|
| `internal/store/bgm.go` | New — full CRUD, tag search, scene assignment operations |
| `internal/store/bgm_test.go` | New — 14 test cases covering all operations |

### Testing Standards

- 14 test cases covering: create, get, get-not-found, list, update, update-not-found, delete, delete-not-found, delete-protection, search-by-tags, assign-upsert, confirm, get-assignment, list-assignments
- Uses testify assertions
- Each test opens a fresh in-memory SQLite database

### References

- [Source: internal/store/bgm.go] — full implementation
- [Source: internal/store/bgm_test.go] — 14 comprehensive tests
- [Pattern: internal/store/character.go] — Epic 14 CRUD pattern reference

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6

### Completion Notes List

- Implemented full CRUD: `CreateBGM`, `GetBGM`, `ListBGMs`, `UpdateBGM`, `DeleteBGM`
- Implemented `SearchByMoodTags` with SQL LIKE + CASE/SUM relevance ranking
- Implemented scene assignment operations: `AssignBGMToScene`, `ConfirmSceneBGM`, `GetSceneBGMAssignment`, `ListSceneBGMAssignments`
- Delete protection prevents orphaned scene_bgm_assignments references
- Upsert pattern (ON CONFLICT DO UPDATE) for scene assignments
- All 14 test cases pass, 20 packages green

### File List

- `internal/store/bgm.go` (new) — BGM store CRUD, tag search, scene assignment
- `internal/store/bgm_test.go` (new) — 14 comprehensive test cases
