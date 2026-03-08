# Story 1.7: Project State Machine & Transitions

Status: ready-for-dev

## Story

As a developer,
I want a robust state machine that governs project lifecycle transitions within SQLite transactions,
So that the project always has a consistent, valid state even during failures.

## Acceptance Criteria

1. **State Machine Validation** (AC:1)
   - Given the state machine is implemented in `service/project.go`
   - When a state transition is requested
   - Then the system validates the transition against the allowed transition map (pending → scenario_review → approved → generating_assets → assembling → complete)
   - And invalid transitions return a TransitionError with current state, requested state, and allowed transitions
   - And this satisfies FR22

2. **Transactional State Updates** (AC:2)
   - Given a state transition is valid
   - When it is executed
   - Then the state update runs within a SQLite transaction
   - And the transition timestamp is recorded in the project record
   - And the previous state is preserved in the execution log for audit

3. **Concurrent Access Safety** (AC:3)
   - Given a concurrent state change is attempted
   - When both try to update the state simultaneously
   - Then SQLite's serialized writes ensure only one succeeds
   - And the other receives a TransitionError

4. **Project CRUD via Service Layer** (AC:4)
   - Given a ProjectService wraps store operations
   - When projects are created, retrieved, or listed
   - Then all operations go through the service layer with proper validation

## Tasks / Subtasks

- [ ] Task 1: Create ProjectService in `internal/service/project.go` (AC: #1, #2, #4)
  - [ ] 1.1 Define ProjectService struct with Store dependency
  - [ ] 1.2 Implement NewProjectService(store) constructor
  - [ ] 1.3 Implement CreateProject(ctx, scpID, workspacePath) — creates with pending status
  - [ ] 1.4 Implement GetProject(ctx, id) — delegates to store
  - [ ] 1.5 Implement ListProjects(ctx) — delegates to store
  - [ ] 1.6 Implement TransitionProject(ctx, id, newStatus) — transactional state machine

- [ ] Task 2: Implement transactional transition (AC: #2, #3)
  - [ ] 2.1 Begin SQLite transaction
  - [ ] 2.2 SELECT project with FOR UPDATE semantics (SQLite serialization)
  - [ ] 2.3 Validate transition using domain.CanTransition
  - [ ] 2.4 UPDATE project status and updated_at
  - [ ] 2.5 INSERT execution_log entry recording old→new state
  - [ ] 2.6 Commit transaction

- [ ] Task 3: Write comprehensive tests (AC: #1, #2, #3, #4)
  - [ ] 3.1 Test valid transitions through full lifecycle
  - [ ] 3.2 Test invalid transitions return TransitionError
  - [ ] 3.3 Test project creation with pending status
  - [ ] 3.4 Test transition records execution log
  - [ ] 3.5 Test not-found project returns NotFoundError

- [ ] Task 4: Final verification
  - [ ] 4.1 `go build ./...` — zero errors
  - [ ] 4.2 `go test ./...` — all tests pass
  - [ ] 4.3 `go vet ./...` — zero warnings

## Dev Notes

### Architecture Constraints

- Service layer in `internal/service/` — imports domain/, store/
- CLI and API will depend on service layer (not store directly)
- State transitions MUST be transactional
- Execution log records all state changes for audit
- Use `domain.CanTransition()` for validation (already exists)
- Use `domain.TransitionError` for invalid transitions (already exists)
- Generate project ID using UUID (add google/uuid dependency)

### Existing Code

- `domain/project.go` — Project struct, status constants, AllowedTransitions map, CanTransition(), Transition()
- `domain/errors.go` — TransitionError, NotFoundError, ValidationError
- `store/project.go` — CreateProject, GetProject, ListProjects, UpdateProject
- `store/execution_log.go` — CreateExecutionLog
- `store/store.go` — Store with DB(), Begin transaction support via DB()

### Files this story creates
- `internal/service/project.go`
- `internal/service/project_test.go`

### Files that MUST NOT be modified
- `internal/domain/*` — stable
- `internal/store/*` — stable

### References
- [Source: _bmad-output/planning-artifacts/epics.md#Story 1.7]
- [Source: _bmad-output/planning-artifacts/architecture.md#State Machine]
- [Source: _bmad-output/planning-artifacts/prd.md#FR22]

## Dev Agent Record

### Agent Model Used
Claude Opus 4.6

### Completion Notes List

### File List
