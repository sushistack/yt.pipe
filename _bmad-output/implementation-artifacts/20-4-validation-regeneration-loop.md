# Story 20.4: Validation-Regeneration Loop (EFR3)

Status: done

## Story

As a system,
I want automatic regeneration when image quality falls below threshold,
So that the pipeline can self-correct poor image generations without human intervention.

## Acceptance Criteria

1. `ValidateAndRegenerate()` method on `ImageValidatorService` accepts a `RegenerateFn` callback to avoid circular dependency
2. Image is validated via `ValidateImage()` — if score >= threshold, result is returned (pass)
3. If score < threshold, `regenerateFn` is called and image is re-validated, up to `maxAttempts` times
4. Each attempt's score and reason are logged
5. Final `validation_score` is persisted to `shot_manifests` via `store.UpdateValidationScore()`
6. When all attempts fail, the best-scoring image is kept, `ShouldRegenerate: false` (exhausted), warning logged with all scores
7. When LLM does not support vision (`ValidateImage` returns nil,nil), returns nil,nil (skip)

## Tasks / Subtasks

- [x] Task 1: Add `*store.Store` dependency to `ImageValidatorService` (AC: #5)
  - [x] 1.1 Add `store` field to struct
  - [x] 1.2 Update `NewImageValidatorService` constructor to accept `*store.Store`
  - [x] 1.3 Update existing test `setupValidator` to pass store (nil for existing tests)

- [x] Task 2: Implement `ValidateAndRegenerate()` method (AC: #1-#6)
  - [x] 2.1 Define `RegenerateFn` type alias
  - [x] 2.2 Implement loop: validate → check threshold → regenerate → repeat
  - [x] 2.3 Track best-scoring result across attempts
  - [x] 2.4 Log each attempt's score and reasons
  - [x] 2.5 Persist final score via `store.UpdateValidationScore()`
  - [x] 2.6 Handle all-attempts-failed: keep best, set `ShouldRegenerate: false`, warn log

- [x] Task 3: Handle edge cases (AC: #7)
  - [x] 3.1 Return nil,nil when ValidateImage returns nil,nil (vision not supported)
  - [x] 3.2 Return error when ValidateImage fails
  - [x] 3.3 Return error when regenerateFn fails

- [x] Task 4: Add unit tests
  - [x] 4.1 Test pass on first attempt (score >= threshold)
  - [x] 4.2 Test regeneration then pass on second attempt
  - [x] 4.3 Test all attempts exhausted — best score kept
  - [x] 4.4 Test vision not supported returns nil,nil
  - [x] 4.5 Test regenerateFn error propagated
  - [x] 4.6 Test ValidateImage error propagated
  - [x] 4.7 Test score persisted to store
  - [x] 4.8 Test maxAttempts default (0 → 1)

## Dev Notes

### Method Signature

```go
type RegenerateFn func(ctx context.Context) error

func (s *ImageValidatorService) ValidateAndRegenerate(
    ctx context.Context,
    imagePath string,
    originalPrompt string,
    characterRefs []imagegen.CharacterRef,
    threshold int,
    maxAttempts int,
    projectID string,
    sceneNum int,
    sentenceStart int,
    cutNum int,
    regenerateFn RegenerateFn,
) (*domain.ValidationResult, error)
```

### Store Dependency

`ImageValidatorService` gains an optional `*store.Store` field. When non-nil, final score is persisted. Constructor updated but existing callers can pass nil.

### Regeneration Loop Logic

```
best = nil
for attempt := 1; attempt <= maxAttempts; attempt++ {
    result = ValidateImage(...)
    if result == nil { return nil, nil }  // vision not supported
    result.Evaluate(threshold)
    log attempt score
    if best == nil || result.Score > best.Score { best = result }
    if !result.ShouldRegenerate { break }  // passed
    if attempt < maxAttempts { regenerateFn(ctx) }
}
if best.ShouldRegenerate { best.ShouldRegenerate = false; warn }
persist best.Score
return best, nil
```

### Previous Story Outputs

- Story 20-1: `CompleteWithVision()`, `ErrNotSupported`, FallbackChain
- Story 20-2: `ValidationResult` with `CalculateScore()`/`Evaluate()`, `Store.UpdateValidationScore()`
- Story 20-3: `ImageValidatorService` with `ValidateImage()`

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Completion Notes List

- Added `*store.Store` as optional dependency to `ImageValidatorService` (nil-safe)
- `RegenerateFn` type alias: `func(ctx context.Context) error` — callback pattern avoids circular dependency
- `ValidateAndRegenerate()` loop: validate → evaluate threshold → track best → regenerate if needed → repeat up to maxAttempts
- Best-scoring result kept across all attempts; `ShouldRegenerate` set to false when attempts exhausted
- Final score persisted via `store.UpdateValidationScore()` when store is non-nil; failure logged as warning (non-fatal)
- maxAttempts < 1 defaults to 1 (safety guard)
- 8 unit tests covering: first-attempt pass, second-attempt pass, all-exhausted, vision-not-supported, regenerateFn error, validateImage error, store persistence, maxAttempts default

### Change Log

- 2026-03-18: Story 20.4 implementation complete

### File List

- internal/service/image_validator.go (modified — added store dependency, RegenerateFn type, ValidateAndRegenerate method)
- internal/service/image_validator_test.go (modified — updated setupValidator, added 8 new tests)
