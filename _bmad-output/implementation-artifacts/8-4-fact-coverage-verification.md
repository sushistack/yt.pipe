# Story 8-4: Fact Coverage Verification

## Status: Done

## Implementation Summary

### Modified Files
- `internal/service/fact_coverage.go` — Enhanced with: `FactCoverageItem` struct (per-fact details with scene tracking), `SuggestFactPlacements()` (recommends scenes for uncovered facts using category-based heuristics), `FormatDetailedReport()` (per-fact covered/missing status with scene numbers), `categorizeFactKey()` (maps fact keys to categories: physical_description, anomalous_properties, containment_procedures, discovery, incidents, general)
- `internal/service/fact_coverage_test.go` — Added tests for: `TestVerifyFactCoverage_Details`, `TestSuggestFactPlacements`, `TestFormatCoverageReport_Pass`, `TestFormatCoverageReport_Warn`, `TestFormatDetailedReport`, `TestCategorizeFactKey`

### Architecture Decisions
- Coverage threshold configurable via `scenario.fact_coverage_threshold` (default 80%)
- Per-fact detail tracking: each fact has covered/missing status and list of scene numbers where it appears
- Fact placement suggestions use category-based heuristics: physical_description → early scenes, anomalous_properties → mid scenes, incidents → later scenes
- Coverage report formats: summary (PASS/WARN with percentage) and detailed (per-fact with scene references)
- Cross-scene deduplication: same fact in multiple scenes counted once for coverage

### Acceptance Criteria Met
- [x] Coverage calculation: tagged facts / total facts × 100
- [x] Configurable threshold (default 80%)
- [x] Coverage report: total facts, covered, missing list, percentage
- [x] Below-threshold: lists missing facts with categories
- [x] `SuggestFactPlacements()` suggests scenes for missing facts
- [x] Detailed report: per-fact status with scene numbers
- [x] All tests pass
