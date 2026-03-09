# Story 16-1: State Machine Extension & Scene Approval Domain Model

## Overview
Extend the project state machine with `image_review` and `tts_review` states, create the SceneApproval domain model, and add database migration for scene approval tracking.

## Changes

### 1. `internal/domain/project.go` — State Machine Extension
- Add constants: `StatusImageReview`, `StatusTTSReview`
- Remove: `StatusGeneratingAssets` (replaced by image_review → tts_review flow)
- Update `allowedTransitions`:
  - `approved → image_review`
  - `image_review → tts_review`
  - `tts_review → assembling`
- Backward compat: `--skip-approval` auto-transitions through review states

### 2. `internal/domain/scene_approval.go` — Domain Model
- `SceneApproval` struct: project_id, scene_num, asset_type, status, attempts, updated_at
- Asset types: "image", "tts"
- Status flow: pending → generated → approved (or generated → rejected → generated → approved)
- `ValidateSceneApproval()` validation function

### 3. `internal/store/migrations/007_scene_approvals.sql` — DB Migration
- `scene_approvals` table with composite PK (project_id, scene_num, asset_type)
- CHECK constraints for asset_type and status enums
- Index: `idx_scene_approvals_project`

### 4. Test Updates
- `domain/scene_approval_test.go` — model validation tests
- `domain/project_test.go` — updated state transition tests (approval + skip-approval paths)
- `store/store_test.go` — schema version 7, scene_approvals table creation test

## Acceptance Criteria
- [x] State machine: pending → scenario_review → approved → image_review → tts_review → assembling → complete
- [x] --skip-approval compatibility: image_review and tts_review auto-transitioned
- [x] SceneApproval model with per-scene status tracking
- [x] Migration 007 creates scene_approvals table
- [x] All existing tests updated and passing
