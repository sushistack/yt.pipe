# Story 20.2: Image Validation Domain Model & Storage

Status: done

## Story

As a system,
I want validation scores stored per-shot in the database,
So that quality assessment results persist across pipeline runs and inform approval decisions.

## Acceptance Criteria

1. Migration `015_validation_score.sql` adds `validation_score INTEGER` column (nullable) to `shot_manifests` table
2. `ValidationResult` struct defined in `internal/domain/` with: `Score` (0-100), `PromptMatch` (0-100), `CharacterMatch` (0-100 or -1), `TechnicalScore` (0-100), `Reasons` ([]string), `ShouldRegenerate` (bool)
3. Overall `Score` is computed as a weighted average: `prompt_match*0.5 + character_match*0.3 + technical_score*0.2`. When `CharacterMatch == -1`, weight redistributed to `prompt_match*0.7 + technical_score*0.3`
4. `ShouldRegenerate` is `true` when `Score < threshold`
5. `Store.UpdateValidationScore(projectID, sceneNum, sentenceStart, cutNum, score)` updates `validation_score` column for the matching `shot_manifests` row
6. `Store.GetValidationScore(projectID, sceneNum, sentenceStart, cutNum)` returns `*int` (nil if not validated)
7. `ShotManifest` domain struct extended with `ValidationScore *int` field

## Tasks / Subtasks

