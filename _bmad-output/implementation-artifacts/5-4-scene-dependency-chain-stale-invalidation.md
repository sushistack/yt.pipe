# Story 5.4: Scene Dependency Chain & Stale Invalidation

Status: done

## Story
As a creator, I want upstream changes to automatically invalidate downstream artifacts so the pipeline rebuilds only what's needed, so that I never have stale outputs after modifying a scene's scenario, prompt, or audio.

## Acceptance Criteria
- [x] Dependency chain defined: scenario section → image prompt → image, scenario section → narration → TTS audio → timing → subtitle
- [x] Scene manifest invalidates all downstream artifacts when upstream changes
- [x] Invalidated artifacts marked as `stale` in the manifest
- [x] Only stale artifacts regenerated on next pipeline run
- [x] Targeted invalidation: prompt edit only invalidates image (not TTS/subtitle)
- [x] Execution log records which artifacts were invalidated and why

## Implementation

### Dependency Graph
- `internal/pipeline/dependency.go`:
  - `AssetType` enum: narration, prompt, image, audio, timing, subtitle
  - `dependencyChain` map defines downstream relationships:
    - narration → {prompt, image, audio, timing, subtitle} (full cascade)
    - prompt → {image} (targeted)
    - audio → {timing, subtitle}
    - timing → {subtitle}
    - image, subtitle → {} (leaf nodes, no downstream)
  - `DependencyTracker` struct: manages invalidation via store
  - `InvalidateDownstream()`: clears hashes for all downstream assets, sets status to "stale"
  - `DetectChanges()`: compares current content hashes against stored manifests, triggers invalidation for changed scenes
  - `GetStaleScenes()`: returns scene numbers with status "stale"
  - `InvalidationResult`: records scene_num, changed_asset, list of invalidated asset types

### Existing Infrastructure Used
- `internal/service/pipeline_orchestrator.go`:
  - `InvalidateDownstream()` — file-manifest-based invalidation (complementary to DB-level)
  - `SceneManifest` — per-scene asset hash tracking

### Tests
- `internal/pipeline/dependency_test.go`: 5 tests
  - DependencyChain: validates chain lengths and leaf nodes
  - InvalidateDownstream_Narration: narration change clears all downstream hashes, sets stale
  - InvalidateDownstream_PromptOnly: prompt change only clears image hash, preserves audio/subtitle
  - InvalidateDownstream_LeafNode: image change has no downstream effect
  - GetStaleScenes: returns only scenes with stale status

## Dev Agent Record
### Agent Model Used
Claude Opus 4.6

### File List
- `internal/pipeline/dependency.go` — New: DependencyTracker, dependency chain graph, invalidation logic
- `internal/pipeline/dependency_test.go` — New: 5 unit tests

### Change Log
- 2026-03-08: Initial implementation with all acceptance criteria met
