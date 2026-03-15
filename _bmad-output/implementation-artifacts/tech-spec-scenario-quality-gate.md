---
title: 'Scenario Quality Gate with Critic Agent'
slug: 'scenario-quality-gate'
created: '2026-03-15'
status: 'completed'
stepsCompleted: [1, 2, 3, 4]
tech_stack: ['Go 1.25.7', 'llm.LLM interface (Complete method)', '4-stage scenario pipeline', 'checkpoint resume system']
files_to_modify:
  - 'internal/service/scenario_pipeline.go'
  - 'internal/service/scenario_quality_gate.go (new)'
  - 'internal/service/scenario_quality_gate_test.go (new)'
  - 'internal/service/scenario_pipeline_test.go'
  - 'templates/scenario/03_writing.md'
  - 'templates/scenario/critic_agent.md (new)'
  - 'tests/e2e/helpers_test.go'
  - 'tests/e2e/pipeline_test.go'
code_patterns:
  - 'ScenarioPipeline.Run() orchestration with runStageWithCheckpoint'
  - 'callLLM(ctx, stage, prompt) → StageResult'
  - 'parseScenarioFromWriting(content, scpID) → *domain.ScenarioOutput'
  - 'parseReviewReport(content) → *ReviewReport'
  - 'applyCorrections(scenario, corrections) → *domain.ScenarioOutput'
  - 'Template variable substitution: strings.ReplaceAll(tmpl, "{var}", value)'
  - 'Checkpoint files: {workspacePath}/stages/{stage}.json'
test_patterns:
  - 'mockLLM.On("Complete", mock.Anything, mock.MatchedBy(func(msgs []llm.Message) bool {...}), mock.Anything)'
  - 'containsString(msgs[0].Content, "Research") for stage routing'
  - 'sampleWritingOutput() / sampleReviewOutput() canned JSON'
  - 'Checkpoint resume: pre-create stage JSON files, verify LLM only called for remaining stages'
---

# Tech-Spec: Scenario Quality Gate with Critic Agent

**Created:** 2026-03-15

## Overview

### Problem Statement

The 4-stage scenario pipeline's Stage 4 review is a self-review — the same LLM evaluates its own writing. `storytelling_score`, `overall_pass`, and `coverage_pct` are captured but never enforced. Format guide standards (hook types, emotional curve, immersion devices) are injected into prompts but compliance is never verified. Result: low-quality scenarios pass through unchecked.

### Solution

Two-layer quality gate after Stage 4 Review:
- **Layer 1 (Code)**: Structural validation — hook pattern, mood variation, immersion device count, scene count. Fast, objective, no LLM cost.
- **Layer 2 (Critic Agent)**: "SCP Content Director" role-play agent evaluates format guide compliance, engagement quality, and viewer retention via `llm.Complete()` with a dedicated critic prompt. Returns structured pass/fail with specific feedback.

On failure, Stage 3 (Writing) + Stage 4 (Review) are retried with accumulated feedback from both layers. MaxAttempts: 3 total (original + 2 retries). Structure (Stage 2) is preserved across retries.

### Scope

**In Scope:**
- Layer 1: Code-based structural validators (hook, mood, immersion, scene count, fact coverage)
- Layer 2: Critic Agent with SCP Content Director persona and format_guide.md evaluation
- Writing+Review retry loop with feedback injection (MaxAttempts: 3)
- Checkpoint compatibility (retry state persisted)
- Unit tests for quality gate logic
- E2E test coverage for quality gate paths

**Out of Scope:**
- Real LLM API integration (fake plugin in E2E)
- Stage 2 (Structure) retry
- User manual approval process
- Sentence-level Korean character count validation

## Context for Development

### Codebase Patterns