- [x] Task 1: Add `ValidationResult` domain model (AC: #2, #3, #4)
  - [x] 1.1 Add `ValidationResult` struct to `internal/domain/validation.go`
  - [x] 1.2 Add `CalculateScore()` method with weighted average logic (character-absent redistribution)
  - [x] 1.3 Add `Evaluate(threshold int)` that sets `Score` and `ShouldRegenerate`

- [x] Task 2: Extend `ShotManifest` with `ValidationScore` (AC: #7)
  - [x] 2.1 Add `ValidationScore *int` field to `ShotManifest` in `internal/domain/manifest.go`

- [x] Task 3: Create DB migration (AC: #1)
  - [x] 3.1 Create `internal/store/migrations/015_validation_score.sql` with `ALTER TABLE shot_manifests ADD COLUMN validation_score INTEGER`

- [x] Task 4: Add store methods (AC: #5, #6)
  - [x] 4.1 Add `UpdateValidationScore(projectID, sceneNum, sentenceStart, cutNum, score int) error` to `internal/store/shot_manifest.go`
  - [x] 4.2 Add `GetValidationScore(projectID, sceneNum, sentenceStart, cutNum int) (*int, error)` to `internal/store/shot_manifest.go`
  - [x] 4.3 Update existing `GetShotManifest` and `ListShotManifestsByScene` to include `validation_score` in SELECT/Scan

- [x] Task 5: Add unit tests
  - [x] 5.1 Test `ValidationResult.CalculateScore()` with character present (weighted: 50/30/20)
  - [x] 5.2 Test `ValidationResult.CalculateScore()` with `CharacterMatch == -1` (redistributed: 70/30)
  - [x] 5.3 Test `Evaluate()` sets `ShouldRegenerate` based on threshold
  - [x] 5.4 Test `Store.UpdateValidationScore()` and `GetValidationScore()` round-trip
  - [x] 5.5 Test `GetValidationScore()` returns nil for unvalidated shots
  - [x] 5.6 Test migration applies cleanly (`:memory:` store creation succeeds via all other store tests)

## Dev Notes

### Architecture Constraints

- Table name is `shot_manifests` (NOT `scene_manifests` — architecture doc reference was outdated)
- Migration number is `015` (next after existing 013_cut_decomposition.sql — 014 may be reserved)
- `validation_score` column is nullable (`INTEGER` without NOT NULL) — nil means "not validated"
- `ValidationResult` is a domain model, NOT a DB entity — only `Score` is persisted to DB

### Existing Patterns to Follow

| Pattern | Location | Description |
|---------|----------|-------------|
| ShotManifest struct | `internal/domain/manifest.go:18-31` | Shot-level manifest with composite key |
| Store CRUD | `internal/store/shot_manifest.go` | Full CRUD on shot_manifests table |
| Migration format | `internal/store/migrations/013_cut_decomposition.sql` | ALTER TABLE pattern |
| Nullable int scan | Use `sql.NullInt64` in store scan, convert to `*int` |
| Composite key | `(project_id, scene_num, sentence_start, cut_num)` — 3-level addressing |

### ValidationResult Score Calculation

```go
func (v *ValidationResult) CalculateScore() {
    if v.CharacterMatch == -1 {
        // No character: redistribute weight
        v.Score = int(float64(v.PromptMatch)*0.7 + float64(v.TechnicalScore)*0.3)
    } else {
        v.Score = int(float64(v.PromptMatch)*0.5 + float64(v.CharacterMatch)*0.3 + float64(v.TechnicalScore)*0.2)
    }
}
```

### Store Method Signatures

```go
// UpdateValidationScore sets the validation_score for a specific shot.
func (s *Store) UpdateValidationScore(projectID string, sceneNum, sentenceStart, cutNum, score int) error

// GetValidationScore returns the validation score for a shot, or nil if not validated.
func (s *Store) GetValidationScore(projectID string, sceneNum, sentenceStart, cutNum int) (*int, error)
```

### SQL for nullable integer scan

```go
var score sql.NullInt64
err := row.Scan(&score)
if score.Valid {
    val := int(score.Int64)
    return &val, nil
}
return nil, nil
```

### Previous Story Intelligence (20-1)

- `ErrNotSupported` sentinel added to `llm` package
- Mock regenerated — includes `CompleteWithVision()`
- FallbackChain supports vision with ErrNotSupported skip

### Project Structure Notes

Files to create:
1. `internal/domain/validation.go` — ValidationResult struct + score calculation
2. `internal/store/migrations/015_validation_score.sql` — ALTER TABLE migration

Files to modify:
3. `internal/domain/manifest.go` — Add `ValidationScore *int` to ShotManifest
4. `internal/store/shot_manifest.go` — Add UpdateValidationScore, GetValidationScore, update existing queries

Test files:
- `internal/domain/validation_test.go` — Score calculation tests
- `internal/store/shot_manifest_test.go` — Store CRUD tests for validation_score

### References

- [Source: _bmad-output/planning-artifacts/architecture.md#EFR3 lines 1381-1466]
- [Source: _bmad-output/planning-artifacts/epics.md#Story 20.2 lines 3616-3640]
- [Source: internal/domain/manifest.go — ShotManifest struct]
- [Source: internal/store/shot_manifest.go — existing CRUD methods]
- [Source: internal/store/migrations/012_shot_manifests.sql — table definition]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Debug Log References

### Completion Notes List

- ValidationResult with CalculateScore() and Evaluate() in domain/validation.go
- ShotManifest extended with ValidationScore *int (nullable)
- Migration 015_validation_score.sql adds nullable INTEGER column
- Store: UpdateValidationScore + GetValidationScore + updated GetShotManifest/ListShotManifestsByScene
- 7 domain tests + 5 store tests added, all passing
- All 7 ACs satisfied

### Change Log

- 2026-03-18: Story 20.2 implementation complete

### File List

- internal/domain/validation.go (new — ValidationResult struct + score calculation)
- internal/domain/validation_test.go (new — 7 tests for score calculation)
- internal/domain/manifest.go (modified — added ValidationScore *int to ShotManifest)
- internal/store/migrations/015_validation_score.sql (new — ALTER TABLE migration)
- internal/store/shot_manifest.go (modified — added UpdateValidationScore, GetValidationScore, updated existing queries)
- internal/store/manifest_test.go (modified — added 5 validation score store tests)
