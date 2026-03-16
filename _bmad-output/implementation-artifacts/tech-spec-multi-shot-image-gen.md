---
title: 'Multi-Shot Image Generation per Scene'
slug: 'multi-shot-image-gen'
created: '2026-03-16'
status: 'ready-for-dev'
stepsCompleted: [1, 2, 3, 4]
tech_stack: ['Go 1.25.7', 'SQLite', 'LLM (OpenAI-compatible)', 'SiliconFlow FLUX', 'CapCut JSON']
files_to_modify:
  - 'internal/domain/scene.go'
  - 'internal/domain/manifest.go'
  - 'internal/service/shot_breakdown.go'
  - 'internal/service/image_gen.go'
  - 'internal/service/image_prompt.go (DELETE)'
  - 'internal/service/image_prompt_test.go (DELETE)'
  - 'internal/service/assembler.go'
  - 'internal/pipeline/runner.go'
  - 'internal/pipeline/incremental.go'
  - 'internal/pipeline/incremental_test.go'
  - 'internal/store/manifest.go'
  - 'internal/store/migrations/012_shot_manifests.sql'
  - 'internal/plugin/output/capcut/capcut.go'
  - 'internal/plugin/output/interface.go'
  - 'internal/api/assets.go'
  - 'templates/image/01_shot_breakdown.md'
code_patterns:
  - 'Plugin interface pattern (imagegen.ImageGen, llm.LLM)'
  - 'Domain model → Store CRUD → Service layer'
  - 'Hash-based incremental build (SceneSkipChecker)'
  - 'Fault-tolerant loops (continue on error, collect partial results)'
test_patterns:
  - 'Same-package *_test.go with testify'
  - 'Table-driven tests for edge cases'
---

# Tech-Spec: Multi-Shot Image Generation per Scene

**Created:** 2026-03-16

## Overview

### Problem Statement

The current pipeline generates exactly 1 image per scene. For a typical 10-minute video with 9 scenes, this yields 9 images — averaging 67 seconds per image. Competing SCP YouTube channels swap images every 10-15 seconds, which is 4-6x more frequent. The low image density drives viewer drop-off.

### Solution

Adopt a **1 sentence = 1 shot** model: each sentence in the narration becomes an independent shot with its own image. For a 9-scene video with ~6-10 sentences per scene, this produces 54-90 images — hitting the 10-15 second swap target naturally.

The Shot Breakdown LLM stage is simplified: instead of deciding *how many* shots to create, it decides *how to visualize* each sentence (camera angle, composition, entity visibility). The legacy `image_prompt.go` template path is deleted. A `VideoPath` field is added to Shot for future image-to-video (i2v) support.

### Scope

**In Scope:**
- `Shot` domain model with `VideoPath` field (i2v-ready) added to `Scene` as `[]Shot`
- 1 sentence = 1 shot mapping — deterministic, no LLM grouping decisions
- Shot Breakdown template rewritten: input is individual sentence → output is shot description
- `ShotBreakdownPipeline` returns `[]ShotDescription` per scene (one per sentence)
- `ImageGenService` generates images per shot with shot-level character auto-reference
- `shot_manifests` SQLite table for shot-level incremental builds
- CapCut assembler places N video clips per scene using sentence-level WordTimings
- Delete legacy `image_prompt.go` and `ImagePromptResult` type entirely
- `SplitNarrationSentences()` helper for Korean sentence boundary detection

**Out of Scope:**
- Image-to-video (i2v) generation (model field reserved only)
- Shot count user override UI
- Shot transition effects (manual in CapCut)
- Parallel image generation optimization (follow-up)
- Dashboard/API endpoint changes beyond removing `ImagePromptResult`
- Approval workflow changes (stays at scene level)

## Context for Development

### Codebase Patterns

1. **Plugin Architecture**: All external services (LLM, ImageGen, TTS) behind interfaces in `internal/plugin/`.
2. **Domain → Store → Service layering**: Domain structs in `internal/domain/`, SQLite CRUD in `internal/store/`, business logic in `internal/service/`.
3. **Hash-based incremental builds**: `SceneSkipChecker` in `internal/pipeline/incremental.go` compares content hashes. Extended to shot-level: sentence text hash = content_hash.
4. **Fault-tolerant loops**: `GenerateAllImages()` continues on per-scene errors and returns partial results. Same pattern for per-shot generation.
5. **Parallel generation**: `runner.go:runParallelGeneration()` spawns image + TTS goroutines. Image goroutine now iterates shots within each scene.
6. **Scene merging**: `runner.go:mergeSceneData()` combines image + TTS + timing data. Updated to carry `[]Shot` through and resolve shot timings from WordTimings.

