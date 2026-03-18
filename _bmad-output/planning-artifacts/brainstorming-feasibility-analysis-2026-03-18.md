---
name: brainstorming-feasibility-analysis
description: Feasibility analysis and concretization of 25 confirmed brainstorming ideas
date: '2026-03-18'
inputDocuments:
  - _bmad-output/brainstorming/brainstorming-session-2026-03-17-1000.md
  - _bmad-output/planning-artifacts/prd.md
analyst: Jay + Claude
status: approved
---

# Brainstorming Feasibility Analysis & Re-ranking

**Date:** 2026-03-18
**Input:** 25 confirmed ideas from brainstorming session (2026-03-17)
**Method:** Codebase technical analysis + implementation feasibility review

## Codebase Technical Context

Key findings from codebase exploration that inform feasibility:

| Component | Status | Implication |
|-----------|--------|-------------|
| FFmpeg integration | **Not present** | Audio/video processing requires new addition |
| Plugin interfaces | **Well-designed** | ImageGen, TTS, Assembler all interface-based — easy to extend |
| Shot.VideoPath | **Field exists** | i2v support partially prepared in data model |
| Scene.Mood | **Field exists** | SFX/color grading auto-mapping has foundation |
| BGM in CapCut | **Implemented** | BGMAssignment with volume, fade, ducking |
| Audio mixing | **Delegated to CapCut** | No direct audio processing — FFmpeg needed for direct rendering |
| Shot timing | **StartSec/EndSec exist** | Sentence-level sync data model ready |
| TextMaterial styling | **Exists in CapCut types** | Dynamic subtitle styling feasible |
| Incremental build | **Hash-based, working** | Extension points available for new asset types |

---

## Tier 1 — Quick Wins

### #39 YouTube Chapters Auto-generation

| Item | Detail |
|------|--------|
| **What** | Generate YouTube chapter format (`0:00 Intro\n1:23 Containment...`) from scene timing data (`timeline.json`) |
| **How** | `TimingResolver` already computes per-scene StartSec/EndSec. Convert scene title (Mood/VisualDesc) to chapter name → text file output |
| **Dependencies** | None. Existing data sufficient |
| **Scope** | ~30 lines of code |
| **Risk** | Nearly zero |
| **Phase** | **Phase 1 (MVP)** |

### #41 Scene Transition Audio Crossfade

| Item | Detail |
|------|--------|
| **What** | Apply audio fade-out/fade-in at scene transitions |
| **How** | **Method A (chosen):** Add `fade_in`/`fade_out` attributes to CapCut Segment (currently BGM-only → extend to narration track). **Method B (future):** FFmpeg audio crossfade |
| **Dependencies** | Method A: none. Method B: FFmpeg |
| **Scope** | Method A: CapCut Segment attribute addition only |
| **Risk** | Verify CapCut supports fade attributes on narration segments |
| **Phase** | **Phase 1 (MVP)** — Method A. FFmpeg crossfade deferred to Phase 2 with #4 |

### #40 Scene Mood-based Color Grading

| Item | Detail |
|------|--------|
| **What** | Apply color tone based on Scene.Mood ("horror", "documentary", etc.) |
| **How** | **Method A (chosen):** Include color tone directives in image generation prompts (prompt-level). **Method B:** Go image post-processing (brightness/contrast/hue). **Method C:** FFmpeg filters (`colorbalance`, `curves`, `eq`) |
| **Dependencies** | Method A: none |
| **Scope** | Prompt template modification |
| **Risk** | Method A is most natural with zero quality degradation risk |
| **Phase** | **Phase 1 (MVP)** — Method A (prompt-based). Post-processing grading deferred to Phase 2 |

### #12 BGM Mood Preset Pool

| Item | Detail |
|------|--------|
| **What** | Register BGM files by mood tag in library |
| **How** | `BGMService` already exists. BGM registration CLI + mood tag mapping + auto-recommendation |
| **Dependencies** | Royalty-free BGM file collection (external task) |
| **Scope** | Already covered by FR57~60 |
| **Risk** | None |
| **Phase** | **Already in PRD** — no changes needed |

### #24 Glossary Auto-expansion

