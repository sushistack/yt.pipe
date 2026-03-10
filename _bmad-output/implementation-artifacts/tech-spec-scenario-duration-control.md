---
title: 'Scenario Duration Control'
slug: 'scenario-duration-control'
created: '2026-03-10'
status: 'ready-for-dev'
stepsCompleted: [1, 2, 3, 4]
tech_stack: ['Go 1.25.7', 'cobra CLI v1.10.2', 'chi/v5 REST API', 'text/template', 'SQLite (modernc)', 'testify v1.11.1', 'mockery v2']
files_to_modify: ['internal/config/types.go', 'internal/config/config.go', 'internal/domain/scenario.go', 'internal/plugin/llm/interface.go', 'internal/plugin/llm/openai.go', 'internal/service/scenario_pipeline.go', 'internal/service/scenario.go', 'internal/api/pipeline.go', 'internal/cli/stage_cmds.go', 'internal/cli/run_cmd.go', 'templates/scenario/02_structure.md', 'internal/service/scenario_test.go']
code_patterns: ['mapstructure tags for config', 'plugin interface + factory registry', '4-stage pipeline with checkpoint JSON', 'domain error types (ValidationError)', 'options struct pattern (not variadic)', 'strings.ReplaceAll for template placeholders']
test_patterns: ['*_test.go same package', 'setupTestXxx(t) helper with t.Helper()', 'testify assert/require', 'mockery-generated mocks in internal/mocks', ':memory: SQLite for tests']
---

# Tech-Spec: Scenario Duration Control

**Created:** 2026-03-10

## Overview

### Problem Statement

Scenario generation is currently locked to a fixed duration (default 10 minutes via `ScenarioConfig.TargetDurationMin`). There is no way for users to specify a target video length when generating scenarios. This prevents creating diverse YouTube content ranging from short-form (~1 minute) to long-form (up to 20 minutes). The PRD specifies an optional `length` parameter in the scenario generation API, but it is not yet implemented.

### Solution

Add a `length` parameter to both the REST API (`POST /api/projects/{scp-id}/scenario`) and CLI (`yt-pipe generate scenario --length`). API accepts minutes (float64), internally converted to seconds for consistency with `AudioDuration` and timeline systems. No separate "short mode" — duration value alone drives prompt guideline selection (≤1 min triggers short-form guidelines automatically). Introduce a `ScenarioOptions` struct for extensibility (future `Angle` parameter). Enhance the Stage 2 Structure LLM prompt with duration-tiered scene count and narrative structure guidelines. Support scenario regeneration by reusing Stage 1 (Research) results and re-running from Stage 2. API response includes `estimated_duration_sec` total for user judgment.

### Scope

**In Scope:**
- API & CLI `length` parameter (float64 minutes, range 0.5–20, default 5)
- `ScenarioOptions` struct (extensible for future `Angle` parameter)
- Internal duration unit: seconds (API accepts minutes, converts internally)
- Duration-tiered guidelines in Stage 2 Structure prompt (scene count, 4-act proportions, narration density)
- Scenario regeneration: reuse Stage 1 (Research), re-run Stage 2+ with new length
- Input validation (min 0.5 min, max 20 min)
- Persist chosen duration in scenario metadata
- API response includes `estimated_duration_sec` total

**Out of Scope:**
- Angle parameter implementation (struct placeholder only)
- TTS / image generation stage changes
- Existing scenario data migration
- Duration enforcement post-TTS (duration is a target, not a guarantee; actual length depends on TTS output)

## Context for Development

### Codebase Patterns

- Config uses `mapstructure` tags, 5-level priority chain (CLI flags > env > project YAML > global YAML > defaults)
- Plugin architecture: LLM interface behind `plugin/llm/`, implementations registered via `Registry.Register()`
- Scenario pipeline is 4-stage (research → structure → writing → review) in `service/scenario_pipeline.go`
- **Two code paths for scenario generation**: Pipeline path (`ScenarioPipeline.Run`) and Direct path (`LLM.GenerateScenario`) — both need duration support
- Stage 2 Structure template already receives `{target_duration}` placeholder via `strings.ReplaceAll` (line 251 of scenario_pipeline.go)
- Stage 2 template already expects `estimated_duration_sec` per scene in JSON output, but domain model doesn't capture it
- State machine: `pending → scenario_review → approved → image_review → tts_review → assembling → complete`
- API response envelope: `api.Response{Success, Data, Error, Timestamp, RequestID}`
- CLI: one command per file, commands call service layer
- Options structs used throughout (not variadic functional options)
- Pipeline checkpoint files: `{stagesDir}/01_research.json`, `02_structure.json`, etc.

