# Story 18.1: Server Plugin Injection and Service Initialization

Status: review

## Story

As a n8n workflow orchestrator,
I want the API server to have fully initialized plugins and services,
so that API endpoints can execute real pipeline operations instead of returning stubs.

## Acceptance Criteria

1. **Given** the `serve` command is executed with valid configuration
   **When** the API server starts
   **Then** plugins (LLM, ImageGen, TTS, Output) are created from config using the same pattern as CLI's `run` command
   **And** services (ImageGenService, TTSService, AssemblerService, ScenarioService) are initialized with the plugin instances
   **And** services are injected into the Server struct via ServerOption functions

2. **Given** a plugin fails to initialize (e.g., invalid API key)
   **When** the server startup proceeds
   **Then** the server logs a warning for the failed plugin
   **And** the `/health` endpoint reports the degraded plugin status
   **And** endpoints depending on the failed plugin return `502 API_UPSTREAM_ERROR`

3. **Given** the server is running with initialized services
   **When** `GET /health` is called
   **Then** the response includes plugin availability status for each plugin type (llm, imagegen, tts, output)
   **And** response time is under 1 second (NFR3)

## Tasks / Subtasks

- [x] Task 1: Refactor plugin creation into shared utility (AC: #1)
  - [x] 1.1: Extract `createPlugins` logic from `internal/cli/plugins.go` into a shared function accessible by both CLI and API
  - [x] 1.2: Create `createPluginsGraceful` variant that returns partial results on failure instead of erroring
  - [x] 1.3: Register all providers (Gemini, Qwen, DeepSeek, SiliconFlow, DashScope) in shared registry
- [x] Task 2: Extend Server struct with service fields and ServerOption functions (AC: #1)
  - [x] 2.1: Add fields to Server struct: `scenarioSvc`, `imageGenSvc`, `ttsSvc`, `assemblerSvc`, `glossary`, `pluginStatus`
  - [x] 2.2: Create `ServerOption` functions: `WithScenarioService`, `WithImageGenService`, `WithTTSService`, `WithAssemblerService`, `WithGlossary`
  - [x] 2.3: Add `pluginStatus` map tracking availability per plugin type
- [x] Task 3: Update `serve_cmd.go` to initialize plugins and inject services (AC: #1)
  - [x] 3.1: Load config and create plugins using graceful initialization (log warning on failure, continue)
  - [x] 3.2: Create service instances: `NewScenarioService`, `NewImageGenService`, `NewTTSService`, `NewAssemblerService`
  - [x] 3.3: Load glossary from config path
  - [x] 3.4: Pass all services as `ServerOption` to `api.NewServer`
- [x] Task 4: Enhance `/health` endpoint with plugin status (AC: #2, #3)
  - [x] 4.1: Add `plugins` field to health response with per-type availability (llm, imagegen, tts, output)
  - [x] 4.2: Report overall status as "degraded" if any plugin is unavailable
- [x] Task 5: Add plugin availability check helper for handlers (AC: #2)
  - [x] 5.1: Create `requirePlugin(pluginType)` helper method on Server that returns 502 if plugin unavailable
  - [x] 5.2: Wire into existing stub handlers (handleGenerateImages, handleGenerateTTS, handleRunPipeline)
- [x] Task 6: Write unit tests (AC: #1, #2, #3)
  - [x] 6.1: Test server initialization with all plugins available
  - [x] 6.2: Test server initialization with partial plugin failure (graceful degradation)
  - [x] 6.3: Test /health endpoint returns correct plugin status
  - [x] 6.4: Test handler returns 502 when required plugin is unavailable

## Dev Notes

### Current State Analysis

**serve_cmd.go** (line 56): Currently creates server with NO plugin initialization:
```go
srv := api.NewServer(db, c)
```
No plugins, no services, no glossary. All API handlers are stubs.

**run_cmd.go** (lines 63-98): Has the correct pattern - calls `createPlugins(cfg)` then initializes pipeline.Runner with all plugins and services. This is the reference implementation to replicate.

**plugins.go**: Contains `createPlugins()` function and global `pluginRegistry` with provider registrations (init function). This is CLI-scoped - needs to be accessible to the serve command too.

**Server struct** (server.go lines 21-32): Already has `registry *plugin.Registry` field and `WithRegistry` option, but no service fields.

### Key Architecture Patterns

1. **Plugin Creation Chain** (from `plugins.go`):
   - Create config map from `config.Config` fields
   - Call `pluginRegistry.Create(pluginType, provider, configMap)`
   - Type-assert result to specific interface (e.g., `llm.LLM`)

2. **Service Dependencies**:
   - `ScenarioService`: needs `store`, `llm.LLM`, `ProjectService`
   - `ImageGenService`: needs `imagegen.ImageGen`, `store`, `logger`
   - `TTSService`: needs `tts.TTS`, `glossary.Glossary`, `store`, `logger`
   - `AssemblerService`: needs `output.Assembler`, `ProjectService`

3. **Graceful Degradation Pattern**:
   - CLI `createPlugins` fails hard (returns error) - appropriate for CLI
   - Server needs soft-fail: log warning, mark plugin unavailable, continue startup
   - Handler-level check: return 502 if required plugin is nil

### Implementation Approach

**Option A (Recommended): Shared registry, separate creation functions**
- Keep `pluginRegistry` in `internal/cli/plugins.go` but export it via `PluginRegistry()` (already exists)
- Create a new `createServerPlugins()` in `serve_cmd.go` that uses the same registry but handles errors gracefully
- This avoids changing the existing CLI flow

**Option B: Extract to internal/plugin/factory.go**
- Move plugin creation to a shared package
- Both CLI and serve command use same factory
- Cleaner but larger refactor scope

**Go with Option A** to minimize blast radius for this story.

### Project Structure Notes

- `internal/cli/plugins.go`: Plugin registry and creation (CLI scope)
- `internal/cli/serve_cmd.go`: Serve command - PRIMARY MODIFICATION TARGET
- `internal/api/server.go`: Server struct - ADD service fields and options
- `internal/api/health.go`: Health handler - ENHANCE with plugin status
- `internal/api/pipeline.go`: Pipeline handlers - ADD plugin availability checks
- `internal/api/assets.go`: Asset handlers - ADD plugin availability checks

### Constraints

- **No new packages**: Use existing `internal/api`, `internal/cli` packages
- **No breaking changes**: Existing `NewServer()` signature must remain backward compatible (options pattern)
- **Plugin registration**: Must register same providers as CLI (Gemini, Qwen, DeepSeek, SiliconFlow, DashScope)
- **No DB migration**: Uses existing schema

### References

- [Source: internal/cli/plugins.go] - Plugin registry and createPlugins function
- [Source: internal/cli/run_cmd.go] - Pipeline execution with plugin initialization
- [Source: internal/cli/serve_cmd.go] - Current serve command (no plugins)
- [Source: internal/api/server.go] - Server struct and options pattern
- [Source: internal/api/health.go] - Current health handler (basic status only)
- [Source: internal/api/pipeline.go] - Stub pipeline handlers
- [Source: internal/api/assets.go] - Stub asset handlers
- [Source: internal/service/image_gen.go:31] - NewImageGenService constructor
- [Source: internal/service/tts.go:29] - NewTTSService constructor
- [Source: internal/service/assembler.go:27] - NewAssemblerService constructor
- [Source: internal/service/scenario.go:23] - NewScenarioService constructor
- [Source: _bmad-output/planning-artifacts/epics.md#Epic18] - Story 18.1 acceptance criteria
- [Source: _bmad-output/planning-artifacts/architecture.md] - Dependency direction rules

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6

### Debug Log References

### Completion Notes List

- Added `PluginSet` struct and `createPluginsGraceful()` to plugins.go for graceful plugin initialization
- Extended Server struct with `scenarioSvc`, `imageGenSvc`, `ttsSvc`, `assemblerSvc`, `pluginStatus` fields
- Added 5 new ServerOption functions: WithScenarioService, WithImageGenService, WithTTSService, WithAssemblerService, WithPluginStatus
- Updated serve_cmd.go to initialize plugins, create services, load glossary, and inject into server
- Enhanced /health endpoint to return plugin availability status and "degraded" when plugins unavailable
- Added requirePlugin() helper that returns 502 API_UPSTREAM_ERROR for unavailable plugins
- Wired plugin checks into handleRunPipeline (llm), handleGenerateImages (imagegen), handleGenerateTTS (tts)
- Added tests: health with all/partial/no plugins, 502 for unavailable plugins
- Updated existing tests to account for plugin availability checks
- All tests pass (go test ./...), no regressions, go vet clean

### File List

- internal/cli/plugins.go (modified) - Added PluginSet, createPluginsGraceful
- internal/cli/serve_cmd.go (modified) - Plugin init, service creation, glossary loading
- internal/api/server.go (modified) - Service fields, ServerOption functions, requirePlugin helper
- internal/api/health.go (modified) - Plugin status in health response
- internal/api/pipeline.go (modified) - LLM plugin check in handleRunPipeline
- internal/api/assets.go (modified) - ImageGen/TTS plugin checks in handlers
- internal/api/server_test.go (modified) - Updated health test, added plugin status tests
- internal/api/pipeline_test.go (modified) - Added plugin unavailable test, updated existing tests
- internal/api/assets_test.go (modified) - Added plugin unavailable tests, updated existing tests
