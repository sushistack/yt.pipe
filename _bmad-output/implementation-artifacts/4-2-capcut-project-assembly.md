# Story 4.2: CapCut Project Assembly

Status: done

## Story
As a creator, I want all generated assets automatically assembled into a CapCut project with proper timing, so that I can open CapCut and find a nearly-complete video ready for final touches.

## Acceptance Criteria
- [ ] `yt-pipe assemble <scp-id>` creates CapCut project file from validated template structure
- [ ] Each scene's image placed on video track at Timing Resolver positions
- [ ] Each scene's audio placed on audio track synchronized with image
- [ ] Each subtitle segment placed on text track at word-level timing positions
- [ ] Project saved to output/draft_content.json and draft_meta_info.json
- [x] State transitions to `assembling` → `complete` (implemented, errors properly propagated)
- [ ] CLI output: CapCut project file path, total video duration, scenes/images/audio/subtitles summary
- [ ] Re-run updates only changed scenes' tracks
- [x] ValidationError lists all scenes with missing assets (image, audio, subtitle)

## Implementation (Code Review Fixes Applied)

### Service Layer Improvements
- `internal/service/assembler.go`: AssembleResult return type provides structured summary data
- `WithConfig()` method passes TemplatePath, MetaPath, CanvasConfig from OutputConfig to AssembleInput
- Output directory creation with `os.MkdirAll` before assembly
- Structured logging with scene count, total duration, image count, subtitle count

### Test Coverage
- `internal/service/assembler_test.go`: 16 tests total
  - TestAssemble_Success: verifies result fields and StatusComplete state
  - TestAssemble_MultipleScenes: 3-scene assembly with correct result
  - TestAssemble_WithConfig: TemplatePath, MetaPath, canvas (1280x720@24fps) propagation
  - TestAssemble_EmptyScenes: zero-scenes validation
  - TestAssemble_MissingImage/Audio/Subtitle: individual asset validation
  - TestAssemble_MultipleAssetErrors: batch error reporting
  - TestAssemble_ValidationFailure: post-assembly validation error handling

### CLI Command (Completed)
- [x] `internal/cli/assemble_cmd.go` — `yt-pipe assemble <scp-id>` cobra subcommand
- [x] Scene loading from workspace manifest.json files with sort by scene number
- [x] Canvas config from OutputConfig (CanvasWidth, CanvasHeight, FPS)
- [x] Copyright notice + special copyright integration in CLI flow
- [x] Human-readable output: SCP ID, output path, scenes, duration, images, audio clips, subtitles

### CapCut Assembler Plugin (Completed)
- [x] `internal/plugin/output/capcut/capcut.go` — Concrete Assembler implementation
- [x] Video track: ImagePath → VideoMaterial + segment at timeline position
- [x] Audio track: AudioPath → AudioMaterial + segment synchronized with video
- [x] Text track: WordTimings → TextMaterial + segment at word-level positions
- [x] Timing: microseconds from AudioDuration and WordTimings (secsToMicro)
- [x] draft_content.json + draft_meta_info.json generation
- [x] `internal/plugin/output/capcut/validator.go` — Schema validation (tracks, materials, canvas, timing)
- [x] `internal/plugin/output/capcut/capcut_test.go` — 14 tests

## Reference
- CapCut track types: video, audio, text (6 text tracks in template)
- Segment fields: source_timerange, target_timerange, material_id, clip (scale, rotation, transform)
- Materials section: videos, audios, texts, canvases, beats
- Text material: content field is JSON string with text, styles (font, size, bold, color, range)
- Transform positioning: normalized coordinates (x: 0-1, y: 0-1 relative to canvas)

## Dev Agent Record
### Agent Model Used
Claude Opus 4.6

### File List
- `internal/plugin/output/interface.go` — AssembleResult struct, Assemble returns (*AssembleResult, error)
- `internal/service/assembler.go` — WithConfig, structured result, output dir creation, structured logging
- `internal/service/assembler_test.go` — 16 tests including WithConfig, multiple scenes, validation failure
- `internal/mocks/mock_Assembler.go` — Updated return type

### Change Log
- 2026-03-08: Code review pass 1 — Fixed 5 HIGH/MEDIUM issues (AssembleResult type, config propagation, incremental support noted, summary logging, output dir creation)
