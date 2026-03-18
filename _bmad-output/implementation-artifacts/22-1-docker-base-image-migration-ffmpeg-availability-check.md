# Story 22.1: Docker Base Image Migration & FFmpeg Availability Check

Status: ready-for-dev

## Story

As a system operator,
I want the Docker runtime image to include FFmpeg,
so that the FFmpeg rendering pipeline can execute in containerized environments.

## Acceptance Criteria

1. **Dockerfile runtime stage changed from `scratch` to `alpine:3.21`**
   - `ffmpeg`, `ca-certificates`, and `tzdata` installed via `apk add --no-cache`
   - Non-root user `appuser` (UID 65534) created via `adduser -D -u 65534 appuser`
   - Binary (`/yt-pipe`), templates (`/templates`), and `ENTRYPOINT` unchanged
   - `docker build` succeeds and `docker run` starts the server on port 8080

2. **`checkFFmpegAvailable()` returns clear error when FFmpeg is not installed**
   - Error message: `"ffmpeg binary not found in PATH: install ffmpeg or use Docker image with ffmpeg included"`
   - Satisfies ENFR3

3. **`checkFFmpegAvailable()` succeeds when FFmpeg is installed**
   - No error returned
   - FFmpeg binary path stored for later use by FFmpegAssembler

## Tasks / Subtasks

- [ ] Task 1: Modify Dockerfile runtime stage (AC: #1)
  - [ ] Change `FROM scratch` to `FROM alpine:3.21`
  - [ ] Add `RUN apk add --no-cache ffmpeg ca-certificates tzdata`
  - [ ] Add `RUN adduser -D -u 65534 appuser`
  - [ ] Remove existing CA cert copy (alpine provides via apk)
  - [ ] Keep `COPY --from=builder /app/bin/yt-pipe /yt-pipe`
  - [ ] Keep `COPY --from=builder /app/templates /templates`
  - [ ] Keep `USER 65534:65534` → change to `USER appuser`
  - [ ] Keep `EXPOSE 8080` and `ENTRYPOINT ["/yt-pipe"]`
- [ ] Task 2: Create `internal/plugin/output/ffmpeg/` package (AC: #2, #3)
  - [ ] Create `internal/plugin/output/ffmpeg/ffmpeg.go`
  - [ ] Implement `checkFFmpegAvailable() (string, error)` using `exec.LookPath("ffmpeg")`
  - [ ] Return ffmpeg path on success, domain error on failure
- [ ] Task 3: Unit tests for `checkFFmpegAvailable` (AC: #2, #3)
  - [ ] Test with PATH containing ffmpeg → no error, path returned
  - [ ] Test error message content matches ENFR3 spec
  - [ ] Use `ffmpegtest` build tag for tests requiring actual ffmpeg binary
  - [ ] Non-tagged test: mock/verify error message format without ffmpeg dependency

## Dev Notes

### Architecture Constraints

- **Package location:** `internal/plugin/output/ffmpeg/ffmpeg.go` — this is where all ffmpeg output plugin code lives [Source: architecture.md#EFR6]
- **Interface compliance:** FFmpegAssembler will implement `output.Assembler` (Stories 22.2-22.4), but this story only creates the package and `checkFFmpegAvailable()`
- **No registry registration yet** — Story 22-4 handles plugin registry integration
- **Build tag:** Integration tests requiring actual ffmpeg binary use `//go:build ffmpegtest` [Source: architecture.md#EFR6 Test Strategy]

### Dockerfile Current State

Current Dockerfile (`Dockerfile` at project root):
- Build stage: `FROM golang:1.25` with CGO_ENABLED=0, `-trimpath -ldflags="-s -w"`
- Runtime stage: `FROM scratch` — copies CA certs separately, binary, templates
- User: `65534:65534` (numeric, works with scratch)
- Must change to alpine-based approach per architecture decision

### Key Implementation Details

```dockerfile
# Target Dockerfile runtime stage:
FROM alpine:3.21
RUN apk add --no-cache ffmpeg ca-certificates tzdata
RUN adduser -D -u 65534 appuser
COPY --from=builder /app/bin/yt-pipe /yt-pipe
COPY --from=builder /app/templates /templates
USER appuser
EXPOSE 8080
ENTRYPOINT ["/yt-pipe"]
```

```go
// internal/plugin/output/ffmpeg/ffmpeg.go
package ffmpeg

import (
    "fmt"
    "os/exec"
)

// checkFFmpegAvailable verifies ffmpeg is in PATH and returns its path.
func checkFFmpegAvailable() (string, error) {
    path, err := exec.LookPath("ffmpeg")
    if err != nil {
        return "", fmt.Errorf("ffmpeg binary not found in PATH: install ffmpeg or use Docker image with ffmpeg included")
    }
    return path, nil
}
```

### Testing Strategy

1. **Unit test (no build tag):** Test that `checkFFmpegAvailable` returns the expected error message format — can use a PATH manipulation approach or simply test the error string contract
2. **Integration test (`//go:build ffmpegtest`):** Test with actual ffmpeg binary present — verifies path discovery works on systems with ffmpeg installed
3. **Docker build test:** Manual verification that `docker build` succeeds

### Existing Patterns to Follow

- **Plugin package structure:** Follow `internal/plugin/output/capcut/` pattern — package per provider
- **Error handling:** Use `fmt.Errorf` for infrastructure errors (not domain errors) since ffmpeg absence is an environment issue
- **Test files:** `*_test.go` in same package, use `testify` assertions
- **Logging:** Use `log/slog` structured logger (injected via constructor in later stories)

### Project Structure Notes

- New directory: `internal/plugin/output/ffmpeg/` — aligns with architecture decision [Source: architecture.md lines 1649-1658]
- Existing output interface: `internal/plugin/output/interface.go` — `Assembler` interface with `Assemble()` and `Validate()` methods
- Existing capcut impl: `internal/plugin/output/capcut/capcut.go` — reference implementation pattern

### References

- [Source: architecture.md#Docker Base Image Change] — scratch → alpine decision with Dockerfile example
- [Source: architecture.md#EFR6] — FFmpegAssembler struct, checkFFmpegAvailable() signature
- [Source: prd-enhancement.md#ENFR3] — FFmpeg Docker inclusion + error message requirement
- [Source: epics.md#Story 22.1] — Full acceptance criteria with BDD format

## Dev Agent Record

### Agent Model Used

### Debug Log References

### Completion Notes List

### File List
