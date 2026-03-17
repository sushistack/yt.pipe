# Future Ideas (Brainstorming 2026-03-17)

Ideas that are promising but not for immediate implementation.

## A/B Test Pattern — Scenario Variants (#19)
- Generate 2-3 scenario variants for the same SCP (tone: documentary vs horror vs mystery)
- Compare previews and select the best version
- Paradigm shift from single output to **candidate generation → selection**

## Progressive Quality Rendering — Game Engine LOD Pattern (#20)
- First pass: low-res images + basic TTS for quick full preview (under 5 min)
- After approval: regenerate with high-res images + high-quality TTS
- Like game Level of Detail: rough draft → approve → final polish
- Combine with Shift-Left Approval (#7) for maximum review speed improvement

## Viewer Preference Learning — Netflix Recommendation Pattern (#21)
- Collect YouTube Analytics (views, retention rate, per-segment drop-off) from published videos
- Learn which scene structures, image styles, narration tones perform best
- Feed back into next scenario generation
- Completes the generate → publish → analyze → learn loop — pipeline becomes self-improving

## Scenario + Image Prompt Co-Generation (#13)
- LLM generates narration AND visual direction hints simultaneously during scenario stage
- Eliminates separate shot breakdown inference, author intent directly embedded
- Risk: current 4-stage scenario flow already works well, may not be worth disrupting

## Zero-Touch Pipeline (#34)
- SCP ID input → video production → YouTube upload fully unmanned
- Zero human intervention when all quality gates pass, alert only on failure
- Extreme end of 80/20 philosophy → 100/0 target with strong safeguards
- Prerequisite: all quality gates (structural + Critic + Qwen-VL vision) proven reliable

## 4-Layer Audio Mixing (#37)
- Game sound design pattern: Narration + BGM + SFX + Ambient as separate layers
- Auto-mix volumes/panning per scene mood via FFmpeg
- Blocker: Need to build SFX/ambient sound library first

## Sentence-Level Emotion TTS (#38)
- Audiobook production pattern: dynamic speech rate/tone per sentence, not per scene
- Auto-map sentence emotion analysis to TTS mood presets at sentence granularity
- Blocker: DashScope CosyVoice API may not support per-sentence mood switching

## SCP Wiki Change Detection (#43)
- Auto-detect new/trending SCP articles via RSS/scraping
- "Make a video about this SCP?" notification
- Nice-to-have, not core pipeline

## Reference-Based Project Initialization (#30)
- Input a reference YouTube video URL → AI analyzes style/mood/tone/pacing
- Auto-extract configuration values (style preset, mood sequence, pacing profile)
- "I want videos like THIS" → pipeline auto-configures
- Reverse engineering from output to settings
