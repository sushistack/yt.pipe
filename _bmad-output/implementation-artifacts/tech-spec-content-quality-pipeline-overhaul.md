---
title: 'Content Quality Pipeline Overhaul'
slug: 'content-quality-pipeline-overhaul'
created: '2026-03-15'
status: 'ready-for-dev'
stepsCompleted: [1, 2, 3, 4]
elicitationCompleted: [party-mode, adr, ux-focus-group, pre-mortem, first-principles, what-if, comparative-matrix]
tech_stack:
  - 'Go 1.25.7'
  - 'DashScope Qwen3-TTS-VC (qwen3-tts-vc-2026-01-22)'
  - 'SiliconFlow FLUX (Qwen/Qwen-Image-Edit — NEEDS PoC VERIFICATION)'
  - 'OpenAI-compatible LLM (scenario prompts)'
  - 'SQLite (character/voice storage)'
files_to_modify:
  - 'templates/scenario/01_research.md'
  - 'templates/scenario/02_structure.md'
  - 'templates/scenario/03_writing.md'
  - 'templates/scenario/04_review.md'
  - 'templates/scenario/format_guide.md (NEW)'
  - 'internal/plugin/imagegen/interface.go'
  - 'internal/plugin/imagegen/siliconflow.go'
  - 'internal/plugin/tts/dashscope.go'
  - 'internal/plugin/tts/interface.go'
  - 'internal/service/scenario_pipeline.go'
  - 'internal/service/image_gen.go'
  - 'internal/service/tts.go'
  - 'internal/service/character.go'
  - 'internal/config/types.go'
  - 'internal/cli/tts_cmd.go'
  - 'internal/cli/character_cmd.go'
  - 'internal/store/migrations/007_voice_cache_and_character_image.sql (NEW)'
  - 'internal/store/voice_cache.go (NEW)'
code_patterns:
  - 'Placeholder injection via strings.ReplaceAll() in scenario_pipeline.go'
  - 'Plugin interface pattern: single Generate() method, opts struct for config'
  - 'Service layer wraps plugin calls with retry, file I/O, manifest updates'
  - 'CLI commands open fresh DB connection, auto-close via cleanup'
  - 'DashScope HTTP: POST to /api/v1/services/aigc/multimodal-generation/generation'
test_patterns:
  - 'Mock HTTP server for API testing (dashscope_test.go pattern)'
  - 'testify assertions'
  - 'internal/mocks package for mock TTS/ImageGen'
---

# Tech-Spec: Content Quality Pipeline Overhaul

**Created:** 2026-03-15

## Overview

### Problem Statement

The pipeline's three core outputs (scenario, images, voice) are below production quality. Scenarios lack strong hooks and have not been validated against proven YouTube formats. Images lack cross-scene character consistency (pure text-to-image with no reference image support). Voice output uses a generic preset voice with no personality.

### Solution

Three parallel improvement tracks:

**Track 1: Scenario Quality Improvement**

Core insight: Current prompts are optimized for **accurate** scenarios, not **engaging** scenarios. The fundamental goal is audience retention — viewers watch to the end and come back for more. Channel analysis provides evidence and examples; the real deliverable is embedding storytelling principles into prompts.

- Analyze popular SCP YouTube channel formats from `docs/scp.yt.channels.analysis.md` (all channels, no bias toward specific formats)
- Apply analysis findings to scenario prompt templates (`templates/scenario/01~04`)
- Inject Format Guide into Stage 1~3 (not just Stage 2) — each stage receives relevant portion
- Add storytelling quality check to Stage 4 (Review) — verify hook strength, information curve, emotional transitions (not just fact accuracy)
- Video length selection: separate tech spec already exists (`tech-spec-scenario-duration-control.md`) — reference only, not in this scope

**Prompt changes driven by first principles:**

| Current State | Change | Target Stage |
|--------------|--------|-------------|
| No hook guidance | Add hook type library (question, shock, mystery, contrast) with examples | Stage 2, 3 |
| No information disclosure ordering | Add progressive disclosure principle — "reveal key facts in 3 waves" | Stage 2, 3 |
| Mood is per-scene label only | Add inter-scene emotional curve design — adjacent scenes must differ in mood | Stage 2 |
| No viewer immersion devices | Add immersion guide — 2nd person intervention, situation hypotheticals, sensory description | Stage 3 |
| Fixed scene count (8-12) | Length-based scene count guide (5min=5-6, 10min=8-10, 15min=12-15) | Stage 2 |
| Hardcoded 4-Act ratio (15/30/40/15) | Channel-analysis-informed ratios + emotional curve design instruction | Stage 2 |
| Review checks only fact accuracy | Add storytelling quality checks — hook strength, info curve, emotional variation | Stage 4 |

**Required conditions for a "good" SCP scenario (audience retention drivers):**

1. **Opening hook** (first 15s) — stop the scroll. Question/shock/mystery/contrast.
2. **Progressive information disclosure** — never reveal everything at once. SCP docs naturally hide info ([REDACTED], [DATA EXPUNGED]) — exploit this.
3. **Emotional transition points** — minimum 2-3 tone shifts (mystery → horror → awe → lingering).
4. **Viewer proxy** — immerse the viewer as a Foundation employee/D-class. 2nd person, sensory detail.
5. **Cliffhanger transitions** — each scene ending pulls toward the next.
6. **Reinterpretation moment** — mid-to-late perspective shift.
7. **Open ending** — leave unanswered questions (inherent SCP trait).

**Track 2: Character-based Image-to-Image Generation**
- Add "character generation" pipeline stage: LLM generates 3-4 protagonist appearance candidates → high-quality model generates images → user selects one → permanently saved to DB (reusable as future reference)
- Scene image generation switches to **Qwen/Qwen-Image-Edit** API using selected character image as reference (image-to-image)
- Scene type classification: character-present scenes use image-edit, background-only scenes use existing text-to-image
- Add `Edit()` method to existing `ImageGen` interface (not a separate interface — same provider)
- Character generation/selection as independent commands (not mid-pipeline blocking): `yt-pipe character generate` → user reviews files → `yt-pipe character select` → then `pipeline run`
- Character reuse: when same SCP is re-run, prompt user to reuse existing character (with description + image path for recall)