- **Pipeline orchestration**: `ScenarioPipeline.Run()` in `scenario_pipeline.go` (line 127-220) — sequential stage execution with checkpoint resume
- **LLM calls**: `callLLM(ctx, stage, prompt)` sends single `{Role: "user", Content: prompt}` message, returns `StageResult{Content, InputTokens, OutputTokens}`
- **Checkpoint system**: Each stage saves `{workspacePath}/stages/{stage}.json` (StageResult JSON). On resume, cached stages are skipped.
- **Template substitution**: `strings.ReplaceAll(tmpl, "{var}", value)` — variables like `{scp_id}`, `{scene_structure}`, `{format_guide}`, `{glossary_section}`
- **Review output**: `ReviewReport` struct with `OverallPass`, `CoveragePct`, `StorytellingScore`, `Issues[]`, `Corrections[]`, `StorytellingIssues[]`
- **Quality gate insertion point**: After Stage 4 review in `Run()`. The retry loop wraps Stages 3-4 together.
- **Writing JSON format**: `parseScenarioFromWriting()` expects `{scp_id, title, scenes[{scene_num, narration, visual_description, fact_tags, mood}], metadata}`
- **Fact coverage threshold**: `ScenarioPipelineConfig.FactCoverageThreshold` default 80% — exists but never enforced
- **Storytelling threshold**: Hardcoded 70 — exists but only logs warning

### Files to Reference

| File | Purpose |
| ---- | ------- |
| `internal/service/scenario_pipeline.go` | Main pipeline orchestration — Run(), callLLM(), checkpoints, parseScenarioFromWriting(), applyCorrections() |
| `internal/service/scenario_pipeline_test.go` | 12 tests — mock LLM routing by prompt content, checkpoint resume, correction application |
| `templates/scenario/03_writing.md` | Writing stage prompt — hook rules, immersion techniques, sentence pacing, output JSON format |
| `templates/scenario/04_review.md` | Review stage prompt — 6-dimension evaluation, storytelling subscores, output JSON format |
| `templates/scenario/format_guide.md` | Quality standards — hook types, progressive disclosure, emotional curve, immersion devices, scene count/pacing, act structure |
| `internal/plugin/llm/interface.go` | `LLM.Complete(ctx, []Message, CompletionOptions) → (*CompletionResult, error)` |
| `internal/domain/scenario.go` | `ScenarioOutput`, `SceneScript` — no JSON tags, default Go field names |

### Technical Decisions

1. **Retry loop wraps Stages 3+4 together**: Stage 4's `ReviewReport.Corrections` are valuable for text-level fixes. Re-running both ensures corrections are fresh for each attempt.
2. **Layer 1 fail → immediate retry (skip Critic Agent)**: Code checks are free. If structural violations exist, no need to spend LLM tokens on Critic. Inject Layer 1 feedback directly into retry.
3. **Layer 1 pass → Layer 2 Critic Agent check**: Only invoke Critic when code structure is valid. Critic catches subtle format/engagement issues code can't detect.
4. **Critic Agent uses `llm.Complete()` with user prompt role injection**: Consistent with existing pipeline pattern (no system messages used anywhere in pipeline).
5. **Critic Agent verdict**: Returns `verdict` ("pass" / "retry" / "accept_with_notes") + viewer-centric checks (`hook_effective`, `retention_risk`, `ending_impact`) + concrete `feedback` string + per-scene `scene_notes[]`.
6. **Feedback accumulation**: Each retry injects previous feedback via `{quality_feedback}` template variable. Concrete and actionable ("Scene 1의 Hook이 없다. Shock 타입으로 교체하라"), NOT abstract.
7. **Checkpoint compatibility**: Delete `03_writing.json` + `04_review.json` + `critic_agent.json` checkpoint files before retry. On resume, the last checkpoint state determines where to continue.
8. **MaxAttempts: 3**: Total 3 attempts (original + 2 retries). On exhaustion, select the BEST attempt by: verdict priority (pass > accept_with_notes > retry), then Layer 1 pass count.
9. **Quality gate as separate file**: `scenario_quality_gate.go` — clean separation from pipeline orchestration.

## Implementation Plan

### Tasks

#### Task 1: Create quality gate structs and Layer 1 validators (`internal/service/scenario_quality_gate.go`)