| Item | Detail |
|------|--------|
| **What** | Auto-detect new SCP terms during scenario generation → suggest glossary additions |
| **How** | After scenario generation, LLM call: "Extract SCP terminology from this scenario and suggest pronunciation guides" → diff against existing glossary → present new entries to user → approved entries added to glossary.json |
| **Dependencies** | 1 additional LLM call, `Glossary` write method needed |
| **Scope** | LLM call + Glossary write method |
| **Risk** | LLM term extraction accuracy (expected to be good for SCP domain) |
| **Phase** | **Phase 1 (MVP)** — FR14 enhancement |

### #28 Batch Preview Single Approval

| Item | Detail |
|------|--------|
| **What** | Replace per-scene approval with batch preview → flag problem scenes only ("exception-based") |
| **How** | `ApprovalService` batch mode: show all scenes (image thumbnail + first narration sentence + mood) in one view → user flags problem scene numbers → rest auto-approved |
| **Dependencies** | CLI UX change, ApprovalService batch mode |
| **Scope** | Medium — UX design needed |
| **Risk** | CLI image preview challenge (file path list vs browser HTML preview vs API dashboard) |
| **Phase** | **Phase 2** — best paired with API dashboard (FR56) |

### Tier 1 Summary

| # | Idea | Phase | Implementation |
|---|------|-------|---------------|
| #39 | YouTube Chapters | **Phase 1** | timeline.json → chapter text |
| #41 | Audio crossfade | **Phase 1** | CapCut Segment fade attributes |
| #40 | Color grading | **Phase 1** | Prompt-based mood color directives |
| #12 | BGM preset pool | **Existing FR** | No change needed |
| #24 | Glossary auto-expand | **Phase 1** | LLM term extraction + glossary write |
| #28 | Batch preview approval | **Phase 2** | API dashboard + batch approval |

---

## Tier 2 — Core Features

### #4 FFmpeg Direct Video Rendering

| Item | Detail |
|------|--------|
| **What** | Render images + audio + subtitles → MP4 directly via FFmpeg (alongside or instead of CapCut) |
| **How** | Go `os/exec` FFmpeg call. New `ffmpeg.Assembler` implementing existing `output.Assembler` interface. Input: scene images (with duration) + audio concat + subtitle burn-in or soft subs |
| **Dependencies** | FFmpeg binary (include in Docker image) |
| **Scope** | New Assembler implementation — fits cleanly into existing plugin architecture |
| **Risk** | Low — image slideshow + audio is FFmpeg's most basic use case |
| **FFmpeg sketch** | `ffmpeg -f concat -i images.txt -i audio_concat.wav -vf "subtitles=subs.srt" -c:v libx264 -c:a aac output.mp4` |
| **Phase** | **Phase 2** |

### #15+22 Multimodal LLM Image Quality Verification (Qwen-VL)

| Item | Detail |
|------|--------|
| **What** | After image generation, auto-evaluate "prompt consistency", "character appearance match", "technical defects" via Qwen-VL → re-generate if below threshold |
| **How** | New `ImageValidator` interface. Send generated image + original prompt + character ID card to Qwen-VL → score (0~100) + reasons. Below threshold (e.g., 70) → auto-regenerate (max 3 attempts) |
| **Dependencies** | Qwen-VL API access (DashScope or compatible endpoint), LLM interface vision extension |
| **Scope** | LLM interface vision capability + validation loop logic |
| **Risk** | Qwen-VL cost (1 additional API call per image), evaluation subjectivity (mitigated by prompt engineering) |
| **Phase** | **Phase 2** |

### #8+26 AI Quality Score-based Selective Review / Auto-approval

| Item | Detail |
|------|--------|
| **What** | Based on #15 verification scores: high-score scenes auto-approved / low-score scenes queued for human review |
| **How** | `ApprovalService` auto-approve mode. Config: `auto_approve_threshold: 80`. Score ≥ threshold → auto-approve + log. Score < threshold → add to review queue |
| **Dependencies** | #15 (image verification) **must precede** |
| **Scope** | Logic layer on top of #15 — low complexity itself |
| **Risk** | Auto-approved images may be sub-par → start with high threshold (90), gradually lower |
| **Phase** | **Phase 2** — implement together with #15 |

### #33 Scene Mood-based SFX Auto-insertion

