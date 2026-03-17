---
title: 'Cinematic Cut Decomposition Pipeline'
slug: 'cinematic-cut-decomposition-pipeline'
created: '2026-03-17'
status: 'completed'
stepsCompleted: [1, 2, 3, 4]
tech_stack: ['Go 1.25.7', 'SQLite', 'SiliconFlow FLUX', 'OpenAI-compatible LLM', 'chi REST', 'cobra CLI']
files_to_modify:
  - 'internal/domain/scene.go'
  - 'internal/domain/scenario.go'
  - 'internal/domain/manifest.go'
  - 'internal/service/shot_breakdown.go'
  - 'internal/service/image_gen.go'
  - 'internal/plugin/imagegen/interface.go'
  - 'internal/store/shot_manifest.go'
  - 'internal/store/migrations/013_cut_decomposition.sql'
  - 'internal/pipeline/runner.go'
  - 'templates/image/01_shot_breakdown.md'
  - 'templates/image/02_shot_to_prompt.md'
  - 'templates/scenario/03_writing.md'
code_patterns:
  - 'Plugin architecture: LLM, ImageGen, TTS behind interfaces'
  - 'String template replacement for LLM prompts (strings.ReplaceAll)'
  - 'JSON structured output from LLM (extractJSONFromContent)'
  - 'Incremental builds via hash-based ShotManifest'
  - 'Pipe-filter: Scene as self-contained asset bundle'
  - 'Error handling: domain error types (ValidationError, NotFoundError, PipelineError)'
  - 'Retry with backoff (internal/retry package)'
test_patterns:
  - '*_test.go in same package'
  - 'testify assertions (assert, require)'
  - 'SQLite :memory: for store tests'
  - 'Table-driven tests for domain logic'
---

# Tech-Spec: Cinematic Cut Decomposition Pipeline

**Created:** 2026-03-17

## Overview

### Problem Statement

The current 1-sentence = 1-shot fixed mapping produces visually flat content. A sentence like "문을 열자 빛이 들어왔다" contains two distinct visual beats (door opening, light streaming in), but the current pipeline compresses this into a single image. Conversely, consecutive sentences like "SCP-173이 긴 복도를 걸어간다. 발소리가 울린다. 형광등이 깜빡인다." depict the same visual scene but generate 3 separate images, creating unnecessary visual discontinuity.

Additionally, `Character.StyleGuide` exists in the domain model but is never injected into image prompts, and there is no project-level or scene-level style/palette system for visual coherence.

### Solution

Replace the fixed sentence-to-shot mapping with an LLM-based **bidirectional visual beat analysis**:
- **Split**: 1 sentence → N cuts (multiple visual beats in one sentence)
- **Merge**: N sentences → 1 cut (same visual scene across multiple sentences)

The LLM determines the optimal cut boundaries based on visual beats, not sentence boundaries. Introduce structured visual metadata at the Scene level (location, characters, palette, mood) that feeds into cut generation as context. Implement a hierarchical style guide system: project-level defaults with scene-level overrides.

**LLM optimization is a first-class concern** — the cut decomposition must minimize LLM calls through batching, efficient prompt design, and smart caching to control API costs and latency.

### Scope

**In Scope:**
- LLM-based bidirectional cut decomposition (split 1→N and merge N→1 based on visual beats)
- Scene-level structured visual metadata (location, characters present, color palette, mood/atmosphere)
- Project-level style guide (base palette, art style) + Scene-level override system
- `Character.StyleGuide` activation in image prompt injection
- LLM call optimization: batch processing, prompt efficiency, response caching, cost control
- `sanitizeImagePrompt()` art style suffix → `StyleConfig.ArtStyle` based (replace hardcoded anime suffix)
- Replaces existing `tech-spec-multi-shot-image-gen.md`