- **File:** `internal/service/scenario_quality_gate.go` (new)
- **Action:** Create the quality gate module with:

  1. **`QualityGateConfig` struct**:
     ```go
     type QualityGateConfig struct {
         MaxAttempts           int     // default 3
         FactCoverageThreshold float64 // default 80.0
         MinSceneCount         int     // default 5 (for 10min target)
         MaxSceneCount         int     // default 15
         MinImmersionCount     int     // default 3 ("당신" occurrences)
     }
     ```

  2. **`QualityGateResult` struct**:
     ```go
     type QualityGateResult struct {
         Pass           bool
         Layer1Pass     bool
         Layer2Verdict  string // "pass" | "retry" | "accept_with_notes" | "" (not run)
         Violations     []QualityViolation
         CriticFeedback string
         SceneNotes     []CriticSceneNote
     }
     ```

  3. **`QualityViolation` struct**: `{Check string, SceneNum int, Description string}`

  4. **`CriticVerdict` struct** (parsed from Critic Agent JSON):
     ```go
     type CriticVerdict struct {
         Verdict       string            `json:"verdict"`
         HookEffective bool              `json:"hook_effective"`
         RetentionRisk string            `json:"retention_risk"`
         EndingImpact  string            `json:"ending_impact"`
         Feedback      string            `json:"feedback"`
         SceneNotes    []CriticSceneNote `json:"scene_notes"`
     }
     type CriticSceneNote struct {
         SceneNum   int    `json:"scene_num"`
         Issue      string `json:"issue"`
         Suggestion string `json:"suggestion"`
     }
     ```

  5. **`RunLayer1(scenario *domain.ScenarioOutput, reviewReport *ReviewReport, cfg QualityGateConfig) []QualityViolation`**:
     - **Hook check**: `scenario.Scenes[0].Narration` must NOT start with "SCP-" pattern (regex `^SCP-\d+`)
     - **Mood variation**: Iterate adjacent scene pairs, flag consecutive same `Mood` values
     - **Immersion count**: Count occurrences of "당신" across all `Narration` fields, require ≥ `cfg.MinImmersionCount`
     - **Scene count**: `len(scenario.Scenes)` must be within `[cfg.MinSceneCount, cfg.MaxSceneCount]`
     - **Fact coverage**: If `reviewReport != nil`, check `reviewReport.CoveragePct >= cfg.FactCoverageThreshold`
     - Returns slice of `QualityViolation` — empty means Layer 1 pass

  6. **`RunLayer2(ctx context.Context, llm llm.LLM, scenario *domain.ScenarioOutput, formatGuide string) (*CriticVerdict, error)`**:
     - Load critic prompt template from `sp.criticTemplate`
     - Substitute `{scenario_json}` (marshal scenario), `{format_guide}`
     - Call `llm.Complete()` with single user message containing the critic prompt
     - Parse JSON response into `CriticVerdict`
     - Return verdict

  7. **`BuildFeedbackString(violations []QualityViolation, criticVerdict *CriticVerdict) string`**:
     - Combine Layer 1 violations and Critic feedback into a single structured feedback string
     - Format suitable for injection into Writing prompt `{quality_feedback}` variable

- **Notes:** All functions are package-level (not methods on ScenarioPipeline) for testability. `RunLayer2` takes `llm.LLM` as parameter to use the same fake in tests.

#### Task 2: Create Critic Agent prompt template (`templates/scenario/critic_agent.md`)

- **File:** `templates/scenario/critic_agent.md` (new)
- **Action:** Create the SCP Content Director persona prompt:

  ```markdown
  You are an SCP Content Director with 10 years of experience producing viral SCP YouTube content.
  Your job is to evaluate this scenario RUTHLESSLY from the viewer's perspective.

  ## Your Evaluation Criteria

  {format_guide}

  ## The Scenario to Evaluate

  {scenario_json}

  ## Evaluation Instructions

  Answer these questions honestly:
  1. **Hook (Scene 1)**: Would a casual YouTube viewer stay past the first 5 seconds? Is the opening line a genuine hook (Question/Shock/Mystery/Contrast)?
  2. **Retention**: Would a viewer watch past 1 minute? Is information revealed progressively or front-loaded?
  3. **Emotional Curve**: Do moods vary between scenes? Or is it monotone throughout?
  4. **Immersion**: Does the narration pull the viewer IN (2nd person, sensory details, hypotheticals)?
  5. **Ending**: Would a viewer like/subscribe after watching? Does it leave lingering impact?

  ## Output Format (JSON only, no markdown fences)

  {
    "verdict": "pass" | "retry" | "accept_with_notes",
    "hook_effective": true/false,
    "retention_risk": "low" | "medium" | "high",
    "ending_impact": "strong" | "medium" | "weak",
    "feedback": "Concrete, actionable improvement instructions in Korean. Be specific about which scenes need what changes.",
    "scene_notes": [
      {"scene_num": 1, "issue": "description of problem", "suggestion": "specific fix"}
    ]
  }

  Rules:
  - "pass": Scenario is production-ready. Would get >50% watch-through rate.
  - "retry": Significant issues that require rewriting. Be specific in feedback.
  - "accept_with_notes": Passable but not great. Note improvements for future reference.
  - feedback MUST be in Korean and MUST be specific ("Scene 1을 Shock Hook으로 교체: 'SCP-173은 14명의 재단 인원을 살해했습니다'")
  - Do NOT be generous. If it's mediocre, say "retry".
  ```