### Files to Reference

| File | Purpose | Key Lines |
| ---- | ------- | --------- |
| `internal/config/types.go` | `ScenarioConfig.TargetDurationMin` (int) | L63-67 |
| `internal/config/config.go` | Default value `v.SetDefault("scenario.target_duration_min", 10)` | L156 |
| `internal/service/scenario_pipeline.go` | `ScenarioPipelineConfig` struct, `Run()`, Stage 2 `{target_duration}` replacement | L29-34, L88-116, L251 |
| `internal/service/scenario.go` | `GenerateScenarioForProject()` — calls LLM without duration param | L142-193 |
| `internal/plugin/llm/interface.go` | `LLM.GenerateScenario()` signature — no duration param | L34-44 |
| `internal/plugin/llm/openai.go` | `buildScenarioPrompt()` — no duration guidance | L247-274, L313-334 |
| `internal/domain/scenario.go` | `ScenarioOutput`, `SceneScript` — no `EstimatedDurationSec` | L4-18 |
| `internal/domain/scene.go` | `Scene.AudioDuration` (float64, from TTS) | L4-16 |
| `internal/api/pipeline.go` | `handleRunPipeline` — no duration in request body | L136-201 |
| `internal/cli/stage_cmds.go` | `scenarioGenerateCmd` — no `--length` flag | L24-29 |
| `internal/cli/run_cmd.go` | `runRunCmd` — no `--length` flag | L34-131 |
| `templates/scenario/02_structure.md` | Stage 2 prompt with `{target_duration}` and `estimated_duration_sec` | Full file |
| `internal/service/timing.go` | `TimingResolver`, `Timeline.TotalDurationSec` | L31-36, L42-59 |
| `internal/service/scenario_test.go` | Existing scenario tests (no duration coverage) | Full file |

### Technical Decisions

- **No separate "short mode"**: Duration value alone drives behavior — ≤1 min auto-applies short-form prompt guidelines, no API mode flag needed
- **Config field name preserved**: `TargetDurationMin` (int→float64 only), YAML key `scenario.target_duration_min` and env `YTP_SCENARIO_TARGET_DURATION_MIN` unchanged for backward compatibility. Conversion to seconds (`*60`) happens at service layer ingress.
- **`ScenarioOptions` struct**: New domain struct with `Length float64` (minutes) field, extensible for future `Angle string` parameter. Passed through pipeline.
- **LLM interface change**: `GenerateScenario(..., opts domain.ScenarioOptions)` — breaking change, requires mockery regeneration.
- **`ScenarioOutput.TotalEstimatedDurationSec float64`**: Dedicated field (not Metadata map) for type safety. Sum of per-scene `EstimatedDurationSec`.
- **Direct path deprecation direction**: Both Pipeline and Direct paths updated for now; future consolidation into Pipeline-only noted.
- **Prompt strategy**: Single prompt template with embedded duration-tier guideline table (not separate templates per duration). Tiers: ≤1min (3-5 scenes, hook+core), 1-5min (8-15 scenes, 4-act), 5-10min (15-25 scenes, 4-act full), 10-20min (25-35 scenes, subplots)
- **Default**: 5 minutes (changed from current 10 min default)
- **Regeneration**: Reuse Stage 1 (Research) checkpoint, invalidate Stage 2+ checkpoints, re-run pipeline. SCP research data is length-agnostic.
- **Duration is a target, not a guarantee**: LLM generates approximate structure; actual video length depends on TTS output. API response includes `estimated_duration_sec` for user judgment.

## Implementation Plan

### Tasks

#### Layer 1: Domain & Config (no dependencies)

