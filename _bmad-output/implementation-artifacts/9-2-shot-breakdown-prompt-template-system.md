# Story 9.2: Shot Breakdown Prompt Template System

Status: done

## Story

As a creator,
I want the system to use a 2-stage LLM pipeline to generate cinematic image prompts from scene descriptions,
So that each scene has a well-composed, consistent image prompt for FLUX generation.

## Acceptance Criteria

1. Two prompt templates in `templates/image/`: `01_shot_breakdown.md` (scene→shot) and `02_shot_to_prompt.md` (shot→image prompt)
2. `ShotBreakdownPipeline` in `service/shot_breakdown.go` orchestrates the 2-stage pipeline
3. Stage 1 outputs structured `ShotDescription` JSON (shot_number, role, camera_type, entity_visible, subject, lighting, mood, motion)
4. Stage 2 outputs `ShotPromptResult` JSON (prompt, negative_prompt, entity_visible)
5. Safety sanitization removes dangerous terms and adds cinematic suffix
6. Sequential generation with previous shot context for visual continuity
7. Partial failure tolerance — continues processing remaining scenes

## Implementation Summary

### Files Created
- `templates/image/01_shot_breakdown.md` — Stage 1 template (scene→shot decomposition)
- `templates/image/02_shot_to_prompt.md` — Stage 2 template (shot→image prompt)
- `internal/service/shot_breakdown.go` — 2-stage pipeline implementation
- `internal/service/shot_breakdown_test.go` — Tests for pipeline, sanitization, continuity

### Key Types
- `ShotBreakdownPipeline` — Orchestrator with LLM and loaded templates
- `ShotDescription` — Stage 1 output (structured shot info)
- `ShotPromptResult` — Stage 2 output (final prompt text)
- `ScenePromptInput/Output` — Pipeline I/O types
- `sanitizeImagePrompt()` — Case-preserving dangerous term removal + cinematic suffix

### Design Decisions
- Sequential scene processing (not parallel) to carry `previousCtx` for visual continuity
- `extractJSONFromContent` reused from scenario pipeline for LLM response parsing
- Cinematic suffix added automatically if not present in LLM output

## References
- [Source: _bmad-output/planning-artifacts/epics.md#Story 9.2]
- [Source: templates/scenario/] — pattern reference for template structure
