# Story 11.3: Copyright & Licensing Metadata Integration

Status: done

## Story

As a creator,
I want CC-BY-SA 3.0 copyright attribution and SCP-specific licensing warnings included automatically,
So that every assembled project is legally compliant without manual effort.

## Acceptance Criteria

1. Assembly generates `description.txt` in `{project}/output/` with: SCP Foundation credit, original author(s) from `meta.json`, CC-BY-SA 3.0 license URL, AI-generated content disclosure — formatted for YouTube description paste
2. Additional copyright conditions from `meta.json` trigger CLI warning and are appended to `description.txt`
3. Warning recorded in execution log
4. `yt-pipe assemble <SCP-ID> --check-license` validates all required attribution fields in meta.json, reports missing fields as warnings (assembly still proceeds), includes result in assembly summary
5. License check result included in assembly summary output

## Tasks / Subtasks

- [ ] Task 1: Wire copyright generation into assembly pipeline flow (AC: #1, #2)
  - [ ] In pipeline runner's runAssembleStage(), call GenerateCopyrightNotice after assembly
  - [ ] In Resume(), call GenerateCopyrightNotice + LogSpecialCopyright after assembly
  - [ ] Load meta.json from SCP data path for author info
- [ ] Task 2: Add --check-license flag to assemble command (AC: #4, #5)
  - [ ] Add flag to assembleCmd
  - [ ] Implement license field validation (check author, copyright_notes in meta.json)
  - [ ] Report missing fields as warnings, don't block assembly
  - [ ] Include license check summary in CLI output
- [ ] Task 3: Enhance copyright notice with additional conditions (AC: #2, #3)
  - [ ] Append special copyright conditions to description.txt
  - [ ] Display CLI warning for additional conditions
  - [ ] Record warning in structured log
- [ ] Task 4: Add tests for copyright integration and license check

## Dev Notes

### Existing Code (DO NOT Reinvent)
- `internal/service/assembler.go` — GenerateCopyrightNotice(), CheckSpecialCopyright(), LogSpecialCopyright() all fully implemented
- `ccBySA3Template` constant already formatted for YouTube description
- `internal/workspace/scp_data.go` — MetaFile struct with Author, CopyrightNotes fields
- `internal/pipeline/runner.go` — runAssembleStage() and Resume() exist but don't call copyright functions

### What Needs to Change
1. **Pipeline Runner**: After assembly in runAssembleStage() and Resume(), load SCP meta.json and call copyright functions
2. **CLI**: Add `--check-license` flag to assemble command
3. **AssemblerService**: Add license check method that validates meta.json fields
4. **Copyright Notice**: When special conditions exist, append them to description.txt (currently only writes to separate copyright_warning.json)

### Architecture Compliance
- Copyright functions are in service layer (AssemblerService + standalone functions)
- SCP data loaded via workspace package
- Structured logging via slog
- CLI flags via Cobra

### Testing Standards
- Unit tests for license check validation
- Test copyright notice generation with/without special conditions
- Test --check-license flag behavior

### References
- [Source: internal/service/assembler.go#GenerateCopyrightNotice] — Copyright notice generation
- [Source: internal/service/assembler.go#CheckSpecialCopyright] — Special copyright check
- [Source: internal/workspace/scp_data.go#MetaFile] — SCP metadata structure
- [Source: internal/pipeline/runner.go#Resume] — Pipeline assembly flow

## Dev Agent Record

### Agent Model Used
Claude Opus 4.6

### Completion Notes List
- `GenerateCopyrightNotice` and `LogSpecialCopyright` now called automatically after assembly in both pipeline runner paths
- `LogSpecialCopyright` enhanced to append special conditions to description.txt (not just copyright_warning.json)
- `--check-license` flag validates author and copyright_notes fields in meta.json
- License check results displayed before and after assembly in CLI output

### File List
- internal/cli/assemble_cmd.go (added: --check-license flag, license check flow)
- internal/service/assembler.go (added: CheckLicenseFields, LicenseCheckResult, enhanced LogSpecialCopyright to append to description.txt)
- internal/service/assembler_test.go (added: TestCheckLicenseFields*, TestLogSpecialCopyright_AppendsToDescriptionTxt)
- internal/pipeline/runner.go (added: generateCopyright method called after assembly)