**Out of Scope:**
- TTS/subtitle pipeline changes (shot timing remapping handled downstream naturally)
- CapCut assembly logic changes
- New image generation provider additions
- Scenario generation pipeline (4-stage) structural changes
- Cut decomposition approval workflow (no approval step — regenerate-only flow)

## Context for Development

### Codebase Patterns

- **Scene as core unit**: Self-contained asset bundle (image, audio, subtitles, metadata). `domain.Scene` struct in `scene.go`.
- **Plugin architecture**: LLM, TTS, ImageGen, Output behind interfaces. All external deps abstracted.
- **Incremental builds**: `ShotManifest` tracks `(project_id, scene_num, shot_num)` with content hash. `SceneSkipChecker` filters unchanged shots. Hash-based skip via `FilterShotsForImageGen()`.
- **Current shot pipeline flow**:
  1. `SplitNarrationSentences()` → `[]string` (punctuation-based sentence splitting)
  2. Per-sentence loop: `runShotBreakdown()` → individual LLM call → `ShotDescription` JSON
  3. Per-sentence loop: `runShotToPrompt()` → individual LLM call → `ShotPromptResult` JSON
  4. Total: **2N LLM calls per scene** (N = number of sentences)
- **Template system**: String replacement (`strings.ReplaceAll`) with `{placeholder}` tokens. Templates loaded from `templates/image/` directory.
- **Frozen Descriptor Protocol**: Entity visual consistency via verbatim descriptor injection. 3-tier validation (exact → fuzzy → auto-correct).
- **Character injection**: `image_gen.go:69-79` — when `entityVisible=true`, loads character via `CheckExistingCharacter(scpID)`, creates `CharacterRef{Name, VisualDescriptor, ImagePromptBase}`. **`StyleGuide` field is NOT included** despite existing in the Character model.
- **Context continuity**: `previousCtx` string carries `"Camera: X, Subject: Y, Lighting: Z, Mood: W"` between shots.
- **Prompt sanitization**: `sanitizeImagePrompt()` removes dangerous terms, **hardcodes anime style suffix** — must be replaced with `StyleConfig.ArtStyle` based suffix.

### Technical Constraints & Preferences

- **LLM Cost Optimization**: Current pipeline makes 2N LLM calls per scene (N sentences × 2 stages). **Must batch all sentences in a scene into a single Stage 1 LLM call** that returns all cuts at once.
- **Stage Merging**: Merge cut decomposition + cinematography into a single LLM call (new Stage 1). Keep Stage 2 (shot-to-prompt) per-cut for FLUX prompt quality. Result: **1 + M calls per scene** (1 batch call + M individual prompt calls, where M = total cuts).
- **Prompt Efficiency**: Scene-level metadata (location, palette, mood) injected once as context preamble in Stage 1. Not repeated per-cut in Stage 2 — only the cut-specific data is passed.
- **Caching**: Extend incremental build to cut level. Cache key must include **both narration hash AND scene metadata hash** — same sentence with different scene metadata (e.g., palette change) should invalidate cache.
- **Style Override Pattern**: Go zero-value merge — project style as base, scene fields override only when non-zero/non-empty.
- **No Approval Flow**: No cut-level or scene-level approval. Per-cut regeneration only. Existing state machine unchanged.
- **Orphan Cleanup**: When cut count changes on rebuild, orphaned images and manifest rows must be cleaned up.
- **Backward Compatibility**: `Scene.ImagePath` set to first cut's image (existing pattern at `image_gen.go:254`). `Shot.ShotNum` kept as deprecated alias to avoid breaking CapCut assembler and API responses.
- **Bidirectional Cut Mapping**: Cuts use `SentenceStart`/`SentenceEnd` range instead of single `SentenceNum`. A merged cut spans `SentenceStart=3, SentenceEnd=5`. A split cut has `SentenceStart == SentenceEnd`.
- **JSON Robustness**: Stage 1 returns large JSON arrays (scene-batch). Add JSON repair/recovery logic for malformed LLM output.

