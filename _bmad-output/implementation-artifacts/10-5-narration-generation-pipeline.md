# Story 10.5: Narration Generation Pipeline

Status: done

## Story

As a creator,
I want to run `yt-pipe tts generate <SCP-ID>` to generate narration audio for all scenes,
So that each scene has a matching audio file ready for CapCut assembly.

## Acceptance Criteria

1. For each scene: pronunciation conversion -> DashScope TTS -> save audio/timing/subtitle
2. Audio saved as `{project}/scenes/{scene_num}/narration.mp3`
3. Word timings saved as `{project}/scenes/{scene_num}/timing.json`
4. Subtitles saved as `{project}/scenes/{scene_num}/subtitle.srt`
5. CLI displays per-scene progress: "Scene 3/10: converting pronunciation... synthesizing... saved (1.8s, 45.2s audio)"
6. Sequential scene processing to respect API rate limits
7. Failed scenes logged, generation continues with remaining
8. Re-run skips already-generated scenes (unless `--force`)
9. `--scenes 3,5` flag for selective scene regeneration
10. Total audio duration and estimated API cost displayed at completion

## Tasks / Subtasks

- [ ] Update `internal/service/tts.go` - integrate pronunciation + subtitle generation
- [ ] Update CLI tts generate command with `--force` and `--scenes` flags
- [ ] Add timing.json save logic
- [ ] Add progress display
- [ ] Tests

## Dev Notes

### Integration Points
- **PronunciationService** (Story 10.2): Called before TTS synthesis
- **SubtitleService** (Story 10.4): Called after TTS synthesis with word timings
- **TTSService.SynthesizeAll**: Already has partial failure handling and scene filtering

### Existing CLI Infrastructure
- `runStageCmd(service.StageTTSSynthesize)` already wired
- `buildRunner` creates pipeline.Runner with all plugins

### Skip Logic
- Check scene manifest for existing audio hash
- Skip if audio exists and source (narration text) hasn't changed
- `--force` bypasses skip logic

### References
- [Source: _bmad-output/planning-artifacts/epics.md#Story 10.5]
- [Source: internal/service/tts.go] - existing TTS service
- [Source: internal/cli/stage_cmds.go] - CLI patterns
