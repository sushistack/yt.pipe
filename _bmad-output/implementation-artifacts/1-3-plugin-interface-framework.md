# Story 1.3: Plugin Interface Framework

Status: review

## Story

As a developer,
I want standardized plugin interfaces for all external integrations with mock implementations,
So that each pipeline module can be developed and tested independently without external API dependencies.

## Acceptance Criteria

1. **Four Plugin Interfaces Defined** (AC:1)
   - Given the plugin package is created
   - When plugin interfaces are defined
   - Then four interfaces exist: LLM (`plugin/llm/interface.go`), TTS (`plugin/tts/interface.go`), ImageGen (`plugin/imagegen/interface.go`), OutputAssembler (`plugin/output/interface.go`)
   - And each interface's methods accept `context.Context` as the first parameter
   - And each interface uses `domain/` types for input/output (Scene, ScenarioOutput, etc.)

2. **Common Plugin Base** (AC:2)
   - Given `plugin/base.go` defines common helpers
   - When a plugin implementation is created
   - Then it can use shared PluginConfig loading and Timeout helpers
   - And the common retry helper from `internal/retry/retry.go` is available
   - And the retry helper supports configurable max attempts, exponential backoff, and retries only on network timeout/429/5xx errors

3. **Mock Auto-Generation** (AC:3)
   - Given mockery is configured
   - When `make generate` (`go generate ./...`) is run
   - Then mock implementations for all 4 plugin interfaces are auto-generated in `internal/mocks/`
   - And unit tests can use these mocks to test service layer without external API calls (NFR23)

4. **Plugin Registry** (AC:4)
   - Given a plugin registry exists
   - When a plugin type is specified in YAML (e.g., `llm.provider: openai`)
   - Then the corresponding implementation is selected and initialized at startup
   - And an unknown provider returns a `PluginError` with available providers listed

## Tasks / Subtasks

