# Story 6-3: Quality Feedback & Pipeline Metrics

## Status: Done

## Implementation Summary

### New Files
- `internal/store/migrations/002_feedback.sql` — `feedback` table with indexes
- `internal/domain/feedback.go` — `Feedback` struct, `ValidAssetTypes`, `ValidRatings`
- `internal/store/feedback.go` — `CreateFeedback`, `ListFeedbackByProject`, `ListAllFeedback`
- `internal/store/feedback_test.go` — CRUD tests for feedback store
- `internal/service/metrics.go` — `BuildPipelineMetrics()` aggregates projects, logs, and feedback
- `internal/service/metrics_test.go` — Tests for metrics builder
- `internal/cli/feedback_cmd.go` — `yt-pipe feedback <scp-id> --scene N --type image --rating good [--comment "..."]`
- `internal/cli/feedback_cmd_test.go` — Validation tests
- `internal/cli/metrics_cmd.go` — `yt-pipe metrics [scp-id]` with --json-output

### Modified Files
- `internal/store/store_test.go` — Updated schema version assertion (1 → 2)

### Database Migration
```sql
-- 002_feedback.sql
CREATE TABLE feedback (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    project_id TEXT NOT NULL REFERENCES projects(id),
    scene_num INTEGER NOT NULL,
    asset_type TEXT NOT NULL,  -- 'image', 'audio', 'subtitle', 'scenario'
    rating TEXT NOT NULL,      -- 'good', 'bad', 'neutral'
    comment TEXT,
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);
```

### CLI Usage
```bash
# Submit feedback
yt-pipe feedback SCP-001 --scene 3 --type image --rating good
yt-pipe feedback SCP-001 --scene 5 --type audio --rating bad --comment "pronunciation issue"

# View metrics (global)
yt-pipe metrics

# View metrics (per-project)
yt-pipe metrics SCP-001

# JSON output
yt-pipe metrics --json-output
```

### Acceptance Criteria Met
- [x] `yt-pipe feedback <scp-id> --scene N --type T --rating R` records feedback
- [x] `feedback` table in SQLite with proper migration
- [x] `yt-pipe metrics` shows overall and per-project aggregation
- [x] Success rate, cost, feedback breakdown by type and rating
- [x] --json-output support
- [x] All tests pass