- **Notes:** `{format_guide}` and `{scenario_json}` are the only template variables. The persona is embedded in the prompt text itself.

#### Task 3: Add `{quality_feedback}` to Writing template (`templates/scenario/03_writing.md`)

- **File:** `templates/scenario/03_writing.md`
- **Action:** Add a conditional feedback section at the end of the template, before the output format:

  ```markdown
  {quality_feedback}
  ```

  This variable is:
  - Empty string on first attempt (no visible change to prompt)
  - On retry: contains structured feedback like:
    ```
    ## ⚠️ QUALITY IMPROVEMENT REQUIRED (Attempt 2/3)

    Your previous scenario was rejected. Fix these specific issues:

    ### Code Validation Failures:
    - Scene 1: Hook missing — narration starts with "SCP-173은..." instead of a hook. Use Shock or Mystery type.
    - Scene 3-4: Same mood "tense" — change Scene 4 mood to "awe" or "horror".

    ### Content Director Feedback:
    Scene 1을 Shock Hook으로 교체하세요. 시청자가 5초 안에 이탈할 것입니다.
    Scene 6의 ending이 허무합니다. 미해결 미스터리로 마무리하세요.

    DO NOT repeat the same mistakes. Address each issue above.
    ```

- **Notes:** Place the `{quality_feedback}` variable AFTER the existing content rules section but BEFORE the output JSON format section. On first attempt, `strings.ReplaceAll(tmpl, "{quality_feedback}", "")` produces no visible change.

#### Task 4: Integrate quality gate into pipeline `Run()` (`internal/service/scenario_pipeline.go`)