- [ ] Task 1: Add `ScenarioOptions` struct and update domain models
  - File: `internal/domain/scenario.go`
  - Action:
    - Add `ScenarioOptions` struct: `Length float64` (minutes, 0 = use config default), placeholder `Angle string` field (unused)
    - Add `EstimatedDurationSec float64` field to `SceneScript`
    - Add `TotalEstimatedDurationSec float64` field to `ScenarioOutput`
    - Add validation method `func (o ScenarioOptions) Validate() error` returning `ValidationError` for out-of-range (< 0.5 or > 20, unless 0 for default)
  - Notes: Domain layer — stdlib + uuid only, no internal imports

- [ ] Task 2: Change config type from int to float64
  - File: `internal/config/types.go`
  - Action: Change `TargetDurationMin int` → `TargetDurationMin float64` in `ScenarioConfig`
  - Notes: `mapstructure:"target_duration_min"` tag unchanged. YAML key stays `scenario.target_duration_min`.

- [ ] Task 3: Update default value
  - File: `internal/config/config.go`
  - Action: Change `v.SetDefault("scenario.target_duration_min", 10)` → `v.SetDefault("scenario.target_duration_min", 5.0)`
  - Notes: Default changes from 10 min to 5 min

#### Layer 2: Plugin Interface (depends on Task 1)

- [ ] Task 4: Update LLM interface signature
  - File: `internal/plugin/llm/interface.go`
  - Action: Change `GenerateScenario` signature to accept `opts domain.ScenarioOptions` as last parameter:
    ```go
    GenerateScenario(ctx context.Context, scpID string, mainText string, facts []domain.FactTag, metadata map[string]string, opts domain.ScenarioOptions) (*domain.ScenarioOutput, error)
    ```
  - Notes: Breaking change — all implementations and mocks must update

- [ ] Task 5: Update OpenAI implementation
  - File: `internal/plugin/llm/openai.go`
  - Action:
    - Update `GenerateScenario` method signature to accept `opts domain.ScenarioOptions`
    - Update `buildScenarioPrompt` to accept `targetDurationMin float64` and include duration-tiered guidance in the prompt text
    - Duration tier logic: ≤1min → "short-form, 3-5 scenes, hook+core only"; 1-5min → "8-15 scenes, 4-act structure"; 5-10min → "15-25 scenes, full 4-act with transitions"; 10-20min → "25-35 scenes, subplots and deeper exploration"
  - Notes: `opts.Length` is minutes; if 0, caller should have resolved default before calling

- [ ] Task 6: Regenerate mocks
  - File: `internal/mocks/` (auto-generated)
  - Action: Run `mockery` to regenerate `MockLLM` from updated interface
  - Notes: `.mockery.yaml` already has LLM interface registered

#### Layer 3: Pipeline & Service (depends on Tasks 2, 4)

- [ ] Task 7: Update `ScenarioPipelineConfig` and pipeline
  - File: `internal/service/scenario_pipeline.go`
  - Action:
    - Change `ScenarioPipelineConfig.TargetDurationMin` from `int` to `float64`
    - Update `NewScenarioPipeline` default fallback: `if cfg.TargetDurationMin <= 0 { cfg.TargetDurationMin = 5.0 }`
    - Update Stage 2 placeholder replacement to use `fmt.Sprintf("%.1f", sp.config.TargetDurationMin)` instead of `%d`
    - Add `ScenarioOptions` parameter to `Run()` method signature; pass `opts.Length` to override `sp.config.TargetDurationMin` when non-zero
  - Notes: Pipeline path — `{target_duration}` placeholder already exists in Stage 2 template

- [ ] Task 8: Update scenario service to pass options
  - File: `internal/service/scenario.go`
  - Action:
    - Add `opts domain.ScenarioOptions` parameter to `GenerateScenarioForProject` signature
    - Resolve default: if `opts.Length == 0`, set `opts.Length = ss.config.Scenario.TargetDurationMin`
    - Validate via `opts.Validate()`
    - Pass `opts` to both `ss.llm.GenerateScenario()` and `pipeline.Run()` calls
    - After scenario generation, compute `TotalEstimatedDurationSec` from scene sum
    - Persist `opts.Length` in `ScenarioOutput.Metadata["target_duration_min"]`
  - Notes: Service layer does minutes→seconds conversion when needed internally