| Item | Detail |
|------|--------|
| **What** | Auto-place SFX (door creak, alarm, footsteps, etc.) based on Scene.Mood |
| **How** | Similar to BGM: `SFXLibrary` + `SFXAssignment` models. Mood→SFX mapping table (horror→[door_creak, scream, alarm], tension→[heartbeat, clock]). Place as separate audio track in CapCut assembly |
| **Dependencies** | Royalty-free SFX file collection, verify CapCut multi-audio-track support |
| **Scope** | Clone BGMService pattern + SFX mapping logic |
| **Risk** | SFX timing — precise sync with narration needs sentence-level mapping. **Initial version: scene-level ambient SFX only** → later evolve to sentence-level |
| **Phase** | **Phase 2** |

### #23 Dynamic Subtitle Styling

| Item | Detail |
|------|--------|
| **What** | Change subtitle color/size/font based on Scene.Mood (horror→red glow, calm→white default) |
| **How** | CapCut `TextMaterial` already has style attributes (size, bold, color range). Add mood→style mapping table. Apply dynamically during assembly |
| **Dependencies** | None — extends existing CapCut assembly logic |
| **Scope** | Style mapping table + TextMaterial generation logic modification |
| **Risk** | Low. CapCut TextMaterial style attributes already understood |
| **Phase** | **Phase 2** |

### #16 Sentence-level Subtitle-Image Sync

| Item | Detail |
|------|--------|
| **What** | Refine image transitions from scene-level to sentence/Shot-level. When subtitle "SCP-173 moved" appears, image transitions to exactly that Shot's image |
| **How** | `Shot` model already has `StartSec`/`EndSec` + `SentenceText` + `ImagePath`. Verify if CapCut assembly uses this data. If yes: already implemented. If no: modify assembly logic |
| **Dependencies** | Shot timing accuracy (based on TTS WordTiming) |
| **Scope** | Data model ready. Assembly logic verification/modification only |
| **Risk** | Low |
| **Phase** | **Phase 2** (possible Phase 1 upgrade after assembly logic verification) |

### #17 Thumbnail + Title + Description Auto-generation

| Item | Detail |
|------|--------|
| **What** | Auto-select most impactful scene image → synthesize text overlay → generate YouTube-optimized title/description/tags |
| **How** | (1) Image selection: most entity_visible scene or LLM "select most impactful scene". (2) Text overlay: Go image/draw or FFmpeg drawtext. (3) Title/description: 1 LLM call |
| **Dependencies** | LLM call, image processing (Go or FFmpeg) |
| **Scope** | Image composition somewhat complex |
| **Risk** | Thumbnail design quality — text placement/font selection matters. Preset template approach is realistic |
| **Phase** | **Phase 2** |

### #44 Style Preset System

| Item | Detail |
|------|--------|
| **What** | Bundle image style, subtitle style, TTS mood, color tone into a single "preset" for management |
| **How** | Extend existing StyleConfig structure. Preset CRUD + per-project preset selection |
| **Dependencies** | Existing config system extension |
| **Scope** | Config structure expansion |
| **Risk** | Low |
| **Phase** | **Phase 2** |

### #9 Qwen Image Provider

| Item | Detail |
|------|--------|
| **What** | Add Qwen image generation model as alternative provider alongside SiliconFlow FLUX |
| **How** | New `imagegen.ImageGen` interface implementation. DashScope API-based (already used for TTS) |
| **Dependencies** | Qwen image generation API docs, DashScope account (already exists) |
| **Scope** | Clone SiliconFlow implementation pattern |
| **Risk** | Qwen image model quality for SCP domain unknown → testing required |
| **Phase** | **Phase 2** |

### Tier 2 Summary

| # | Idea | Phase | Dependency | Core Implementation |
|---|------|-------|-----------|-------------------|
| #4 | FFmpeg rendering | **Phase 2** | None | New Assembler implementation |
| #15+22 | Qwen-VL quality verification | **Phase 2** | Qwen-VL API | LLM vision extension |
| #8+26 | Auto-approval | **Phase 2** | #15 | ApprovalService extension |
| #33 | SFX auto-insertion | **Phase 2** | SFX library | Clone BGM pattern |
| #23 | Dynamic subtitles | **Phase 2** | None | TextMaterial style mapping |
| #16 | Sentence-level sync | **Phase 2** | None | Assembly logic fix |
| #17 | Thumbnail + meta | **Phase 2** | LLM | Image composition + LLM |
| #44 | Style presets | **Phase 2** | None | Config structure extension |
| #9 | Qwen image | **Phase 2** | DashScope | ImageGen implementation |