- [x] Task 1: Implement retry helper (AC: #2)
  - [x] 1.1 Create `internal/retry/retry.go` — replace doc.go placeholder (DELETE doc.go after)
  - [x] 1.2 Implement `Do(ctx context.Context, maxAttempts int, baseDelay time.Duration, fn func() error) error`
  - [x] 1.3 Implement exponential backoff with jitter
  - [x] 1.4 Implement retryable error classification: retry on network timeout, 429, 5xx; no retry on 400, 401, 403
  - [x] 1.5 Implement `RetryableError` interface for error classification
  - [x] 1.6 Write unit tests in `internal/retry/retry_test.go`

- [x] Task 2: Create plugin base (AC: #2)
  - [x] 2.1 Create `internal/plugin/base.go` — replace doc.go placeholder (DELETE doc.go after)
  - [x] 2.2 Define `PluginConfig` struct (Name, Timeout, MaxRetries, BaseDelay)
  - [x] 2.3 Define `TimeoutHelper` function: wraps a function with context timeout
  - [x] 2.4 Write unit tests in `internal/plugin/base_test.go`

- [x] Task 3: Define LLM plugin interface (AC: #1)
  - [x] 3.1 Create `internal/plugin/llm/interface.go` — replace doc.go placeholder (DELETE doc.go after)
  - [x] 3.2 Define `LLM` interface with methods: `GenerateScenario(ctx, scpData) (ScenarioOutput, error)`, `RegenerateSection(ctx, scenario, sectionNum, instruction) (SceneScript, error)`
  - [x] 3.3 Add `//go:generate mockery --name=LLM --output=../../mocks --outpkg=mocks` directive
  - [x] 3.4 Write compile-time interface check test

- [x] Task 4: Define TTS plugin interface (AC: #1)
  - [x] 4.1 Create `internal/plugin/tts/interface.go` — replace doc.go placeholder (DELETE doc.go after)
  - [x] 4.2 Define `TTS` interface with methods: `Synthesize(ctx, text, voice) (audioBytes, WordTimings, error)`, `SynthesizeWithOverrides(ctx, text, voice, overrides) (audioBytes, WordTimings, error)`
  - [x] 4.3 Add `//go:generate mockery` directive
  - [x] 4.4 Write compile-time interface check test

- [x] Task 5: Define ImageGen plugin interface (AC: #1)
  - [x] 5.1 Create `internal/plugin/imagegen/interface.go` — replace doc.go placeholder (DELETE doc.go after)
  - [x] 5.2 Define `ImageGen` interface with methods: `Generate(ctx, prompt, opts) (imageBytes, error)`, `GenerateBatch(ctx, prompts, opts) ([]imageBytes, error)`
  - [x] 5.3 Add `//go:generate mockery` directive
  - [x] 5.4 Write compile-time interface check test

- [x] Task 6: Define OutputAssembler plugin interface (AC: #1)
  - [x] 6.1 Create `internal/plugin/output/interface.go` — replace doc.go placeholder (DELETE doc.go after)
  - [x] 6.2 Define `Assembler` interface with methods: `Assemble(ctx, project, scenes) (outputPath, error)`, `Validate(ctx, outputPath) error`
  - [x] 6.3 Add `//go:generate mockery` directive
  - [x] 6.4 Write compile-time interface check test

- [x] Task 7: Implement plugin registry (AC: #4)
  - [x] 7.1 Create `internal/plugin/registry.go` — registry for mapping provider names to factory functions
  - [x] 7.2 Implement `Registry` struct with `Register(pluginType, providerName, factory)` and `Create(pluginType, providerName, cfg) (interface{}, error)`
  - [x] 7.3 Return `PluginError` with available provider list when provider not found
  - [x] 7.4 Write unit tests in `internal/plugin/registry_test.go`

- [x] Task 8: Install mockery and generate mocks (AC: #3)
  - [x] 8.1 Ensure `go install github.com/vektra/mockery/v2@latest` is available
  - [x] 8.2 Add `.mockery.yaml` config at project root for mock output settings
  - [x] 8.3 Run `go generate ./...` — verify mocks generated in `internal/mocks/`
  - [x] 8.4 Write a sample test using a generated mock to verify mock usability

- [x] Task 9: Final verification
  - [x] 9.1 `go build ./...` — zero errors
  - [x] 9.2 `go test ./...` — all tests pass (including Story 1.1, 1.2 tests)
  - [x] 9.3 `go vet ./...` — zero warnings
  - [x] 9.4 `go generate ./...` — mocks regenerate cleanly
  - [x] 9.5 Verify no import cycles between packages

## Dev Notes

### Critical Architecture Constraints

**Plugin 4 Types (Architecture-mandated):**
- **LLM** — scenario generation, section regeneration
- **TTS** — text-to-speech synthesis with word timings
- **ImageGen** — image generation from prompts
- **OutputAssembler** — final project assembly (CapCut is default implementation)

**Plugin File Structure (Architecture-mandated):**
```
internal/plugin/
├── base.go                    # Common: PluginConfig, Timeout helper
├── base_test.go
├── registry.go                # Provider name → factory mapping
├── registry_test.go
├── llm/
│   └── interface.go           # LLM interface (replaces doc.go)
├── tts/
│   └── interface.go           # TTS interface (replaces doc.go)
├── imagegen/
│   └── interface.go           # ImageGen interface (replaces doc.go)
└── output/
    └── interface.go           # Assembler interface (replaces doc.go)

internal/retry/
└── retry.go                   # Retry helper (replaces doc.go)
    retry_test.go

internal/mocks/
├── mock_LLM.go                # mockery auto-generated
├── mock_TTS.go
├── mock_ImageGen.go
└── mock_Assembler.go
```

**Retry Helper (Architecture-mandated):**
```go
// internal/retry/retry.go
// MUST NOT import any other internal/ packages

func Do(ctx context.Context, maxAttempts int, baseDelay time.Duration, fn func() error) error
```
- Exponential backoff: `baseDelay * 2^attempt` with jitter (random 0-25%)
- Retryable: network timeout, HTTP 429, HTTP 5xx
- Non-retryable: HTTP 400, 401, 403, context cancellation
- Use `RetryableError` interface pattern for error classification:
```go
type RetryableError interface {
    error
    IsRetryable() bool
}
```
- If error does NOT implement `RetryableError`, default to retryable (conservative)
- Context cancellation always stops retry immediately

**Plugin Interface Design Principles:**
1. All methods: `context.Context` as first parameter
2. All methods return `error` as last return value
3. Use `domain/` types for input/output — NOT plugin-specific types
4. Each interface in its own sub-package to avoid import cycles
5. Implementations created later in separate stories (Epic 2-4)

### Interface Specifications

**LLM Interface (`plugin/llm/interface.go`):**
```go
package llm

import (
    "context"
    "github.com/jay/youtube-pipeline/internal/domain"
)

// LLM defines the interface for language model plugins.
type LLM interface {
    // GenerateScenario generates a complete scenario from SCP data.
    GenerateScenario(ctx context.Context, scpID string, mainText string, facts []domain.FactTag, metadata map[string]string) (*domain.ScenarioOutput, error)

    // RegenerateSection regenerates a single scene's script based on instruction.
    RegenerateSection(ctx context.Context, scenario *domain.ScenarioOutput, sceneNum int, instruction string) (*domain.SceneScript, error)
}
```

**Design option for dev consideration:** The `GenerateScenario` input parameters are primitives. An alternative is to define a `domain.SCPData` struct bundling `scpID`, `mainText`, `facts`, `metadata` into one type, giving a cleaner signature: `GenerateScenario(ctx, data domain.SCPData)`. This requires adding a new type to `domain/` (which is allowed — domain is enriched progressively). Story 2.1 will need `SCPData` in `workspace/scp_data.go` anyway, so defining it in `domain/` now or deferring are both valid. Dev's judgment call — either approach is acceptable.

**TTS Interface (`plugin/tts/interface.go`):**
```go
package tts

import (
    "context"
    "github.com/jay/youtube-pipeline/internal/domain"
)

// SynthesisResult holds the output of TTS synthesis.
type SynthesisResult struct {
    AudioData   []byte
    WordTimings []domain.WordTiming
    DurationSec float64
}

// TTS defines the interface for text-to-speech plugins.
type TTS interface {
    // Synthesize converts text to speech audio with word-level timing.
    Synthesize(ctx context.Context, text string, voice string) (*SynthesisResult, error)

    // SynthesizeWithOverrides applies pronunciation overrides from the glossary.
    SynthesizeWithOverrides(ctx context.Context, text string, voice string, overrides map[string]string) (*SynthesisResult, error)
}
```

**ImageGen Interface (`plugin/imagegen/interface.go`):**
```go
package imagegen

import "context"

// ImageResult holds the output of image generation.
type ImageResult struct {
    ImageData []byte
    Format    string // "png", "jpg", "webp"
    Width     int
    Height    int
}

// GenerateOptions holds optional parameters for image generation.
type GenerateOptions struct {
    Width   int
    Height  int
    Model   string
    Style   string
    Seed    int64
}

// ImageGen defines the interface for image generation plugins.
type ImageGen interface {
    // Generate creates a single image from a prompt.
    Generate(ctx context.Context, prompt string, opts GenerateOptions) (*ImageResult, error)
}
```
NOTE: No `GenerateBatch` in the interface — batching is the service layer's responsibility (loop + goroutines). Keep plugin interface minimal. Architecture says "1 scene = 1 image" in MVP.

NOTE: `GenerateOptions.Seed` supports reproducibility. When a seed is used, the service layer should record it in the scene manifest so results can be reproduced. This is out of scope for this story but relevant for Epic 3 (image generation) and Epic 5 (incremental builds).

**OutputAssembler Interface (`plugin/output/interface.go`):**
```go
package output

import (
    "context"
    "github.com/jay/youtube-pipeline/internal/domain"
)

// AssembleInput contains all assets needed for final project assembly.
type AssembleInput struct {
    Project    domain.Project
    Scenes     []domain.Scene
    OutputDir  string
}

// Assembler defines the interface for output format assembly plugins.
type Assembler interface {
    // Assemble creates the final output project from all scene assets.
    Assemble(ctx context.Context, input AssembleInput) (outputPath string, err error)

    // Validate checks if a previously assembled output is still valid.
    Validate(ctx context.Context, outputPath string) error
}
```

### Plugin Base Specification

```go
// internal/plugin/base.go
package plugin

import (
    "context"
    "time"
)

// PluginConfig holds common configuration for all plugins.
type PluginConfig struct {
    Name       string
    Timeout    time.Duration
    MaxRetries int
    BaseDelay  time.Duration
}

// DefaultPluginConfig returns sensible defaults.
func DefaultPluginConfig(name string) PluginConfig {
    return PluginConfig{
        Name:       name,
        Timeout:    120 * time.Second, // NFR10: configurable, default 120s
        MaxRetries: 3,                 // NFR6: max 3 retries
        BaseDelay:  1 * time.Second,
    }
}

// WithTimeout executes fn with a context timeout derived from PluginConfig.
func WithTimeout(ctx context.Context, timeout time.Duration, fn func(ctx context.Context) error) error {
    ctx, cancel := context.WithTimeout(ctx, timeout)
    defer cancel()
    return fn(ctx)
}
```

### Plugin Registry Specification

```go
// internal/plugin/registry.go
package plugin

// PluginType identifies the category of plugin.
type PluginType string

const (
    PluginTypeLLM       PluginType = "llm"
    PluginTypeTTS       PluginType = "tts"
    PluginTypeImageGen  PluginType = "imagegen"
    PluginTypeOutput    PluginType = "output"
)

// Factory creates a plugin instance from configuration.
// The returned interface{} must be type-asserted to the specific plugin interface.
type Factory func(cfg map[string]interface{}) (interface{}, error)

// Registry manages plugin provider registration and creation.
type Registry struct {
    factories map[PluginType]map[string]Factory
}

func NewRegistry() *Registry
func (r *Registry) Register(pluginType PluginType, provider string, factory Factory)
func (r *Registry) Create(pluginType PluginType, provider string, cfg map[string]interface{}) (interface{}, error)
func (r *Registry) Providers(pluginType PluginType) []string
```
- `Create` returns `PluginError` if provider not found, listing available providers via `Providers()`
- Registry is NOT a singleton — create in main/bootstrap, pass via DI
- No init() registration — explicit registration in bootstrap code

### Mockery Configuration

Create `.mockery.yaml` at project root:
```yaml
with-expecter: true
dir: "internal/mocks"
outpkg: "mocks"
packages:
  github.com/jay/youtube-pipeline/internal/plugin/llm:
    interfaces:
      LLM:
  github.com/jay/youtube-pipeline/internal/plugin/tts:
    interfaces:
      TTS:
  github.com/jay/youtube-pipeline/internal/plugin/imagegen:
    interfaces:
      ImageGen:
  github.com/jay/youtube-pipeline/internal/plugin/output:
    interfaces:
      Assembler:
```

Add `//go:generate go run github.com/vektra/mockery/v2@latest` in each interface file OR use a single `//go:generate` in `internal/mocks/generate.go`.

**IMPORTANT:** mockery v2 can be run as `go run github.com/vektra/mockery/v2@latest` without global install. This is preferred for reproducible builds. Add to `go.mod` as a tool dependency:
```bash
go get github.com/vektra/mockery/v2@latest
```

### Previous Story Intelligence (Story 1.2)

**Learnings from Story 1.2:**
- Viper uses `viper.New()` instances, not global singleton — same pattern: avoid global state in plugins
- `mapstructure` tags are required on config structs — plugin config will use `map[string]interface{}` from Viper
- `doc.go` placeholder files from Story 1.1 MUST be deleted when real implementations are created
- Config types exist in `internal/config/types.go` — each plugin config has Provider, APIKey, and type-specific fields
- `internal/config/doc.go` was correctly deleted in Story 1.2 — follow same pattern for plugin doc.go files
- Code review caught dead code (unused functions) — ensure no dead code in this story

**Files from Story 1.2 that MUST NOT be modified:**
- `internal/config/config.go` — config loading is stable
- `internal/config/types.go` — config types are stable
- `internal/cli/root.go` — Viper integration is stable
- `internal/cli/config_cmd.go` — config commands are stable
- `internal/domain/*` — domain models are stable
- `internal/store/*` — store implementation is stable

**Files from Story 1.1 to DELETE (placeholders being replaced):**
- `internal/plugin/doc.go` → replaced by `base.go` + `registry.go`
- `internal/plugin/llm/doc.go` → replaced by `interface.go`
- `internal/plugin/tts/doc.go` → replaced by `interface.go`
- `internal/plugin/imagegen/doc.go` → replaced by `interface.go`
- `internal/plugin/output/doc.go` → replaced by `interface.go`
- `internal/retry/doc.go` → replaced by `retry.go`

### Git Intelligence

**Recent commits:**
- `5f6ce0d` — chore: add BMAD project artifacts and planning docs
- `79c29b4` — feat: scaffold project foundation (Story 1.1)

**Uncommitted changes (Story 1.2 work):**
- `config.example.yaml` — updated with full documentation
- `go.mod` / `go.sum` — added viper dependency
- `internal/cli/root.go` — Viper integration
- `internal/config/doc.go` — deleted
- New files: `internal/config/config.go`, `internal/config/types.go` (already committed or staged)
- New files: `internal/cli/config_cmd.go`, config tests

**IMPORTANT:** Story 1.2 changes appear uncommitted. The dev agent should NOT modify these files and should ensure `go test ./...` passes including Story 1.2's tests.

### Testing Strategy

**Unit tests for retry package:**
```go
// internal/retry/retry_test.go
func TestDo_SuccessOnFirstAttempt(t *testing.T)
func TestDo_SuccessAfterRetries(t *testing.T)
func TestDo_ExhaustsMaxAttempts(t *testing.T)
func TestDo_ExponentialBackoff(t *testing.T)        // verify delay increases — CAUTION: do NOT assert exact timing (flaky). Instead, verify delays are within expected range (e.g., ±50ms) or inject a clock/sleep function to make backoff testable without real delays
func TestDo_ContextCancellation(t *testing.T)       // stops immediately
func TestDo_NonRetryableError(t *testing.T)          // stops on 400/401/403
func TestDo_RetryableError(t *testing.T)             // retries on 429/5xx
func TestDo_DefaultRetryable(t *testing.T)           // unknown errors default to retryable
```

**Unit tests for plugin base:**
```go
// internal/plugin/base_test.go
func TestDefaultPluginConfig(t *testing.T)
func TestWithTimeout_Success(t *testing.T)
func TestWithTimeout_Exceeds(t *testing.T)
```

**Unit tests for registry:**
```go
// internal/plugin/registry_test.go
func TestRegistry_RegisterAndCreate(t *testing.T)
func TestRegistry_UnknownProvider(t *testing.T)      // returns PluginError — assert error message contains available provider names
func TestRegistry_Providers(t *testing.T)
func TestRegistry_MultipleTypes(t *testing.T)
```

**Mock usability test:**
```go
// internal/plugin/llm/interface_test.go (or in mocks test)
func TestMock_LLMGenerateScenario(t *testing.T)      // verify mock works with testify
```

**Test helper:** Use `t.Setenv()` for env vars, `t.TempDir()` for temp files.

### Dependency Direction (Import Cycle Prevention)

```
retry/        ← plugin/, service/ (MUST NOT import other internal/ packages)
domain/       ← plugin/, service/, store/, workspace/ (pure data, no external deps)
plugin/       → domain/, retry/ (base.go)
plugin/llm/   → domain/ (interface.go)
plugin/tts/   → domain/ (interface.go)
plugin/imagegen/ → (no domain import needed for Generate)
plugin/output/ → domain/ (interface.go)
mocks/        → domain/, plugin/llm/, plugin/tts/, plugin/imagegen/, plugin/output/
```

**CRITICAL:** `retry/` MUST NOT import any other `internal/` package — it is a leaf dependency.
**CRITICAL:** `plugin/base.go` can import `retry/` but sub-packages (`llm/`, `tts/`, etc.) should NOT import `plugin/` parent package to avoid cycles. Sub-packages only import `domain/`.

### Anti-Patterns (FORBIDDEN)

1. **NO** global plugin registry — create in bootstrap, pass via DI
2. **NO** `init()` in plugin packages — explicit registration only
3. **NO** importing parent `plugin/` package from sub-packages (`llm/`, `tts/`, etc.)
4. **NO** plugin-specific types replacing domain types — use `domain.Scene`, `domain.ScenarioOutput`
5. **NO** hardcoded retry delays — always configurable
6. **NO** `sync.Once` or lazy initialization in plugins — explicit New() constructors
7. **NO** real API calls in any test — use mocks exclusively
8. **NO** Option pattern — use simple PluginConfig struct

### Architecture Compliance Checklist

- [ ] Package names: lowercase single word — `plugin`, `retry`, `llm`, `tts`, `imagegen`, `output`, `mocks`
- [ ] File names: `snake_case.go` — `interface.go`, `base.go`, `registry.go`, `retry.go`
- [ ] Exported types: `PascalCase` — `LLM`, `TTS`, `ImageGen`, `Assembler`, `Registry`
- [ ] Test naming: `Test{Function}_{Scenario}` — `TestDo_SuccessOnFirstAttempt`
- [ ] All interface methods: `context.Context` as first parameter
- [ ] All interface methods: `error` as last return value
- [ ] `retry/` has ZERO imports from other `internal/` packages
- [ ] No import cycles between packages
- [ ] `domain/` types used for interface input/output
- [ ] Mockery generates mocks in `internal/mocks/`
- [ ] Error wrapping: `fmt.Errorf("plugin llm: %w", err)`
- [ ] Constructor pattern: `NewRegistry()` — explicit dependencies
- [ ] All doc.go placeholders deleted when replaced by real files

### Project Structure Notes

- All 6 doc.go placeholder files from Story 1.1 should be deleted and replaced with real implementations
- `internal/mocks/` directory will be populated by mockery auto-generation
- `internal/mocks/.gitkeep` can be removed once mocks are generated
- `.mockery.yaml` goes at project root (same level as `go.mod`)
- No new CLI commands in this story — this is purely internal infrastructure

### References

- [Source: _bmad-output/planning-artifacts/architecture.md#Plugin Structure — plugin/{type}/interface.go pattern]
- [Source: _bmad-output/planning-artifacts/architecture.md#Implementation Patterns — retry helper, context propagation]
- [Source: _bmad-output/planning-artifacts/architecture.md#Cross-Cutting Concerns — dependency inversion, NFR23]
- [Source: _bmad-output/planning-artifacts/architecture.md#Anti-Patterns — no init(), no Option pattern, no global vars]
- [Source: _bmad-output/planning-artifacts/architecture.md#Selected Stack — testify + mockery for testing]
- [Source: _bmad-output/planning-artifacts/architecture.md#Architectural Boundaries — dependency direction]
- [Source: _bmad-output/planning-artifacts/epics.md#Story 1.3 — Acceptance Criteria]
- [Source: _bmad-output/planning-artifacts/prd.md#NFR9 — Standardized plugin interfaces]
- [Source: _bmad-output/planning-artifacts/prd.md#NFR10 — External API timeout default 120s]
- [Source: _bmad-output/planning-artifacts/prd.md#NFR23 — Test substitutes for plugins]
- [Source: _bmad-output/planning-artifacts/prd.md#NFR6 — Max 3 retries with progressive delay]
- [Source: _bmad-output/implementation-artifacts/1-2-configuration-management-system.md — Previous story learnings]

## Change Log

- 2026-03-08: Implemented Story 1.3 Plugin Interface Framework — all 9 tasks complete, 4 plugin interfaces defined, retry helper, plugin base, registry, mockery mocks generated, all tests pass.

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (claude-opus-4-6)

### Debug Log References

No issues encountered during implementation.

### Completion Notes List

- **Task 1 (Retry Helper):** Implemented `Do()` with exponential backoff (baseDelay * 2^attempt), jitter (0-25%), `RetryableError` interface. Unknown errors default to retryable. Context cancellation stops immediately. 11 unit tests pass.
- **Task 2 (Plugin Base):** Implemented `PluginConfig` (Name, Timeout=120s, MaxRetries=3, BaseDelay=1s), `DefaultPluginConfig()`, `WithTimeout()`. 4 unit tests pass.
- **Task 3 (LLM Interface):** Defined `LLM` interface with `GenerateScenario` and `RegenerateSection` methods using domain types. go:generate directive added.
- **Task 4 (TTS Interface):** Defined `TTS` interface with `Synthesize` and `SynthesizeWithOverrides` methods. `SynthesisResult` struct includes AudioData, WordTimings, DurationSec.
- **Task 5 (ImageGen Interface):** Defined `ImageGen` interface with `Generate` method. `ImageResult` and `GenerateOptions` structs defined. No `GenerateBatch` per architecture (service layer responsibility).
- **Task 6 (OutputAssembler Interface):** Defined `Assembler` interface with `Assemble` and `Validate` methods. `AssembleInput` struct bundles Project, Scenes, OutputDir.
- **Task 7 (Plugin Registry):** Implemented `Registry` with `Register`, `Create`, `Providers` methods. `PluginError` returned for unknown providers with available providers listed. Thread-safe with sync.RWMutex. 8 unit tests pass.
- **Task 8 (Mockery):** Created `.mockery.yaml` config, generated mocks for all 4 interfaces in `internal/mocks/`. Mock usability test verifies LLM mock with expecter pattern.
- **Task 9 (Verification):** `go build ./...` zero errors, `go test ./...` all pass (including Story 1.1/1.2), `go vet ./...` zero warnings, mocks regenerate cleanly, no import cycles.

### File List

**New files:**
- `internal/retry/retry.go` — Retry helper with exponential backoff
- `internal/retry/retry_test.go` — 11 unit tests for retry
- `internal/plugin/base.go` — PluginConfig, DefaultPluginConfig, WithTimeout
- `internal/plugin/base_test.go` — 4 unit tests for plugin base
- `internal/plugin/registry.go` — Plugin registry with provider mapping
- `internal/plugin/registry_test.go` — 8 unit tests for registry
- `internal/plugin/llm/interface.go` — LLM plugin interface
- `internal/plugin/llm/interface_test.go` — Compile-time interface check
- `internal/plugin/tts/interface.go` — TTS plugin interface
- `internal/plugin/tts/interface_test.go` — Compile-time interface check
- `internal/plugin/imagegen/interface.go` — ImageGen plugin interface
- `internal/plugin/imagegen/interface_test.go` — Compile-time interface check
- `internal/plugin/output/interface.go` — Assembler plugin interface
- `internal/plugin/output/interface_test.go` — Compile-time interface check
- `internal/mocks/mock_LLM.go` — Auto-generated mock (mockery)
- `internal/mocks/mock_TTS.go` — Auto-generated mock (mockery)
- `internal/mocks/mock_ImageGen.go` — Auto-generated mock (mockery)
- `internal/mocks/mock_Assembler.go` — Auto-generated mock (mockery)
- `internal/mocks/mock_test.go` — Mock usability test
- `.mockery.yaml` — Mockery configuration

**Deleted files:**
- `internal/retry/doc.go` — Placeholder replaced by retry.go
- `internal/plugin/doc.go` — Placeholder replaced by base.go + registry.go
- `internal/plugin/llm/doc.go` — Placeholder replaced by interface.go
- `internal/plugin/tts/doc.go` — Placeholder replaced by interface.go
- `internal/plugin/imagegen/doc.go` — Placeholder replaced by interface.go
- `internal/plugin/output/doc.go` — Placeholder replaced by interface.go
- `internal/mocks/.gitkeep` — Removed (mocks now generated)

**Modified files:**
- `go.sum` — Updated with mockery transitive dependencies