- [ ] Task 9: Add regeneration support with selective stage invalidation
  - File: `internal/service/scenario_pipeline.go`
  - Action:
    - Add `Regenerate(ctx, scpData, workspacePath, opts ScenarioOptions) (*PipelineResult, error)` method
    - Logic: keep `{stagesDir}/01_research.json` checkpoint, delete `02_structure.json`, `03_writing.json`, `04_review.json`
    - Call existing `Run()` with new options — pipeline checkpoint logic will skip Stage 1 (exists) and re-run Stage 2+
  - Notes: SCP research data is length-agnostic; only structure/writing/review depend on duration

#### Layer 4: API & CLI (depends on Tasks 7, 8)

- [ ] Task 10: Add `length` to API request body
  - File: `internal/api/pipeline.go`
  - Action:
    - Add `Length float64 \`json:"length"\`` field to `handleRunPipeline` request body struct
    - Construct `domain.ScenarioOptions{Length: body.Length}` and pass to service
    - Include `TotalEstimatedDurationSec` in API response data
    - Add `regenerate` mode option alongside existing `scenario` and `full` modes
  - Notes: `length` is optional (0 = use config default). Validation happens in domain layer.

- [ ] Task 11: Add `--length` CLI flag
  - File: `internal/cli/stage_cmds.go`
  - Action:
    - Add `--length` float64 flag to `scenarioGenerateCmd` (default 0, meaning use config)
    - Construct `domain.ScenarioOptions{Length: length}` and pass to service
  - Notes: Also bind via viper for env override `YTP_SCENARIO_LENGTH`

- [ ] Task 12: Add `--length` flag to run command
  - File: `internal/cli/run_cmd.go`
  - Action:
    - Add `--length` float64 flag to `runRunCmd`
    - Pass through to pipeline runner
  - Notes: Consistent with stage_cmds.go flag

#### Layer 5: Prompt Template (independent, can be done anytime)

- [ ] Task 13: Enhance Stage 2 Structure template with duration-tier guidelines
  - File: `templates/scenario/02_structure.md`
  - Action: Add duration-tier guideline table after the existing `{target_duration}` usage:
    ```
    ## Duration Guidelines

    Based on the target duration of {target_duration} minutes, follow these guidelines:

    | Duration | Scene Count | Structure | Narration Density |
    |----------|-------------|-----------|-------------------|
    | ≤1 min   | 3-5 scenes  | Hook + Core only | Dense, punchy, ~15-20 sec/scene |
    | 1-5 min  | 8-15 scenes | 4-act (Hook 15%, Properties 30%, Incidents 40%, Resolution 15%) | Moderate, ~20-30 sec/scene |
    | 5-10 min | 15-25 scenes | Full 4-act with transitions | Standard, ~25-35 sec/scene |
    | 10-20 min| 25-35 scenes | 4-act with subplots and deeper exploration | Detailed, ~30-40 sec/scene |

    Choose the tier matching your target duration and design scenes accordingly.
    ```
  - Notes: Keep existing template structure, append guidelines. LLM will use `estimated_duration_sec` per scene.

#### Layer 6: Tests (depends on all above)

- [ ] Task 14: Update existing scenario tests
  - File: `internal/service/scenario_test.go`
  - Action:
    - Update all `GenerateScenarioForProject` calls to pass `domain.ScenarioOptions{}`
    - Update mock expectations for new `GenerateScenario` signature
    - Add test: default length resolution (opts.Length=0 → config default)
    - Add test: custom length passed through to LLM
    - Add test: validation error for out-of-range length
  - Notes: Use testify assert/require, setupTestXxx pattern

- [ ] Task 15: Add config type change test
  - File: `internal/config/config_test.go`
  - Action:
    - Verify `TargetDurationMin` loads as float64
    - Verify default is 5.0
    - Verify YAML `target_duration_min: 0.5` parses correctly
  - Notes: If config_test.go doesn't exist, add tests to nearest existing test file

