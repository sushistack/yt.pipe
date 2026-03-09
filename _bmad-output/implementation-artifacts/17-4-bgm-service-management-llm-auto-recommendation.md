# Story 17.4: BGM Service — Management & LLM Auto-Recommendation

Status: done

## Story

As a developer,
I want a BGM service layer with library management and LLM-based auto-recommendation per scene,
So that BGMs can be automatically matched to scenes based on narrative mood analysis.

## Acceptance Criteria

1. **Given** the service layer
   **When** `NewBGMService(store, llm)` is called
   **Then** a `BGMService` is created with store and LLM dependencies

2. **Given** a valid BGM input
   **When** `CreateBGM()` is called
   **Then** input validation runs (name, file_path, license_type)
   **And** file existence is verified via `os.Stat`
   **And** UUID is generated for the BGM ID
   **And** the BGM is persisted via store

3. **Given** an existing BGM
   **When** `UpdateBGM()` is called with partial fields
   **Then** only non-empty fields are updated (merge semantics)
   **And** license_type is validated if provided

4. **Given** scenes with narration text and BGMs in the library
   **When** `AutoRecommendBGMs(ctx, projectID, scenes)` is called
   **Then** LLM analyzes scene narrations and suggests mood tags from the available tag set
   **And** `SearchByMoodTags` finds the best matching BGM per scene
   **And** top match is assigned with default parameters (0dB volume, 2000ms fade, -12dB ducking)
   **And** assignments are marked `AutoRecommended=true, Confirmed=false`

5. **Given** an empty BGM library
   **When** `AutoRecommendBGMs()` is called
   **Then** it returns nil (no-op) without calling LLM

6. **Given** pending scene assignments
   **When** `GetPendingConfirmations()` is called
   **Then** only unconfirmed assignments are returned

7. **Given** a scene assignment
   **When** `ConfirmBGM()` or `ReassignBGM()` or `AdjustBGMParams()` is called
   **Then** the assignment is updated correctly in the store

8. **Given** confirmed BGM assignments
   **When** `GetCredits()` is called
   **Then** credit entries are returned with deduplication (same BGM across scenes = one credit)

## Tasks / Subtasks

- [x] Task 1: Implement BGMService struct and constructor (AC: #1)
  - [x] 1.1: Define `BGMService` with `store` and `llm` fields
  - [x] 1.2: `NewBGMService(store, llm)` constructor
- [x] Task 2: Implement library management (AC: #2-#3)
  - [x] 2.1: `CreateBGM()` with validation (name, file_path, license_type, os.Stat)
  - [x] 2.2: `GetBGM()`, `ListBGMs()`, `DeleteBGM()` — thin wrappers
  - [x] 2.3: `UpdateBGM()` with merge semantics (non-empty fields only)
- [x] Task 3: Implement LLM auto-recommendation (AC: #4-#5)
  - [x] 3.1: Build scene summaries for LLM prompt
  - [x] 3.2: Collect available mood tags from all BGMs
  - [x] 3.3: Prompt LLM for JSON array of `{scene_num, mood_tags}`
  - [x] 3.4: Parse LLM response and search for matching BGMs
  - [x] 3.5: Assign top match with default parameters
  - [x] 3.6: Return nil for empty library (no LLM call)
- [x] Task 4: Implement confirmation workflow (AC: #6-#7)
  - [x] 4.1: `GetPendingConfirmations()` — filter unconfirmed
  - [x] 4.2: `ConfirmBGM()` — delegate to store
  - [x] 4.3: `ReassignBGM()` — verify new BGM exists, update assignment
  - [x] 4.4: `AdjustBGMParams()` — update volume/fade/ducking parameters
- [x] Task 5: Implement credits generation (AC: #8)
  - [x] 5.1: `GetCredits()` — deduplicated `output.CreditEntry` list
- [x] Task 6: Write tests
  - [x] 6.1: Service tests with mock LLM for auto-recommendation
  - [x] 6.2: Validation tests for CreateBGM
  - [x] 6.3: Credits deduplication test
- [x] Task 7: Run full test suite
  - [x] 7.1: `make test` passes
  - [x] 7.2: `make lint` passes

## Dev Notes

### LLM Prompt Design

```go
prompt := fmt.Sprintf(`Analyze the mood and atmosphere of each scene and recommend suitable background music mood tags.

Available mood tags: %v

Scenes:
%s

Return a JSON array where each element has "scene_num" (int) and "mood_tags" (string array from available tags).
Return ONLY the JSON array, no other text.`, availableTags, sceneSummaries)
```

- **Per-scene LLM calls**: Uses a single LLM call for all scenes (not batch per-scene). LLM returns JSON array mapping scene numbers to mood tags.
- **Tag constraint**: Prompt includes only available tags from the existing BGM library to ensure SearchByMoodTags returns results.
- **Partial failure tolerance**: If a scene gets no tag matches, it's skipped (no assignment).

### Default Assignment Parameters

| Parameter | Default | Rationale |
|-----------|---------|-----------|
| VolumeDB | 0 dB | Full volume, adjustable later |
| FadeInMs | 2000 ms | 2-second fade-in for smooth transition |
| FadeOutMs | 2000 ms | 2-second fade-out |
| DuckingDB | -12 dB | Reduce BGM by 12dB during narration |

### Credits Deduplication

```go
seen := make(map[string]bool)
for _, a := range assignments {
    if !a.Confirmed || seen[a.BGMID] {
        continue
    }
    seen[a.BGMID] = true
    // ... build credit entry
}
```

### Files Touched

| File | Change |
|------|--------|
| `internal/service/bgm.go` | New — BGMService with CRUD, auto-recommendation, confirmation, credits |
| `internal/service/bgm_test.go` | New — Service tests with mock LLM |

### References

- [Source: internal/service/bgm.go] — full implementation
- [Source: internal/service/bgm_test.go] — tests
- [Pattern: internal/service/mood.go] — Epic 15 service pattern reference
- [Pattern: internal/service/character.go] — Epic 14 service pattern reference

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6

### Completion Notes List

- Implemented `BGMService` with full library management (Create, Get, List, Update, Delete)
- Implemented `AutoRecommendBGMs` using LLM mood analysis with constrained tag selection
- Implemented confirmation workflow: `GetPendingConfirmations`, `ConfirmBGM`, `ReassignBGM`, `AdjustBGMParams`
- Implemented `GetCredits` with deduplication via `seen` map
- CreateBGM validates name, file_path (os.Stat), and license_type before persistence
- UpdateBGM uses merge semantics — only non-empty fields are overwritten
- Empty library early-return prevents unnecessary LLM calls
- All tests pass with mock LLM, 20 packages green

### File List

- `internal/service/bgm.go` (new) — BGMService with CRUD, LLM auto-recommendation, confirmation workflow, credits
- `internal/service/bgm_test.go` (new) — Service tests with mock LLM
