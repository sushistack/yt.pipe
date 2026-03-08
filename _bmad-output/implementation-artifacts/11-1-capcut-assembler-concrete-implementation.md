# Story 11.1: CapCut Assembler Concrete Implementation

Status: done

## Story

As a creator,
I want the CapCut assembler to work with real generated assets from the scenario, image, and TTS pipelines,
So that I can open a fully assembled project in CapCut immediately after pipeline completion.

## Acceptance Criteria

1. `yt-pipe assemble <SCP-ID>` CLI command loads all scene assets from workspace and produces CapCut draft JSON
2. For each scene, reads: `scenes/{num}/image.png`, `scenes/{num}/narration.mp3`, `scenes/{num}/timing.json`, `scenes/{num}/subtitle.srt`
3. Video track: one segment per scene with image as material; Audio track: one segment per scene with narration MP3; Text track: subtitle segments from SRT mapped to CapCut text format
4. Output: `draft_content.json` and `draft_meta_info.json` in `{project}/output/`, validates against CapCut schema from Story 4.1
5. Pre-assembly validation: lists all scenes with missing assets (which files missing per scene), halts with clear error referencing `yt-pipe status <SCP-ID> --scenes`
6. Re-run assembly without affecting completed scenes

## Tasks / Subtasks

- [ ] Task 1: Add `yt-pipe assemble <scp-id>` CLI command (AC: #1)
  - [ ] Add assembleCmd to stage_cmds.go with proper Cobra wiring
  - [ ] Wire to pipeline runner's runAssembleStage (already exists)
  - [ ] Display assembly summary (scene count, duration, file paths)
- [ ] Task 2: Enhance pre-assembly validation with detailed error messages (AC: #5)
  - [ ] Check each scene dir for image.png, audio file, timing.json, subtitle file
  - [ ] Produce per-scene missing-asset report
  - [ ] Error message references `yt-pipe status <SCP-ID> --scenes`
- [ ] Task 3: Integrate copyright generation into assembly flow (AC: #4)
  - [ ] After assembly, call GenerateCopyrightNotice and LogSpecialCopyright
  - [ ] Load meta.json for SCP author info
- [ ] Task 4: Add unit tests for new CLI command and enhanced validation
- [ ] Task 5: Verify end-to-end with existing capcut_test.go suite

## Dev Notes

### Existing Code (DO NOT Reinvent)
- `internal/plugin/output/capcut/capcut.go` — Full CapCut assembler with buildDraftProject, already produces draft_content.json + draft_meta_info.json
- `internal/plugin/output/capcut/types.go` — All CapCut format types (DraftProject, Track, Segment, Materials, etc.)
- `internal/plugin/output/capcut/validator.go` — Schema validation (version, canvas, tracks, materials, timing)
- `internal/service/assembler.go` — AssemblerService with Assemble(), GenerateCopyrightNotice(), CheckSpecialCopyright(), LogSpecialCopyright()
- `internal/pipeline/runner.go` — runAssembleStage() already loads scenes from dir and calls assembler
- `internal/cli/stage_cmds.go` — buildRunner() wires all plugins, parseSceneNums() exists

### What Needs to Change
1. **CLI**: Add `assembleCmd` and `assembleGenerateCmd` in stage_cmds.go, register under rootCmd
2. **Pipeline Runner**: Enhance `runAssembleStage()` to also call copyright generation after assembly
3. **AssemblerService**: Improve validation error message format to list per-scene missing files with recovery command

### Architecture Compliance
- Plugin pattern: CapCut assembler implements `output.Assembler` interface
- Service layer: `AssemblerService` wraps plugin with business logic
- State machine: assembly transitions StatusAssembling → StatusComplete
- CLI adapter: Cobra commands delegate to pipeline runner
- File naming: `snake_case.go`, tests co-located as `_test.go`

### Testing Standards
- testify assert/mock, mockery for interface mocks
- Co-located `_test.go` files
- No real API calls, use workspace fixtures

### References
- [Source: internal/plugin/output/capcut/capcut.go] — CapCut assembler implementation
- [Source: internal/service/assembler.go] — Assembly service + copyright
- [Source: internal/pipeline/runner.go#runAssembleStage] — Assembly stage in pipeline
- [Source: internal/cli/stage_cmds.go#buildRunner] — Plugin wiring

## Dev Agent Record

### Agent Model Used
Claude Opus 4.6

### Completion Notes List
- Enhanced `assemble_cmd.go` with `--check-license` flag, pre-assembly validation via `ValidateSceneAssets()`, copyright generation, and license check summary
- Added `ValidateSceneAssets()` to `assembler.go` with per-scene missing-asset report and recovery command reference
- Wired copyright generation into pipeline runner's `runAssembleStage()` and `Resume()` via `generateCopyright()` method
- Added unit tests for ValidateSceneAssets, CheckLicenseFields

### File List
- internal/cli/assemble_cmd.go (enhanced: --check-license flag, license check, validation, copyright)
- internal/cli/stage_cmds.go (updated: removed duplicate assembleCmd, added DefaultSceneDuration to RunnerConfig)
- internal/service/assembler.go (added: ValidateSceneAssets, CheckLicenseFields, LicenseCheckResult, enhanced LogSpecialCopyright)
- internal/service/assembler_test.go (added: TestValidateSceneAssets*, TestCheckLicenseFields*, TestLogSpecialCopyright_AppendsToDescriptionTxt)
- internal/pipeline/runner.go (added: generateCopyright method, defaultSceneDuration field)