- **File:** `internal/service/scenario_pipeline.go`
- **Action:**

  1. **Add fields to `ScenarioPipelineConfig`**:
     ```go
     MaxAttempts           int     // default 3
     ```

  2. **Add fields to `ScenarioPipeline` struct**:
     ```go
     criticTemplate string // loaded from templates/scenario/critic_agent.md
     ```

  3. **Load critic template in `NewScenarioPipeline()`** (after line 122):
     ```go
     criticPath := filepath.Join(cfg.TemplatesDir, "scenario", "critic_agent.md")
     if data, err := os.ReadFile(criticPath); err == nil {
         sp.criticTemplate = string(data)
     }
     ```

  4. **Add `QualityGateConfig` to `PipelineResult`**:
     ```go
     Attempts     int  `json:"attempts"`      // how many writing attempts were made
     ```

  5. **Refactor `Run()` to wrap Stages 3-4 in a retry loop** (replace current lines 160-202):
     ```go
     maxAttempts := sp.config.MaxAttempts
     if maxAttempts <= 0 { maxAttempts = 3 }

     qgConfig := QualityGateConfig{
         MaxAttempts:           maxAttempts,
         FactCoverageThreshold: sp.config.FactCoverageThreshold,
         MinSceneCount:         sceneCountRange(sp.config.TargetDurationMin).min,
         MaxSceneCount:         sceneCountRange(sp.config.TargetDurationMin).max,
         MinImmersionCount:     3,
     }

     var bestAttempt *writingAttempt
     qualityFeedback := ""

     for attempt := 1; attempt <= maxAttempts; attempt++ {
         // Delete previous Stage 3+4 checkpoints for retry
         if attempt > 1 {
             os.Remove(filepath.Join(stagesDir, "03_writing.json"))
             os.Remove(filepath.Join(stagesDir, "04_review.json"))
             os.Remove(filepath.Join(stagesDir, "critic_agent.json"))
         }

         // Stage 3: Writing (with quality_feedback injected)
         writingContent, err := sp.runStageWithCheckpoint(ctx, StageWriting, stagesDir, func() (*StageResult, error) {
             return sp.runWriting(ctx, scpData, structureContent.Content, visualRef, glossarySection, qualityFeedback)
         })
         if err != nil { return nil, err }

         // Stage 4: Review
         reviewContent, err := sp.runStageWithCheckpoint(ctx, StageReview, stagesDir, func() (*StageResult, error) {
             return sp.runReview(ctx, scpData, writingContent.Content, visualRef, factSheet, glossarySection)
         })
         if err != nil { return nil, err }

         // Parse scenario + review
         scenario, parseErr := parseScenarioFromWriting(writingContent.Content, scpData.SCPID)
         if parseErr != nil { return nil, parseErr }

         reviewReport, _ := parseReviewReport(reviewContent.Content)
         if reviewReport != nil {
             scenario = applyCorrections(scenario, reviewReport.Corrections)
         }

         // Quality Gate Layer 1
         violations := RunLayer1(scenario, reviewReport, qgConfig)
         thisAttempt := &writingAttempt{scenario, reviewReport, writingContent, reviewContent, violations, nil, attempt}

         if len(violations) > 0 {
             // Layer 1 fail → skip Critic, retry immediately
             qualityFeedback = BuildFeedbackString(violations, nil)
             bestAttempt = selectBest(bestAttempt, thisAttempt)
             slog.Warn("quality gate: Layer 1 failed", "attempt", attempt, "violations", len(violations))
             continue
         }

         // Quality Gate Layer 2: Critic Agent
         if sp.criticTemplate != "" {
             verdict, criticErr := RunLayer2(ctx, sp.llm, scenario, sp.formatGuide, sp.criticTemplate)
             if criticErr == nil {
                 thisAttempt.criticVerdict = verdict
                 if verdict.Verdict == "pass" || verdict.Verdict == "accept_with_notes" {
                     bestAttempt = thisAttempt
                     break // Quality gate passed!
                 }
                 qualityFeedback = BuildFeedbackString(violations, verdict)
             }
         } else {
             // No critic template → Layer 1 pass is sufficient
             bestAttempt = thisAttempt
             break
         }

         bestAttempt = selectBest(bestAttempt, thisAttempt)
         slog.Warn("quality gate: Critic rejected", "attempt", attempt, "verdict", thisAttempt.criticVerdict.Verdict)
     }

     // Use best attempt
     result.Scenario = bestAttempt.scenario
     result.ReviewReport = bestAttempt.reviewReport
     result.Attempts = bestAttempt.attempt
     ```

  6. **Update `runWriting()` signature** to accept `qualityFeedback string` parameter:
     ```go
     func (sp *ScenarioPipeline) runWriting(ctx context.Context, scpData *workspace.SCPData,
         sceneStructure, visualRef, glossarySection, qualityFeedback string) (*StageResult, error) {
         // ... existing template substitution ...
         prompt = strings.ReplaceAll(prompt, "{quality_feedback}", qualityFeedback)
         return sp.callLLM(ctx, StageWriting, prompt)
     }
     ```

  7. **Add `writingAttempt` struct and `selectBest()` helper**:
     ```go
     type writingAttempt struct {
         scenario       *domain.ScenarioOutput
         reviewReport   *ReviewReport
         writingStage   *StageResult
         reviewStage    *StageResult
         violations     []QualityViolation
         criticVerdict  *CriticVerdict
         attempt        int
     }

     func selectBest(current, candidate *writingAttempt) *writingAttempt {
         if current == nil { return candidate }
         // Priority: fewer violations + better verdict
         currentScore := attemptScore(current)
         candidateScore := attemptScore(candidate)
         if candidateScore > currentScore { return candidate }
         return current
     }
     ```

  8. **Add `sceneCountRange()` helper**:
     ```go
     func sceneCountRange(targetDurationMin int) (min, max int) {
         switch {
         case targetDurationMin <= 5:  return 4, 7
         case targetDurationMin <= 10: return 7, 12
         case targetDurationMin <= 15: return 10, 16
         default:                      return 7, 12
         }
     }
     ```

- **Notes:** The `runWriting()` signature change requires updating the one existing call site (currently line ~276). The `qualityFeedback` parameter is empty string for the first call within the loop.

#### Task 5: Create quality gate unit tests (`internal/service/scenario_quality_gate_test.go`)

