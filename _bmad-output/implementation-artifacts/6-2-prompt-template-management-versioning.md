# Story 6-2: Prompt Template Management & Versioning

## Status: Done

## Implementation Summary

### New Files
- `internal/template/manager.go` — `Manager` for loading, validating, and versioning Go text/template files from a `templates/` directory
  - `LoadAll()` — loads all `.tmpl` files, fail-fast on syntax errors
  - `Load(name)` — load single template by name
  - `Get(name)` — get parsed `*template.Template`
  - `GetVersion(name)` — SHA-256 truncated version hash
  - `List()` — list all loaded templates with info
  - `ValidateFile(path)` — standalone validation for a template file
  - `HashContent(content)` — SHA-256 content hash for version tracking
- `internal/template/manager_test.go` — Comprehensive tests (load all, syntax error fail-fast, empty template, version changes, execute)
- `templates/image_prompt.tmpl` — Default image prompt template (matches existing built-in default)

### Modified Files
- `internal/config/types.go` — Added `TemplatesPath` field to `Config`
- `internal/config/config.go` — Added `templates_path` default ("") and env var tracking (`YTP_TEMPLATES_PATH`)

### Architecture Decisions
- Template versioning uses truncated SHA-256 (first 8 bytes / 16 hex chars), consistent with existing `hashTemplate()` in `image_prompt.go`
- Manager supports multiple named templates (extensible for narration, subtitle templates in future)
- Fail-fast: syntax errors during `LoadAll()` immediately return error — pipeline won't start with broken templates
- Empty templates are rejected as errors
- Non-existent templates directory is silently accepted (templates are optional)
- Existing `image_prompt.go` template loading remains backward-compatible — Manager is an additional, higher-level abstraction

### Configuration
```yaml
templates_path: ./templates  # or absolute path
```

### Acceptance Criteria Met
- [x] `templates/` directory with Go text/template support
- [x] SHA-256 version hashing per template
- [x] Template version available via `GetVersion(name)` for output metadata
- [x] Fail-fast on missing or syntax-error templates
- [x] All tests pass
