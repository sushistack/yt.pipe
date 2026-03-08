# Story 6-4: Project Cleanup & Disk Management

## Status: Done

## Implementation Summary

### New Files
- `internal/service/cleanup.go` — `CleanProject()` for intermediate artifact removal, `GetDiskUsage()` for workspace analysis
- `internal/service/cleanup_test.go` — Tests for clean (intermediates, all, dry-run) and disk usage
- `internal/cli/clean_cmd.go` — `yt-pipe clean <scp-id>` with `--all`, `--dry-run`, `--status`, `--json-output`
- `internal/cli/clean_cmd_test.go` — Tests for byte formatting

### Architecture Decisions
- **Intermediate cleanup** removes: `prompt.txt`, `manifest.json`, `checkpoint.json`, `timing.json`, `words.json` per scene
- **Final outputs preserved**: `scenario.json`, `output/*`, images, audio, subtitles
- **`--all` flag**: Removes entire workspace directory + sets project status to "archived"
- **`--dry-run`**: Lists files that would be deleted with their sizes
- **`--status`**: Shows disk usage breakdown by category (images, audio, subtitles, scenario, output, other)

### CLI Usage
```bash
# Clean intermediate artifacts (preserves final output)
yt-pipe clean SCP-001

# Preview what would be deleted
yt-pipe clean SCP-001 --dry-run

# Remove everything and archive project
yt-pipe clean SCP-001 --all

# Show disk usage breakdown
yt-pipe clean SCP-001 --status

# JSON output
yt-pipe clean SCP-001 --status --json-output
```

### Acceptance Criteria Met
- [x] `yt-pipe clean <scp-id>` removes intermediates, preserves final outputs
- [x] `--all` removes everything + archives project
- [x] `--dry-run` lists deletion candidates without deleting
- [x] `--status` shows disk usage breakdown by category
- [x] All tests pass
