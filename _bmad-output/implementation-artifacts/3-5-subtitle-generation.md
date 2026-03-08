# Story 3.5: Subtitle Generation

Status: done

## Story
As a creator, I want subtitles automatically generated from narration timing data with accurate SCP terminology.

## Implementation
- `internal/service/subtitle.go`: SubtitleService with GenerateSubtitles() (word grouping + glossary), SaveSceneSubtitles() (per-scene JSON + SRT), SaveAllSubtitles() (batch with combined project-level files)
- `internal/service/subtitle_test.go`: 10 tests covering generation, empty, default max words, glossary spelling, SRT format, JSON save, combined files, scene filtering
- Glossary: canonicalSpelling() applies glossary canonical term for SCP terminology (AC2)
- Output: subtitle.json + subtitle.srt per scene, subtitles.json + subtitles.srt at project level
- Scene filtering: filterScenes() for selective regeneration (AC4)
- Manifest: SQLite update with subtitle SHA-256 hash
- Default 8 words per subtitle segment

## Dev Agent Record
### Agent Model Used
Claude Opus 4.6