### Files to Reference

| File | Purpose |
| ---- | ------- |
| `internal/domain/scene.go` | Scene + WordTiming structs — add Shot struct here |
| `internal/domain/manifest.go` | SceneManifest struct — reference pattern for ShotManifest |
| `internal/service/shot_breakdown.go` | 2-stage LLM pipeline — rewrite Stage 1 for per-sentence input |
| `internal/service/image_gen.go` | ImageGenService — refactor to shot-level generation |
| `internal/service/image_prompt.go` | Legacy prompt gen — DELETE entirely |
| `internal/pipeline/runner.go` | Pipeline orchestrator — update image gen stage + merge |
| `internal/pipeline/incremental.go` | Skip checker — extend to shot-level hashing |
| `internal/store/manifest.go` | Manifest CRUD — add shot manifest methods |
| `internal/store/migrations/001_initial.sql` | Reference schema for scene_manifests |
| `internal/plugin/output/capcut/capcut.go` | CapCut assembler — multi-clip per scene |
| `internal/plugin/output/interface.go` | AssembleInput/AssembleResult types |
| `templates/image/01_shot_breakdown.md` | Shot breakdown LLM template — rewrite for per-sentence |
| `templates/image/02_shot_to_prompt.md` | Shot-to-prompt LLM template (unchanged) |

### Technical Decisions

1. **1 sentence = 1 shot (Option A)**: Deterministic mapping eliminates LLM grouping decisions. Shot count equals sentence count. No `start_sentence_idx` / `end_sentence_idx` needed — shot N corresponds to sentence N. Benefits: predictable output, trivial incremental builds (sentence hash = content_hash), simpler timing resolution, better i2v compatibility (shorter clips = more natural motion).

2. **Shot timing from WordTimings**: Each sentence's time range is derived from WordTimings — first word's `StartSec` to last word's `EndSec`. No LLM involvement in timing. Computed post-TTS in the merge stage.

3. **`VideoPath` field for future i2v**: Shot struct includes `VideoPath string`. Empty = use ImagePath as still image. Non-empty = CapCut uses video clip. Field is reserved now; VideoGenService plugin comes in a follow-up epic.

4. **Separate `shot_manifests` table**: New table with `(project_id, scene_num, shot_num)` composite key. Scene-level manifests remain for audio/subtitle hashing. When a sentence changes, only that shot is invalidated.

5. **Delete legacy `image_prompt.go`**: `GenerateImagePrompts()` and `ImagePromptResult` replaced entirely by `ShotBreakdownPipeline` output. All callers updated.

6. **Stage 1 role change**: Shot Breakdown Stage 1 no longer decides "how many shots" — it receives one sentence and decides "how to frame this sentence visually" (camera, composition, entity visibility). Stage 2 (shot-to-prompt) is unchanged.

## Implementation Plan

### Tasks

#### Task 1: Domain Model — Shot struct and Scene extension
**File:** `internal/domain/scene.go`

Add `Shot` struct and `SplitNarrationSentences()` helper:

```go
// Shot represents a single visual shot within a scene.
// Each shot maps 1:1 to a narration sentence.
type Shot struct {
	ShotNum        int     `json:"shot_num"`         // 1-based, equals sentence index + 1
	Role           string  `json:"role"`             // establishing | action | reaction | detail | transition
	CameraType     string  `json:"camera_type"`
	EntityVisible  bool    `json:"entity_visible"`
	ImagePrompt    string  `json:"image_prompt"`     // sanitized prompt for image generation
	NegativePrompt string  `json:"negative_prompt"`
	ImagePath      string  `json:"image_path"`       // path to generated image file
	VideoPath      string  `json:"video_path"`       // reserved for future i2v; empty = still image
	StartSec       float64 `json:"start_sec"`        // resolved post-TTS from WordTimings
	EndSec         float64 `json:"end_sec"`          // resolved post-TTS from WordTimings
	SentenceText   string  `json:"sentence_text"`    // the narration sentence this shot visualizes
}
```

Add `Shots []Shot` field to existing `Scene` struct (after `SubtitlePath`).

Add sentence splitting helper:

```go
// SplitNarrationSentences splits Korean narration into sentences.
// Splits on sentence-ending punctuation: 다. 요. 까? 죠. etc.
// Preserves quoted text as part of the containing sentence.
func SplitNarrationSentences(narration string) []string
```

