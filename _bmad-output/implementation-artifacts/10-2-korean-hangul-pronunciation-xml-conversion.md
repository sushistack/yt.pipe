# Story 10.2: Korean Hangul Pronunciation XML Conversion

Status: done

## Story

As a creator,
I want English terms and numbers in the narration to be converted to Korean pronunciation before TTS,
So that the narration sounds natural without awkward English pronunciation breaks.

## Acceptance Criteria

1. 2-tier conversion: deterministic glossary substitution first, then LLM for remaining context-dependent conversions
2. Glossary substitutions: "SCP" -> "에스씨피", "Keter" -> "케테르", etc. applied before LLM call
3. LLM handles numbers contextually: "2시" -> "두 시", "2025년" -> "이천이십오 년"
4. Template stored at `templates/tts/scenario_refine.md`
5. Output is valid XML in `<script>` format with speaker tags on separate lines
6. Converted text saved as `{project}/scenes/{scene_num}/narration_refined.xml`
7. Glossary overrides take precedence over LLM-generated pronunciations

## Tasks / Subtasks

- [ ] Create `internal/service/pronunciation.go` - PronunciationService
- [ ] Create `templates/tts/scenario_refine.md` - prompt template
- [ ] Create `internal/service/pronunciation_test.go`

## Dev Notes

### Existing Patterns
- **Glossary system** (`internal/glossary/glossary.go`): `Pronunciation(term)` returns override or term itself
- **LLM interface** (`internal/plugin/llm/interface.go`): `Complete(ctx, messages, opts)` for LLM calls
- **Template loading**: Go `text/template` or raw file read from `templates/` path
- **Workspace**: `workspace.WriteFileAtomic()` for saving output
- **TTSService.buildOverrides()**: Already builds overrides map from glossary

### Conversion Flow
1. Load narration text
2. Apply glossary substitutions (deterministic, case-insensitive)
3. Call LLM with scenario_refine template for remaining conversions
4. Parse and validate XML output
5. Save to narration_refined.xml

### References
- [Source: _bmad-output/planning-artifacts/epics.md#Story 10.2]
- [Source: internal/glossary/glossary.go] - glossary system
- [Source: internal/service/tts.go] - existing TTS service integration