### Files to Reference

| File | Purpose | Change Type |
| ---- | ------- | ----------- |
| `internal/domain/scene.go` | Scene + Shot structs, `SplitNarrationSentences()` | **Modify** |
| `internal/domain/scenario.go` | `ScenarioOutput`, `SceneScript` | **Modify** |
| `internal/domain/manifest.go` | `ShotManifest` struct | **Modify** |
| `internal/domain/character.go` | Character model with `StyleGuide` field | No change |
| `internal/service/shot_breakdown.go` | 2-stage pipeline | **Major rewrite** |
| `internal/service/image_gen.go` | Shot image generation + character injection | **Modify** |
| `internal/plugin/imagegen/interface.go` | `CharacterRef` struct | **Modify** |
| `internal/store/shot_manifest.go` | CRUD for shot_manifests table | **Modify** |
| `internal/store/migrations/012_shot_manifests.sql` | Current schema | Reference only |
| `internal/pipeline/runner.go` | Pipeline orchestration | **Modify** |
| `templates/image/01_shot_breakdown.md` | LLM prompt: per-sentence shot breakdown | **Rewrite** |
| `templates/image/02_shot_to_prompt.md` | LLM prompt: shot → FLUX prompt | **Modify** |
| `templates/scenario/03_writing.md` | Scenario writing stage | **Modify** |
| `internal/service/frozen_descriptor.go` | Frozen Descriptor validation | No change |

### Technical Decisions

1. **Stage 1 is scene-batched**: One LLM call per scene processes ALL sentences and returns ALL cuts with cinematography. Primary LLM optimization — reduces 2N calls to 1.
2. **Stage 2 remains per-cut**: FLUX prompt generation requires per-cut precision. Batching here risks quality degradation.
3. **Bidirectional cut mapping**: Cuts use `SentenceStart`/`SentenceEnd` range. Split: `Start == End`, multiple cuts. Merge: `Start < End`, single cut covering multiple sentences. LLM decides both directions autonomously.
4. **3-level addressing**: `(sceneNum, sentenceStart, cutNum)` as composite key throughout domain, store, and service layers.
5. **StyleGuide flows through CharacterRef**: Added to `imagegen.CharacterRef`, injected in `image_gen.go`, consumed in `02_shot_to_prompt.md` template.
6. **Scene visual metadata sourced from scenario stage**: `SceneScript` extended with `Location`, `CharactersPresent`, `ColorPalette`, `Atmosphere` — populated by scenario Stage 3 (writing template).
7. **Project-level style stored in project YAML config**: Base art style and palette. Scene-level overrides via zero-value merge.
8. **`sanitizeImagePrompt()` refactored**: Hardcoded anime suffix replaced with `StyleConfig.ArtStyle` based suffix. Dangerous term removal preserved.
9. **Cache key includes scene metadata**: `hash(narration + location + palette + atmosphere)` to invalidate on metadata changes.
10. **`ShotNum` kept as deprecated alias**: Prevents breaking changes in API, CLI, and CapCut assembler. New code uses `CutNum`.

## Implementation Plan

### Tasks

- [x] **Task 1: Domain model — Scene visual metadata on SceneScript**
  - File: `internal/domain/scenario.go`
  - Action: Add structured visual metadata fields to `SceneScript`:
    ```go
    Location          string   `json:"location"`
    CharactersPresent []string `json:"characters_present"`
    ColorPalette      string   `json:"color_palette"`
    Atmosphere        string   `json:"atmosphere"`
    ```
  - Notes: For existing scenarios without these fields, zero values are acceptable — the pipeline treats empty fields as "use project defaults".

