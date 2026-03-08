# Story 9.5: Selective Scene Image Regeneration

Status: done

## Story

As a creator,
I want to regenerate images for specific scenes without re-running the entire pipeline,
So that I can iterate on individual scenes quickly.

## Acceptance Criteria

1. CLI command `yt-pipe image regenerate <scp-id> --scenes 3,5,7` regenerates specific scenes
2. Also supports `--scene 3` for single scene regeneration
3. Backs up existing image as `image.prev.{ext}` before regeneration
4. Uses existing image prompts from scenario data
5. Pipeline Runner exposes `RunImageRegenerate` method
6. Validates that at least one scene number is specified

## Implementation Summary

### Files Created/Modified
- `internal/cli/stage_cmds.go` — Added `imageRegenerateCmd` with --scenes, --scene, --edit-prompt flags
- `internal/pipeline/runner.go` — Added `RunImageRegenerate` method
- `internal/service/image_gen.go` — Added `BackupSceneImage` function

### Key Flow
1. Parse scene numbers from CLI flags
2. Find project by SCP ID
3. Load scenario and generate prompts
4. Backup existing images for selected scenes
5. Call `GenerateAllImages` with scene number filter
6. Log completion summary

### CLI Usage
```
yt-pipe image regenerate SCP-173 --scenes 3,5,7
yt-pipe image regenerate SCP-173 --scene 3
```

## References
- [Source: _bmad-output/planning-artifacts/epics.md#Story 9.5]
- [Source: internal/cli/stage_cmds.go] — CLI integration
- [Source: internal/pipeline/runner.go] — pipeline runner