**Track 3: TTS Voice Cloning**
- Use DashScope `qwen3-tts-vc-2026-01-22` model (dedicated voice cloning model)
- Flow: local voice sample (.mp3) → `qwen-voice-enrollment` API creates voice → synthesize with cloned voice
- Jay provides voice samples; implementation includes test code for easy verification
- Extend existing DashScope provider to support VC model
- Cache cloned voice ID at project level (avoid per-scene enrollment calls)
- `yt-pipe tts test-voice` command: test voice sample → save voice ID for production use (`--save-voice-id`)
- Dedicated TTS VC config section (enrollment model ≠ synthesis model)

### Scope

**In Scope:**
- Channel format analysis (key features, animation/live-action, length, platform usage) → prompt template improvements
- Character generation/selection/storage flow + image-edit API integration
- Character reuse flow for same SCP entity
- TTS voice cloning integration + project-level voice ID caching + test code for verification

**Out of Scope:**
- Adding new LLM providers
- CapCut assembly changes
- BGM/mood system changes
- Automated YouTube channel video crawling/analysis
- Streaming TTS output
- Video length selection (separate tech spec: `tech-spec-scenario-duration-control.md`)

## Context for Development

### Codebase Patterns (Step 2 Deep Investigation)

**Architecture:**
- Plugin interface pattern: single-method interfaces (`Generate()`, `Synthesize()`) with opts structs
- Service layer wraps plugins: retry logic, file I/O, manifest hash updates, partial failure handling
- Placeholder injection via `strings.ReplaceAll()` in scenario_pipeline.go (no template engine)
- LLM calls use user-only messages (no system prompt)
- CLI commands open fresh DB per invocation, auto-close via cleanup function

**Track 1 — Scenario Pipeline Internals:**
- 4-stage pipeline: `runResearch()` → `runStructure()` → `runWriting()` → `runReview()`
- Each stage loads template, replaces placeholders, calls `sp.llm.Complete()`
- Inter-stage data: research output → `extractVisualIdentity()` → visual ref reused in stages 2-4
- Stage 4 produces `ReviewReport` with corrections → `applyCorrections()` applied to final scenario
- **FINDING**: `ScenarioService.GenerateScenario()` uses legacy one-shot LLM call, NOT the 4-stage pipeline. Pipeline is used separately.
- **FINDING**: No `{format_guide}` placeholder exists yet. Pattern to add: same as `{glossary_section}` — load file, inject into stages 1-3

**Track 2 — Image Generation Internals:**
- `ImageGen` interface: single `Generate(ctx, prompt, opts)` method
- `GenerateOptions`: Width, Height, Model, Style, Seed, CharacterRefs
- **CRITICAL FINDING**: `CharacterRefs` are populated by `ImageGenService` (line 51) but **NEVER READ** by `SiliconFlowProvider.Generate()`. The refs are collected but unused — no prompt composition from character data.
- **CRITICAL FINDING**: SiliconFlow API only exposes `/images/generations` endpoint. **No image-edit/inpaint endpoint exists.** Must verify if `Qwen/Qwen-Image-Edit` model is actually available via SiliconFlow, or if a different API endpoint is needed.
- **PoC SCOPE EXPANDED**: Must check (a) SiliconFlow `/v1/images/edits` endpoint, (b) Qwen-Image-Edit model via `/v1/images/generations` with image input, (c) alternative providers (DashScope direct, Stability AI). If none work → fallback to CharacterRef prompt injection.
- **INDEPENDENT WIN**: CharacterRef prompt composition must be implemented regardless of image-edit outcome. This is the fallback AND a standalone improvement.
- Character **manual CRUD** fully implemented: domain model, store, service, CLI commands (create/list/show/update/delete) all exist. However, LLM-powered candidate generation (`GenerateCandidates`) and selection flow (`SelectCandidate`) are **entirely new code** — not extensions of existing CRUD.
- Character DB schema finalized (migration 004): id, scp_id, canonical_name, aliases (JSON), visual_descriptor, style_guide, image_prompt_base. New migration needed for `selected_image_path` column (Task 3.0).

**Track 3 — TTS Voice Cloning Internals:**
- `TTS` interface: `Synthesize()` and `SynthesizeWithOverrides()` (for glossary)
- `DashScopeProvider`: HTTP POST to `/api/v1/services/aigc/multimodal-generation/generation`
- Voice is passed as string in `qwenInput.Voice` field — clone voice IDs work if prefixed `cosyvoice-clone-`
- `isCloneVoice()` exists but NOT integrated into synthesis logic
- **FINDING**: CLI `tts register-voice` command already exists (`internal/cli/tts_cmd.go` lines 15-132) — partial implementation using old API format (`cosyvoice-clone-v1`). Needs refactoring to new `qwen-voice-enrollment` + base64 data URI format per `docs/qwen3.tts.vc.md`.
- **FINDING**: Qwen3 TTS does NOT return word-level timestamps (line 274 comment). Existing fallback generates subtitles from narration text + duration. Must verify VC model behaves same.
- Mood preset system fully integrated: DB-backed presets → per-scene assignment → TTSOptions injection
- `TTSConfig` struct: Provider, Endpoint, APIKey, Model, Voice, Language, Format, Speed — needs `Clone` subsection added

### Files to Modify

| File | Change Type | Purpose |
| ---- | ----------- | ------- |
| `templates/scenario/format_guide.md` | **NEW** | Format Reference Guide (storytelling principles + channel analysis) |
| `templates/scenario/01_research.md` | Modify | Add `{format_guide}` placeholder for hook/structure guidance |
| `templates/scenario/02_structure.md` | Modify | Add `{format_guide}` placeholder for scene structure/emotional curve |
| `templates/scenario/03_writing.md` | Modify | Add `{format_guide}` placeholder for narration style/immersion |
| `templates/scenario/04_review.md` | Modify | Add storytelling quality checks (hook, info curve, emotion variation) |
| `internal/service/scenario_pipeline.go` | Modify | Load format_guide.md, inject `{format_guide}` in stages 1-3 (lines 236-265) |
| `internal/plugin/imagegen/interface.go` | Modify | Add `Edit()` method + `EditOptions` struct |
| `internal/plugin/imagegen/siliconflow.go` | Modify | Implement `Edit()` with Qwen-Image-Edit API + compose CharacterRefs into prompts |
| `internal/plugin/tts/interface.go` | Modify | Add `CreateVoice()` method for enrollment |
| `internal/plugin/tts/dashscope.go` | Modify | Implement `CreateVoice()` (base64 audio → enrollment API), integrate `isCloneVoice()` |
| `internal/service/tts.go` | Modify | Voice ID caching logic, auto re-enrollment on failure |
| `internal/service/image_gen.go` | Modify | Scene-type classification (character vs background), Edit() call routing |
| `internal/service/character.go` | Modify | Add `GenerateCandidates()`, `SelectCharacter()` methods |
| `internal/config/types.go` | Modify | Add `TTSCloneConfig` struct under TTSConfig |
| `internal/cli/tts_cmd.go` | Modify | Extend `register-voice` → `test-voice` with `--save-voice-id` |
| `internal/cli/character_cmd.go` | Modify | Add `generate` and `select` subcommands |

