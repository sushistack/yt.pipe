# Story 8-7: Minimal E2E Smoke Test

## Status: Done

## Implementation Summary

### New Files
- `internal/service/scenario_pipeline_test.go` — Comprehensive pipeline tests using mock LLM provider:
  - `TestScenarioPipeline_Run_Success` — Full 4-stage pipeline E2E: correct stage ordering, inter-stage data passing, token usage aggregation, artifact file creation
  - `TestScenarioPipeline_Run_ResumeFromCheckpoint` — Checkpoint/resume: pre-creates stage 1+2 JSON files, verifies only stages 3+4 are called
  - `TestScenarioPipeline_GlossaryInjection` — Empty glossary produces empty section
  - `TestScenarioPipeline_StageFailure` — Stage 1 failure produces descriptive error
  - `TestExtractVisualIdentity` — Extracts Frozen Descriptor section from research output
  - `TestExtractVisualIdentity_NotFound` — Fallback when section missing
  - `TestParseScenarioFromWriting` — Parses writing stage JSON into ScenarioOutput
  - `TestParseReviewReport` — Parses review stage JSON into ReviewReport
  - `TestApplyCorrections` — Patch-based corrections applied to narration
  - `TestExtractJSONFromContent` — Handles plain JSON and fenced code blocks

### Architecture Decisions
- All tests use mock LLM provider from `internal/mocks` (no real API calls)
- Mock uses `testify/mock` with `MatchedBy` matchers to verify correct prompt content per stage
- Test validates: stage ordering, inter-stage data passing, checkpoint creation, error handling, token aggregation
- Tests complete in under 5 seconds (no network calls)
- Real API integration tests deferred to Story 12.4

### Acceptance Criteria Met
- [x] Mock-based unit tests validate 4-stage pipeline orchestration
- [x] Correct stage ordering verified
- [x] Inter-stage data passing verified
- [x] Checkpoint creation and resume verified
- [x] Error handling verified
- [x] No real API calls — all mocked
- [x] Tests complete in under 5 seconds
- [x] All tests pass