- **File:** `internal/service/scenario_quality_gate_test.go` (new)
- **Action:** Create comprehensive tests:

  1. **`TestLayer1_HookCheck`**: Scene 1 starting with "SCP-173은" → violation; "눈을 감지 마세요" → pass
  2. **`TestLayer1_MoodVariation`**: Adjacent same mood → violation; alternating moods → pass
  3. **`TestLayer1_ImmersionCount`**: 0 "당신" → violation; 3+ → pass
  4. **`TestLayer1_SceneCount`**: 3 scenes (too few) → violation; 8 scenes → pass; 20 scenes (too many) → violation
  5. **`TestLayer1_FactCoverage`**: `CoveragePct` 60% with threshold 80% → violation; 85% → pass
  6. **`TestLayer1_AllPass`**: Valid scenario with all checks passing → empty violations
  7. **`TestLayer2_ParseCriticVerdict`**: Valid JSON → parsed correctly; malformed JSON → error
  8. **`TestBuildFeedbackString`**: Violations + Critic feedback → structured Korean feedback string
  9. **`TestSelectBest`**: Pass > accept_with_notes > retry; fewer violations wins among same verdict

- **Notes:** Use `testify` assert/require. No mock LLM needed for Layer 1 tests (pure functions). Layer 2 parsing test uses canned JSON strings.

#### Task 6: Update pipeline integration tests (`internal/service/scenario_pipeline_test.go`)

- **File:** `internal/service/scenario_pipeline_test.go`
- **Action:**

  1. **`TestScenarioPipeline_QualityGate_PassFirstAttempt`**: Mock LLM returns high-quality writing + Critic returns `"pass"` → pipeline completes in 1 attempt
  2. **`TestScenarioPipeline_QualityGate_RetryOnLayer1Fail`**: First writing has no hook → Layer 1 rejects → retry with feedback → second attempt passes
  3. **`TestScenarioPipeline_QualityGate_RetryOnCriticReject`**: Layer 1 passes but Critic returns `"retry"` → retry with Critic feedback → second attempt passes
  4. **`TestScenarioPipeline_QualityGate_MaxAttemptsExhausted`**: All 3 attempts rejected → best attempt selected, pipeline doesn't fail
  5. **`TestScenarioPipeline_QualityGate_NoCriticTemplate`**: Critic template not loaded → Layer 1 pass is sufficient, no Critic call
  6. **Update existing `TestScenarioPipeline_Run_Success`**: Add critic mock expectation (returns "pass")

- **Notes:** Mock LLM `Complete()` needs routing by prompt content — existing pattern uses `containsString(msgs[0].Content, "...")`. Add new matchers for "Content Director" (Critic) and "{quality_feedback}" presence (retry indicator).

#### Task 7: Update E2E test helpers and pipeline test (`tests/e2e/`)

- **File:** `tests/e2e/helpers_test.go`, `tests/e2e/pipeline_test.go`
- **Action:**

  1. **Update `fakeLLM.Complete()` in `helpers_test.go`**: Add routing for Critic Agent prompt (detect "Content Director" or "verdict" in prompt). Return `{"verdict": "pass", "hook_effective": true, ...}` JSON.
  2. **Update `TestPipeline_GenerateScenario` in `pipeline_test.go`**: Verify that after scenario generation, the "Scenes" heading appears AND scene count is correct (demonstrates full pipeline including quality gate ran).
  3. **Add `TestPipeline_GenerateScenario_QualityGateRetry`** (optional, if time permits): Use a fakeLLM that returns low-quality writing on first call and high-quality on second, verify pipeline still completes.

- **Notes:** E2E tests already use `fakeLLM` with call-counter routing. Adding Critic Agent response is a natural extension.

### Acceptance Criteria

**AC1: Layer 1 code validation detects structural issues**
- Given a scenario where Scene 1 narration starts with "SCP-173은 유클리드 등급의..."
- When Layer 1 quality gate runs
- Then a `QualityViolation` with Check="hook_pattern" is returned

**AC2: Layer 1 detects mood monotony**
- Given a scenario with Scene 3 mood="tense" and Scene 4 mood="tense"
- When Layer 1 quality gate runs
- Then a violation with Check="mood_variation" referencing scenes 3-4 is returned

**AC3: Layer 1 checks immersion device count**
- Given a scenario with 1 occurrence of "당신" across all narrations
- When Layer 1 quality gate runs
- Then a violation with Check="immersion_count" is returned (requires ≥3)