### Files to Reference (Read-Only Context)

| File | Purpose |
| ---- | ------- |
| `docs/scp.yt.channels.analysis.md` | Channel format analysis (input for Format Guide) |
| `docs/qwen3.tts.vc.md` | Voice Cloning API documentation (enrollment flow) |
| `internal/domain/character.go` | Character struct definition (no changes needed) |
| `internal/store/character.go` | Character DB operations (no changes needed) |
| `internal/store/migrations/004_characters.sql` | Character table schema (no changes needed) |
| `internal/service/image_prompt.go` | Image prompt sanitization (reference for prompt composition) |
| `internal/service/scenario.go` | ScenarioService (uses legacy LLM, not pipeline — context only) |
| `internal/domain/mood_preset.go` | MoodPreset + SceneMoodAssignment (reference for voice integration) |

### Technical Decisions

- **Voice Cloning Model**: `qwen3-tts-vc-2026-01-22` — must match between enrollment and synthesis
- **Voice Enrollment API**: `qwen-voice-enrollment` model via `/api/v1/services/audio/tts/customization` endpoint
- **Voice ID Caching**: Clone voice once per project, cache the returned voice ID — avoid per-scene enrollment
- **Voice Test → Production Binding**: `yt-pipe tts test-voice --sample X --text Y --save-voice-id <name>` saves tested voice ID to project, reusable in pipeline without re-enrollment
- **Multi-Voice Comparison**: Test output filenames include sample name (`tts-test-{sample_name}.wav`) for easy A/B comparison
- **TTS VC Config**: Separate config section needed (`tts.vc_model`, `tts.vc_sample_path`) since enrollment and synthesis use different models
- **Image Edit Provider**: `Qwen/Qwen-Image-Edit` — PoC must verify availability via SiliconFlow or alternative provider (DashScope direct, Stability AI)
- **CharacterRef Prompt Composition**: Implement prompt injection from CharacterRefs regardless of image-edit outcome. Compose `VisualDescriptor` + `ImagePromptBase` into image generation prompt text. This is both the fallback AND standalone improvement.
- **Reference Image Consistency**: Selected character image path stored in project metadata — all scene generation and regeneration uses the same reference
- **Reference Image Auto-Resize**: Auto-resize reference image to match image-edit API input requirements if needed
- **ImageGen Interface**: Add `Edit()` method to existing `ImageGen` interface. Provider that doesn't support it returns `ErrNotSupported` — service falls back to `Generate()` with CharacterRef-enhanced prompt
- **TTS Register-Voice Refactor**: Existing `tts register-voice` CLI uses old API format (`cosyvoice-clone-v1`). Refactor to `qwen-voice-enrollment` + base64 data URI. Extend with `test-voice` subcommand.
- **Character Images**: Permanently stored in DB, reusable across projects for same SCP entity
- **Character Selection UX**: CLI = save images to `workspace/{scp}/characters/` + number selection; API = image card response
- **Character Generation as Independent Command**: `character generate` and `character select` are standalone CLI commands — `pipeline run` requires character selection to be complete beforehand (no mid-pipeline blocking)
- **Character Candidate Regeneration**: `--regenerate` flag on `character generate` to discard and recreate all candidates
- **Character Candidate Metadata**: Save description text alongside each image (`candidate_{N}.txt`) for review without opening image files
- **Character Reuse**: When re-running pipeline for same SCP, offer to reuse existing character with description text + image path display for recall
- **Channel Analysis Axes**: Key features, animation/live-action type, video length distribution, platform usage patterns
- **Channel Analysis**: Open-ended analysis of all channels in the doc — no preconception about which format to adopt
- **Format Guide Purpose**: Not "which format to copy" but "evidence for storytelling principles that drive audience retention"
- **Scenario Quality ≠ Fact Coverage**: Fact accuracy is necessary but not sufficient. Stage 4 Review must check both fact accuracy AND storytelling quality (hook, info curve, emotional variation)
- **Prompt Design Philosophy**: Shift from "describe the SCP accurately" to "tell a compelling story about the SCP accurately"
- **Duration Control**: Out of scope — separate tech spec exists (`tech-spec-scenario-duration-control.md`)

## Implementation Plan

### Phase 0: PoC Gates (Week 1, parallel with Track 1)

- [ ] Task 0.1: Image-Edit API PoC
  - File: `internal/plugin/imagegen/siliconflow_edit_poc_test.go` (NEW, `//go:build integration`)
  - Action: Write integration test that (a) checks SiliconFlow `/v1/images/edits` endpoint availability, (b) tests `Qwen/Qwen-Image-Edit` model via `/v1/images/generations` with image input param, (c) if both fail, test DashScope direct API for image-edit
  - Action: Generate 1 character reference image, then run 10 diverse scene prompts with same reference. Count how many scenes maintain character recognizability.
  - Notes: Result determines Track 2 scope: ≥7/10 → full edit, 5-6 → hybrid, ≤4 → fallback to CharacterRef prompt injection only

