# Epic 20 Retrospective: AI Image Quality Validation (EFR3)

**Date:** 2026-03-18
**Status:** Done
**Stories:** 5/5 completed

## Summary

Epic 20 implemented automated AI-powered image quality validation using multimodal LLM vision capabilities. The system evaluates generated images against original prompts and character references, automatically regenerating sub-threshold images up to a configurable limit.

## Stories Completed

| Story | Title | Key Deliverable |
|-------|-------|----------------|
| 20-1 | LLM Vision Interface Extension | `CompleteWithVision()` on LLM interface + FallbackChain |
| 20-2 | Image Validation Domain Model & Storage | `ValidationResult` + migration 015 + store CRUD |
| 20-3 | Image Validator Service Core | `ImageValidatorService.ValidateImage()` |
| 20-4 | Validation-Regeneration Loop | `ValidateAndRegenerate()` with callback pattern |
| 20-5 | Pipeline Integration & Config | `SetValidator()` + `ImageValidation` config |

## What Went Well

1. **Clean plugin architecture** — `CompleteWithVision()` followed the established `ErrNotSupported` pattern from `imagegen.Edit()`, making vision support a natural extension
2. **Callback pattern for regeneration** — `RegenerateFn` cleanly avoided circular dependency between `ImageValidatorService` and `ImageGenService`
3. **Backward compatibility** — `image_validation.enabled: false` default means zero impact on existing pipelines
4. **Comprehensive test coverage** — ~30 new tests across plugin, domain, store, and service layers
5. **Incremental story sequencing** — Each story built cleanly on the previous one's outputs

## What Could Be Improved

1. **Code duplication** — `doVisionRequest()` and `doRequest()` in openai.go share ~60 lines of identical HTTP/response logic. Consider extracting a shared `doHTTPRequest()` in a future refactoring pass
2. **`extractValidationJSON()` duplicated** — Same code-fence stripping logic exists in both `openai.go` and `image_validator.go`. Should be extracted to a shared utility
3. **No live API test** — Unlike previous epics, no `liveapi` build-tagged test was added for actual Qwen-VL validation. Consider adding in a future sprint

## Architecture Decisions

- **ErrNotSupported in llm package** — Placed in `interface.go` alongside types (not `errors.go`) for co-location. Both `llm.ErrNotSupported` and `imagegen.ErrNotSupported` exist independently
- **Store optional in ImageValidatorService** — `*store.Store` is nil-safe, allowing tests without DB setup
- **ValidationResult as domain model, not DB entity** — Only `Score` (int) is persisted; full result stays in-memory
- **Non-blocking validation** — Image generation succeeds even when validation encounters errors

## Files Changed

### New Files
- `internal/domain/validation.go` — ValidationResult struct
- `internal/domain/validation_test.go` — Score calculation tests
- `internal/service/image_validator.go` — ImageValidatorService
- `internal/service/image_validator_test.go` — Validator tests
- `internal/service/image_gen_test.go` — Pipeline integration tests
- `internal/store/migrations/015_validation_score.sql` — DB migration

### Modified Files
- `internal/plugin/llm/interface.go` — VisionMessage, ContentPart, ErrNotSupported, CompleteWithVision
- `internal/plugin/llm/openai.go` — Vision request types, CompleteWithVision impl
- `internal/plugin/llm/fallback.go` — CompleteWithVision delegation
- `internal/plugin/llm/openai_test.go` — Vision tests
- `internal/plugin/llm/fallback_test.go` — Vision fallback tests
- `internal/domain/manifest.go` — ShotManifest.ValidationScore
- `internal/store/shot_manifest.go` — UpdateValidationScore, GetValidationScore
- `internal/store/manifest_test.go` — Validation score store tests
- `internal/service/image_gen.go` — SetValidator, SetValidationConfig, validation in GenerateShotImage
- `internal/config/types.go` — ImageValidation config struct
- `internal/config/config.go` — Default values
- `internal/mocks/mock_LLM.go` — Regenerated

## Metrics

- **Total new tests:** ~30
- **New source files:** 6
- **Modified source files:** 12
- **Migration:** 1 (015_validation_score.sql)
- **Config additions:** `image_validation` section with 4 fields

## Next Steps

- **Epic 21 (EFR4/EFR5)** — Can now use `validation_score` for auto-approval and batch review
- Consider live API test with Qwen-VL in a dedicated test sprint
- Monitor validation latency against ENFR2 (5 seconds/image)
