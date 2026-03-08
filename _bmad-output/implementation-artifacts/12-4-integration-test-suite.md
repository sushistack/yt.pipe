# Story 12.4: Integration Test Suite

Status: done

## Story

As a creator,
I want a comprehensive integration test suite that validates the full pipeline with real APIs,
So that regressions are caught before they affect my production workflow.

## Acceptance Criteria

1. Integration tests use `//go:build integration` build tag, skipped in regular `go test ./...`
2. Tests skip with descriptive message when API keys are not set
3. Test cases: TestGeminiScenarioGeneration, TestSiliconFlowImageGeneration, TestDashScopeTTSSynthesis, TestFullPipelineE2E, TestFallbackChainActivation
4. Appropriate timeouts: scenario 120s, image 60s, TTS 30s, E2E 300s
5. Tests clean up workspace after completion
6. `make test-integration` target wraps `go test -tags=integration -timeout 600s ./...`
7. `make test` runs only unit tests (no build tag)

## Tasks / Subtasks

- [ ] Task 1: Create integration test file structure (AC: #1, #2)
  - [ ] Create tests/integration/pipeline_test.go with build tag
  - [ ] Add helper for API key checking and skip
  - [ ] Add test workspace setup/cleanup helpers
- [ ] Task 2: Implement individual provider tests (AC: #3, #4)
  - [ ] TestGeminiScenarioGeneration: 4-stage output validation
  - [ ] TestSiliconFlowImageGeneration: single image, validate ImageResult
  - [ ] TestDashScopeTTSSynthesis: Korean sentence, validate audio + word timings
  - [ ] TestFallbackChainActivation: invalid primary key, validate fallback
- [ ] Task 3: Implement full E2E test (AC: #3, #4, #5)
  - [ ] TestFullPipelineE2E: complete pipeline for SCP-173 fixture
  - [ ] Validate final output files exist
  - [ ] Cleanup workspace after test
- [ ] Task 4: Update Makefile (AC: #6, #7)
  - [ ] Add `test-integration` target
  - [ ] Verify `test` target excludes integration tests

## Dev Notes

- Test file: `tests/integration/pipeline_test.go`
- Build tag: `//go:build integration` (Go 1.17+ syntax)
- API keys from env: GEMINI_API_KEY, SILICONFLOW_API_KEY, DASHSCOPE_API_KEY
- Use testdata/scp-173/ fixture for E2E test (or create minimal fixture)
- Use store.New(":memory:") for test database
- Follow existing test patterns: testify assert/require, t.Helper(), t.Cleanup()
- Existing test structure: internal/*_test.go for unit tests

### References

- [Source: internal/plugin/llm/openai_test.go - LLM test patterns]
- [Source: internal/plugin/imagegen/siliconflow_test.go - ImageGen test patterns]
- [Source: internal/plugin/tts/dashscope_test.go - TTS test patterns]
- [Source: internal/service/scenario_pipeline_test.go - Service test patterns]
- [Source: Makefile - existing targets]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6

### Completion Notes List

- Created integration test file with `//go:build integration` build tag
- 5 test functions: TestGeminiScenarioGeneration, TestSiliconFlowImageGeneration, TestDashScopeTTSSynthesis, TestFallbackChainActivation, TestFullPipelineE2E
- Helper functions: testdataPath(), skipIfNoKey(), testLogger(), testStore(), testWorkspace()
- Created SCP-173 test fixtures: testdata/SCP-173/{meta.json, facts.json, main.txt}
- Added `test-integration` Makefile target
- Code review: fixed FallbackChain constructor signature, removed unused variable, fixed field name DurationSec, fixed glossary.New()

### File List

- tests/integration/pipeline_test.go
- testdata/SCP-173/meta.json
- testdata/SCP-173/facts.json
- testdata/SCP-173/main.txt
- Makefile