- [ ] Task 0.2: TTS Voice Cloning `test-voice` CLI Command
  - File: `internal/cli/tts_cmd.go`
  - Action: Add `test-voice` subcommand: `yt-pipe tts test-voice --sample <path> --text <text> [--save-voice-id <name>]`
  - Action: Implement voice enrollment via `qwen-voice-enrollment` model + base64 data URI (per `docs/qwen3.tts.vc.md` Python example)
  - Action: Call synthesis with `qwen3-tts-vc-2026-01-22` model using returned voice ID
  - Action: Save output WAV to `tts-test-{sample_basename}.wav`. Print sample quality guidelines.
  - Action: If `--save-voice-id` provided, persist voice ID + sample path + timestamp to project DB
  - File: `internal/plugin/tts/dashscope.go`
  - Action: Add `CreateVoice(ctx, audioPath string, preferredName string) (string, error)` method. HTTP POST to `/api/v1/services/audio/tts/customization` with JSON body: `{"model": "qwen-voice-enrollment", "input": {"action": "create", "target_model": "qwen3-tts-vc-2026-01-22", "preferred_name": "<name>", "audio": {"data": "data:{mime};base64,{encoded}"}}}`
  - Action: Parse response: voice parameter is at `response.output.voice` (NOT `output.voice_id`). Return this string directly — do NOT prepend `cosyvoice-clone-` prefix (the API returns the full voice identifier)
  - File: `internal/plugin/tts/dashscope_test.go`
  - Action: Add unit test for `CreateVoice()` with mock HTTP server
  - Notes: Jay uses this immediately to verify voice samples. Blocks Track 3 full integration.

### Phase 1: Track 1 — Scenario Quality (Week 1-2, main focus)

- [ ] Task 1.1: Create Format Reference Guide
  - File: `templates/scenario/format_guide.md` (NEW)
  - Action: Analyze `docs/scp.yt.channels.analysis.md` across 4 axes (key features, animation/live-action, length, platform usage)
  - Action: Synthesize findings with First Principles 7 conditions into structured guide:
    - Section A: Hook Type Library (question, shock, mystery, contrast) with SCP-specific examples
    - Section B: Progressive Disclosure Pattern — 3-wave information reveal
    - Section C: Emotional Curve Design — mood transition rules for adjacent scenes
    - Section D: Viewer Immersion Devices — 2nd person, sensory, hypothetical
    - Section E: Scene Count & Pacing Guide by target duration
    - Section F: Act Structure (channel-analysis-informed ratios, not hardcoded)
  - Notes: Document language is English (per `document_output_language`). Content informed by channel analysis but structured as principles, not channel-specific rules.

- [ ] Task 1.2: Inject Format Guide into Scenario Pipeline
  - File: `internal/service/scenario_pipeline.go`
  - Action: Add `formatGuide string` field to `ScenarioPipeline` struct
  - Action: In `NewScenarioPipeline()`, after stage template loading loop, load `format_guide.md` via `os.ReadFile(filepath.Join(cfg.TemplatesDir, "scenario", "format_guide.md"))`. If file not found, set `sp.formatGuide = ""` (graceful degradation — do NOT use the existing hard-fail pattern for stage templates)
  - Action: In `runResearch()`, `runStructure()`, and `runWriting()`: add `prompt = strings.ReplaceAll(prompt, "{format_guide}", sp.formatGuide)` alongside existing placeholder replacements
  - File: `internal/service/scenario_pipeline_test.go`
  - Action: Add `{format_guide}` to test template strings. Verify replacement in stage outputs.
  - Notes: If format_guide.md doesn't exist, use empty string (graceful degradation for backward compatibility)

- [ ] Task 1.3: Add Format Guide Placeholders to Templates
  - File: `templates/scenario/01_research.md`
  - Action: Add `{format_guide}` section after glossary section. Context: "Use the following format guide to identify narrative hooks and dramatic structure during research."
  - File: `templates/scenario/02_structure.md`
  - Action: Add `{format_guide}` section. Context: "Apply the following storytelling principles when designing scene structure, emotional curve, and pacing."
  - File: `templates/scenario/03_writing.md`
  - Action: Add `{format_guide}` section. Context: "Apply the following immersion and narration techniques when writing Korean narration scripts."
  - Notes: Each template receives the full format guide but with stage-specific framing instruction above it.

- [ ] Task 1.4: Add Storytelling Quality Checks to Stage 4 Review
  - File: `templates/scenario/04_review.md`
  - Action: Add new review category "Storytelling Quality" alongside existing 5 categories (SCP Classification, Anomalous Properties, Containment, Visual Identity, Fact Coverage)
  - Action: Storytelling checks: (a) Hook strength — does scene 1 open with a hook type from the library? (b) Information curve — are key facts distributed across 3+ scenes? (c) Emotional variation — do adjacent scenes have different moods? (d) Immersion devices — count 2nd person/sensory/hypothetical occurrences (min 3 per scenario)
  - Action: Add `storytelling_score` to ReviewReport JSON output (0-100 scale)
  - Action: Add `storytelling_issues` array parallel to existing `issues` array
  - File: `internal/service/scenario_pipeline.go`
  - Action: Add `StorytellingScore int` and `StorytellingIssues []ReviewIssue` fields to `ReviewReport` struct (search for `type ReviewReport struct`)
  - Action: In `parseReviewReport()` function definition (search for `func.*parseReviewReport`), parse `storytelling_score` and `storytelling_issues` from Stage 4 JSON output into the new struct fields
  - Action: In `Run()` method, after `parseReviewReport()` call, log warning if `reviewReport.StorytellingScore < 70`
  - Notes: Storytelling issues are warnings only (no auto-correction), unlike fact errors which trigger `applyCorrections()`. Do NOT confuse the `parseReviewReport` call site in `Run()` with the function definition — modify the function definition.

### Phase 2: Track 3 — TTS Voice Cloning Full Integration (Week 2)

- [ ] Task 3.0: Database Migration for Voice Cache + Character Image Metadata
  - File: `internal/store/migrations/007_voice_cache_and_character_image.sql` (NEW)
  - Action: Create `voice_cache` table: `project_id TEXT PRIMARY KEY, voice_id TEXT NOT NULL, sample_path TEXT NOT NULL, created_at TEXT NOT NULL`
  - Action: Add `selected_image_path TEXT NOT NULL DEFAULT ''` column to `characters` table (for storing the selected candidate image path)
  - File: `internal/store/voice_cache.go` (NEW)
  - Action: Add CRUD methods: `GetCachedVoice(projectID) (*VoiceCache, error)`, `CacheVoice(projectID, voiceID, samplePath)`, `DeleteCachedVoice(projectID)`
  - File: `internal/store/character.go`
  - Action: Add `UpdateSelectedImagePath(characterID, imagePath string) error` method
  - Notes: Migration number may differ — check latest migration file. Voice cache is per-project, not per-SCP. Character image path is per-character record.

