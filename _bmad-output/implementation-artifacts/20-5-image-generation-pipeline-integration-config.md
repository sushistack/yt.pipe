# Story 20.5: Image Generation Pipeline Integration & Config (EFR3)

Status: done

## Story

As a pipeline operator,
I want image validation to be automatically triggered after image generation when enabled,
So that poor-quality images are caught and regenerated without manual intervention.

## Acceptance Criteria

1. `ImageGenService` has a `SetValidator(v *ImageValidatorService)` method following `SetCharacterService()` pattern
2. `ImageGenService` has a `SetValidationConfig(cfg *ValidationConfig)` method for threshold/max_attempts
3. When both validator and validationConfig are set, `GenerateShotImage()` calls `ValidateAndRegenerate()` after successful image generation
4. The `regenerateFn` callback re-generates the image using the same parameters and overwrites the same file path
5. Validation score is logged after validation completes
6. When validator is nil (default), behavior is identical to pre-EFR3 — no validation occurs
7. When vision is not supported (`ValidateAndRegenerate` returns nil,nil), a warning is logged and generation continues
8. `ImageValidation` config struct added to `config/types.go` with `Enabled`, `Threshold` (default 70), `MaxAttempts` (default 3), `Model` (default "qwen-vl-max")
9. Config defaults registered in `config/config.go`

## Tasks / Subtasks

- [x] Task 1: Add `ImageValidation` config struct and defaults (AC: #8, #9)
  - [x] 1.1 Add `ImageValidation` struct to `config/types.go`
  - [x] 1.2 Add `ImageValidation` field to `Config` struct
  - [x] 1.3 Register defaults in `config/config.go` `setDefaults()`

- [x] Task 2: Add validation integration to `ImageGenService` (AC: #1, #2, #3, #6)
  - [x] 2.1 Add `validator *ImageValidatorService` field
  - [x] 2.2 Add `validationConfig *ValidationConfig` field
  - [x] 2.3 Add `ValidationConfig` struct with `Threshold` and `MaxAttempts`
  - [x] 2.4 Add `SetValidator()` method
  - [x] 2.5 Add `SetValidationConfig()` method
  - [x] 2.6 Add validation call after `updateCutManifestImageHash` in `GenerateShotImage()`

- [x] Task 3: Implement regeneration callback (AC: #4)
  - [x] 3.1 Build `regenerateFn` that re-calls `imageGen.Generate()` with same opts/prompt
  - [x] 3.2 Overwrite same `imagePath` with new image data
  - [x] 3.3 Update image hash in manifest after regeneration

- [x] Task 4: Handle edge cases (AC: #5, #7)
  - [x] 4.1 Log validation score on success
  - [x] 4.2 Log warning when validation fails (error)
  - [x] 4.3 Log warning when vision not supported (nil result)

- [x] Task 5: Add unit tests
  - [x] 5.1 Test GenerateShotImage without validator (backward compat)
  - [x] 5.2 Test GenerateShotImage with validator — pass on first attempt
  - [x] 5.3 Test GenerateShotImage with validator — regeneration triggered
  - [x] 5.4 Test GenerateShotImage with validator — validation error logged, generation succeeds
  - [x] 5.5 Test SetValidator / SetValidationConfig setters

## Dev Notes

### ValidationConfig

```go
type ValidationConfig struct {
    Threshold   int
    MaxAttempts int
}
```

### Integration point in GenerateShotImage

After `updateCutManifestImageHash()`, when `s.validator != nil && s.validationConfig != nil`:
1. Build `regenerateFn` that calls `s.imageGen.Generate()` and writes to same `imagePath`
2. Call `s.validator.ValidateAndRegenerate(ctx, imagePath, prompt, opts.CharacterRefs, threshold, maxAttempts, projectID, sceneNum, sentenceStart, cutNum, regenerateFn)`
3. Log result or warning on error

### Config YAML example

```yaml
image_validation:
  enabled: true
  threshold: 70
  max_attempts: 3
  model: qwen-vl-max
```