- [x] **Task 2: Domain model — Bidirectional cut-aware Shot and ShotKey**
  - File: `internal/domain/scene.go`
  - Action:
    - Add new fields to `Shot`:
      ```go
      SentenceStart int `json:"sentence_start"` // first sentence covered (1-based)
      SentenceEnd   int `json:"sentence_end"`   // last sentence covered (start == end for split, start < end for merge)
      CutNum        int `json:"cut_num"`         // cut number within the sentence range
      ```
    - Keep `ShotNum` as deprecated computed field (`ShotNum` = sequential index for backward compat)
    - Update `ShotKey` to use sentence range:
      ```go
      type ShotKey struct {
          SceneNum      int
          SentenceStart int
          CutNum        int
      }
      ```
    - Add `SceneVisualMeta` struct:
      ```go
      type SceneVisualMeta struct {
          Location          string   `json:"location"`
          CharactersPresent []string `json:"characters_present"`
          ColorPalette      string   `json:"color_palette"`
          Atmosphere        string   `json:"atmosphere"`
      }
      ```
    - Add `VisualMeta SceneVisualMeta` field to `Scene` struct
  - Notes: `SplitNarrationSentences()` unchanged — still used for sentence extraction. LLM determines cut boundaries.

- [x] **Task 3: Domain model — ShotManifest with sentence range key**
  - File: `internal/domain/manifest.go`
  - Action: Add fields to `ShotManifest`:
    ```go
    SentenceStart int // first sentence in range
    SentenceEnd   int // last sentence in range
    CutNum        int
    ```
  - Keep `ShotNum` for migration compatibility, mark as deprecated.

- [x] **Task 4: Database migration — cut decomposition schema**
  - File: `internal/store/migrations/013_cut_decomposition.sql`
  - Action:
    ```sql
    ALTER TABLE shot_manifests ADD COLUMN sentence_start INTEGER NOT NULL DEFAULT 0;
    ALTER TABLE shot_manifests ADD COLUMN sentence_end INTEGER NOT NULL DEFAULT 0;
    ALTER TABLE shot_manifests ADD COLUMN cut_num INTEGER NOT NULL DEFAULT 0;
    -- Migrate existing data: shot_num becomes sentence_start, sentence_end = sentence_start, cut_num = 1
    UPDATE shot_manifests SET sentence_start = shot_num, sentence_end = shot_num, cut_num = 1 WHERE sentence_start = 0;
    -- New index for 3-level key queries (keep old UNIQUE intact for SQLite compat)
    CREATE INDEX IF NOT EXISTS idx_shot_manifests_cut
        ON shot_manifests(project_id, scene_num, sentence_start, cut_num);
    ```
  - Notes: Keep existing `UNIQUE(project_id, scene_num, shot_num)` — SQLite can't drop inline constraints without table rebuild. New queries use the new index. Old constraint becomes harmless.

- [x] **Task 5: Store layer — sentence-range key queries**
  - File: `internal/store/shot_manifest.go`
  - Action:
    - Update `CreateShotManifest()` to include `sentence_start`, `sentence_end`, `cut_num`
    - Update `GetShotManifest()` signature: `(projectID, sceneNum, sentenceStart, cutNum)`
    - Update `UpdateShotManifest()` to use new key
    - Add `DeleteShotManifestsByScene(projectID, sceneNum)` for full scene orphan cleanup
    - Add `ListShotManifestsByScene(projectID, sceneNum)` for cut count comparison before regeneration
  - Notes: Orphan cleanup operates at scene level (not sentence level) because merged cuts can span sentence boundaries — old per-sentence cleanup would miss cross-sentence orphans.

- [x] **Task 6: CharacterRef — add StyleGuide field**
  - File: `internal/plugin/imagegen/interface.go`
  - Action: Add `StyleGuide string` to `CharacterRef`:
    ```go
    type CharacterRef struct {
        Name             string
        VisualDescriptor string
        ImagePromptBase  string
        StyleGuide       string // character-specific style rules
    }
    ```

