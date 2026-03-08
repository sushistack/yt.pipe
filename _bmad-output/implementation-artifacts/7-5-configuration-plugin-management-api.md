# Story 7-5: Configuration & Plugin Management API

## Status: Done

## Implementation Summary

### New Files
- `internal/api/config_handler.go` — Config and plugin management handlers: `GET /api/v1/config` (masked sensitive values), `PATCH /api/v1/config` (runtime update with persistence), `GET /api/v1/plugins` (registered plugins list), `PUT /api/v1/plugins/:type/active` (switch active plugin)
- `internal/api/config_handler_test.go` — Tests for config read/update, API key masking, plugin listing, plugin switching, validation errors

### Architecture Decisions
- API keys are masked in GET response (show only prefix for identification)
- PATCH validates changes before applying; 400 if validation fails with no changes applied
- Plugin listing shows: name, type (LLM/TTS/ImageGen/OutputAssembler), status (active/available), configuration
- Plugin switching validates plugin name against registry; 400 for unknown plugins with available list
- Config changes persist to YAML config file

### Acceptance Criteria Met
- [x] `GET /api/v1/config` returns config with masked API keys
- [x] `PATCH /api/v1/config` updates and persists settings
- [x] `GET /api/v1/plugins` lists all registered plugins
- [x] `PUT /api/v1/plugins/:type/active` switches active plugin
- [x] Validation prevents invalid config changes
- [x] All tests pass
