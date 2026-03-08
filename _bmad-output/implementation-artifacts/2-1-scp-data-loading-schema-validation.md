# Story 2.1: SCP Data Loading & Schema Validation

Status: ready-for-dev

## Story

As a creator,
I want to input an SCP ID and have the system automatically load and validate its structured data,
So that I can start the content pipeline with confidence that the source data is correct.

## Acceptance Criteria

1. Given a valid SCP ID, load facts.json, meta.json, main.txt from `{scp_data_path}/{SCP-ID}/`
2. Validate schema version fields in facts.json and meta.json
3. Return NotFoundError for missing SCP directories or files
4. Return ValidationError for schema mismatches
5. Data is read-only, handled by `workspace/scp_data.go`

## Tasks

- [ ] Define SCP data types in `internal/workspace/scp_data.go`
- [ ] Implement LoadSCPData(scpDataPath, scpID) function
- [ ] Implement schema version validation
- [ ] Write tests in `internal/workspace/scp_data_test.go`

## Dev Notes

- SCP data files: facts.json (keyed facts), meta.json (metadata), main.txt (article text)
- Schema version: "1.0" expected
- Path pattern: `{scp_data_path}/SCP-{id}/` (e.g., `/data/raw/SCP-173/`)
- workspace/ imports only domain/

## Dev Agent Record
### Agent Model Used
Claude Opus 4.6