**AC4: Layer 2 Critic Agent evaluates and returns structured verdict**
- Given a valid scenario and Critic template loaded
- When Layer 2 Critic Agent is invoked via `llm.Complete()`
- Then a `CriticVerdict` with `verdict`, `hook_effective`, `retention_risk`, `ending_impact`, and `feedback` is returned

**AC5: Layer 1 fail skips Layer 2 Critic**
- Given a scenario that fails Layer 1 (e.g., no hook)
- When quality gate runs
- Then Layer 2 Critic Agent is NOT invoked (no additional LLM call)
- And the feedback string contains Layer 1 violations only

**AC6: Writing retry injects feedback**
- Given a first attempt rejected by quality gate
- When Stage 3 Writing is retried
- Then the Writing prompt contains the `{quality_feedback}` section with specific issues from the previous attempt

**AC7: MaxAttempts respected**
- Given MaxAttempts=3 and all attempts fail quality gate
- When the retry loop completes
- Then exactly 3 writing attempts are made (not more)
- And the pipeline returns the best attempt (not an error)

**AC8: Best attempt selection works**
- Given 3 attempts where attempt 2 had the best verdict ("accept_with_notes") and fewest violations
- When MaxAttempts exhausted
- Then attempt 2's scenario is used as the final output

**AC9: Checkpoint compatibility on retry**
- Given a retry that deletes Stage 3+4 checkpoints
- When the pipeline crashes and resumes
- Then the latest checkpoint state is used correctly (no stale data from previous attempt)

**AC10: E2E pipeline generates scenario through quality gate**
- Given the E2E test server with fakeLLM (4-stage pipeline enabled)
- When "Generate Scenario" is clicked in the browser
- Then the scenario passes through the quality gate (Critic returns "pass")
- And scenes appear on the project detail page

**AC11: PipelineResult includes attempt count**
- Given a pipeline run that took 2 attempts to pass
- When `PipelineResult` is returned
- Then `result.Attempts == 2`

## Additional Context

### Dependencies

- **Existing:** `llm.LLM` interface (Complete method), `domain.ScenarioOutput`, `domain.SceneScript`
- **New template:** `templates/scenario/critic_agent.md` (loaded at pipeline init alongside existing templates)
- **No new Go dependencies** — uses existing `strings`, `regexp`, `encoding/json`, `os`

### Testing Strategy

- **Unit tests** (`scenario_quality_gate_test.go`): Layer 1 pure functions (no mocks needed), Layer 2 JSON parsing, feedback builder, best-attempt selection — 9+ tests
- **Integration tests** (`scenario_pipeline_test.go`): Full pipeline with quality gate loop — mock LLM routing for multi-attempt scenarios — 5+ new tests + update 1 existing
- **E2E tests** (`tests/e2e/`): Verify quality gate integrates with browser-driven pipeline flow — update fakeLLM Critic routing + verify scenario generation still works end-to-end
- **Manual verification**: Run with real LLM to verify Critic Agent produces meaningful feedback (not part of automated tests)

### Notes

- **Layer 1 checks (code-based, objective)**:
  - Hook pattern: Scene 1 narration must NOT start with "SCP-" pattern (regex `^SCP-\d+`)
  - Mood variation: No two adjacent scenes with same `Mood` value
  - Immersion count: "당신" count ≥ 3 across all scene narrations
  - Scene count: Within range for target duration (7-12 for 10min default)
  - Fact coverage: `ReviewReport.CoveragePct >= config.FactCoverageThreshold` (80% default)
- **Layer 2 Critic Agent** evaluates from YouTube viewer perspective:
  - "Would you click this?" (Hook effectiveness)
  - "Would you watch past 1 minute?" (Retention risk)
  - "Would you like/subscribe?" (Ending impact)
- Feedback is **concrete and actionable** ("Scene 1을 Shock Hook으로 교체: 'SCP-173은 14명의 재단 인원을 살해했습니다'"), NOT abstract ("퀄리티를 높여주세요")
- On final attempt failure, pipeline selects the **BEST attempt** by: verdict priority (pass > accept_with_notes > retry), then Layer 1 violation count
- **Risk: LLM may not produce valid JSON for Critic verdict** — `RunLayer2` should gracefully degrade (return nil verdict, not error) so pipeline doesn't fail on Critic parse errors
- **Risk: `runWriting()` signature change** — only 1 call site in `Run()` method, but existing tests will need updating for the new parameter
