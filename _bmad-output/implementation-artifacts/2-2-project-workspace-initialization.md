# Story 2.2: Project Workspace Initialization

Status: done

## Story
As a creator, I want each SCP project isolated in its own directory with a structured scene layout.

## Implementation
- `internal/workspace/project.go`: InitProject, InitSceneDir, WriteFileAtomic, ProjectExists
- `internal/workspace/project_test.go`: 6 tests covering directory creation, atomic writes, existence checks
- Layout: `{workspace}/{scp-id}-{timestamp}/scenes/{scene-num}/`
- Atomic file writes using temp file + rename pattern

## Dev Agent Record
### Agent Model Used
Claude Opus 4.6
