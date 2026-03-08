# Story 3.4: Timing Resolver

Status: done

## Story
As a developer, I want a timing resolver that interprets TTS audio timing into image transitions and subtitle synchronization data.

## Implementation
- `internal/service/timing.go`: TimingResolver struct with ResolveTimings(), BuildTimeline(), SaveTimingFiles(), UpdateSceneTiming()
- `internal/service/timing_test.go`: 9 tests covering basic timing, word timings with offset, subtitle segments, empty/single scene, file saving, timeline building
- SceneTiming: scene start/end/duration, word timings (offset to absolute), transition points, subtitle segments
- Timeline: project-level total duration + all scene timings
- SubtitleSegment: groups words into 8-word chunks with start/end timing
- File output: per-scene timing.json + project-level timeline.json
- Plugin-agnostic: normalizes timing from any TTS plugin to internal format

## Dev Agent Record
### Agent Model Used
Claude Opus 4.6
