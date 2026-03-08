# Story 7-6: Webhook Notifications

## Status: Done

## Implementation Summary

### New Files
- `internal/api/webhook.go` — Webhook notification system: fan-out delivery to configured URLs on state changes, exponential backoff retry (1s, 2s, 4s, max 3 retries), non-blocking (failures don't block pipeline)
- `internal/api/webhook_test.go` — Tests for webhook delivery, retry on failure, fan-out independence, no-op when unconfigured

### Architecture Decisions
- Webhook payload: `{"event": "state_change", "project_id": "...", "scp_id": "...", "previous_state": "...", "new_state": "...", "timestamp": "..."}`
- Fan-out: all configured URLs receive notifications independently
- Failure of one URL does not affect delivery to others
- Retry uses exponential backoff (1s, 2s, 4s) with max 3 attempts
- All delivery attempts logged with status code and response time
- No errors logged when no webhook URLs configured
- Configured via YAML: `webhooks.urls: ["https://n8n.local/webhook/yt-pipe"]`

### Acceptance Criteria Met
- [x] HTTP POST sent to configured URLs on state transitions
- [x] Retry with exponential backoff (3 retries max)
- [x] Fan-out: independent delivery per URL
- [x] Webhook failures don't block pipeline execution
- [x] No-op when no URLs configured
- [x] All tests pass
