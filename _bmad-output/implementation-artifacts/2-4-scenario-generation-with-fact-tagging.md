# Story 2.4: Scenario Generation with Fact Tagging

Status: done

## Story
As a creator, I want the system to generate a narration scenario from SCP data with fact tagging so that each scene's narration is traceable to source facts.

## Implementation
- `internal/service/scenario.go`: ScenarioService with GenerateScenario(), RegenerateSection(), ApproveScenario(), LoadScenarioFromFile(), renderScenarioMarkdown()
- `internal/service/scenario_test.go`: 7 tests covering generation, LLM error, approval, wrong state, regeneration, file loading
- Uses LLM plugin for scenario generation, store for state persistence, workspace for file I/O
- Fact tags embedded per scene in structured ScenarioOutput format

## Dev Agent Record
### Agent Model Used
Claude Opus 4.6
