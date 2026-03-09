# Story 16-3: Approval Service — Per-Scene Workflow Orchestration

## Overview
Service layer that orchestrates per-scene generate-preview-approve/reject/regenerate cycles.

## Changes

### `internal/service/approval.go`
- `ApprovalService` struct with store, imageGenSvc, ttsSvc, logger dependencies
- `InitApprovals(projectID, sceneCount, assetType)` — initialize records for all scenes
- `MarkGenerated(projectID, sceneNum, assetType)` — validate current status, delegate to store
- `ApproveScene(projectID, sceneNum, assetType)` — validate status=generated before approving
- `RejectScene(projectID, sceneNum, assetType)` — validate status=generated before rejecting
- `AutoApproveAll(projectID, assetType)` — bulk approve with log warning
- `AllApproved(projectID, assetType)` — gate check delegation
- `GetApprovalStatus(projectID, assetType)` — summary (total, approved, pending, rejected counts)

### `internal/service/approval_test.go`
- Full test coverage with in-memory store

## Acceptance Criteria
- [x] Service validates status transitions before delegating to store
- [x] AutoApproveAll bulk-approves all scenes
- [x] GetApprovalStatus returns accurate summary
- [x] All tests pass