- [ ] Task 3.1: Add TTS Clone Config
  - File: `internal/config/types.go`
  - Action: Add `TTSCloneConfig` struct: `Model string`, `SamplePath string`, `PreferredName string`. Add `Clone TTSCloneConfig` field to `TTSConfig`.
  - File: `internal/config/config.go`
  - Action: Add defaults: `v.SetDefault("tts.clone.model", "qwen3-tts-vc-2026-01-22")`, `v.SetDefault("tts.clone.preferred_name", "narrator")`
  - File: `internal/config/config_test.go`
  - Action: Add test for clone config loading from YAML

- [ ] Task 3.2: Rewrite `register-voice` CLI (existing code is non-functional stub)
  - File: `internal/cli/tts_cmd.go`
  - Action: The existing `ttsRegisterVoiceCmd` is a **non-working placeholder** — it uses a fabricated endpoint (`/api/v1/services/aigc/voice/register`) and model (`cosyvoice-clone-v1`) that don't exist. The comment at line ~72 confirms: "This implementation uses the local path as a placeholder."
  - Action: **Rewrite entirely** to use `DashScopeProvider.CreateVoice()` from Task 0.2
  - Action: Read audio file from `--audio` flag path, pass to `CreateVoice()` which handles base64 encoding internally
  - Action: Output voice ID to stdout. If `--project` flag provided, persist to project DB via `store.CacheVoice()` from Task 3.0.

- [ ] Task 3.3: Voice ID Caching in TTS Service
  - File: `internal/service/tts.go`
  - Action: Before first scene synthesis, check project DB for cached voice ID (by project ID)
  - Action: If no cached voice ID and `tts.clone.sample_path` is configured: call `CreateVoice()`, cache result with timestamp
  - Action: On synthesis 401/403: auto re-enroll from `tts.clone.sample_path`, update cache, retry once
  - Action: If voice ID age > 7 days, log warning: "Voice ID created N days ago. Run `yt-pipe tts test-voice` to verify."
  - File: `internal/plugin/tts/interface.go`
  - Action: Add separate `VoiceCloner` interface (NOT on `TTS` — ISP: not all TTS providers support cloning): `type VoiceCloner interface { CreateVoice(ctx context.Context, audioPath string, preferredName string) (string, error) }`. DashScopeProvider implements both `TTS` and `VoiceCloner`.
  - Action: In `TTSService`, accept optional `VoiceCloner` via `SetVoiceCloner(vc VoiceCloner)` (same pattern as `ImageGenService.SetCharacterService()`)
  - File: `internal/service/tts_test.go`
  - Action: Add tests for voice ID caching, auto re-enrollment, staleness warning

- [ ] Task 3.4: Integrate Clone Voice into Synthesis Flow
  - File: `internal/plugin/tts/dashscope.go`
  - Action: In `synthesize()` method, if `isCloneVoice(voice)` is true, switch model to `tts.clone.model` config value (instead of default `qwen3-tts-flash`)
  - Action: Ensure `qwenInput.Voice` receives the clone voice ID string
  - File: `internal/plugin/tts/dashscope_test.go`
  - Action: Add test: clone voice ID → correct model used in request body

### Phase 3: Track 2 — Character Image System (Week 3, gated by PoC)

- [ ] Task 2.1: CharacterRef Prompt Composition (independent of image-edit)
  - File: `internal/plugin/imagegen/siliconflow.go`
  - Action: In `Generate()`, read `opts.CharacterRefs` and compose into prompt: prepend `"Character: {VisualDescriptor}. {ImagePromptBase}. "` to user prompt
  - Action: If multiple CharacterRefs, join with "; "
  - File: `internal/plugin/imagegen/siliconflow_test.go`
  - Action: Add test: CharacterRefs populated → verify composed prompt contains descriptor text
  - Notes: This is the independent win — improves character consistency even without image-edit.

- [ ] Task 2.2: Add `Edit()` to ImageGen Interface (if PoC passes)
  - File: `internal/plugin/imagegen/interface.go`
  - Action: Add `EditOptions` struct: `SourceImage []byte`, `Width int`, `Height int`, `Model string`, `Seed int64`
  - Action: Add `Edit(ctx context.Context, sourceImage []byte, prompt string, opts EditOptions) (*ImageResult, error)` to `ImageGen` interface
  - Notes: Skip this task if PoC result ≤4/10. Implement regardless if ≥5/10.

- [ ] Task 2.3: Implement `Edit()` in SiliconFlow Provider (if PoC passes)
  - File: `internal/plugin/imagegen/siliconflow.go`
  - Action: Implement `Edit()` method targeting the endpoint/model identified by PoC
  - Action: Base64-encode source image, include in request body
  - Action: If provider doesn't support edit, return `ErrNotSupported`
  - File: `internal/plugin/imagegen/siliconflow_test.go`
  - Action: Add unit test with mock HTTP server for Edit endpoint

- [ ] Task 2.4: Scene-Type Classification in ImageGenService
  - File: `internal/service/image_gen.go`
  - Action: Add `SetSelectedCharacterImage(imagePath string)` method to `ImageGenService` — called during pipeline init after character reuse/selection check
  - Action: In `GenerateSceneImage()`, after CharacterRef matching: if `len(refs) > 0` AND `s.selectedCharacterImage != ""` → read image file, call `Edit()` with image bytes as source
  - Action: If `len(refs) == 0` (background scene) → call `Generate()` as before (existing text-to-image path)
  - Action: On `Edit()` returning `ErrNotSupported` → fallback to `Generate()` with CharacterRef-enhanced prompt (from Task 2.1)
  - Action: Mark `generation_method` in manifest: `"image_edit"`, `"text_to_image"`, or `"fallback_t2i"`
  - Notes: Selected character image path comes from `characters.selected_image_path` column (Task 3.0 migration). Load via `CharacterService.GetCharacter()` → read `SelectedImagePath` field.

