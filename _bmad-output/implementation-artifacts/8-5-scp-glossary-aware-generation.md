# Story 8-5: SCP Glossary-Aware Generation

## Status: Done

## Implementation Summary

### Modified Files
- `internal/service/scenario_pipeline.go` — Added `buildGlossarySection()` method to `ScenarioPipeline`: builds a terminology reference section from the glossary for injection into each prompt template via `{glossary_section}` placeholder. Filters glossary terms relevant to the target SCP. Empty glossary produces empty section (no noise in prompts).

### Architecture Decisions
- Glossary injection uses existing `internal/glossary` package (thread-safe lookup)
- Each of the 4 prompt templates includes `{glossary_section}` placeholder
- Glossary section format: term, definition, preferred usage, Korean translation where applicable
- Object class validation (Safe, Euclid, Keter, Thaumiel, Apollyon) handled by review stage
- Custom terms added via `yt-pipe glossary add` persist in SQLite and are included in subsequent prompts
- Empty glossary → empty section → no extra tokens consumed

### Acceptance Criteria Met
- [x] Glossary terms injected into each prompt template
- [x] Injected glossary includes term, definition, preferred usage
- [x] SCP object class validation via review stage
- [x] Custom terms supported via glossary add
- [x] Empty glossary produces no injection
- [x] All tests pass
