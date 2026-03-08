# Story 11.2: Timing Resolver — Narration-Driven Scene Synchronization

Status: done

## Story

As a creator,
I want scene images to be displayed for exactly the duration of their narration audio,
So that visuals and audio are perfectly synchronized in the final video.

## Acceptance Criteria

1. Each scene's image display duration matches its narration audio duration exactly
2. Scene transitions at boundary between consecutive narration segments (no overlap, no gap)
3. Total video duration equals sum of all scene narration durations
4. Subtitle start/end times offset by cumulative duration of preceding scenes; project-level `timeline.json` contains per-scene absolute timestamps
5. Scenes with no narration use configurable default duration (`assembly.default_scene_duration`, default: 3s), included in timeline with image but no audio segment
6. Re-run after TTS regeneration: only affected scene timing updated, subsequent offsets recalculated

## Tasks / Subtasks

- [ ] Task 1: Add `assembly.default_scene_duration` to config (AC: #5)
  - [ ] Add DefaultSceneDuration field to OutputConfig in types.go
  - [ ] Set default value of 3.0 in config.go
  - [ ] Update config_test.go
- [ ] Task 2: Handle no-narration scenes in TimingResolver (AC: #5)
  - [ ] In ResolveTimings(), use default duration when AudioDuration == 0
  - [ ] Skip audio track segment for no-narration scenes in CapCut assembler
- [ ] Task 3: Ensure timeline.json has complete absolute timestamps (AC: #4)
  - [ ] Verify SaveTimingFiles produces correct per-scene timing.json and timeline.json
  - [ ] Verify subtitle offsets are cumulative across scenes
- [ ] Task 4: Wire default scene duration from config into pipeline runner (AC: #5)
  - [ ] Pass config value through RunnerConfig to TimingResolver
- [ ] Task 5: Add tests for no-narration scene handling and re-run scenarios

## Dev Notes

### Existing Code (DO NOT Reinvent)
- `internal/service/timing.go` — Full TimingResolver with ResolveTimings(), BuildTimeline(), SaveTimingFiles(), UpdateSceneTiming()
- Already calculates absolute offsets, builds subtitle segments from word timings
- `internal/plugin/output/capcut/capcut.go` — buildDraftProject() uses scene.AudioDuration for timing
- `internal/pipeline/runner.go` — Resume() already calls timingResolver.ResolveTimings() and SaveTimingFiles()

### What Needs to Change
1. **Config**: Add `DefaultSceneDuration float64` to OutputConfig, default 3.0
2. **TimingResolver**: Accept default duration parameter; use it when scene.AudioDuration == 0
3. **CapCut Assembler**: Handle zero-duration scenes (skip audio segment, use default image duration)
4. **Pipeline Runner**: Pass default scene duration config through to TimingResolver

### Architecture Compliance
- Config via Viper with `mapstructure` tags
- TimingResolver is a service-layer component, not a plugin
- Timing data flows: TTS → TimingResolver → Subtitle + Assembler
- All timing in seconds (float64), CapCut uses microseconds (int64 conversion in capcut.go)

### Testing Standards
- Unit tests in `internal/service/timing_test.go` and `internal/plugin/output/capcut/capcut_test.go`
- Test zero-duration scenes, offset recalculation, timeline.json output

### References
- [Source: internal/service/timing.go] — TimingResolver implementation
- [Source: internal/config/types.go#OutputConfig] — Output configuration
- [Source: internal/plugin/output/capcut/capcut.go#buildDraftProject] — CapCut timing conversion

## Dev Agent Record

### Agent Model Used
Claude Opus 4.6

### Completion Notes List
- Added `DefaultSceneDuration` config field with 3.0s default via Viper
- TimingResolver now accepts configurable default duration via `WithDefaultSceneDuration()` builder
- Zero-duration scenes use default duration in both TimingResolver and CapCut assembler
- CapCut assembler skips audio segment for scenes without AudioPath
- Pipeline runner passes config default scene duration through to TimingResolver

### File List
- internal/config/types.go (added: DefaultSceneDuration field to OutputConfig)
- internal/config/config.go (added: output.default_scene_duration Viper default)
- internal/service/timing.go (added: WithDefaultSceneDuration, defaultSceneDuration field, zero-duration fallback)
- internal/service/timing_test.go (added: TestResolveTimings_ZeroDurationUsesDefault, TestWithDefaultSceneDuration, TestWithDefaultSceneDuration_IgnoresInvalid)
- internal/plugin/output/capcut/capcut.go (added: zero-duration fallback, conditional audio segment)
- internal/plugin/output/capcut/types.go (added: DefaultSceneDurationSec constant)
- internal/pipeline/runner.go (updated: pass defaultSceneDuration to TimingResolver)
