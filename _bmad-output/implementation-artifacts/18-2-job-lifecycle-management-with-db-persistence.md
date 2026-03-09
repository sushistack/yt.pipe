# Story 18.2: Job Lifecycle Management with DB Persistence

Status: done

## Story

As a n8n workflow orchestrator,
I want pipeline jobs to be tracked in the database with full lifecycle management,
so that job status survives server restarts and stale jobs are automatically recovered.

## Acceptance Criteria

1. **Given** a pipeline job is created via POST /run, /images/generate, or /tts/generate
   **When** the job is persisted to the database
   **Then** the job record includes: id, project_id, type, status, progress, result, error, created_at, updated_at

2. **Given** the server restarts while jobs are in "running" status
   **When** `InitJobLifecycle()` is called during server startup
   **Then** all stale "running" jobs are marked as "failed" with error message "server restarted"
   **And** the count of recovered jobs is logged

3. **Given** completed/failed jobs exist beyond the retention period
   **When** `PurgeOldJobs()` runs during server startup
   **Then** jobs older than `JobRetentionDays` (default: 7) are deleted
   **And** running jobs are never purged

4. **Given** a job is running for a project
   **When** `GET /projects/{id}/status` is called
   **Then** the response includes in-memory progress if available
   **Or** falls back to DB for the most recent job record

5. **Given** a specific job ID
   **When** `GET /jobs/{jobId}` is called
   **Then** full job details are returned including elapsed time, result, and error fields

## Tasks / Subtasks

- [x] Task 1: Extend jobManager with type-based tracking (AC: #1)
  - [x] 1.1: Add `getByType(projectID, jobType)` method using composite key "projectID:jobType"
  - [x] 1.2: Add `startTyped(projectID, jobType, jobID, cancel)` method
  - [x] 1.3: Add `removeTyped(projectID, jobType)` method
- [x] Task 2: Add DB job query methods to store (AC: #2, #3, #4)
  - [x] 2.1: Implement `GetLatestJobByProject(projectID)` — returns most recent job or nil
  - [x] 2.2: Implement `MarkStaleJobsFailed(errMsg)` — bulk update running → failed
  - [x] 2.3: Implement `PurgeOldJobs(retentionDays)` — delete old completed/failed/cancelled jobs
  - [x] 2.4: Implement `GetRunningJobByProjectAndType(projectID, jobType)` — duplicate detection
- [x] Task 3: Implement InitJobLifecycle on Server (AC: #2, #3)
  - [x] 3.1: Call `MarkStaleJobsFailed` on startup to recover interrupted jobs
  - [x] 3.2: Call `PurgeOldJobs` with configurable retention days
  - [x] 3.3: Log recovered and purged job counts
- [x] Task 4: Enhance handleGetStatus with DB fallback (AC: #4)
  - [x] 4.1: Check in-memory jobManager first for real-time progress
  - [x] 4.2: Fall back to `GetLatestJobByProject` from DB when no in-memory job exists
  - [x] 4.3: Include elapsed_sec, progress_pct, result, and error fields
- [x] Task 5: Implement handleGetJob endpoint (AC: #5)
  - [x] 5.1: Route `GET /jobs/{jobId}` to handler
  - [x] 5.2: Return full job details with RFC3339 timestamps and computed elapsed_sec
  - [x] 5.3: Include completed_at for terminal states (complete, failed, cancelled)
- [x] Task 6: Add JobRetentionDays config field (AC: #3)
  - [x] 6.1: Add `JobRetentionDays` field to config.Config
  - [x] 6.2: Set default value of 7 in config loading
- [x] Task 7: Write unit tests (AC: #1-#5)
  - [x] 7.1: Test jobManager type-based operations (getByType, startTyped, removeTyped)
  - [x] 7.2: Test store methods: GetLatestJobByProject, MarkStaleJobsFailed, PurgeOldJobs, GetRunningJobByProjectAndType
  - [x] 7.3: Test handleGetStatus with in-memory job and DB fallback
  - [x] 7.4: Test handleGetJob endpoint
  - [x] 7.5: Test InitJobLifecycle on startup

## Dev Notes

### Key Architecture Decisions

- **Composite key pattern**: jobManager uses "projectID:jobType" as map key for typed jobs, allowing concurrent image and TTS jobs per project
- **DB as source of truth**: In-memory jobManager is for real-time progress; DB is for persistence across restarts
- **Retention cleanup**: Only terminal-state jobs (complete/failed/cancelled) are purged; running jobs are never deleted

### References

- [Source: internal/api/pipeline.go] - jobManager with type-based methods
- [Source: internal/store/job.go] - DB query methods
- [Source: internal/config/types.go] - JobRetentionDays field
- [Source: internal/config/config.go] - Default retention value

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6

### Completion Notes List

- Extended jobManager with getByType, startTyped, removeTyped using composite key pattern
- Added 4 new store methods: GetLatestJobByProject, MarkStaleJobsFailed, PurgeOldJobs, GetRunningJobByProjectAndType
- Implemented InitJobLifecycle for stale job recovery and retention cleanup on server startup
- Enhanced handleGetStatus with in-memory → DB fallback chain
- Added handleGetJob endpoint at GET /jobs/{jobId} with full detail response
- Added JobRetentionDays config field with default 7
- All tests pass, no regressions

### File List

- internal/api/pipeline.go (modified) - jobManager type-based methods, InitJobLifecycle, handleGetJob, handleGetStatus enhancement
- internal/api/server.go (modified) - GET /jobs/{jobId} route registration
- internal/store/job.go (modified) - GetLatestJobByProject, MarkStaleJobsFailed, PurgeOldJobs, GetRunningJobByProjectAndType
- internal/config/types.go (modified) - JobRetentionDays field
- internal/config/config.go (modified) - Default value for JobRetentionDays
- internal/api/pipeline_test.go (modified) - Job lifecycle and status tests
- internal/store/job_test.go (modified) - Store method tests
