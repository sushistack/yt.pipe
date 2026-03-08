# Story 1.4: Initial Setup Wizard

Status: review

## Story

As a creator,
I want a guided setup wizard that configures API keys, data paths, and default profiles,
So that I can get the pipeline running quickly without manually editing config files.

## Acceptance Criteria

1. **Interactive Setup Wizard** (AC:1)
   - Given the creator runs `yt-pipe init`
   - When the wizard starts
   - Then it prompts step-by-step for: LLM API key, SiliconFlow API key, TTS provider selection + API key, SCP data directory path, project workspace path
   - And each input is validated before proceeding to the next step
   - And this satisfies FR31

2. **API Key Validation** (AC:2)
   - Given an API key is entered during setup
   - When the wizard validates it
   - Then a lightweight validation request is sent to the corresponding API endpoint
   - And success or failure is clearly displayed with actionable error messages
   - And this satisfies FR32

3. **Config File Generation** (AC:3)
   - Given setup is complete
   - When the wizard finishes
   - Then a global config file is written to `$HOME/.yt-pipe/config.yaml` with all configured values
   - And API keys are stored as references to environment variable names (not plaintext) with instructions to set them
   - And the wizard displays a summary of configured settings and suggests running a test command

4. **Plugin Swap via Config** (AC:4)
   - Given a creator wants to change the image generation plugin
   - When they edit the YAML config `imagegen.provider` field
   - Then the plugin is swapped on next pipeline execution without code changes
   - And this satisfies FR33

## Tasks / Subtasks

