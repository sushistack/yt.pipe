# Story 3.3: TTS Narration & Pronunciation

Status: done

## Story
As a creator, I want TTS narration synthesized from the scenario with correct SCP terminology pronunciation.

## Implementation
- `internal/service/tts.go`: TTSService with SynthesizeScene() (single scene + retry + manifest), SynthesizeAll() (batch with scene filtering + partial failure), buildOverrides() for glossary pronunciation
- `internal/service/tts_test.go`: 6 tests covering success, glossary overrides, batch, partial failure, scene filtering, audio backup
- Retry: 3 attempts with 1s exponential backoff via retry.Do()
- Glossary: buildOverrides() extracts pronunciation map, SynthesizeWithOverrides() applies them
- Audio backup: .bak file created before overwriting existing audio (AC3)
- Scene filtering: filterSceneScripts() for selective re-synthesis
- Manifest: SQLite update with audio SHA-256 hash, failed scene marking
- Output: audio.mp3 per scene with word-level timing data

## Dev Agent Record
### Agent Model Used
Claude Opus 4.6