Implementation notes:
- Use regex: split on `[.?!]\s+` but not inside quotes
- Handle Korean-specific endings: `~다.`, `~요.`, `~까?`, `~죠.`, `~네.`
- Handle ellipsis (`...`) — don't split mid-ellipsis
- Trim whitespace, filter empty strings

**Tests:** `internal/domain/scene_test.go`
- Basic splitting: "첫 문장이다. 두 번째 문장이다." → ["첫 문장이다.", "두 번째 문장이다."]
- Quoted text preservation: `그는 "가지 마라." 라고 말했다.` → single sentence
- Ellipsis: "그것은... 무언가였다." → single sentence
- Single sentence input → 1-element slice
- Empty/whitespace input → empty slice

#### Task 2: Domain Model — ShotManifest struct
**File:** `internal/domain/manifest.go`

```go
// ShotManifest tracks the generation state of a single shot for incremental builds.
type ShotManifest struct {
	ProjectID   string
	SceneNum    int
	ShotNum     int
	ContentHash string    // SHA-256 of sentence text (input)
	ImageHash   string    // SHA-256 of generated image file
	GenMethod   string    // "image_edit", "text_to_image", "fallback_t2i"
	Status      string    // "pending", "generated", "failed"
	UpdatedAt   time.Time
}
```

#### Task 3: SQLite Migration — shot_manifests table
**File:** `internal/store/migrations/012_shot_manifests.sql`

```sql
CREATE TABLE shot_manifests (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    project_id TEXT NOT NULL REFERENCES projects(id),
    scene_num INTEGER NOT NULL,
    shot_num INTEGER NOT NULL,
    content_hash TEXT NOT NULL DEFAULT '',
    image_hash TEXT NOT NULL DEFAULT '',
    gen_method TEXT NOT NULL DEFAULT 'text_to_image',
    status TEXT NOT NULL DEFAULT 'pending',
    updated_at TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE(project_id, scene_num, shot_num)
);
```

#### Task 4: Store — Shot manifest CRUD
**File:** `internal/store/manifest.go`

Add methods following the existing `SceneManifest` CRUD pattern:

```go
func (s *Store) CreateShotManifest(m *domain.ShotManifest) error
func (s *Store) GetShotManifest(projectID string, sceneNum, shotNum int) (*domain.ShotManifest, error)
func (s *Store) ListShotManifestsByScene(projectID string, sceneNum int) ([]*domain.ShotManifest, error)
func (s *Store) UpdateShotManifest(m *domain.ShotManifest) error
func (s *Store) DeleteShotManifestsByScene(projectID string, sceneNum int) error
```

- `CreateShotManifest`: INSERT with `updated_at = datetime('now')`, same pattern as `CreateManifest`
- `GetShotManifest`: SELECT by `(project_id, scene_num, shot_num)`, return `NotFoundError` if missing
- `ListShotManifestsByScene`: SELECT all shots for a scene, ORDER BY shot_num
- `UpdateShotManifest`: UPDATE by composite key, return `NotFoundError` if 0 rows affected
- `DeleteShotManifestsByScene`: DELETE WHERE project_id AND scene_num — used when scene narration changes to invalidate all shots

**Tests:** `internal/store/manifest_test.go`
- CRUD happy path
- GetShotManifest not found → NotFoundError
- UpdateShotManifest not found → NotFoundError
- DeleteShotManifestsByScene removes all shots for that scene, leaves other scenes intact
- UNIQUE constraint violation on duplicate (project_id, scene_num, shot_num)

#### Task 5: Shot Breakdown Template — Per-sentence input
**File:** `templates/image/01_shot_breakdown.md`

Rewrite the template. Key change: input is now a **single sentence** (not a full scene synopsis).

```markdown
You are a professional anime art director. Given a single narration sentence from an SCP horror/documentary anime, create a shot description for one illustrated frame.

## Entity Visual Identity
{entity_visual_identity}

## Frozen Descriptor (USE VERBATIM when entity is visible)
{frozen_descriptor}

## Shot Context
- Scene Number: {scene_number}
- Shot Number: {shot_number} of {total_shots} in this scene
- Sentence: {sentence}
- Emotional Beat: {emotional_beat}
- Previous Shot Context: {previous_shot_context}

## Instructions

Create a single shot description for this sentence. The shot must:
1. Visualize ONLY the content of this specific sentence
2. Choose the most impactful camera angle for the emotional beat
3. If the entity is visible, the `subject` field MUST start with the FROZEN DESCRIPTOR text verbatim
4. Maintain visual continuity with the previous shot

## Output Format (JSON only)

{
  "shot_number": {shot_number},
  "role": "establishing | action | reaction | detail | transition",
  "camera_type": "wide | medium | close-up | extreme close-up | POV | over-the-shoulder | bird's eye | low angle",
  "entity_visible": true/false,
  "subject": "description of what is shown",
  "lighting": "description of lighting setup",
  "mood": "single word mood descriptor",
  "motion": "camera or subject motion description"
}

Return ONLY valid JSON, no additional text.
```