- [x] **Task 7: ImageGenService — StyleGuide injection + bidirectional key**
  - File: `internal/service/image_gen.go`
  - Action:
    - At line 72-75: add `StyleGuide: char.StyleGuide` to `CharacterRef` construction
    - Update `GenerateShotImage()` signature: replace `shotNum int` with `sentenceStart, cutNum int`
    - Update file naming: `cut_{sentenceStart}_{cutNum}.{ext}` (was `shot_{shotNum}.{ext}`)
    - Update manifest operations to use `(sentenceStart, cutNum)` key
    - Update `findExistingShotImage()`, `BackupShotImage()` to new naming
    - Update `GenerateAllShotImages()` → `GenerateAllCutImages()`:
      - Accept `[]CutOutput` per scene
      - Use `ShotKey{SceneNum, SentenceStart, CutNum}` for skip map
      - Orphan cleanup at scene level: before generating any cuts for a scene, compare new cut list with existing manifests. Delete manifests and images not in the new cut list.
  - Notes: `Scene.ImagePath = first cut's image` preserved. `Shot.ShotNum` set to sequential index for backward compat.

- [x] **Task 8: Project-level StyleConfig + sanitize refactor**
  - File: `internal/domain/style.go` (new file)
  - Action:
    ```go
    type StyleConfig struct {
        ArtStyle     string `json:"art_style" yaml:"art_style"`         // e.g. "dark horror anime", "watercolor", "oil painting"
        ColorPalette string `json:"color_palette" yaml:"color_palette"` // e.g. "desaturated blues and grays"
        Mood         string `json:"mood" yaml:"mood"`                   // e.g. "ominous, unsettling"
        StyleSuffix  string `json:"style_suffix" yaml:"style_suffix"`   // appended to all prompts, replaces hardcoded anime suffix
    }

    func MergeSceneStyle(project StyleConfig, scene SceneVisualMeta) StyleConfig {
        merged := project
        if scene.ColorPalette != "" { merged.ColorPalette = scene.ColorPalette }
        if scene.Atmosphere != "" { merged.Mood = scene.Atmosphere }
        return merged
    }
    ```
  - File: `internal/service/shot_breakdown.go` (sanitize function)
  - Action: Refactor `sanitizeImagePrompt()` — remove hardcoded anime suffix. Accept `StyleConfig.StyleSuffix` as parameter. If empty, use legacy default for backward compat.

- [x] **Task 9: LLM Template — Scene-batch bidirectional cut decomposition (Stage 1 rewrite)**
  - File: `templates/image/01_shot_breakdown.md`
  - Action: Complete rewrite with bidirectional cut mapping:
    - **Input placeholders**: `{scene_number}`, `{full_narration}`, `{sentences_json}`, `{entity_visual_identity}`, `{frozen_descriptor}`, `{scene_location}`, `{scene_characters}`, `{scene_palette}`, `{scene_atmosphere}`, `{scene_mood}`, `{previous_scene_last_cut_context}`, `{style_guide}`
    - **Core bidirectional instructions**:
      ```
      Analyze the scene narration for VISUAL BEATS — distinct, visualizable moments.
      Cut boundaries are determined by visual content, NOT sentence boundaries:
      - SPLIT: A sentence with multiple visual beats → multiple cuts (e.g., "문을 열자 빛이 들어왔다" → 2 cuts: door opening, light streaming in)
      - MERGE: Multiple sentences depicting the same visual scene → one cut (e.g., walking down corridor + footsteps echoing + lights flickering → 1 establishing shot)
      - Each cut MUST have sentence_start and sentence_end indicating which sentences it covers
      - Maximum 3 cuts per sentence (split guard)
      ```
    - **Output format**:
      ```json
      [
        {
          "sentence_start": 1,
          "sentence_end": 1,
          "cut_num": 1,
          "visual_beat": "door opening",
          "role": "action",
          "camera_type": "medium",
          "entity_visible": false,
          "subject": "...",
          "lighting": "...",
          "mood": "...",
          "motion": "..."
        },
        {
          "sentence_start": 3,
          "sentence_end": 5,
          "cut_num": 1,
          "visual_beat": "long corridor walking sequence",
          "role": "establishing",
          "camera_type": "wide",
          ...
        }
      ]
      ```
    - **Scene context preamble**: Location, palette, atmosphere injected once at top, not repeated per cut.
  - Notes: This single call replaces N individual `runShotBreakdown()` calls. LLM autonomously decides split/merge.

