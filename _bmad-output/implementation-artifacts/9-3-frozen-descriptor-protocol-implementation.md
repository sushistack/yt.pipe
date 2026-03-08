# Story 9.3: Frozen Descriptor Protocol Implementation

Status: done

## Story

As a creator,
I want the system to enforce visual consistency of the SCP entity across all generated images,
So that the entity always looks the same from scene to scene.

## Acceptance Criteria

1. `FrozenDescriptorService` extracts descriptor from research with priority: "Frozen Descriptor" > "Visual Identity" > "Physical Description"
2. Saves/loads descriptor to/from project workspace as `frozen_descriptor.txt`
3. 3-tier validation in `ValidateInPrompt`: verbatim match → fuzzy (>=95%) → auto-correct
4. Fuzzy matching uses word-level overlap (tokenize, ignore words <3 chars)
5. Auto-correction prepends frozen descriptor to prompt when validation fails
6. Markdown stripping from extracted sections (bold, italic, bullets)

## Implementation Summary

### Files Created
- `internal/service/frozen_descriptor.go` — Full protocol implementation
- `internal/service/frozen_descriptor_test.go` — 14+ tests

### Key Functions
- `ExtractFromResearch(researchContent)` — Multi-pass section extraction with priority
- `SaveToWorkspace(projectPath, descriptor)` — Atomic file write
- `LoadFromWorkspace(projectPath)` — File read with not-exist handling
- `ValidateInPrompt(prompt, frozenDescriptor, entityVisible)` — 3-tier validation
- `computeSimilarity(prompt, descriptor)` — Word-level overlap ratio
- `tokenize(text)` — Word extraction (>=3 chars, stripped punctuation)
- `stripMarkdown(line)` — Remove bullets, bold, italic markers

### Test Coverage
- Extraction from "Frozen Descriptor" section (priority 1)
- Fallback to "Visual Identity" section (priority 2)
- No matching section returns empty
- Markdown stripping in extracted content
- Workspace save/load round-trip
- Load from non-existent path (returns empty, no error)
- Verbatim match validation
- Fuzzy match validation (>=95% threshold)
- Auto-correct validation (descriptor prepended)
- Not-applicable cases (entity not visible, empty descriptor)
- Similarity computation (exact, partial, no match)
- Tokenization
- Markdown stripping

## References
- [Source: _bmad-output/planning-artifacts/epics.md#Story 9.3]
- [Source: internal/workspace/] — atomic file write pattern
