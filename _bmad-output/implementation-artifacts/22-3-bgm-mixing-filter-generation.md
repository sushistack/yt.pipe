# Story 22.3: BGM Mixing Filter Generation

Status: ready-for-dev

## Story

As a system,
I want FFmpeg audio filter expressions generated for BGM mixing with volume control, fade, and ducking,
so that background music is properly integrated into the rendered video.

## Acceptance Criteria

1. **`generateBGMFilter(bgmAssignments, narrationDurations, totalDuration)` generates complex filter string**
   - Volume adjustment per BGM track (from BGMAssignment.VolumeDB)
   - Fade-in at track start (default 2s, from BGMAssignment.FadeInMs)
   - Fade-out at track end (default 2s, from BGMAssignment.FadeOutMs)
   - Ducking during narration segments (default -12dB, from BGMAssignment.DuckingDB)

2. **No BGM assignments → no filter applied**
   - Returns empty string and no error when bgmAssignments is nil/empty
   - Audio stream contains only narration (passthrough)

3. **Multiple BGM tracks with overlapping ranges**
   - Tracks mixed using FFmpeg `amix` filter with proper timing

## Tasks / Subtasks

- [ ] Task 1: Implement `generateBGMFilter` in `bgm.go` (AC: #1, #2, #3)
- [ ] Task 2: Unit tests for BGM filter generation

## Dev Notes

### Key Implementation
- File: `internal/plugin/output/ffmpeg/bgm.go`
- Uses existing `output.BGMAssignment` struct (VolumeDB, FadeInMs, FadeOutMs, DuckingDB)
- Filter generates FFmpeg `-filter_complex` expression
- Default fade: 2000ms (2s), default ducking: -12dB

### References
- [Source: architecture.md#EFR6] — BGM mixing filter specification
- [Source: epics.md#Story 22.3] — Full acceptance criteria
- [Source: output/interface.go] — BGMAssignment struct definition

## Dev Agent Record

### Agent Model Used
### Debug Log References
### Completion Notes List
### File List