- [x] **Task 10: LLM Template — Shot-to-prompt with StyleGuide + palette (Stage 2 update)**
  - File: `templates/image/02_shot_to_prompt.md`
  - Action: Add placeholders:
    - `{style_guide}` — character-specific style rules
    - `{scene_palette}` — merged color palette
    - `{scene_atmosphere}` — merged atmosphere
    - `{art_style}` — project art style (replaces hardcoded "anime illustration" assumption)
  - Notes: `{frozen_descriptor}` and `{shot_json}` remain. New fields additive.

- [x] **Task 11: ShotBreakdownPipeline — major rewrite with bidirectional mapping**
  - File: `internal/service/shot_breakdown.go`
  - Action:
    - **New types**:
      ```go
      type CutDescription struct {
          SentenceStart int    `json:"sentence_start"`
          SentenceEnd   int    `json:"sentence_end"`
          CutNum        int    `json:"cut_num"`
          VisualBeat    string `json:"visual_beat"`
          Role          string `json:"role"`
          CameraType    string `json:"camera_type"`
          EntityVisible bool   `json:"entity_visible"`
          Subject       string `json:"subject"`
          Lighting      string `json:"lighting"`
          Mood          string `json:"mood"`
          Motion        string `json:"motion"`
      }

      type CutOutput struct {
          SentenceStart  int
          SentenceEnd    int
          CutNum         int
          CutDesc        *CutDescription
          PromptResult   *CutPromptResult
          FinalPrompt    string
          NegativePrompt string
      }

      type SceneCutOutput struct {
          SceneNum int
          Cuts     []CutOutput
      }
      ```
    - **New `SceneCutInput`** (replaces `ScenePromptInput`):
      ```go
      type SceneCutInput struct {
          SceneNum             int
          Narration            string
          Mood                 string
          Location             string
          CharactersPresent    []string
          ColorPalette         string
          Atmosphere           string
          EntityVisualIdentity string
          FrozenDescriptor     string
          StyleGuide           string
          StyleConfig          StyleConfig // for sanitize and suffix
          PreviousLastCutCtx   string
      }
      ```
    - **Rewrite `GenerateScenePrompt()`** → `GenerateSceneCuts()`:
      1. `SplitNarrationSentences()` to get sentences (for template injection as `{sentences_json}`)
      2. Build Stage 1 prompt with all sentences + scene metadata
      3. **Single LLM call** → parse `[]CutDescription` JSON array
      4. **Validate**: enforce max 3 cuts per sentence, validate sentence ranges within bounds
      5. **JSON robustness**: attempt repair on malformed JSON (unclosed brackets, trailing commas)
      6. For each cut: `runCutToPrompt()` (Stage 2, per-cut) with style context
      7. Carry `previousCtx` between cuts for continuity
      8. Return `*SceneCutOutput`
    - **Remove**: `SentencePromptInput`, `ShotOutput`, `ScenePromptOutput`, `runShotBreakdown()`
    - **Keep + rename**: `runCutToPrompt()`, `sanitizeImagePrompt(styleSuffix)`, `formatCutContext()`

- [x] **Task 12: Pipeline runner — wire Scene metadata + style**
  - File: `internal/pipeline/runner.go` (lines ~1220-1265)
  - Action:
    - Update `generateShotImages()` → `generateCutImages()`:
      - Load project `StyleConfig` from project config
      - Build `SceneCutInput` per scene from `SceneScript` metadata + merged style
      - Pass to `GenerateSceneCuts()`
    - Update `SceneSkipChecker` to use `ShotKey{SceneNum, SentenceStart, CutNum}`
    - Wire `char.StyleGuide` into pipeline

