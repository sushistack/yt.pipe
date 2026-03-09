# Story 18.6: Pipeline Run Handler with Stage-Based Execution

Status: done

## Story

As a n8n workflow orchestrator,
I want the POST /run endpoint to support scenario-only mode (default) and full pipeline mode,
so that I can control the execution granularity from n8n workflows.

## Acceptance Criteria

1. **Given** POST /projects/{id}/run is called without a mode field
   **When** the handler processes the request
   **Then** it defaults to "scenario" mode
   **And** generates the scenario using ScenarioService.GenerateScenarioForProject
   **And** transitions the project to "scenario_review" state
   **And** the job status becomes "waiting_approval"

2. **Given** POST /projects/{id}/run is called with mode: "full"
   **When** the handler processes the request
   **Then** it runs the full pipeline via pipeline.Runner.RunWithOptions
   **And** auto-approve is enabled for API mode
   **And** progress is tracked via ProgressFunc callback

3. **Given** POST /projects/{id}/run is called with mode: "scenario" and LLM is available
   **When** scenario generation completes
   **Then** a state_change webhook is fired (previous_state → scenario_review)
   **And** the job record is updated to "waiting_approval" with progress 100

4. **Given** a pipeline is already running for the project
   **When** a duplicate run request is received
   **Then** 409 CONFLICT is returned

5. **Given** an invalid mode value is provided
   **When** the handler validates the request
   **Then** 400 BAD_REQUEST is returned with a descriptive message

6. **Given** mode: "full" but no PipelineRunner is configured
   **When** the handler attempts execution
   **Then** the job fails with "pipeline runner not configured" error
   **And** a job_failed webhook is fired

## Tasks / Subtasks

- [x] Task 1: Define run mode constants and request parsing (AC: #1, #5)
  - [x] 1.1: Define RunModeScenario and RunModeFull constants
  - [x] 1.2: Parse mode and dryRun from request body
  - [x] 1.3: Default to RunModeScenario when mode is empty
  - [x] 1.4: Validate mode is one of "scenario" or "full"
- [x] Task 2: Replace executePipeline stub with mode dispatcher (AC: #1, #2)
  - [x] 2.1: Add mode parameter to executePipeline
  - [x] 2.2: Handle dry-run as quick-complete with webhook
  - [x] 2.3: Dispatch to executeScenarioOnly or executeFullPipeline based on mode
- [x] Task 3: Implement executeScenarioOnly (AC: #1, #3)
  - [x] 3.1: Check scenarioSvc availability
  - [x] 3.2: Load SCP data from workspace
  - [x] 3.3: Call scenarioSvc.GenerateScenarioForProject
  - [x] 3.4: Set job status to "waiting_approval"
  - [x] 3.5: Fire state_change webhook
- [x] Task 4: Implement executeFullPipeline (AC: #2, #6)
  - [x] 4.1: Check pipelineRunner availability
  - [x] 4.2: Set ProgressFunc for real-time job record updates
  - [x] 4.3: Call pipelineRunner.RunWithOptions with AutoApprove: true
  - [x] 4.4: Handle success/failure with appropriate webhooks
- [x] Task 5: Add WithPipelineRunner ServerOption (AC: #2)
  - [x] 5.1: Add pipelineRunner field to Server struct
  - [x] 5.2: Add WithPipelineRunner option function
- [x] Task 6: Add GenerateScenarioForProject to ScenarioService (AC: #1)
  - [x] 6.1: Implement method that generates scenario for an existing project
- [x] Task 7: Write unit tests (AC: #1-#6)
  - [x] 7.1: Test scenario-only mode (default)
  - [x] 7.2: Test full pipeline mode
  - [x] 7.3: Test dry-run mode
  - [x] 7.4: Test duplicate run detection (409)
  - [x] 7.5: Test invalid mode (400)
  - [x] 7.6: Test missing pipeline runner (fail with webhook)

## Dev Notes

### Key Architecture Decisions

- **Scenario-only as default**: n8n workflows typically need granular control; scenario + pause at review is the expected pattern
- **Auto-approve in full mode**: API full pipeline mode sets AutoApprove: true since approval is managed externally by n8n
- **JobStatusWaitingApproval**: New status distinct from "running" — indicates scenario is complete and awaiting external approval
- **SCP data loading**: Uses workspace.LoadSCPData to read SCP content files before scenario generation

### References

- [Source: internal/api/pipeline.go] - executePipeline, executeScenarioOnly, executeFullPipeline
- [Source: internal/api/server.go] - WithPipelineRunner option
- [Source: internal/service/scenario.go] - GenerateScenarioForProject
- [Source: internal/pipeline/runner.go] - RunWithOptions

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6

### Completion Notes List

- Replaced executePipeline stub with mode-based dispatcher
- Implemented executeScenarioOnly: loads SCP data, generates scenario, transitions to scenario_review
- Implemented executeFullPipeline: uses pipeline.Runner with auto-approve and progress callback
- Added RunModeScenario/RunModeFull constants, JobStatusWaitingApproval status
- Added WithPipelineRunner ServerOption
- Added GenerateScenarioForProject to ScenarioService
- Dry-run returns immediately with job_complete webhook
- All tests pass, no regressions

### File List

- internal/api/pipeline.go (modified) - Mode parsing, executePipeline, executeScenarioOnly, executeFullPipeline
- internal/api/server.go (modified) - WithPipelineRunner option, pipelineRunner field
- internal/service/scenario.go (modified) - GenerateScenarioForProject method
- internal/api/pipeline_test.go (modified) - Mode validation, scenario/full/dry-run tests
