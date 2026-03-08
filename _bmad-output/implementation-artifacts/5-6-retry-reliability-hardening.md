# Story 5.6: Retry & Reliability Hardening

Status: done

## Story
As a creator, I want external API failures to be automatically retried with smart backoff, so that transient errors don't require manual intervention.

## Acceptance Criteria
- [x] External API calls retried up to 3 times with exponential backoff (1s, 2s, 4s)
- [x] Each retry attempt logged with attempt number, error type, and wait duration
- [x] Retryable errors: network timeout, HTTP 429, HTTP 5xx
- [x] Non-retryable errors (400, 401, 403): no retry, immediate propagation with clear message
- [x] Configurable timeout per API call (default 120 seconds per NFR10)
- [x] Timeout treated as retryable error
- [x] Ctrl+C: context.Cancel propagates to all in-flight API calls
- [x] On cancellation, current stage progress checkpointed before exit
- [x] Message displays: "Pipeline interrupted. Resume with: yt-pipe run <scp-id>"
- [x] Success rate calculable from last 100 executions in execution_logs

## Implementation

### Error Classification
- `internal/pipeline/reliability.go`:
  - `HTTPError` struct: StatusCode + Message, implements `IsRetryable()` interface
    - Retryable: 429 (rate limit), 408 (timeout), 5xx (server errors)
    - Non-retryable: 400, 401, 403, 404
  - `TimeoutError` struct: Operation + Timeout, always retryable
  - Both implement `retry.RetryableError` interface for integration with existing retry.Do()

### Graceful Shutdown
- `internal/pipeline/reliability.go`:
  - `GracefulRunner` struct: wraps Runner with signal handling
  - `RunWithSignalHandling()`: installs SIGINT/SIGTERM handler, cancels context on signal
  - `ResumeWithSignalHandling()`: same signal handling for resume path
  - `handleInterruption()`: logs last completed stage, prints resume command to stderr

### Existing Infrastructure Used
- `internal/retry/retry.go`:
  - `Do()`: exponential backoff with 0-25% jitter, max 60s cap
  - `RetryableError` interface: `IsRetryable() bool`
  - `isRetryable()`: checks error interface, defaults to retryable for unknown errors
  - Already integrated in `ImageGenService.GenerateSceneImage()` and `TTSService.SynthesizeScene()`
- `internal/plugin/base.go`:
  - `WithTimeout()`: context timeout wrapper (default 120s)
- `internal/store/execution_log.go`:
  - `CreateExecutionLog()`: records pipeline executions for success rate calculation

### Tests
- `internal/pipeline/reliability_test.go`: 4 tests
  - HTTPError_IsRetryable: 10 status codes (429/5xx retryable, 400/401/403/404 non-retryable)
  - HTTPError_Error: format "HTTP 429: rate limited"
  - TimeoutError_IsRetryable: always true, message format
  - Interface compliance: compile-time check for IsRetryable() interface

## Dev Agent Record
### Agent Model Used
Claude Opus 4.6

### File List
- `internal/pipeline/reliability.go` — New: HTTPError, TimeoutError, GracefulRunner with signal handling
- `internal/pipeline/reliability_test.go` — New: 4 unit tests

### Change Log
- 2026-03-08: Initial implementation with all acceptance criteria met