- [x] **Task 13: Orphan cleanup logic (scene-level)**
  - File: `internal/service/image_gen.go`
  - Action: Before generating cuts for a scene, compare new cut list with existing manifests:
    ```go
    func (s *ImageGenService) cleanupOrphanCuts(projectID, projectPath string, sceneNum int, newCuts []CutOutput) {
        existing, _ := s.store.ListShotManifestsByScene(projectID, sceneNum)
        newKeySet := buildCutKeySet(newCuts) // set of (sentenceStart, cutNum)
        for _, m := range existing {
            key := fmt.Sprintf("%d_%d", m.SentenceStart, m.CutNum)
            if !newKeySet[key] {
                s.store.DeleteShotManifest(projectID, sceneNum, m.SentenceStart, m.CutNum)
                removeOrphanCutImage(projectPath, sceneNum, m.SentenceStart, m.CutNum)
            }
        }
    }
    ```
  - Notes: Scene-level cleanup (not sentence-level) because merged cuts span sentence boundaries.

- [x] **Task 14: Update scenario writing template for visual metadata**
  - File: `templates/scenario/03_writing.md`
  - Action: Extend output JSON schema per scene with: `location`, `characters_present`, `color_palette`, `atmosphere`.
  - Notes: Minimal change to scenario pipeline — only writing template output format extended.

### Acceptance Criteria

- [x] **AC 1 (Split)**: Given a scene with narration "문을 열자 빛이 들어왔다. 복도는 어두웠다.", when cut decomposition runs, then at least 3 cuts are generated (2 from first sentence + 1 from second) with distinct `visual_beat` values.

- [x] **AC 2 (Merge)**: Given a scene with narration "SCP-173이 긴 복도를 걸어간다. 발소리가 울린다. 형광등이 깜빡인다.", when cut decomposition runs, then these 3 sentences may produce 1-2 cuts (not 3), with `sentence_start=1, sentence_end=3` for the merged cut.

- [x] **AC 3 (Batch optimization)**: Given a 9-scene scenario with ~6 sentences per scene, when `GenerateAllSceneCuts()` runs, then exactly 9 Stage 1 LLM calls are made (one per scene, batched).

- [x] **AC 4 (StyleGuide injection)**: Given a `Character` with `StyleGuide: "dark, institutional, muted colors"`, when `GenerateShotImage()` runs with `entityVisible=true`, then the `CharacterRef` includes the `StyleGuide` field.

- [x] **AC 5 (Style merge)**: Given project `StyleConfig{ArtStyle: "dark horror anime", ColorPalette: "desaturated blues"}` and scene `ColorPalette: "warm amber tones"`, when merged, then scene-level palette overrides project-level, but `ArtStyle` preserved.

- [x] **AC 6 (Orphan cleanup)**: Given a scene that previously produced 5 cuts but now produces 3 (different structure), when the pipeline runs, then 2 orphan images and manifest rows are deleted.

- [x] **AC 7 (Cache with metadata)**: Given the same narration but changed `ColorPalette` on the SceneScript, when incremental build checks, then cuts are NOT skipped (metadata hash changed).

- [x] **AC 8 (Cache hit)**: Given identical narration AND scene metadata hashes across builds, when `FilterShotsForImageGen()` checks, then all cuts for that scene are skipped.

- [x] **AC 9 (Scene metadata in prompt)**: Given a `SceneScript` with `Location: "underground laboratory"`, when Stage 1 template is rendered, then the value appears in the context preamble.

- [x] **AC 10 (StyleGuide in Stage 2)**: Given a cut with `EntityVisible=true` and Character with `StyleGuide` set, when `02_shot_to_prompt.md` is rendered, then `{style_guide}` is replaced with the character's style guide.

