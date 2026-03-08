# Story 8-3: SCP Data to Scenario Generation Pipeline

## Status: Done

## Implementation Summary

### New Files
- `internal/service/scenario_pipeline.go` — `ScenarioPipeline` struct with `Run()` method executing 4 stages sequentially (Research→Structure→Writing→Review). Stage functions: `runResearch`, `runStructure`, `runWriting`, `runReview`. Checkpoint/resume support: each stage output saved as JSON in `{workspace}/stages/`. `extractVisualIdentity()` extracts Frozen Descriptor from research output. `applyCorrections()` applies review-stage patch corrections. `parseScenarioFromWriting()` and `parseReviewReport()` for LLM output parsing. `extractJSONFromContent()` strips markdown code fences.

### Modified Files
- `internal/config/types.go` — Added `ScenarioConfig` type with `FactCoverageThreshold` and `TargetDurationMin`
- `internal/config/config.go` — Added scenario config defaults (80% coverage, 10 min duration)

### Architecture Decisions
- Each stage's output saved as intermediate artifact: `{workspace}/stages/01_research.json`, `02_structure.json`, `03_writing.json`, `04_review.json`
- Resume from checkpoint: if a stage's JSON file exists, it's loaded instead of re-executing the LLM call
- Visual Identity Profile extracted from Stage 1 output and passed through as `{scp_visual_reference}` to subsequent stages (Frozen Descriptor Protocol)
- Review stage corrections applied as string replacements to narration (patch-based, not full rewrite)
- Target duration read from config (default: 10 minutes)

### Acceptance Criteria Met
- [x] 4 stages execute sequentially: Research → Structure → Writing → Review
- [x] Each stage output saved as intermediate artifact
- [x] Research output auto-injected into Structure template
- [x] Visual Identity Profile passed through as Frozen Descriptor
- [x] Review corrections applied automatically as patches
- [x] Checkpoint at last completed stage; resume on re-run
- [x] Error messages include which stage failed
- [x] All tests pass