---

## Tier 3 — Game Changers

### #6+27 Remotion/FFmpeg Full CapCut Replacement

| Item | Detail |
|------|--------|
| **What** | Fully remove CapCut dependency. Complete video via programming only |
| **How** | After #4 (FFmpeg) matures, handle transitions/effects via FFmpeg. Or deploy Remotion (Node.js) as sidecar, call from Go via HTTP |
| **Scope** | Decide after #4 validation |
| **Risk** | Transition/effect quality may not match CapCut. Review against "80% automation" philosophy |
| **Phase** | **Phase 3** — depends on #4 results |

### #11 Key Scene i2v (Wan Video)

| Item | Detail |
|------|--------|
| **What** | Convert 2-3 key images to 2-4 second video clips via Wan Video (first frame i2v) |
| **How** | Shot.VideoPath already exists. New `videogen.VideoGen` plugin interface. Wan Video API call → video clip → use video segment instead of image in CapCut/FFmpeg assembly |
| **Dependencies** | Wan Video API, video segment assembly logic |
| **Scope** | New plugin category |
| **Risk** | Wan Video generation time (minutes/clip), cost, style consistency between images and videos |
| **Phase** | **Phase 3** |

### #25 AI Image Transition / Interpolation Frames

| Item | Detail |
|------|--------|
| **What** | Generate interpolation frames between two scene images for smooth transitions |
| **How** | Image interpolation model (FILM, RIFE, etc.) or Wan Video image-to-image mode |
| **Dependencies** | Image interpolation API or local model |
| **Scope** | Experimental |
| **Risk** | Interpolation quality. SCP images (dark/abstract) may produce unnatural results |
| **Phase** | **Phase 3** |

### #29+31 Reverse / Image-driven Pipeline

| Item | Detail |
|------|--------|
| **What** | Google Image search for references → generate key images → reverse-generate scenario from images |
| **How** | (1) Google Image Search API (or SerpAPI) for SCP keywords → collect top N. (2) Analyze collected images via Qwen-VL → extract visual direction. (3) Visual direction + SCP data → image-first scenario generation |
| **Dependencies** | Google Image Search API (or SerpAPI), Qwen-VL |
| **Scope** | Major pipeline flow change |
| **Risk** | Copyright issues (search images for reference only), pipeline complexity increase |
| **Phase** | **Phase 3** |

### #7 Shift-Left Approval (30-second Trailer)

| Item | Detail |
|------|--------|
| **What** | At scenario approval, auto-generate 30-sec trailer with representative image + 10-sec TTS sample |
| **How** | Before scenario approval: generate low-res images for 1-2 scenes + TTS 10-sec sample → FFmpeg mini-video assembly |
| **Dependencies** | #4 (FFmpeg rendering) **must precede** |
| **Scope** | After #4 |
| **Risk** | Cost vs benefit — scenario already reviewable as text |
| **Phase** | **Phase 3** |

### #18 CI/CD Pattern Video Pipeline

| Item | Detail |
|------|--------|
| **What** | SCP ID push → auto pipeline → quality gate → staging (preview) → approval → production (upload) |
| **How** | Existing pipeline + webhooks (FR30) + #8 auto-approval composition. Trigger: API or file watch |
| **Dependencies** | #4, #8, FR30 (webhooks) |
| **Scope** | Orchestration of existing features |
| **Risk** | Low |
| **Phase** | **Phase 3** |

### #35 Dual Profile (Quick/Premium)

| Item | Detail |
|------|--------|
| **What** | Quick: fast basic quality. Premium: high quality with all features (i2v, SFX, color grading, etc.) |
| **How** | Profile config to toggle pipeline stages on/off. `yt-pipe run SCP-173 --profile premium` |
| **Dependencies** | Phase 2/3 features must exist first |
| **Scope** | Config-driven stage toggling |
| **Risk** | Low |
| **Phase** | **Phase 3** |

### #36 Cross-project Asset Registry

| Item | Detail |
|------|--------|
| **What** | Shared asset management across projects (images, SFX, BGM, character ID cards) |
| **How** | SQLite asset metadata table + shared filesystem directory. Hash-based deduplication |
| **Dependencies** | None (existing SQLite + filesystem) |
| **Scope** | Basic structure is simple; search/tagging is incremental |
| **Risk** | Low |
| **Phase** | **Phase 2 (basic) → Phase 3 (advanced)** |