- [x] **AC 11 (Migration)**: Given `013_cut_decomposition.sql` runs on existing data, then existing rows have `sentence_start = old shot_num`, `sentence_end = old shot_num`, `cut_num = 1`.

- [x] **AC 12 (Backward compat)**: Given cuts are generated, then each `Shot` has `ShotNum` set as sequential index, and `Scene.ImagePath` equals the first cut's image path.

- [x] **AC 13 (Max-cuts guard)**: Given LLM returns 5 cuts for a single sentence, when validated, then only the first 3 are kept and the rest discarded with a warning log.

- [x] **AC 14 (Error handling)**: Given Stage 1 LLM returns malformed JSON, when parsed, then JSON repair is attempted. If repair fails, the scene is skipped with error log (pipeline continues).

- [x] **AC 15 (Empty metadata fallback)**: Given a `SceneScript` with empty `Location`/`ColorPalette`/`Atmosphere`, when Stage 1 runs, then project-level defaults are used gracefully (no template rendering errors).

- [x] **AC 16 (Art style suffix)**: Given `StyleConfig{ArtStyle: "watercolor illustration"}`, when `sanitizeImagePrompt()` runs, then the suffix uses "watercolor illustration" — NOT the hardcoded "anime illustration" suffix.

## Additional Context

### Dependencies

- No new external Go dependencies required
- Existing `llm.LLM` interface sufficient — no signature changes
- Existing `imagegen.ImageGen` interface sufficient — only `CharacterRef` struct change
- SQLite migration backward-compatible (new columns with defaults, existing UNIQUE constraint kept)
- Scenario writing template (`03_writing.md`) update recommended but optional — empty metadata fields use project defaults

### Testing Strategy

**Unit Tests:**
- `internal/domain/scene_test.go`: `ShotKey` with sentence range, `SceneVisualMeta` serialization
- `internal/domain/style_test.go`: `MergeSceneStyle()` zero-value logic, all combinations
- `internal/service/shot_breakdown_test.go`:
  - `CutDescription` JSON parsing (split case, merge case, mixed)
  - Max-cuts-per-sentence validation
  - JSON repair logic (unclosed brackets, trailing commas)
  - `formatCutContext()` with merged cuts
  - Cache key computation with metadata hash
- `internal/service/image_gen_test.go`: `StyleGuide` in `CharacterRef`, orphan cleanup with bidirectional cuts, file naming with `sentenceStart_cutNum`
- `internal/store/shot_manifest_test.go`: CRUD with `(scene_num, sentence_start, cut_num)`, `ListShotManifestsByScene()`, scene-level delete

**Integration Tests:**
- Migration: verify `013_cut_decomposition.sql` data migration
- Pipeline: mock LLM returns split+merge JSON → verify correct cut structure and file naming

**Manual Testing:**
- Run on SCP-173 test data → verify split (multi-beat sentences) and merge (continuous scenes)
- Check file naming: `scenes/{sceneNum}/cut_{sentenceStart}_{cutNum}.{ext}`
- Incremental rebuild: modify one sentence → only affected cuts regenerated
- Change scene palette → verify cache invalidation and regeneration

### Notes

- This spec replaces `tech-spec-multi-shot-image-gen.md`
- LLM optimization: 2N → 1+M calls per scene (M = total cuts, can be < N for merged scenes or > N for split sentences)
- **Risk: LLM batch quality** — if full-scene batching degrades cut quality, fall back to 3-4 sentence groups
- **Risk: Cut count explosion** — max 3 cuts per sentence guard enforced in validation
- **Risk: Large JSON parsing** — scene with 6 sentences could produce 6-18 cut objects. Add JSON repair for robustness
- **Future**: Video generation (i2v) per cut — `VideoPath` field already exists on Shot
- **Future**: Stage 2 batching — if per-cut Stage 2 proves stable, batch at scene level for further LLM call reduction (9 additional calls → 0)
