# Story 22.2: FFmpeg Concat & Subtitle File Generation

Status: ready-for-dev

## Story

As a system,
I want image concat lists, audio concat lists, and SRT subtitle files generated from scene data,
so that FFmpeg can consume them as input for the final rendering command.

## Acceptance Criteria

1. **`generateImageConcat(scenes, outputDir)` generates FFmpeg concat demuxer format**
   - Format: `file 'path'\nduration X.X` per image
   - Images ordered by scene number then shot/cut number
   - Duration derived from audio segment timing (Shot.EndSec - Shot.StartSec, or Scene.AudioDuration)
   - Output file: `images.txt` in outputDir

2. **`generateAudioConcat(scenes, outputDir)` generates FFmpeg concat protocol format**
   - Format: `file 'path'` per audio file
   - Audio files ordered by scene number
   - Output file: `audio_concat.txt` in outputDir

3. **`generateSRT(scenes, outputDir)` generates standard SRT subtitle file**
   - Sequential numbering starting at 1
   - `HH:MM:SS,mmm` timing format
   - UTF-8 encoding
   - Reuses existing SubtitleService logic for word grouping
   - Accumulates timing offsets across scenes for absolute timestamps
   - Output file: `subtitles.srt` in outputDir

4. **Empty scene list returns error**
   - All three generators return error when scenes slice is empty
   - Error message: "no scenes to render"

## Tasks / Subtasks

- [ ] Task 1: Implement `generateImageConcat` in `concat.go` (AC: #1, #4)
- [ ] Task 2: Implement `generateAudioConcat` in `concat.go` (AC: #2, #4)
- [ ] Task 3: Implement `generateSRT` in `subtitle.go` (AC: #3, #4)
- [ ] Task 4: Unit tests for all three generators

## Dev Notes

### Key Files to Create/Modify
- `internal/plugin/output/ffmpeg/concat.go` — image and audio concat file generation
- `internal/plugin/output/ffmpeg/subtitle.go` — SRT generation from scene WordTimings
- `internal/plugin/output/ffmpeg/concat_test.go` — tests
- `internal/plugin/output/ffmpeg/subtitle_test.go` — tests

### Existing Patterns
- SubtitleService in `internal/service/subtitle.go` has `FormatSRT()` and `formatSRTTime()` — but those are on the service struct. For the ffmpeg package, implement standalone SRT formatting to avoid circular dependency.
- Scene.Shots: if scene has Shots, each shot has its own ImagePath + StartSec/EndSec. If no shots, use scene-level ImagePath + AudioDuration.
- Scene.WordTimings: word-level timing for subtitle grouping (~8 words per segment)

### References
- [Source: architecture.md#EFR6] — concat demuxer format, file structure
- [Source: epics.md#Story 22.2] — Full acceptance criteria

## Dev Agent Record

### Agent Model Used
### Debug Log References
### Completion Notes List
### File List