Note: `{shot_number}`, `{total_shots}`, and `{sentence}` are new template variables. `{synopsis}` is removed. `{previous_last_shot_context}` renamed to `{previous_shot_context}` (now the immediately preceding shot, not previous scene's last shot).

#### Task 6: ShotBreakdownPipeline — Per-sentence shot generation
**File:** `internal/service/shot_breakdown.go`

**6a. New `SentencePromptInput` struct:**
```go
// SentencePromptInput holds input for generating a shot description for one sentence.
type SentencePromptInput struct {
	SceneNum             int
	ShotNum              int    // 1-based
	TotalShots           int    // total sentences in this scene
	Sentence             string // the specific narration sentence
	EmotionalBeat        string
	EntityVisualIdentity string
	FrozenDescriptor     string
	PreviousShotCtx      string // previous shot's context (within same scene or last shot of prev scene)
}
```

**6b. Update `ScenePromptOutput`:**
```go
type ScenePromptOutput struct {
	SceneNum int          `json:"scene_num"`
	Shots    []ShotOutput `json:"shots"`
}

type ShotOutput struct {
	ShotNum        int              `json:"shot_num"`
	ShotDesc       *ShotDescription `json:"shot_description"`
	PromptResult   *ShotPromptResult `json:"prompt_result"`
	FinalPrompt    string           `json:"final_prompt"`
	NegativePrompt string           `json:"negative_prompt"`
	SentenceText   string           `json:"sentence_text"`
}
```

**6c. Update `GenerateScenePrompt()`:**
```go
func (sp *ShotBreakdownPipeline) GenerateScenePrompt(ctx context.Context, input ScenePromptInput) (*ScenePromptOutput, error)
```
New flow:
1. Split `input.Synopsis` (narration) into sentences via `domain.SplitNarrationSentences()`
2. For each sentence (index i):
   a. Build `SentencePromptInput` with `ShotNum = i + 1`
   b. Run Stage 1 (`runShotBreakdown`) — now takes `SentencePromptInput`, returns single `*ShotDescription`
   c. Run Stage 2 (`runShotToPrompt`) — unchanged, takes `*ShotDescription`
   d. Apply `sanitizeImagePrompt()` to prompt
   e. Build `ShotOutput` with `SentenceText = sentence`
   f. Update `PreviousShotCtx` for next iteration
3. Return `ScenePromptOutput` with all `ShotOutput` entries

**6d. Update `runShotBreakdown()`:**
Change template variable substitution:
- Remove `{synopsis}` replacement
- Add `{sentence}`, `{shot_number}`, `{total_shots}` replacements
- Rename `{previous_last_shot_context}` → `{previous_shot_context}`
- Parse response as single `ShotDescription` (same as before — one JSON object per call)

**6e. Update `GenerateAllScenePrompts()`:**
- `PreviousLastShotCtx` for each scene = last `ShotOutput`'s context from previous scene
- Within each scene, `PreviousShotCtx` chains shot-to-shot

**6f. Keep `ScenePromptInput` for backward compat** but `Synopsis` field now contains the full narration text (split into sentences internally).

**Tests:** `internal/service/shot_breakdown_test.go`
- Mock LLM: scene with 3 sentences → 3 ShotOutputs
- Mock LLM: scene with 1 sentence → 1 ShotOutput
- Previous shot context chains correctly within scene
- Previous shot context chains across scenes
- LLM error on one sentence → that shot marked as error, others continue

#### Task 7: ImageGenService — Shot-level generation
**File:** `internal/service/image_gen.go`

**7a. New `GenerateShotImage()` method:**
```go
func (s *ImageGenService) GenerateShotImage(
	ctx context.Context,
	projectID, projectPath string,
	sceneNum, shotNum int,
	prompt, negativePrompt string,
	entityVisible bool,
	scpID string,
	opts imagegen.GenerateOptions,
) (*domain.Shot, error)
```

Logic extracted from existing `GenerateSceneImage()`:
- Character auto-reference: `entityVisible` is per-shot (key difference)
- Image saved to `scenes/{sceneNum}/shot_{shotNum}.{ext}` (was `image.{ext}`)
- Prompt saved to `scenes/{sceneNum}/shot_{shotNum}_prompt.txt`
- Updates `shot_manifests` via store (not scene_manifests)
- Returns `*domain.Shot` with `ImagePath` and `ShotNum` populated

**7b. New `GenerateAllShotImages()` method:**
```go
func (s *ImageGenService) GenerateAllShotImages(
	ctx context.Context,
	scenePrompts []*ScenePromptOutput,
	projectID, projectPath, scpID string,
	opts imagegen.GenerateOptions,
	skipShots map[ShotKey]bool,
) ([]*domain.Scene, error)
```
- Iterates scenes → shots sequentially
- Skips shots in `skipShots` map (from incremental checker)
- Fault-tolerant: on per-shot error, logs + marks shot_manifest as failed + continues
- Returns `[]*domain.Scene` with `Shots []Shot` populated

**7c. `ShotKey` type** (shared between image_gen and incremental):
```go
// ShotKey uniquely identifies a shot within a project.
type ShotKey struct {
	SceneNum int
	ShotNum  int
}
```
Place in `internal/service/types.go` or `internal/domain/scene.go`.

**7d. Delete old methods:**
- Delete `GenerateSceneImage()` — replaced by `GenerateShotImage()`
- Delete `GenerateAllImages()` — replaced by `GenerateAllShotImages()`
- Delete `filterPrompts()` helper — no longer needed

**7e. Update `BackupSceneImage()`:**
Rename to `BackupShotImage(projectPath string, sceneNum, shotNum int)` — looks for `shot_{shotNum}.*` instead of `image.*`.

**Tests:** `internal/service/image_gen_test.go`
- GenerateShotImage: file saved to correct path `shot_1.png`
- GenerateShotImage: entity_visible=true → character refs injected
- GenerateShotImage: entity_visible=false → no character refs
- GenerateAllShotImages: 2 scenes × 3 shots = 6 images
- GenerateAllShotImages: skip map respected
- GenerateAllShotImages: partial failure continues

#### Task 8: Delete legacy image_prompt.go
**Files to delete:**
- `internal/service/image_prompt.go`
- `internal/service/image_prompt_test.go`

**Callers to update:**

1. `internal/pipeline/runner.go` (~line 759):
   - Remove: `prompts, err := service.GenerateImagePrompts(scenario, nil)`
   - Replace with `ShotBreakdownPipeline.GenerateAllScenePrompts()` call (Task 10)

2. `internal/api/assets.go` (~line 775):
   - Remove: `service.ImagePromptResult{...}` construction
   - Replace with shot-based regeneration: split sentence, run shot breakdown for that sentence, generate image

3. `internal/pipeline/incremental.go`:
   - Remove: `FilterScenesForImageGen()` parameter type `[]service.ImagePromptResult`
   - Replace with shot-level method (Task 9)

4. `internal/pipeline/incremental_test.go`:
   - Remove: all references to `service.ImagePromptResult`
   - Replace with shot-level test cases

After deletion, verify: `grep -r "ImagePromptResult\|GenerateImagePrompts\|image_prompt" --include="*.go"` returns zero hits (excluding `_test.go` for deleted files).

#### Task 9: Incremental Build — Shot-level skip checker
**File:** `internal/pipeline/incremental.go`

**9a. New `FilterShotsForImageGen()` method:**
```go
func (c *SceneSkipChecker) FilterShotsForImageGen(
	projectID string,
	scenePrompts []*service.ScenePromptOutput,
) (toGenerate []service.ShotKey, toSkip []service.ShotKey)
```

Logic per shot:
1. Compute `contentHash = SHA256(shot.SentenceText)`
2. Fetch `shot_manifests` entry for `(projectID, sceneNum, shotNum)`
3. If manifest exists AND `contentHash == manifest.ContentHash` AND `manifest.ImageHash != ""` → skip
4. Else → generate

**9b. Scene-level invalidation:**
Before filtering, check if scene's total sentence count changed vs stored shot count:
```go
storedShots, _ := store.ListShotManifestsByScene(projectID, sceneNum)
if len(storedShots) != len(scenePrompt.Shots) {
    store.DeleteShotManifestsByScene(projectID, sceneNum) // invalidate all
}
```

**9c. Remove old `FilterScenesForImageGen()`** — replaced entirely.

**Tests:** `internal/pipeline/incremental_test.go`
- Shot unchanged (same sentence hash + image exists) → skip
- Shot prompt changed → regenerate
- New shot added (scene expanded) → all shots in scene regenerated
- Shot removed (scene shortened) → all shots in scene regenerated

#### Task 10: Pipeline Runner — Wire multi-shot flow
**File:** `internal/pipeline/runner.go`

**10a. Update `runParallelGeneration()` image goroutine:**

Current:
```go
prompts, err := service.GenerateImagePrompts(scenario, nil)
imageScenes, err = imgSvc.GenerateAllImages(ctx, prompts, projectID, projectPath, opts, nil)
```

New:
```go
// Shot breakdown: generate per-sentence shot descriptions + prompts
shotPipeline, err := service.NewShotBreakdownPipeline(r.llm, service.ShotBreakdownConfig{
    TemplatesDir: r.templatesDir,
})
scenePrompts, err := shotPipeline.GenerateAllScenePrompts(ctx, scenario, frozenDesc, visualIdentity)

// Incremental: skip unchanged shots
toGen, toSkip := r.skipChecker.FilterShotsForImageGen(projectID, scenePrompts)
skipMap := make(map[service.ShotKey]bool, len(toSkip))
for _, k := range toSkip {
    skipMap[k] = true
}

// Image generation: per-shot
imageScenes, err = imgSvc.GenerateAllShotImages(ctx, scenePrompts, projectID, projectPath, scpID, opts, skipMap)
```

**10b. Update `mergeSceneData()`:**
Current merge overlays TTS data onto image scenes. Add shot timing resolution:

```go
func mergeSceneData(imageScenes, ttsScenes []*domain.Scene, timings []domain.SceneTiming) []*domain.Scene {
    // ... existing merge logic (build byNum map, overlay TTS data) ...

    // NEW: Resolve shot timings from WordTimings
    for _, scene := range merged {
        if len(scene.Shots) > 0 && len(scene.WordTimings) > 0 {
            resolveShotTimings(scene)
        }
    }

    return merged
}
```

**10c. New `resolveShotTimings()` helper:**
```go
// resolveShotTimings maps sentence-based shots to WordTiming timestamps.
// Shot N corresponds to sentence N. Sentence boundaries are detected by
// matching sentence text against accumulated WordTiming words.
func resolveShotTimings(scene *domain.Scene) {
    sentences := domain.SplitNarrationSentences(scene.Narration)
    if len(sentences) != len(scene.Shots) {
        // Mismatch — log warning, assign equal duration fallback
        equalDur := scene.AudioDuration / float64(len(scene.Shots))
        for i := range scene.Shots {
            scene.Shots[i].StartSec = float64(i) * equalDur
            scene.Shots[i].EndSec = float64(i+1) * equalDur
        }
        return
    }

    // Build sentence → time range mapping from WordTimings
    sentenceTimings := mapSentencesToTimings(sentences, scene.WordTimings)
    for i := range scene.Shots {
        scene.Shots[i].StartSec = sentenceTimings[i].Start
        scene.Shots[i].EndSec = sentenceTimings[i].End
    }
}

type sentenceTiming struct {
    Start float64
    End   float64
}

// mapSentencesToTimings accumulates word timings to find sentence boundaries.
// Words are concatenated and matched against sentence text to find where each
// sentence starts and ends in the audio timeline.
func mapSentencesToTimings(sentences []string, wordTimings []domain.WordTiming) []sentenceTiming {
    timings := make([]sentenceTiming, len(sentences))
    wordIdx := 0

    for si, sentence := range sentences {
        // Find first word of this sentence
        if wordIdx < len(wordTimings) {
            timings[si].Start = wordTimings[wordIdx].StartSec
        }

        // Count words in sentence, advance wordIdx
        sentenceWords := countWords(sentence)
        endWordIdx := wordIdx + sentenceWords - 1
        if endWordIdx >= len(wordTimings) {
            endWordIdx = len(wordTimings) - 1
        }
        timings[si].End = wordTimings[endWordIdx].EndSec
        wordIdx = endWordIdx + 1
    }

    // Ensure last sentence extends to audio end
    if len(timings) > 0 && len(wordTimings) > 0 {
        timings[len(timings)-1].End = wordTimings[len(wordTimings)-1].EndSec
    }

    return timings
}
```

**10d. Update `loadScenesFromDir()` / `parseSceneManifest()`:**
Scene manifest JSON now includes `shots` array. Update deserialization to populate `scene.Shots`.

**Tests:** `internal/pipeline/runner_test.go`
- `resolveShotTimings`: 3 sentences, 9 WordTimings → 3 shots with correct time ranges
- `resolveShotTimings`: mismatch fallback → equal duration distribution
- `mapSentencesToTimings`: single sentence covers full audio
- `mapSentencesToTimings`: empty WordTimings → zero timings

#### Task 11: CapCut Assembler — Multi-clip per scene
**File:** `internal/plugin/output/capcut/capcut.go`

**11a. Update `buildDraftProject()` video track logic:**

Replace the single-image-per-scene loop body with:

```go
for _, scene := range scenes {
    dur := scene.AudioDuration
    if dur <= 0 {
        dur = DefaultSceneDurationSec
    }
    audioDur := secsToMicro(dur)

    // Video track: one clip per shot (or single image fallback)
    if len(scene.Shots) > 0 {
        for _, shot := range scene.Shots {
            imagePath := shot.ImagePath
            // Future i2v: prefer VideoPath if available
            if shot.VideoPath != "" {
                imagePath = shot.VideoPath
            }

            shotDur := secsToMicro(shot.EndSec - shot.StartSec)
            if shotDur <= 0 {
                shotDur = audioDur / int64(len(scene.Shots)) // equal fallback
            }
            shotStart := timelinePos + secsToMicro(shot.StartSec)

            videoMat := VideoMaterial{
                ID:           newID(),
                Type:         "photo",
                Duration:     shotDur,
                Path:         imagePath,
                Width:        canvas.Width,
                Height:       canvas.Height,
                MaterialName: fmt.Sprintf("scene_%d_shot_%d", scene.SceneNum, shot.ShotNum),
                CategoryName: "local",
            }
            dp.Materials.Videos = append(dp.Materials.Videos, videoMat)

            videoSeg := Segment{
                ID:              newID(),
                SourceTimerange: &TimeRange{Start: 0, Duration: shotDur},
                TargetTimerange: &TimeRange{Start: shotStart, Duration: shotDur},
                Speed:           1.0,
                Volume:          1.0,
                Clip: &Clip{
                    Scale:     &XY{X: 1.0, Y: 1.0},
                    Rotation:  0,
                    Transform: &XY{X: 0, Y: 0},
                    Flip:      &Flip{},
                    Alpha:     1.0,
                },
                MaterialID:        videoMat.ID,
                ExtraMaterialRefs: []string{},
                RenderIndex:       0,
                Visible:           true,
            }
            videoTrack.Segments = append(videoTrack.Segments, videoSeg)
        }
    } else {
        // Backward compat: no shots = single image clip (existing code unchanged)
        videoMat := VideoMaterial{
            ID:           newID(),
            Type:         "photo",
            Duration:     audioDur,
            Path:         scene.ImagePath,
            Width:        canvas.Width,
            Height:       canvas.Height,
            MaterialName: fmt.Sprintf("scene_%d", scene.SceneNum),
            CategoryName: "local",
        }
        dp.Materials.Videos = append(dp.Materials.Videos, videoMat)
        // ... existing segment creation ...
    }

    // Audio + Text tracks: unchanged (still per-scene narration + per-word subtitles)
    // ... existing audio/text code ...

    timelinePos += audioDur
}
```

**11b. Update `AssembleResult`:**
`ImageCount` now counts total shots: `sum(len(scene.Shots))` across all scenes.

**11c. Update `buildDraftMeta()`:**
Image entries in meta should list all shot images, not just one per scene.

**Tests:** `internal/plugin/output/capcut/capcut_test.go`
- 2 scenes × 3 shots → 6 video segments, contiguous timeline
- Scene with 0 shots (backward compat) → single video segment
- Shot with VideoPath set → uses VideoPath instead of ImagePath
- Shot durations sum equals scene audio duration
- Total project duration unchanged (sum of scene audio durations)

#### Task 12: Update API assets handler
**File:** `internal/api/assets.go`

Find the `service.ImagePromptResult` usage (~line 775) and replace:

Current: constructs `ImagePromptResult` for single-scene regeneration.
New: for single-scene regeneration endpoint:
1. Load scene narration
2. Split into sentences
3. Run `ShotBreakdownPipeline.GenerateScenePrompt()` for the scene
4. Run `ImageGenService.GenerateShotImage()` for each shot
5. Return updated scene with all shot images

This keeps the API contract (regenerate a scene) while internally generating all shots.

### Acceptance Criteria

**AC1: 1 sentence = 1 shot**
- Given a scene with N sentences in narration
- When the shot breakdown stage runs
- Then exactly N shots are generated, one per sentence
- And each shot has an independent camera angle and entity_visible flag

**AC2: Shot-level image files**
- Given a scene with N shots
- When image generation completes
- Then N image files exist at `scenes/{sceneNum}/shot_{shotNum}.{ext}`
- And N prompt files exist at `scenes/{sceneNum}/shot_{shotNum}_prompt.txt`

**AC3: Shot timing from WordTimings**
- Given shots mapped to sentences
- When TTS completes and WordTimings are available
- Then each shot has `StartSec` / `EndSec` resolved from sentence word boundaries
- And shots cover the full narration duration without gaps

**AC4: CapCut multi-clip output**
- Given a scene with 5 shots
- When CapCut assembly runs
- Then the video track has 5 contiguous segments for that scene
- And each segment duration matches the shot's resolved timing
- And audio track still has 1 segment per scene (unchanged)

**AC5: Incremental build at shot level**
- Given a previously generated project
- When one sentence in a scene changes
- Then only that shot is regenerated
- And other shots in the same scene are skipped (if sentence unchanged)

**AC6: Scene expansion/contraction**
- Given a scene that previously had 5 sentences, now has 7
- When incremental check runs
- Then all shot_manifests for that scene are invalidated
- And all 7 shots are regenerated

**AC7: Backward compatibility**
- Given a scene with `Shots` slice empty (legacy data)
- When the pipeline runs
- Then it falls back to single-image behavior using `scene.ImagePath`

**AC8: Legacy removal**
- Given the codebase after this change
- When running `grep -r "ImagePromptResult\|GenerateImagePrompts" --include="*.go"`
- Then zero results

**AC9: Character auto-reference per shot**
- Given a scene where shot 1 has `entity_visible=false` and shot 3 has `entity_visible=true`
- When image generation runs
- Then shot 1 uses text_to_image without character ref
- And shot 3 uses image_edit with character reference injection

**AC10: VideoPath field reserved**
- Given a Shot with `VideoPath = ""`
- When CapCut assembler runs
- Then it uses `ImagePath` as a still image clip
- Given a Shot with `VideoPath = "/path/to/video.mp4"` (future)
- Then it uses `VideoPath` as the clip source

**AC11: Korean sentence splitting**
- Given narration: "SCP-173은 콘크리트 조각상이다. 눈을 떼면 움직인다. 매우 위험하다."
- When `SplitNarrationSentences()` runs
- Then returns 3 sentences

## Additional Context

### Dependencies

- No new external Go dependencies
- SQLite migration `012_shot_manifests.sql` must be applied before shot features work
- Template change (`01_shot_breakdown.md`) must land first — all service code depends on new LLM output format

### Testing Strategy

| Test area | Type | Key scenarios |
| --------- | ---- | ------------- |
| `SplitNarrationSentences` | Unit | Korean endings, quotes, ellipsis, edge cases |
| `ShotManifest` store CRUD | Unit | Create, get, update, delete, list, unique constraint |
| `ShotBreakdownPipeline` | Integration (mock LLM) | Multi-sentence scene, single-sentence scene, cross-scene context |
| `GenerateShotImage` | Unit (mock ImageGen) | File paths, character ref injection, manifest update |
| `resolveShotTimings` | Unit | Sentence→WordTiming mapping, mismatch fallback |
| CapCut assembler | Unit | Multi-shot timeline, backward compat, VideoPath |
| Incremental skip | Unit (mock store) | Hash match/mismatch, scene expansion/contraction |

### Notes

- `02_shot_to_prompt.md` template requires NO changes — it processes a single ShotDescription. Called N times per scene (one per sentence) instead of once.
- Shot numbering: 1-based within each scene (`shot_num = sentence_index + 1`).
- Known gap: `SceneManifest.GenerationMethod` field is unused in DB schema. New `shot_manifests.gen_method` properly persists this.
- Approval workflow stays at scene level — approving a scene approves all its shots.
- `VideoPath` field is write-ready but no code sets it in this spec. Future i2v epic will add a `VideoGenService` plugin stage between image_gen and assembly.
- If scenario generation stage already controls sentence length (~10-15 sec/sentence), the 1:1 mapping naturally hits the 10-15 second image swap target without any additional logic.

### Task Dependency Order

```
Task 1 (Shot model + SplitSentences) ─┐
Task 2 (ShotManifest)                 ─┼→ Task 3 (migration) → Task 4 (store CRUD)
                                       │
Task 5 (template rewrite)             ─┼→ Task 6 (pipeline service)
                                       │         │
Task 8 (delete legacy)  ───────────────┤         ↓
                                       │  Task 7 (image gen service)
                                       │         │
                                       └→ Task 9 (incremental) ──→ Task 10 (runner wiring)
                                                                          │
                                                                   Task 11 (CapCut) → Task 12 (API)
```

**Recommended implementation order:** 1 → 2 → 3 → 4 → 5 → 6 → 8 → 7 → 9 → 10 → 11 → 12