- [ ] Task 2.5: Character Candidate Generation CLI
  - File: `internal/cli/character_cmd.go`
  - Action: Add `generate` subcommand: `yt-pipe character generate --scp SCP-682 [--count 4] [--regenerate]`
  - Action: Use LLM to generate N appearance descriptions for the SCP entity
  - Action: Use highest-quality image generation model to create candidate images
  - Action: Save to `workspace/{scp}/characters/candidate_{1..N}.png` + `candidate_{1..N}.txt` (description)
  - Action: If `--regenerate`, delete existing candidates first
  - Action: Print file paths and instructions: "Review images, then run: yt-pipe character select --scp SCP-682 --num <N>"
  - File: `internal/service/character.go`
  - Action: Add `GenerateCandidates(ctx, scpID string, count int) ([]CandidateResult, error)` method

- [ ] Task 2.6: Character Selection CLI
  - File: `internal/cli/character_cmd.go`
  - Action: Add `select` subcommand: `yt-pipe character select --scp SCP-682 --num <N>`
  - Action: Read candidate description from `candidate_{N}.txt`
  - Action: Create/update Character in DB with visual descriptor + image prompt base from description
  - Action: Store selected image path in project metadata
  - Action: Print confirmation: "Character selected. Image path: {path}. Ready for pipeline run."
  - File: `internal/service/character.go`
  - Action: Add `SelectCandidate(scpID string, candidateNum int, workspacePath string) (*domain.Character, error)`

- [ ] Task 2.7: Character Reuse Check
  - File: `internal/service/character.go`
  - Action: Add `CheckExistingCharacter(scpID string) (*domain.Character, error)` — returns nil if no character exists for this SCP
  - File: `internal/cli/character_cmd.go` (generate subcommand)
  - Action: Before generating, call `CheckExistingCharacter()`. If found, print: "Existing character found: {name} — {descriptor}. Image: {path}. [Y] Reuse [N] Generate new"
  - Action: On [Y], skip generation. On [N], proceed with `--regenerate` behavior.

### Acceptance Criteria

#### Track 1: Scenario Quality

- [ ] AC-1.1: Given `templates/scenario/format_guide.md` exists with storytelling principles, when scenario pipeline runs, then `{format_guide}` placeholder is replaced in Stage 1, 2, and 3 prompts with guide content
- [ ] AC-1.2: Given `format_guide.md` does NOT exist, when scenario pipeline runs, then `{format_guide}` placeholder is replaced with empty string (no error, backward compatible)
- [ ] AC-1.3: Given a completed scenario, when Stage 4 Review runs, then output includes `storytelling_score` (0-100) and `storytelling_issues` array in ReviewReport JSON
- [ ] AC-1.4: Given Stage 4 Review returns `storytelling_score < 70`, when pipeline completes, then warning is logged with score and issue count
- [ ] AC-1.5: Given Format Guide includes hook type library, when Stage 4 Review runs, then `storytelling_score` is > 0 (i.e., the storytelling quality check is functional and produces a non-trivial score). Visual quality of hooks verified manually via before/after comparison (not an automated AC).

#### Track 2: Character Image System

- [ ] AC-2.1: Given `opts.CharacterRefs` contains entries, when `SiliconFlowProvider.Generate()` is called, then prompt text includes VisualDescriptor content from refs
- [ ] AC-2.2: Given `yt-pipe character generate --scp SCP-682 --count 4`, when command completes, then 4 PNG images + 4 TXT descriptions exist in `workspace/SCP-682/characters/`
- [ ] AC-2.3: Given candidate images exist, when `yt-pipe character select --scp SCP-682 --num 2`, then Character record created in DB with visual descriptor from `candidate_2.txt` and image path stored in project metadata
- [ ] AC-2.4: Given a Character exists for SCP-682, when `yt-pipe character generate --scp SCP-682` is run, then prompt shows "Existing character found" with reuse option before generating
- [ ] AC-2.5: Given `Edit()` returns `ErrNotSupported`, when image generation runs for a character scene, then service falls back to `Generate()` with CharacterRef-enhanced prompt and marks manifest as `generation_method: "fallback_t2i"`
- [ ] AC-2.6: (If PoC passes) Given a selected character image and scene prompt, when `Edit()` is called, then returned image incorporates the character reference visually

#### Track 3: TTS Voice Cloning

- [ ] AC-3.1: Given `yt-pipe tts test-voice --sample voice.mp3 --text "테스트 문장"`, when command completes, then `tts-test-voice.wav` file is created with synthesized audio
- [ ] AC-3.2: Given `--save-voice-id narrator` flag on test-voice, when command completes, then voice ID is persisted to project DB with sample path and timestamp
- [ ] AC-3.3: Given `tts.clone.sample_path` configured and no cached voice ID, when TTS pipeline runs, then voice is enrolled automatically and ID cached before synthesis
- [ ] AC-3.4: Given cached voice ID and synthesis returns 401/403, when TTS service handles error, then auto re-enrollment is attempted from `tts.clone.sample_path`, synthesis retried with new ID
- [ ] AC-3.5: Given cached voice ID older than 7 days, when pipeline starts, then warning is logged with age and `test-voice` suggestion
- [ ] AC-3.6: Given `isCloneVoice(voice)` returns true, when `DashScopeProvider.Synthesize()` runs, then request uses `tts.clone.model` (not default `qwen3-tts-flash`)
- [ ] AC-3.7: Given voice enrollment via `CreateVoice()`, when API returns voice parameter, then returned string is stored as-is (no prefix manipulation). Verify by checking `response.output.voice` field is non-empty and used directly for synthesis.

## Additional Context

### Dependencies