- [x] Task 1: Create init command scaffold (AC: #1)
  - [x] 1.1 Create `internal/cli/init_cmd.go` with `yt-pipe init` Cobra command
  - [x] 1.2 Register `initCmd` in rootCmd via `init()` function
  - [x] 1.3 Add `--force` flag to allow re-running init on existing config
  - [x] 1.4 Add `--non-interactive` flag for CI/scripted setup (reads from flags/env only)

- [x] Task 2: Implement interactive prompt utilities (AC: #1)
  - [x] 2.1 Create `internal/cli/prompt.go` with reusable prompt helpers
  - [x] 2.2 Implement `promptString(reader, writer, label, defaultVal) (string, error)` — reads line from stdin
  - [x] 2.3 Implement `promptSecret(reader, writer, label) (string, error)` — reads without echo (for API keys)
  - [x] 2.4 Implement `promptSelect(reader, writer, label, options, defaultIdx) (string, error)` — numbered selection
  - [x] 2.5 Implement `promptConfirm(reader, writer, label, defaultYes) (bool, error)` — y/n prompt
  - [x] 2.6 Write unit tests in `internal/cli/prompt_test.go` using `bytes.Buffer` for stdin/stdout

- [x] Task 3: Implement setup wizard flow (AC: #1, #3)
  - [x] 3.1 Implement wizard step sequence in `init_cmd.go`:
    1. Welcome message and explanation
    2. SCP data directory path (validate directory exists or offer to create)
    3. Project workspace path (validate or create)
    4. LLM provider selection (openai) + API key
    5. ImageGen provider selection (siliconflow) + API key
    6. TTS provider selection (openai/google/edge) + API key (skip for edge)
    7. Output provider (capcut, default — auto-selected)
  - [x] 3.2 Validate each path input: check directory exists, or prompt to create it
  - [x] 3.3 Allow skipping optional fields (API keys can be set via env vars later)
  - [x] 3.4 Write unit tests for wizard flow using mock reader/writer

- [x] Task 4: Implement API key validation (AC: #2)
  - [x] 4.1 Create `internal/cli/validate_api.go` with validation functions
  - [x] 4.2 Implement `validateLLMKey(ctx, provider, apiKey) error` — sends minimal completion request to OpenAI-compatible endpoint
  - [x] 4.3 Implement `validateImageGenKey(ctx, provider, apiKey) error` — sends lightweight request to SiliconFlow API
  - [x] 4.4 Implement `validateTTSKey(ctx, provider, apiKey) error` — sends minimal TTS request (or list voices)
  - [x] 4.5 Use `context.WithTimeout` for all validation requests (10s timeout)
  - [x] 4.6 Display clear success/failure messages with actionable guidance on failure
  - [x] 4.7 Make validation optional — user can skip with "skip validation" option
  - [x] 4.8 Write unit tests using `httptest.NewServer` for mock API endpoints

- [x] Task 5: Implement config file generation (AC: #3)
  - [x] 5.1 Create `$HOME/.yt-pipe/` directory if it doesn't exist (mode 0700)
  - [x] 5.2 Generate `config.yaml` with all configured values
  - [x] 5.3 Store API keys as comments with env var instructions, NOT as plaintext values
  - [x] 5.4 Include descriptive comments for each section (reuse format from config.example.yaml)
  - [x] 5.5 If `--force` not set and config exists, prompt for overwrite confirmation
  - [x] 5.6 Write unit tests for config generation using `t.TempDir()`

- [x] Task 6: Implement setup summary and next steps (AC: #3)
  - [x] 6.1 Display formatted summary of all configured settings (secrets masked)
  - [x] 6.2 Show shell export commands for API keys (e.g., `export YTP_LLM_API_KEY="sk-..."`)
  - [x] 6.3 Suggest adding exports to shell profile (`.bashrc`/`.zshrc`)
  - [x] 6.4 Suggest running `yt-pipe config validate` as next step
  - [x] 6.5 Suggest running `yt-pipe config show` to verify merged config

- [x] Task 7: Implement non-interactive mode (AC: #1)
  - [x] 7.1 When `--non-interactive` flag is set, read all values from flags and environment variables
  - [x] 7.2 Add flags for all init values: `--scp-data-path`, `--workspace-path`, `--llm-api-key`, `--imagegen-api-key`, `--tts-provider`, `--tts-api-key`
  - [x] 7.3 Use existing config defaults for unset values
  - [x] 7.4 Skip API key validation in non-interactive mode (can run `config validate` separately)
  - [x] 7.5 Write unit tests for non-interactive flow

- [x] Task 8: Verify plugin swap via config (AC: #4)
  - [x] 8.1 Verify that changing `imagegen.provider` in generated YAML is recognized by config.Load()
  - [x] 8.2 Verify that changing `tts.provider` in generated YAML is recognized by config.Load()
  - [x] 8.3 Write integration test: generate config → modify provider → load config → assert new provider

- [x] Task 9: Final verification
  - [x] 9.1 `go build ./...` — zero errors
  - [x] 9.2 `go test ./...` — all tests pass (including Story 1.1, 1.2, 1.3 tests)
  - [x] 9.3 `go vet ./...` — zero warnings
  - [x] 9.4 Manual test: `go run ./cmd/yt-pipe init` — interactive wizard runs end-to-end
  - [x] 9.5 Manual test: `go run ./cmd/yt-pipe init --non-interactive --scp-data-path /tmp/scp` — non-interactive mode works
  - [x] 9.6 Verify generated config at `$HOME/.yt-pipe/config.yaml` is valid YAML
  - [x] 9.7 Verify `yt-pipe config validate` passes after init

## Dev Notes

### Critical Architecture Constraints

**CLI Structure (Architecture-mandated):**
- All CLI commands live in `internal/cli/`
- `init_cmd.go` is explicitly listed in the architecture's file structure
- Command registered via `init()` function (Cobra's ONLY allowed `init()` exception)
- No business logic in CLI commands — parsing + service calls + output formatting only

**Config File Location (Architecture-mandated):**
- Global config: `$HOME/.yt-pipe/config.yaml`
- Viper 5-level priority chain: CLI flags > env vars (YTP_) > project YAML > global YAML > defaults
- Config loading already implemented in `internal/config/config.go`

**API Key Storage Security:**
- API keys MUST NOT be stored as plaintext in config files
- Store as comments with env var instructions:
  ```yaml
  llm:
    provider: "openai"
    # API key — set via environment variable:
    #   export YTP_LLM_API_KEY="your-key-here"
    # api_key: ""
  ```
- The wizard should output `export` commands to stdout for the user to add to their shell profile

**Anti-Patterns (FORBIDDEN):**
1. **NO** storing API keys as plaintext in generated config
2. **NO** `os.Exit()` or `log.Fatal()` in init command — return errors to Cobra
3. **NO** global state for prompt utilities — pass reader/writer explicitly
4. **NO** importing `service/` or `store/` from CLI init — this is a pure CLI command
5. **NO** Option pattern — use simple function parameters

### Prompt Utility Design

```go
// internal/cli/prompt.go
package cli

import (
    "bufio"
    "fmt"
    "io"
    "strings"
)

// Prompter provides reusable interactive prompt functions.
// reader/writer are injected for testability (default: os.Stdin, os.Stdout).

func promptString(reader io.Reader, writer io.Writer, label string, defaultVal string) (string, error)
func promptSecret(reader io.Reader, writer io.Writer, label string) (string, error)
func promptSelect(reader io.Reader, writer io.Writer, label string, options []string, defaultIdx int) (string, error)
func promptConfirm(reader io.Reader, writer io.Writer, label string, defaultYes bool) (bool, error)
```

**Testing strategy:** All prompt functions accept `io.Reader`/`io.Writer` so tests use `bytes.Buffer` / `strings.NewReader` for deterministic input simulation.

**NOTE on `promptSecret`:** On real terminals, use `golang.org/x/term.ReadPassword()` for suppressing echo. However, for MVP and testability, a simple line read is acceptable — the security concern is about config file storage, not terminal echo. If `golang.org/x/term` is used, add it as a dependency. Dev's judgment call — either approach is acceptable.

### API Key Validation Design

```go
// internal/cli/validate_api.go
package cli

import (
    "context"
    "fmt"
    "net/http"
    "time"
)

// validationTimeout is the default timeout for API key validation requests.
const validationTimeout = 10 * time.Second

func validateLLMKey(ctx context.Context, provider, apiKey string) error
func validateImageGenKey(ctx context.Context, provider, apiKey string) error
func validateTTSKey(ctx context.Context, provider, apiKey string) error
```

**Validation strategies by provider:**

| Provider | Validation Method | Expected Response |
|----------|------------------|-------------------|
| OpenAI LLM | `GET https://api.openai.com/v1/models` with Bearer token | 200 = valid, 401 = invalid |
| SiliconFlow | `GET https://api.siliconflow.cn/v1/models` with Bearer token | 200 = valid, 401 = invalid |
| OpenAI TTS | `GET https://api.openai.com/v1/models` with Bearer token | 200 = valid, 401 = invalid |
| Google TTS | Google Cloud credentials validation | Implementation varies |
| Edge TTS | No API key needed — skip validation | Always valid |

**IMPORTANT:** Use `net/http` standard library for validation. Do NOT import plugin packages — validation is a CLI-level concern, not a plugin concern. The validation requests are simple auth checks, not full plugin operations.

**Error messages should be actionable:**
```
✗ LLM API key validation failed: 401 Unauthorized
  → Check your API key at https://platform.openai.com/api-keys
  → You can skip validation and set the key later via: export YTP_LLM_API_KEY="..."
```

### Config Generation Template

The generated config should follow the same format as `config.example.yaml` but with user-provided values:

```yaml
# youtube.pipeline configuration
# Generated by yt-pipe init on 2026-03-08
#
# Configuration Priority (highest to lowest):
#   1. CLI flags (--config, --verbose, --json-output)
#   2. Environment variables (YTP_ prefix)
#   3. Project config (./config.yaml in working directory)
#   4. Global config ($HOME/.yt-pipe/config.yaml) ← this file
#   5. Built-in defaults

# Data Paths
scp_data_path: "/path/user/entered"
workspace_path: "/path/user/entered"

# LLM Plugin
llm:
  provider: "openai"
  # API key — set via environment variable:
  #   export YTP_LLM_API_KEY="your-key-here"
  # api_key: ""

# Image Generation Plugin
imagegen:
  provider: "siliconflow"
  # API key — set via environment variable:
  #   export YTP_IMAGEGEN_API_KEY="your-key-here"
  # api_key: ""

# TTS Plugin
tts:
  provider: "openai"
  # API key — set via environment variable:
  #   export YTP_TTS_API_KEY="your-key-here"
  # api_key: ""

# Output Assembler
output:
  provider: "capcut"
```

### Init Command Wizard Flow

```
$ yt-pipe init

Welcome to youtube.pipeline setup!
This wizard will configure your pipeline settings.

Step 1/5: Data Paths
  SCP data directory [/data/raw]: /home/user/scp-data
  ✓ Directory exists
  Project workspace [/data/projects]: /home/user/projects
  ✓ Directory created

Step 2/5: LLM Configuration
  Provider [openai]: openai
  API Key: sk-proj-...
  Validating... ✓ API key valid

Step 3/5: Image Generation
  Provider [siliconflow]: siliconflow
  API Key: sf-...
  Validating... ✓ API key valid

Step 4/5: Text-to-Speech
  Provider (1. openai, 2. google, 3. edge): 1
  API Key: sk-proj-...
  Validating... ✓ API key valid

Step 5/5: Output Format
  Provider [capcut]: capcut

✓ Configuration saved to /home/user/.yt-pipe/config.yaml

To set your API keys, add these to your shell profile (~/.bashrc or ~/.zshrc):
  export YTP_LLM_API_KEY="sk-proj-..."
  export YTP_IMAGEGEN_API_KEY="sf-..."
  export YTP_TTS_API_KEY="sk-proj-..."

Next steps:
  1. Add the export commands above to your shell profile
  2. Run: source ~/.bashrc  (or restart your terminal)
  3. Run: yt-pipe config validate  (verify configuration)
  4. Run: yt-pipe config show  (view merged config)
```

### Previous Story Intelligence (Stories 1.2 and 1.3)

**Learnings from Story 1.2 (Configuration Management):**
- Viper uses `viper.New()` instances, not global singleton — init command should NOT use viper directly for config generation; instead write YAML manually or use `gopkg.in/yaml.v3`
- `mapstructure` tags on config structs — generated YAML keys must match these tags exactly
- `config.Load()` returns `*LoadResult` with Config and Sources — init can use this to check existing config
- Config validation via `config.Validate()` — suggest running this after init
- `MaskSecrets()` for display — use when showing summary
- Global config path: `$HOME/.yt-pipe/config.yaml`
- `mergeConfigFile()` silently ignores missing files — init creates the file that was previously missing

**Learnings from Story 1.3 (Plugin Interface Framework):**
- Plugin registry maps provider names to factories — init collects provider choices that will be used later
- Available TTS providers: openai, google, edge (from architecture)
- Available ImageGen providers: siliconflow (from architecture)
- Available LLM providers: openai (from architecture)
- Available Output providers: capcut (from architecture)
- `domain.PluginError` lists available providers — init wizard should show the same options

**Files from Stories 1.2/1.3 that MUST NOT be modified:**
- `internal/config/config.go` — config loading is stable
- `internal/config/types.go` — config types are stable
- `internal/config/config_test.go` — config tests are stable
- `internal/config/types_test.go` — config type tests are stable
- `internal/cli/config_cmd.go` — config commands are stable
- `internal/plugin/*` — plugin interfaces are stable
- `internal/retry/*` — retry helper is stable
- `internal/domain/*` — domain models are stable
- `internal/store/*` — store implementation is stable

**Files from this story that modify existing code:**
- `internal/cli/root.go` — NO changes needed (init_cmd registers via `init()`)

### Git Intelligence

**Recent commits:**
- `5f6ce0d` — chore: add BMAD project artifacts and planning docs
- `79c29b4` — feat: scaffold project foundation (Story 1.1)

**Uncommitted changes:** Stories 1.2 and 1.3 work (config management + plugin interfaces). All tests pass.

**Conventions:**
- Commit messages: `feat:`, `chore:` prefix style
- All code in `internal/` (Go convention for non-exported packages)

### Testing Strategy

**Unit tests for prompt utilities:**
```go
// internal/cli/prompt_test.go
func TestPromptString_WithDefault(t *testing.T)        // empty input returns default
func TestPromptString_WithInput(t *testing.T)           // user input overrides default
func TestPromptString_TrimWhitespace(t *testing.T)      // leading/trailing whitespace trimmed
func TestPromptSecret_ReadsLine(t *testing.T)           // reads secret value
func TestPromptSelect_ValidChoice(t *testing.T)         // numeric selection
func TestPromptSelect_InvalidChoice(t *testing.T)       // out of range re-prompts
func TestPromptSelect_Default(t *testing.T)             // empty input uses default
func TestPromptConfirm_Yes(t *testing.T)
func TestPromptConfirm_No(t *testing.T)
func TestPromptConfirm_Default(t *testing.T)
```

**Unit tests for API validation:**
```go
// internal/cli/validate_api_test.go
func TestValidateLLMKey_Success(t *testing.T)           // mock 200 response
func TestValidateLLMKey_Unauthorized(t *testing.T)      // mock 401 response
func TestValidateLLMKey_Timeout(t *testing.T)           // context timeout
func TestValidateImageGenKey_Success(t *testing.T)
func TestValidateImageGenKey_Unauthorized(t *testing.T)
func TestValidateTTSKey_EdgeSkipsValidation(t *testing.T) // edge provider skips
func TestValidateTTSKey_OpenAI_Success(t *testing.T)
```

**Unit tests for config generation:**
```go
// internal/cli/init_cmd_test.go
func TestGenerateConfig_CreatesDirectory(t *testing.T)  // creates $HOME/.yt-pipe/
func TestGenerateConfig_WritesValidYAML(t *testing.T)   // output is parseable YAML
func TestGenerateConfig_NoPlaintextKeys(t *testing.T)   // API keys not stored as values
func TestGenerateConfig_ForceOverwrite(t *testing.T)    // --force overwrites existing
func TestGenerateConfig_NoForcePrompts(t *testing.T)    // prompts when file exists
func TestInitWizard_FullFlow(t *testing.T)              // end-to-end wizard with mock I/O
func TestInitWizard_NonInteractive(t *testing.T)        // non-interactive flag flow
func TestPluginSwap_ConfigReload(t *testing.T)          // modify provider → reload → assert
```

**Test helper:** Use `t.TempDir()` for config directory, `bytes.Buffer` for stdin/stdout simulation.

### Dependency Direction (Import Cycle Prevention)

```
cli/init_cmd.go → config/ (for Load, Validate, MaskSecrets, Config type)
cli/prompt.go   → (no internal imports — uses io, bufio, strings only)
cli/validate_api.go → (no internal imports — uses net/http, context only)
```

**CRITICAL:** init command does NOT import `plugin/`, `service/`, `store/`, or `domain/`. It is purely a CLI command that generates a YAML config file and validates API keys via HTTP.

### New Dependencies

**Possible new dependency:**
- `golang.org/x/term` — for `ReadPassword()` to suppress echo during API key input
  - Alternative: simple line read (acceptable for MVP, keys are not stored in config anyway)
  - Dev's choice — if adding, use `go get golang.org/x/term@latest`

**No other new dependencies required.** YAML generation can use `fmt.Fprintf` with template strings (simpler than importing a YAML library).

### Architecture Compliance Checklist

- [ ] Package name: `cli` (existing package)
- [ ] File names: `snake_case.go` — `init_cmd.go`, `prompt.go`, `validate_api.go`
- [ ] Test naming: `Test{Function}_{Scenario}` — `TestPromptString_WithDefault`
- [ ] No `domain/`, `plugin/`, `service/`, `store/` imports from init command
- [ ] No `os.Exit()` or `log.Fatal()` — return errors to Cobra
- [ ] API keys NOT stored as plaintext in generated config
- [ ] Constructor pattern: prompt functions accept `io.Reader`/`io.Writer` for testability
- [ ] Error wrapping: `fmt.Errorf("init wizard: %w", err)`
- [ ] Context with timeout for API validation requests
- [ ] Generated YAML keys match `mapstructure` tags in `config/types.go`

### Project Structure Notes

- `init_cmd.go` is listed in the architecture but was not yet created
- Prompt utilities in `prompt.go` are CLI-specific, not reusable outside CLI
- API validation in `validate_api.go` is separate from plugin validation — it's a lightweight auth check, not a plugin operation
- No changes to `root.go` needed — init_cmd registers itself via `init()`

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Story 1.4 — Acceptance Criteria, user story]
- [Source: _bmad-output/planning-artifacts/prd.md#FR31 — Initial setup wizard: API keys, data paths, profiles]
- [Source: _bmad-output/planning-artifacts/prd.md#FR32 — API key validity validation]
- [Source: _bmad-output/planning-artifacts/prd.md#FR33 — Plugin swap via YAML config]
- [Source: _bmad-output/planning-artifacts/architecture.md#Code Organization — init_cmd.go in CLI structure]
- [Source: _bmad-output/planning-artifacts/architecture.md#Cross-Cutting Concerns — Setting Priority Chain]
- [Source: _bmad-output/planning-artifacts/architecture.md#Anti-Patterns — no init(), no Option pattern, no global vars]
- [Source: _bmad-output/planning-artifacts/architecture.md#Architectural Boundaries — CLI depends only on service]
- [Source: _bmad-output/implementation-artifacts/1-2-configuration-management-system.md — Config loading, Viper patterns, types]
- [Source: _bmad-output/implementation-artifacts/1-3-plugin-interface-framework.md — Plugin registry, provider names]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (claude-opus-4-6)

### Debug Log References

None — no blocking issues encountered.

### Completion Notes List

- Task 1: Created `init_cmd.go` with Cobra `init` command, `--force` and `--non-interactive` flags, plus non-interactive value flags
- Task 2: Created `prompt.go` with 4 reusable prompt functions (`promptString`, `promptSecret`, `promptSelect`, `promptConfirm`) using `io.Reader`/`io.Writer` for testability; 16 unit tests in `prompt_test.go`
- Task 3: Implemented full 7-step interactive wizard flow with path validation/creation, optional API key validation, and `lineReader` wrapper to solve bufio over-buffering
- Task 4: Created `validate_api.go` with bearer token validation for OpenAI, SiliconFlow, and TTS providers; edge/google skip; 16 tests using `httptest.NewServer`
- Task 5: Implemented `generateConfig` writing YAML with API keys as comments only (never plaintext), directory permissions 0700, file permissions 0600
- Task 6: Implemented `displaySummary` with masked API keys, export commands, shell profile suggestions, and next-step guidance
- Task 7: Implemented `runWizardNonInteractive` reading from flags with provider defaults, skipping validation; 4 unit tests
- Task 8: 3 integration tests verifying plugin swap via config reload (`imagegen.provider`, `tts.provider`, combined)
- Task 9: All automated checks pass (build, test, vet); manual tests verified interactive wizard, non-interactive mode, YAML validity, and config validate

### Change Log

- 2026-03-08: Implemented Story 1.4 — Initial Setup Wizard (all 9 tasks, all 4 ACs satisfied)

### File List

New files:
- internal/cli/init_cmd.go
- internal/cli/init_cmd_test.go
- internal/cli/prompt.go
- internal/cli/prompt_test.go
- internal/cli/validate_api.go
- internal/cli/validate_api_test.go

No existing files modified (init_cmd registers via init(), no changes to root.go needed).
