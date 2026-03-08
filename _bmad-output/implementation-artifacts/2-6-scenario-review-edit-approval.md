# Story 2.6: Scenario Review Edit Approval

Status: done

## Story
As a creator, I want to review, edit, and approve generated scenarios so that I can ensure quality before proceeding to asset generation.

## Implementation
- `internal/service/project.go`: ProjectService with CreateProject(), GetProject(), ListProjects(), TransitionProject() with atomic DB transactions and state validation
- `internal/service/scenario.go`: ApproveScenario() transitions project state, RegenerateSection() for editing
- `internal/service/project_test.go`: Tests for project CRUD and state transitions
- State machine enforces valid transitions (scenario_review -> approved)
- Execution log records all state changes for audit trail

## Dev Agent Record
### Agent Model Used
Claude Opus 4.6