### #42 Content Calendar + Scheduling

| Item | Detail |
|------|--------|
| **What** | Weekly/monthly content schedule management + auto pipeline trigger |
| **How** | Cron-based scheduler + project queue |
| **Dependencies** | #18 (CI/CD pattern) |
| **Scope** | Scheduler + queue |
| **Risk** | Low |
| **Phase** | **Phase 3** |

### Tier 3 Summary

| # | Idea | Phase | Dependency |
|---|------|-------|-----------|
| #6+27 | Full CapCut replacement | **Phase 3** | #4 |
| #11 | i2v (Wan Video) | **Phase 3** | VideoGen plugin |
| #25 | Image transitions | **Phase 3** | Interpolation model |
| #29+31 | Reverse pipeline | **Phase 3** | Google API, Qwen-VL |
| #7 | Shift-left approval | **Phase 3** | #4 |
| #18 | CI/CD pattern | **Phase 3** | #4, #8 |
| #35 | Dual profile | **Phase 3** | Phase 2 features |
| #36 | Asset registry | **Phase 2~3** | None |
| #42 | Content calendar | **Phase 3** | #18 |

---

## Final Re-ranking

### Phase 1 (MVP) — 4 new additions

| # | New FR | Difficulty |
|---|--------|-----------|
| #39 | YouTube Chapters auto-generation | Very low |
| #41 | Audio crossfade (CapCut fade) | Low |
| #40 | Prompt-based color grading | Low |
| #24 | Glossary auto-expansion (FR14 enhancement) | Low-medium |

### Phase 2 (Growth) — 11 new additions

| # | New FR | Dependency |
|---|--------|-----------|
| #4 | FFmpeg direct rendering | None |
| #15+22 | Qwen-VL quality verification | Qwen-VL API |
| #8+26 | AI auto-approval | #15 |
| #28 | Batch preview approval | API dashboard |
| #33 | SFX auto-insertion | SFX library |
| #23 | Dynamic subtitle styling | None |
| #16 | Sentence-level sync | None |
| #17 | Thumbnail + meta auto-gen | LLM |
| #44 | Style presets | None |
| #9 | Qwen image provider | DashScope |
| #36 | Asset registry (basic) | None |

### Phase 3 (Vision) — 8 additions

| # | New FR |
|---|--------|
| #6+27 | Full CapCut replacement |
| #11 | i2v (Wan Video) |
| #25 | Image transitions |
| #29+31 | Reverse pipeline |
| #7 | Shift-left approval |
| #18 | CI/CD pattern |
| #35 | Dual profile |
| #42 | Content calendar |

---

## Existing FR Impact

| Action | Count | Detail |
|--------|-------|--------|
| **No change needed** | 1 | #12 → FR57~60 already covers |
| **Enhancement** | 2 | #24 → FR14 enhancement, #44 → FR34 enhancement |
| **New FR required** | 21 | See phase breakdown above |
| **Section updates** | 6 | Executive Summary, Success Criteria, Journey 1/2, Domain Requirements (SFX), Risk Mitigation |

## Dependency Graph (Phase 2 critical path)

```
#4 FFmpeg ──→ #6+27 CapCut replacement (Phase 3)
    │──→ #7 Shift-left approval (Phase 3)
    │──→ #18 CI/CD pattern (Phase 3)

#15 Qwen-VL verification ──→ #8+26 Auto-approval ──→ #18 CI/CD (Phase 3)

#33 SFX ──→ #35 Dual profile (Phase 3)
#23 Dynamic subtitles ──→ #35 Dual profile (Phase 3)
#44 Style presets ──→ #35 Dual profile (Phase 3)
```

**Phase 2 priority order (recommended):**
1. #4 FFmpeg (unlocks Phase 3 features)
2. #9 Qwen image + #15+22 Qwen-VL verification (same API ecosystem)
3. #8+26 Auto-approval (depends on #15)
4. #16 Sentence-level sync + #23 Dynamic subtitles (related assembly changes)
5. #33 SFX + #44 Style presets
6. #17 Thumbnail + #28 Batch approval + #36 Asset registry
