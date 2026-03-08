# Story 10.3: Voice Cloning via Config-Level VoiceID

Status: done

## Story

As a creator,
I want to use a cloned voice for narration by specifying a VoiceID in config,
So that my videos have a distinctive, consistent narrator voice without changing any code.

## Acceptance Criteria

1. Config `tts.voice: "cosyvoice-clone-{voice_id}"` triggers voice clone mode
2. DashScope API request includes `"voice_clone": true` when voice ID has clone prefix
3. Standard preset voices (e.g., `longxiaochun`) work without clone flag
4. CLI command `yt-pipe tts register-voice --audio <sample.wav> --name "my-narrator"` registers new voice
5. Returned VoiceID displayed and optionally written to config

## Tasks / Subtasks

- [ ] Update `internal/plugin/tts/dashscope.go` - detect clone voice prefix, set API flag
- [ ] Add `tts register-voice` CLI command in `internal/cli/tts_cmd.go`
- [ ] Add tests for clone detection logic

## Dev Notes

### Voice Clone Detection
- Prefix `cosyvoice-clone-` indicates cloned voice
- Standard voices: `longxiaochun`, `longhua`, etc. - no prefix
- API payload difference: `"voice_clone": true` in parameters

### CLI Pattern
- Follow existing `cobra.Command` patterns in `stage_cmds.go`
- Register under `ttsCmd` parent command

### References
- [Source: _bmad-output/planning-artifacts/epics.md#Story 10.3]
- [Source: internal/cli/stage_cmds.go] - CLI command patterns
