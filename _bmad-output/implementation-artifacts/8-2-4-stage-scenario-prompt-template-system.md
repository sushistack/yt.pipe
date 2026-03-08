# Story 8-2: 4-Stage Scenario Prompt Template System

## Status: Done

## Implementation Summary

### New Files
- `templates/scenario/01_research.md` — Stage 1 Research prompt template: injects `{scp_fact_sheet}`, `{main_text}`, produces structured research packet with Core Identity Summary, Visual Identity Profile (Frozen Descriptor), Key Dramatic Beats, Environment & Atmosphere Notes, Narrative Hooks
- `templates/scenario/02_structure.md` — Stage 2 Structure prompt template: injects `{research_packet}`, `{scp_visual_reference}`, `{target_duration}`, produces scene structure following 4-act format (Hook ~15%, Properties ~30%, Incidents ~40%, Resolution ~15%)
- `templates/scenario/03_writing.md` — Stage 3 Writing prompt template: injects `{scene_structure}`, `{scp_visual_reference}`, produces Korean narration in documentary style (~합니다 register, ≤20 char sentences)
- `templates/scenario/04_review.md` — Stage 4 Review prompt template: injects `{narration_script}`, `{scp_visual_reference}`, `{scp_fact_sheet}`, performs fact-check validation with patch-based corrections

### Architecture Decisions
- Templates stored in `templates/scenario/` directory, loaded by `ScenarioPipeline` at init
- Variable substitution uses simple string replacement: `{scp_fact_sheet}`, `{research_packet}`, `{scp_visual_reference}`, `{target_duration}`, `{scene_structure}`, `{narration_script}`, `{glossary_section}`
- Visual Identity Profile follows Frozen Descriptor format: Silhouette & Build, Head/Face, Body Covering, Hands & Limbs, Carried Items, Organic Integration Note
- Each template supports `{glossary_section}` injection (Story 8-5)

### Acceptance Criteria Met
- [x] 4 template files loaded: `01_research.md`, `02_structure.md`, `03_writing.md`, `04_review.md`
- [x] Templates support all required variable substitutions
- [x] Research template produces Visual Identity Profile (Frozen Descriptor)
- [x] Structure template follows 4-act format
- [x] Writing template produces Korean narration in documentary style
- [x] Review template performs fact-check with patch-based corrections
- [x] All tests pass