- [ ] Task 16: Add API handler test for length parameter
  - File: `internal/api/pipeline_test.go`
  - Action:
    - Test: POST with `{"mode":"scenario","length":5}` → passes ScenarioOptions to service
    - Test: POST with `{"mode":"scenario","length":0.3}` → returns ValidationError (below 0.5)
    - Test: POST with `{"mode":"scenario","length":25}` → returns ValidationError (above 20)
    - Test: POST with `{"mode":"scenario"}` (no length) → uses default
  - Notes: Follow existing API test patterns

### Acceptance Criteria

- [ ] AC 1: Given a user sends `POST /api/projects/{id}/run` with `{"mode":"scenario","length":5}`, when the scenario is generated, then the LLM receives a prompt with "5-minute documentary" guidance and 8-15 scene target.
- [ ] AC 2: Given a user sends `POST /api/projects/{id}/run` with `{"mode":"scenario","length":0.5}`, when the scenario is generated, then the LLM receives short-form guidance (3-5 scenes, hook+core only).
- [ ] AC 3: Given a user sends `POST /api/projects/{id}/run` with `{"mode":"scenario"}` (no length), when the scenario is generated, then the config default (5 min) is used.
- [ ] AC 4: Given a user sends `POST /api/projects/{id}/run` with `{"mode":"scenario","length":0.3}`, when validation runs, then a 400 ValidationError is returned with message indicating min 0.5.
- [ ] AC 5: Given a user sends `POST /api/projects/{id}/run` with `{"mode":"scenario","length":25}`, when validation runs, then a 400 ValidationError is returned with message indicating max 20.
- [ ] AC 6: Given a user runs `yt-pipe scenario generate SCP-173 --length 1`, when the scenario is generated, then the LLM receives 1-minute short-form guidance.
- [ ] AC 7: Given a scenario is generated with any length, when the response is returned, then `TotalEstimatedDurationSec` is populated as the sum of per-scene `EstimatedDurationSec` values.
- [ ] AC 8: Given a project with an existing scenario (Stage 1-4 complete), when the user requests regeneration with a different length, then Stage 1 (Research) checkpoint is preserved and Stage 2-4 are re-executed with the new duration.
- [ ] AC 9: Given config.yaml has `scenario.target_duration_min: 10`, when no `--length` flag or API `length` is provided, then 10 minutes is used (config override works).
- [ ] AC 10: Given `ScenarioOutput` is serialized to JSON, when `TotalEstimatedDurationSec` and per-scene `EstimatedDurationSec` are present, then they appear in the output file.

## Additional Context

### Dependencies

- No new external libraries required
- `mockery` CLI tool must be available for mock regeneration (already in project dev toolchain)
- Existing `.mockery.yaml` config covers LLM interface — no config changes needed
- All changes are backward-compatible with existing config files (YAML key unchanged, int values auto-convert to float64)

### Testing Strategy

**Unit Tests:**
- Domain: `ScenarioOptions.Validate()` — boundary values (0, 0.5, 1, 5, 10, 20, 20.1)
- Config: float64 parsing, default value, env override
- Service: options propagation, default resolution, regeneration stage skip logic
- API: request parsing, validation error responses, response payload

**Integration Tests (if applicable):**
- Full pipeline run with different duration values (requires LLM mock)
- CLI end-to-end with `--length` flag (can be dry-run)

**Manual Testing:**
- Generate scenario with length=0.5, verify ~3-5 scenes
- Generate scenario with length=5 (default), verify ~8-15 scenes
- Generate scenario with length=15, verify ~25-30 scenes
- Regenerate existing scenario with different length, verify Stage 1 preserved

### Notes

**High-risk items:**
- LLM interface signature change is a breaking change — all callers must update simultaneously. Compile errors will guide completeness.
- LLM output quality for extreme durations (0.5 min, 20 min) may need prompt tuning post-implementation. The tier table is a starting point.
- `estimated_duration_sec` from LLM is approximate — actual TTS-generated audio may vary ±30-50%. This is documented as out-of-scope.

**Future considerations (out of scope):**
- Consolidate Direct path into Pipeline-only (deprecate `LLM.GenerateScenario` direct calls)
- Add `Angle` parameter implementation to `ScenarioOptions`
- Post-TTS duration feedback loop (if actual duration deviates significantly, auto-suggest regeneration)
- Duration-based pricing/quota (longer videos cost more API tokens)
