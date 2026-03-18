# Story 22.4: FFmpegAssembler Integration, Registry & Output Selection

Status: ready-for-dev

## Story

As a content creator,
I want to render MP4 videos directly via FFmpeg as an alternative to CapCut project output,
so that I can produce finished videos without manual CapCut assembly.

## Acceptance Criteria

1. **FFmpegAssembler implements `output.Assembler` interface**
   - `Assemble(ctx, input)` generates concat files, SRT, BGM filters (22.2/22.3), executes FFmpeg command
   - Output: 1920x1080 MP4 with libx264 video and AAC audio
   - Returns AssembleResult with output path, scene count, total duration

2. **FFmpegConfig struct added to config/types.go**
   - Preset (string, default "medium"), CRF (int, default 23), AudioBitrate (string, default "192k")
   - Resolution (string, default "1920x1080"), FPS (int, default 30), SubtitleFontSize (int, default 24)

3. **Plugin registry registration**
   - `registry.Create(PluginTypeOutput, "ffmpeg", cfg)` returns FFmpegAssembler
   - FFmpeg availability verified during creation

4. **Output provider selection: "capcut" | "ffmpeg" | "both"**
   - "ffmpeg": only FFmpegAssembler invoked
   - "capcut": only CapCut Assembler (existing behavior, default)
   - "both": both assemblers invoked sequentially in service/assembler.go (~10 lines change)

5. **Graceful degradation**: no subtitles or no BGM → corresponding input omitted from FFmpeg command

## Tasks / Subtasks

- [ ] Task 1: Add FFmpegConfig to config/types.go (AC: #2)
- [ ] Task 2: Implement FFmpegAssembler.Assemble + Validate (AC: #1, #5)
- [ ] Task 3: Add Factory + register in cli/plugins.go (AC: #3)
- [ ] Task 4: Modify service/assembler.go for "both" mode (AC: #4)
- [ ] Task 5: Modify createPluginsGraceful for provider selection (AC: #4)
- [ ] Task 6: Unit tests + integration tests

## Dev Notes

### References
- [Source: architecture.md#EFR6] — Full FFmpeg command, directory structure, performance reqs
- [Source: epics.md#Story 22.4] — Full acceptance criteria

## Dev Agent Record

### Agent Model Used
### Debug Log References
### Completion Notes List
### File List
