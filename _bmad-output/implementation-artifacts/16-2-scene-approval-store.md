# Story 16-2: Scene Approval Store

## Overview
Implement CRUD operations for scene approval tracking in the store layer.

## Changes

### `internal/store/scene_approval.go`
- `InitApproval(projectID, sceneNum, assetType)` — create with status=pending, attempts=0
- `MarkGenerated(projectID, sceneNum, assetType)` — set status=generated, increment attempts
- `Approve(projectID, sceneNum, assetType)` — set status=approved
- `Reject(projectID, sceneNum, assetType)` — set status=rejected
- `GetApproval(projectID, sceneNum, assetType)` — return single record
- `ListApprovalsByProject(projectID, assetType)` — filter by project + asset type
- `AllApproved(projectID, assetType)` — gate: true only if ALL scenes have status=approved
- `BulkApproveAll(projectID, assetType)` — for --skip-approval auto-approve

### `internal/store/scene_approval_test.go`
- Full coverage of all CRUD operations
- AllApproved gate logic tests
- Status transition tests

## Acceptance Criteria
- [x] All CRUD operations work correctly
- [x] AllApproved returns true only when every scene is approved
- [x] BulkApproveAll sets all scenes to approved in one operation
- [x] All tests pass