- DashScope API key with access to `qwen3-tts-vc-2026-01-22` model and `qwen-voice-enrollment` model
- SiliconFlow API key (existing) — image-edit model availability TBD by PoC
- Voice sample audio file (.mp3, clean recording, single speaker) — provided by Jay. Recommended min 10s for quality (not API-enforced — quality guideline only)
- `internal/mocks` package does NOT exist (known issue — `assembler_test.go` references it but it's missing). All new tests MUST use `httptest` mock HTTP servers following the pattern in `dashscope_test.go`, NOT mock interfaces from `internal/mocks`.

### Testing Strategy

**Unit Tests:**
- `scenario_pipeline_test.go`: Verify `{format_guide}` placeholder replacement in all 3 stages + graceful empty-string fallback
- `siliconflow_test.go`: Verify CharacterRef prompt composition (descriptor text appears in request prompt)
- `dashscope_test.go`: Verify `CreateVoice()` request format (base64 data URI, correct model/endpoint). Verify clone voice → model switch.
- `tts_test.go`: Verify voice ID caching, auto re-enrollment on 401/403, staleness warning

**Integration Tests (build tag: `//go:build integration`):**
- `siliconflow_edit_poc_test.go`: Track 2 PoC — real API calls to verify image-edit availability + character consistency across 10 scenes
- TTS voice cloning: real enrollment + synthesis (manual run by Jay via `test-voice` CLI)

**Manual Testing:**
- Track 1: Generate scenario for SCP-173 before/after format guide. Compare hook quality, emotional curve, information pacing.
- Track 2: Review generated character candidates visually. Verify scene images with selected character.
- Track 3: Listen to `tts-test-{sample}.wav` output files. Compare voice similarity to source sample.

### Notes

- **High-Risk Items**: SiliconFlow image-edit endpoint availability (PoC required). Korean voice cloning quality (test-voice gate).
- **Known Limitations**: Stage 4 storytelling checks are advisory (no auto-correction, unlike fact errors). Voice ID TTL unknown — staleness threshold is heuristic.
- **Future Considerations** (out of scope): Instruction control (`qwen3-tts-instruct-flash`) as VC complement for per-scene emotion tuning. ControlNet/IP-Adapter for stronger character consistency. Multi-character support per SCP.

### ROI Analysis

| Track | Effort | Impact | ROI | Key Value |
|-------|--------|--------|-----|-----------|
| Track 1 (Scenario) | **Low** (template/guide work) | **Very High** (audience retention) | ★★★★★ | Viewers stay and come back |
| Track 2 (Image) | **High** (new API, interface, commands, classifier) | **High** (visual consistency) | ★★★☆☆ | Thumbnail/CTR, professional look |
| Track 3 (TTS VC) | **Medium** (provider extension + CLI) | **Medium** (brand voice) | ★★★★☆ | Channel identity, differentiation |

**Priority principle**: If only one track can be completed → Track 1. If two → Track 1 + Track 3. Scenario quality is the foundation — great images/voice cannot save a boring script.

### Execution Priority & Schedule

| Week | Parallel Work Items | Gate/Decision |
|------|-------------------|---------------|
| **Week 1** | Track 1: Format Guide + prompt improvements (main focus) | — |
| | + Track 2 PoC: image-edit API consistency test (background) | PoC result: ≥7 proceed / 5-6 hybrid / ≤4 fallback |
| | + Track 3: `test-voice` CLI command (background) | Jay procures voice samples in parallel |
| **Week 2** | Track 1: Stage 4 storytelling quality check (complete Track 1) | — |
| | + Track 3: full pipeline integration | Based on test-voice results |
| **Week 3-4** | Track 2: full implementation (or fallback decision). Tasks 2.1 (CharacterRef composition) is mandatory. Tasks 2.2-2.7 are PoC-gated and may extend to Week 4 given new infrastructure (DB migration, LLM-based generation). | Based on PoC result from Week 1 |

**Why this order:**
- Track 1 first = lowest effort, highest ROI, immediately improves next video
- Track 2/3 PoCs run in parallel with Track 1 = early gate decisions without blocking main work
- Track 2 last = largest code change + most uncertain. Other tracks done first reduces pressure

**Worst-case triage:**
- Track 2 PoC fails → scope reduces to text-to-image prompt enhancement (low effort)
- Track 3 VC fails → switch to instruction control fallback (independently valuable)
- Both fail → Track 1 alone still delivers significant quality improvement

### Track Dependency Map

```
Track 1 (Scenario) ──────────────────────► independent, start anytime
Track 2 (Image)    ── PoC first ─┬─ pass (≥7/10) ──► full implementation
                                  ├─ partial (5-6) ─► hybrid (closeup=edit, background=t2i)
                                  └─ fail (≤4) ─────► fallback to text-to-image enhancement
Track 3 (TTS VC)   ── test-voice ─┬─ pass ──► full VC integration
                                    └─ fail ──► fallback to instruction control (qwen3-tts-instruct-flash)
```

### Contingency Plans (What-If Analysis)

#### Track 2 Fallback Ladder

| PoC Result | Action | Scope Impact |
|------------|--------|-------------|
| ≥7/10 scenes character-recognizable | Proceed with full image-edit implementation | No change |
| 5-6/10 | Hybrid: character closeup scenes use image-edit, background/wide shots use text-to-image + Frozen Descriptor | Medium — need scene-type classifier to also classify shot distance |
| ≤4/10 | Drop image-edit. Enhance text-to-image with stronger Frozen Descriptor + style guide + consistent seed strategy | Significant scope reduction — remove `Edit()` from interface, focus on prompt engineering |

#### Track 3 Fallback: Instruction Control

If voice cloning quality is insufficient for any reason (sample quality, Korean language limitations, API issues):
- **Fallback model**: `qwen3-tts-instruct-flash` with natural language instruction control
- **Example**: `instructions: "Speak in a deep, calm, authoritative male voice with slight tension. Pace: measured, deliberate. Korean pronunciation."`
- **Trade-off**: Less unique voice identity, but far more controllable emotion/tone per scene
- **Implementation**: Same TTS interface, different model + `instructions` parameter in `TTSOptions`
- **Bonus**: Instruction control is independently valuable — can combine with VC voice for emotion fine-tuning if both work

#### Voice ID Staleness Management

- Store `voice_id` + `created_at` timestamp in project DB
- On pipeline start: if voice ID age > configurable threshold (default 7 days), print warning:
  `"⚠️ Voice ID created 12 days ago. Run 'yt-pipe tts test-voice' to verify quality."`
- On synthesis 401/403: auto re-enroll from `tts.clone.sample_path` → retry → if re-enroll fails (file missing), clear error + fallback option

#### Per-Scene Image Generation Fallback

- If `Edit()` call fails for a specific scene (API error, timeout after 3 retries):
  - Log warning with scene number
  - Automatically fall back to `Generate()` (text-to-image) for that scene with enhanced Frozen Descriptor prompt
  - Mark scene in manifest as `generation_method: "fallback_t2i"` for user awareness
  - Continue pipeline (don't block on single scene failure)

#### Channel Analysis Safety Net

- First Principles 7 storytelling conditions are **independent** of channel analysis results
- If channel analysis yields no actionable patterns → Format Guide built entirely from first principles + general YouTube best practices
- Channel analysis enriches but is not load-bearing

### UX Focus Group Findings (2026-03-15)

Simulated end-to-end user workflow for SCP-682 production. Key friction points identified and resolved:

| # | Friction Point | Severity | Resolution |
|---|---------------|----------|------------|
| 1 | Scenario manual editing lacks guidance | Low | Format Guide injection addresses root cause |
| 2 | Pipeline blocks mid-run waiting for character selection | **High** | Split into independent commands: `character generate` → `character select` → `pipeline run` |
| 3 | All 4 candidates unsatisfactory, no regen option | Medium | Add `--regenerate` flag |
| 4 | Scene regen may use different reference image | Medium | Lock selected character path in project metadata |
| 5 | Reference image resolution mismatch with edit API | Low | Auto-resize logic |
| 6 | Test voice ≠ production voice (different enrollment) | **High** | `--save-voice-id` binds tested voice to project |
| 7 | Hard to compare multiple voice samples | Low | Output filename includes sample name |
| 8 | Can't recall previous character selection on reuse | Low | Show description text + image path |

### Architecture Decision Records

#### ADR-1: Voice ID — Project-Level Caching

- **Decision**: Cache `create_voice` result at project level for reuse
- **Chosen**: 1x enrollment on first TTS run → store voice ID in project metadata (DB) → reuse for all scenes
- **Rejected**: Per-scene enrollment (slow, expensive, consistency risk)
- **Trade-off**: Voice ID may expire (API docs don't specify TTL) → need auto re-enrollment fallback on synthesis failure
- **Mitigation**: On synthesis `401/403`, auto re-enroll and retry once

#### ADR-2: Format Analysis → Prompt Injection

- **Decision**: Maintain analysis as separate `templates/scenario/format_guide.md`, inject via `{format_guide}` placeholder
- **Rejected**: Hardcode analysis into each stage prompt (duplication across 4 templates, hard to maintain)
- **Trade-off**: Extra file to manage, but consistent with existing `{glossary_section}` injection pattern
- **Benefit**: Format guide can be independently updated as new channel insights emerge

#### ADR-3: ImageGen Interface Extension

- **Decision**: Add `Edit(ctx, sourceImage, prompt, opts) (*ImageResult, error)` to existing `ImageGen` interface
- **Rejected**: Separate `ImageEditor` interface (over-engineering — same provider handles both)
- **Trade-off**: Interface grows larger, but avoids config/init duplication across two structs
- **Fallback**: Providers that don't support edit return `ErrNotSupported`

#### ADR-4: Character Selection UX

- **Decision**: CLI = save to `workspace/{scp}/characters/candidate_{1..4}.png` + `yt-pipe character select --num N` / API = image URL array for frontend card UI
- **Rejected**: Terminal inline image rendering (iTerm2 etc.) — limited terminal compatibility
- **Rationale**: Consistent with existing scenario review pattern (file modification → approve). Aligns with "80% automation, 20% manual finishing" philosophy

#### ADR-5: Character Reuse Flow

- **Decision**: On pipeline start, query DB for existing characters with same SCP ID → offer reuse prompt → skip generation if accepted
- **Rejected**: Always regenerate (wasteful, forces repeated selection)
- **Trade-off**: 1 extra DB query per run, but major UX improvement for iterative workflows

#### ADR-6: TTS VC Dedicated Config

- **Decision**: Add `tts.clone` config subsection with `model`, `sample_path`, `preferred_name`
- **Rejected**: Overload existing `tts.model` field with conditional logic (confusing — same field, different semantics)
- **Config structure**:
  ```yaml
  tts:
    clone:
      model: "qwen3-tts-vc-2026-01-22"
      sample_path: "/path/to/voice.mp3"
      preferred_name: "narrator"
  ```
- **Enrollment model**: Always `qwen-voice-enrollment` (hardcoded, not configurable)

#### ADR-7: Channel Analysis — Unbiased Open Analysis

- **Decision**: Analyze all channels equally across 4 axes (key features, animation/live-action, length, platform usage)
- **Rationale**: Biasing toward "static image + narration" channels limits innovation opportunities. Insights from animation or short-form channels may reveal structural patterns applicable to our format
- **Application**: Format Reference Guide covers all channels uniformly; prompt templates consume insights without format preconception

### Adversarial Review Findings (2026-03-15)

15 findings identified, all addressed in spec revision:

| ID | Severity | Fix Applied |
|----|----------|-------------|
| F1 | Critical | Fixed: API response field is `output.voice`, not `output.voice_id`. Removed `cosyvoice-clone-` prefix assumption. |
| F2 | Critical | Fixed: Added `input.action: "create"` to enrollment request body in Task 0.2. |
| F3 | High | Fixed: Removed specific line numbers, use method name search patterns instead. |
| F4 | High | Fixed: Same as F3 — method names over line numbers. |
| F5 | High | Fixed: Task 1.4 now explicitly adds `StorytellingScore`/`StorytellingIssues` fields to `ReviewReport` struct. |
| F6 | High | Fixed: Added Task 3.0 — DB migration for `voice_cache` table + `characters.selected_image_path` column. |
| F7 | Medium | Fixed: Task 3.2 clarifies existing code is non-working stub, requires full rewrite. |
| F8 | Medium | Fixed: Task 2.4 references `selected_image_path` from Task 3.0 migration. |
| F9 | Medium | Fixed: AC-1.5 rewritten to test `storytelling_score > 0`. Manual quality check noted separately. |
| F10 | Medium | Fixed: Dependencies section explicitly states mocks don't exist, mandates `httptest` pattern. |
| F11 | Medium | Fixed: Clarified character system has manual CRUD only, LLM generation is entirely new. |
| F12 | Medium | Fixed: `CreateVoice()` moved to separate `VoiceCloner` interface (ISP compliance). |
| F13 | Low | Fixed: Voice sample "min 10s" marked as quality guideline, not API requirement. |
| F14 | Low | Fixed: Task 1.2 explicitly states graceful degradation differs from stage template hard-fail. |
| F15 | Low | Fixed: Track 2 schedule extended to Week 3-4, acknowledging new infrastructure scope. |
