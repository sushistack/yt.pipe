# Story 10.4: CosyVoice Flash Model & Subtitle Generation

Status: done

## Story

As a creator,
I want to optionally use the Flash model for faster/cheaper TTS and auto-generate subtitles from word timings,
So that I can balance cost vs quality and have synchronized subtitles for my videos.

## Acceptance Criteria

1. `tts.model: cosyvoice-v1-flash` switches to Flash model - config change only
2. SRT subtitle generation from word-level timings in `SynthesisResult.WordTimings`
3. Per-scene subtitle at `{project}/scenes/{scene_num}/subtitle.srt`
4. Subtitle segments: max 2 lines, max 40 chars per line
5. SRT timing format: `HH:MM:SS,mmm --> HH:MM:SS,mmm`
6. Project-wide subtitle file at `{project}/subtitles.srt` with correct time offsets

## Tasks / Subtasks

- [ ] Verify Flash model support in dashscope provider (model param only)
- [ ] Create `internal/service/subtitle.go` - SRT generation from word timings
- [ ] Create `internal/service/subtitle_test.go`

## Dev Notes

### SRT Format
```
1
00:00:00,000 --> 00:00:02,500
첫 번째 자막 텍스트

2
00:00:02,500 --> 00:00:05,000
두 번째 자막 텍스트
```

### Existing Domain Model
- `domain.WordTiming{Word, StartSec, EndSec}` - already in domain/scene.go
- `domain.Scene.WordTimings` - already populated by TTS synthesis

### References
- [Source: _bmad-output/planning-artifacts/epics.md#Story 10.4]
- [Source: internal/domain/scene.go] - WordTiming struct
