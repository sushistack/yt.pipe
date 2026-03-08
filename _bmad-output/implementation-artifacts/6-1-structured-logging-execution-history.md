# Story 6-1: Structured Logging & Execution History

## Status: Done

## Implementation Summary

### New Files
- `internal/logging/logging.go` — slog JSON/text structured logging setup with configurable level and format
- `internal/logging/logging_test.go` — Tests for logging setup (JSON output, text format, level filtering)
- `internal/service/execution_summary.go` — `BuildExecutionSummary()` aggregates execution logs into summary (total duration, stage breakdown, API call counts, estimated cost)
- `internal/service/execution_summary_test.go` — Tests for execution summary builder
- `internal/cli/logs_cmd.go` — `yt-pipe logs <scp-id>` CLI command with `--summary`, `--limit`, `--json-output` flags
- `internal/cli/logs_cmd_test.go` — Tests for duration formatting helper

### Modified Files
- `internal/cli/root.go` — Integrated `logging.Setup()` in `initConfig()` to initialize structured logging based on config `log_level` and `log_format`

### Architecture Decisions
- Used stdlib `log/slog` for structured logging (already used in pipeline/runner.go)
- JSON format by default (matches existing `log_format: json` default in config)
- `logging.Setup()` sets the global slog default so all existing `slog.Info/Debug/Warn` calls automatically get structured output
- Execution summary is computed in-memory from execution_logs table (no new DB tables needed)

### CLI Usage
```bash
# Show execution logs + summary
yt-pipe logs SCP-001

# Summary only
yt-pipe logs SCP-001 --summary

# JSON output
yt-pipe logs SCP-001 --json-output

# Limit entries
yt-pipe logs SCP-001 --limit 100
```

### Acceptance Criteria Met
- [x] slog JSON structured logging initialized from config
- [x] Execution logs queryable via `yt-pipe logs <scp-id>`
- [x] Execution summary: total duration, stages, API calls, estimated cost
- [x] --summary, --limit, --json-output flags
- [x] All tests pass
