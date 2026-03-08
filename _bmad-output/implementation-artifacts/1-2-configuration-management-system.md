# Story 1.2: Configuration Management System

Status: done

## Story

As a creator,
I want a layered configuration system that merges settings from multiple sources with clear priority,
So that I can customize the pipeline at global, project, or command level without conflicts.

## Acceptance Criteria

1. **5-Level Priority Chain** (AC:1)
   - Given Viper is integrated with the config package
   - When configuration is loaded
   - Then the 5-level priority chain is applied: CLI flags > environment variables (`YTP_` prefix) > project YAML (`./config.yaml`) > global YAML (`$HOME/.yt-pipe/config.yaml`) > built-in defaults
   - And this satisfies FR35

2. **Global/Project Config Merging** (AC:2)
   - Given a global config file exists at `$HOME/.yt-pipe/config.yaml`
   - When a project-level `config.yaml` overrides specific keys
   - Then only the overridden keys use project values; all other keys fall back to global config
   - And this satisfies FR34

3. **Structured Config Types** (AC:3)
   - Given the config types are defined
   - When configuration is loaded
   - Then structured types exist for: LLM plugin settings, TTS plugin settings, ImageGen plugin settings, OutputAssembler settings, SCP data path, project workspace path, API server settings, glossary path, logging settings
   - And environment variables like `YTP_LLM_API_KEY`, `YTP_SILICONFLOW_KEY`, `YTP_TTS_API_KEY`, `YTP_API_KEY` are mapped to corresponding config fields

4. **Config Example Documentation** (AC:4)
   - Given `config.example.yaml` is updated
   - When a new user copies it
   - Then all configurable fields are documented with comments explaining each option and its default value

5. **CLI Config Commands** (AC:5)
   - Given the `config show` and `config validate` CLI subcommands are implemented
   - When `yt-pipe config show` is run
   - Then the merged configuration (with secrets masked) is displayed in YAML format
   - And each value includes a comment indicating its source (default, global, project, env, flag) for debugging
   - When `yt-pipe config validate` is run
   - Then required fields are validated as non-empty, port range is validated (1-65535), and path existence is reported as warnings (not errors, since paths may not exist before `yt-pipe init`)

6. **Viper-Cobra Integration** (AC:6)
   - Given the root command has global flags (`--config`, `--verbose`, `--json-output`)
   - When these flags are used
   - Then Viper binds them so CLI flags override all other config sources
   - And the `--config` flag allows specifying a custom config file path

## Tasks / Subtasks

- [x] Task 1: Add Viper dependency to go.mod (AC: #1, #6)
  - [x] 1.1 Run `go get github.com/spf13/viper@latest`
  - [x] 1.2 Run `go mod tidy`
  - [x] 1.3 Verify `go build ./...` still compiles

- [x] Task 2: Create config types (AC: #3)
  - [x] 2.1 Create `internal/config/types.go` with all structured config types
  - [x] 2.2 Write unit tests for default values in `internal/config/types_test.go`

- [x] Task 3: Implement config loading with 5-level priority (AC: #1, #2)
  - [x] 3.1 Create `internal/config/config.go` — replace doc.go placeholder (DELETE doc.go after)
  - [x] 3.2 Implement `Load(configPath string) (*Config, error)` with Viper 5-level chain
  - [x] 3.3 Implement environment variable binding with `YTP_` prefix
  - [x] 3.4 Implement global config discovery at `$HOME/.yt-pipe/config.yaml`
  - [x] 3.5 Implement project-level config discovery at `./config.yaml`
  - [x] 3.6 Register built-in defaults for all config fields
  - [x] 3.7 Write unit tests for priority chain (all 5 levels) in `internal/config/config_test.go`
  - [x] 3.8 Write unit tests for config merging (global + project override)
  - [x] 3.9 Write unit tests for environment variable mapping

- [x] Task 4: Integrate Viper with Cobra root command (AC: #6)
  - [x] 4.1 Update `internal/cli/root.go` — bind Viper to Cobra persistent flags
  - [x] 4.2 Add `initConfig()` function called via `cobra.OnInitialize`
  - [x] 4.3 Wire `--config` flag to Viper config file path
  - [x] 4.4 Wire `--verbose` flag to log level override
  - [x] 4.5 Verify flag values override all other config sources

- [x] Task 5: Implement config CLI subcommands (AC: #5)
  - [x] 5.1 Create `internal/cli/config_cmd.go` — `config show` and `config validate` subcommands
  - [x] 5.2 `config show` prints merged config as YAML with secrets masked (API keys → `***`)
  - [x] 5.3 `config validate` checks: required fields non-empty, port range valid, paths existence as warnings (not errors — paths may not exist before `yt-pipe init`)
  - [x] 5.4 Write tests for secret masking logic (mask any field named `api_key` or containing `key`/`secret` — pattern-based, not hardcoded list)

- [x] Task 6: Update config.example.yaml (AC: #4)
  - [x] 6.1 Update `config.example.yaml` with comprehensive documentation comments for every field
  - [x] 6.2 Include all default values and environment variable mappings in comments

- [x] Task 7: Final verification
  - [x] 7.1 `go build ./...` — zero errors
  - [x] 7.2 `go test ./...` — all tests pass (including Story 1.1 tests)
  - [x] 7.3 `go vet ./...` — zero warnings
  - [x] 7.4 `go run ./cmd/yt-pipe config show` — displays config
  - [x] 7.5 `go run ./cmd/yt-pipe config validate` — validates config

## Dev Notes

### Critical Architecture Constraints

**Viper 5-Level Priority Chain (Architecture-mandated):**
```
CLI flags > env vars (YTP_ prefix) > project YAML (./config.yaml) > global YAML ($HOME/.yt-pipe/config.yaml) > built-in defaults
```

**Viper Implementation Pattern:**
```go
func Load(configPath string) (*Config, error) {
    v := viper.New()

    // 1. Built-in defaults
    v.SetDefault("scp_data_path", "/data/raw")
    v.SetDefault("workspace_path", "/data/projects")
    v.SetDefault("db_path", "/data/db/yt-pipe.db")
    v.SetDefault("api.host", "localhost")
    v.SetDefault("api.port", 8080)
    v.SetDefault("log_level", "info")
    v.SetDefault("log_format", "json")
    // ... all defaults

    // 2. Global config: $HOME/.yt-pipe/config.yaml
    home, _ := os.UserHomeDir()
    globalPath := filepath.Join(home, ".yt-pipe", "config.yaml")
    // Read global config first (lowest file priority)

    // 3. Project config: ./config.yaml
    // Merge on top of global

    // 4. Environment variables: YTP_ prefix
    v.SetEnvPrefix("YTP")
    v.AutomaticEnv()
    v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

    // 5. CLI flags bound via BindPFlag (highest priority)
    // Handled by Cobra integration in root.go

    // If explicit --config path provided, use that instead of discovery
    if configPath != "" {
        v.SetConfigFile(configPath)
    }

    var cfg Config
    err := v.Unmarshal(&cfg)
    return &cfg, err
}
```

**IMPORTANT: Viper merging behavior for nested keys:**
- Viper's `MergeConfigMap` properly handles nested YAML keys
- When project config sets `llm.provider: "anthropic"`, only that key is overridden
- Global `llm.model`, `llm.temperature` etc. are preserved
- Use `v.MergeInConfig()` after each config source read (NOT `v.ReadInConfig()` which replaces)

**CRITICAL: MergeInConfig error handling for missing files:**
Global and project config files may not exist (first run, no global config yet). Handle gracefully:
```go
v.SetConfigFile(globalPath)
if err := v.MergeInConfig(); err != nil {
    if _, ok := err.(viper.ConfigFileNotFoundError); ok {
        // File not found — silently ignore, fall through to defaults
    } else if os.IsNotExist(err) {
        // File not found (os-level) — silently ignore
    } else {
        // Actual parse error — return to caller
        return nil, fmt.Errorf("config load global: %w", err)
    }
}
```
Apply this same pattern for both global and project config reads. Parse errors MUST be reported; missing files MUST be silently ignored.

**Environment Variable Mapping:**
```
YTP_SCP_DATA_PATH     → scp_data_path
YTP_WORKSPACE_PATH    → workspace_path
YTP_DB_PATH           → db_path
YTP_API_HOST          → api.host
YTP_API_PORT          → api.port
YTP_API_KEY           → api.api_key
YTP_LLM_PROVIDER      → llm.provider
YTP_LLM_API_KEY       → llm.api_key
YTP_LLM_MODEL         → llm.model
YTP_IMAGEGEN_PROVIDER  → imagegen.provider
YTP_SILICONFLOW_KEY   → imagegen.api_key
YTP_TTS_PROVIDER      → tts.provider
YTP_TTS_API_KEY       → tts.api_key
YTP_OUTPUT_PROVIDER   → output.provider
YTP_GLOSSARY_PATH     → glossary_path
YTP_LOG_LEVEL         → log_level
YTP_LOG_FORMAT        → log_format
```

**NOTE on `YTP_SILICONFLOW_KEY`:** The PRD/epics reference this specific env var name for the image generation API key. Viper's `AutomaticEnv` with `SetEnvKeyReplacer` maps `YTP_IMAGEGEN_API_KEY` to `imagegen.api_key`. For backward compatibility, explicitly bind `YTP_SILICONFLOW_KEY` as an alias:
```go
v.BindEnv("imagegen.api_key", "YTP_IMAGEGEN_API_KEY", "YTP_SILICONFLOW_KEY")
```

### Config Types Specification

```go
// internal/config/types.go

type Config struct {
    SCPDataPath   string        `mapstructure:"scp_data_path"`
    WorkspacePath string        `mapstructure:"workspace_path"`
    DBPath        string        `mapstructure:"db_path"`
    API           APIConfig     `mapstructure:"api"`
    LLM           LLMConfig     `mapstructure:"llm"`
    ImageGen      ImageGenConfig `mapstructure:"imagegen"`
    TTS           TTSConfig     `mapstructure:"tts"`
    Output        OutputConfig  `mapstructure:"output"`
    GlossaryPath  string        `mapstructure:"glossary_path"`
    LogLevel      string        `mapstructure:"log_level"`
    LogFormat     string        `mapstructure:"log_format"`
}

type APIConfig struct {
    Host   string `mapstructure:"host"`
    Port   int    `mapstructure:"port"`
    APIKey string `mapstructure:"api_key"`
}

type LLMConfig struct {
    Provider    string  `mapstructure:"provider"`
    APIKey      string  `mapstructure:"api_key"`
    Model       string  `mapstructure:"model"`
    Temperature float64 `mapstructure:"temperature"`
    MaxTokens   int     `mapstructure:"max_tokens"`
}

type ImageGenConfig struct {
    Provider string `mapstructure:"provider"`
    APIKey   string `mapstructure:"api_key"`
    Model    string `mapstructure:"model"`
}

type TTSConfig struct {
    Provider string `mapstructure:"provider"`
    APIKey   string `mapstructure:"api_key"`
    Voice    string `mapstructure:"voice"`
    Speed    float64 `mapstructure:"speed"`
}

type OutputConfig struct {
    Provider string `mapstructure:"provider"`
}
```

**`mapstructure` tags are REQUIRED** — Viper uses mapstructure for unmarshaling. Without tags, nested struct fields won't bind.

### Cobra-Viper Integration Pattern

```go
// internal/cli/root.go — updated pattern
var cfgFile string

func init() {
    cobra.OnInitialize(initConfig)
    rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file path")
    rootCmd.PersistentFlags().Bool("verbose", false, "enable verbose output")
    rootCmd.PersistentFlags().Bool("json-output", false, "output in JSON format")
}

func initConfig() {
    // Load config using config.Load(cfgFile)
    // This is called BEFORE any command's Run function
}
```

**CRITICAL:** The current `root.go` uses `rootCmd.PersistentFlags().String("config", ...)` — this must change to `StringVar(&cfgFile, ...)` so the value is captured in a variable accessible to `initConfig()`.

### Previous Story Intelligence (Story 1.1)

**Learnings from Story 1.1:**
- `modernc.org/sqlite` registers as driver name `"sqlite"` (not `"sqlite3"`)
- `go:embed` must be in the same package as the embedded files (placed in `store.go`, not separate `embed.go`)
- Domain package has ZERO external imports — config types MUST NOT import domain
- Cobra's `init()` is the ONLY allowed exception to the "no init() functions" rule
- Empty string JobID caused FK violation in execution_log — use `sql.NullString` for nullable FKs
- `time.Time` used for timestamps in domain models (not string)

**Files from Story 1.1 that this story touches:**
- `internal/config/doc.go` → REPLACE with `config.go` (delete doc.go)
- `internal/cli/root.go` → UPDATE with Viper integration
- `config.example.yaml` → UPDATE with full documentation
- `go.mod` / `go.sum` → ADD viper dependency

**Files from Story 1.1 that MUST NOT be modified:**
- `internal/domain/*` — domain models are stable
- `internal/store/*` — store implementation is stable
- `cmd/yt-pipe/main.go` — entry point is stable

**Code review feedback applied in 1.1:**
- H3 (Viper/Chi not in go.mod) was deferred to THIS story — Viper must be added now
- Chi is NOT needed for this story (API is Story 7.1)

### Testing Strategy

**Unit tests for config package:**
```go
// internal/config/config_test.go

// Test 5-level priority chain
func TestLoad_DefaultValues(t *testing.T)           // defaults applied when no config
func TestLoad_GlobalConfig(t *testing.T)             // global YAML loaded
func TestLoad_ProjectOverridesGlobal(t *testing.T)   // project YAML merges on top
func TestLoad_EnvOverridesFile(t *testing.T)          // env vars override YAML
func TestLoad_CLIFlagOverridesAll(t *testing.T)       // CLI flags have highest priority
func TestLoad_ExplicitConfigPath(t *testing.T)        // --config flag overrides discovery

// Test environment variable mapping
func TestLoad_EnvMapping_LLMApiKey(t *testing.T)
func TestLoad_EnvMapping_SiliconFlowKey(t *testing.T) // backward compat alias
func TestLoad_EnvMapping_NestedKeys(t *testing.T)     // YTP_API_PORT → api.port

// Test config merging
func TestLoad_MergePreservesUnsetKeys(t *testing.T)  // only overridden keys change

// Test resilience
func TestLoad_NoConfigFilesExist(t *testing.T)        // first-run: no files, defaults only — MUST pass
func TestLoad_InvalidYAML(t *testing.T)                // malformed YAML returns clear error message
func TestLoad_GlobalConfigMissing(t *testing.T)        // global file not found is silently ignored

// Test validation
func TestValidate_MissingRequiredFields(t *testing.T)  // empty required fields
func TestValidate_InvalidPort(t *testing.T)
func TestValidate_PathsWarnOnly(t *testing.T)          // non-existent paths produce warnings, not errors
func TestValidate_ValidConfig(t *testing.T)

// Test secret masking
func TestMaskSecrets_ApiKeyFields(t *testing.T)        // any field matching *key*/*secret* is masked
func TestMaskSecrets_NonSecretFieldsUnchanged(t *testing.T)
```

**Test helper: Use `t.TempDir()` for creating temp config files in tests. Use `t.Setenv()` for setting env vars (auto-cleaned up).**

**IMPORTANT:** Viper uses a global singleton by default. Tests MUST create a new `viper.New()` instance per test to avoid state leakage. The `Load()` function should use `viper.New()`, NOT the global `viper.Set*` functions.

### File Structure for This Story

```
internal/config/
├── config.go        # Load(), Validate(), secret masking (REPLACES doc.go)
├── types.go         # Config struct + sub-config structs
└── config_test.go   # Unit tests (temp files + env vars)
    types_test.go    # Default value tests

internal/cli/
├── root.go          # UPDATED: Viper integration + cobra.OnInitialize
└── config_cmd.go    # NEW: config show + config validate subcommands
```

### Config Source Tracking (Enhancement)

`config show` should display where each value originated, aiding debugging:
```yaml
scp_data_path: "/data/raw"          # source: default
workspace_path: "/my/projects"      # source: global config
llm:
  provider: "anthropic"             # source: project config
  api_key: "***"                    # source: env YTP_LLM_API_KEY
  model: "gpt-4"                    # source: global config
api:
  port: 9090                        # source: flag --api-port
```

**Implementation hint:** Track sources by comparing Viper values at each merge stage. After defaults, record all keys as "default". After global merge, diff to find "global config" keys. Repeat for project config. Env/flag sources can be detected via `v.GetString()` vs `v.IsSet()` patterns. Alternatively, maintain a parallel `map[string]string` of key→source during Load().

### Secret Masking Strategy

Mask values using **pattern-based detection**, not a hardcoded field list:
- Any field name containing `key`, `secret`, `token`, or `password` (case-insensitive) → masked as `"***"`
- This future-proofs against new secret fields added in later stories
- Test: non-secret fields like `api.host`, `llm.provider` must NOT be masked

### Anti-Patterns (FORBIDDEN)

1. **NO** global Viper instance — use `viper.New()` in `Load()` for testability
2. **NO** `init()` in config package — initialization via explicit `Load()` call only
3. **NO** importing `domain/` from `config/` — config types are independent
4. **NO** plaintext API keys in `config show` output — always mask secrets
5. **NO** `os.Exit()` or `log.Fatal()` in config package — return errors to caller
6. **NO** Option pattern for Config struct — use simple struct with direct fields
7. **NO** Viper global functions (`viper.Set`, `viper.Get`) — use instance methods only

### Architecture Compliance Checklist

- [ ] Package name: `config` (lowercase single word)
- [ ] File names: `snake_case.go` — `config.go`, `types.go`, `config_test.go`
- [ ] Exported functions: `PascalCase` — `Load`, `Validate`
- [ ] Test naming: `Test{Function}_{Scenario}` — `TestLoad_DefaultValues`
- [ ] No domain/ imports from config/ (no import cycles)
- [ ] `mapstructure` struct tags on all config types
- [ ] YAML field names: `snake_case` — `scp_data_path`, `workspace_path`
- [ ] Env var prefix: `YTP_` with `_` replacing `.` for nested keys
- [ ] Error wrapping: `fmt.Errorf("config load: %w", err)`
- [ ] Constructor function: `Load(configPath string) (*Config, error)` — explicit dependencies

### Project Structure Notes

- `internal/config/doc.go` (placeholder from Story 1.1) should be DELETED when `config.go` is created
- Config package sits at: `internal/config/` — imported by `cli/` and future `api/` packages
- Config does NOT import `service/`, `store/`, `plugin/`, or `domain/`
- Config IS imported by `cli/root.go` for initialization

### References

- [Source: _bmad-output/planning-artifacts/architecture.md#Selected Stack — Viper native 5-level config]
- [Source: _bmad-output/planning-artifacts/architecture.md#Cross-Cutting Concerns — Setting Priority Chain]
- [Source: _bmad-output/planning-artifacts/architecture.md#Implementation Patterns — Anti-Patterns]
- [Source: _bmad-output/planning-artifacts/architecture.md#Project Structure & Boundaries — config/ package]
- [Source: _bmad-output/planning-artifacts/epics.md#Story 1.2 — Acceptance Criteria]
- [Source: _bmad-output/planning-artifacts/prd.md#FR34 — Global and project config overrides]
- [Source: _bmad-output/planning-artifacts/prd.md#FR35 — Config priority chain]
- [Source: _bmad-output/implementation-artifacts/1-1-project-scaffolding-development-environment.md — Previous story context]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6

### Debug Log References

- Viper added via `go get` was removed by `go mod tidy` because no code imported it yet. Fixed by running tidy after config.go was created.

### Completion Notes List

- Task 1: Viper v1.21.0 added to go.mod as direct dependency. Build verified.
- Task 2: Config types created in `internal/config/types.go` — Config, APIConfig, LLMConfig, ImageGenConfig, TTSConfig, OutputConfig with mapstructure tags. 6 unit tests pass.
- Task 3: Config loading implemented with 5-level priority chain using `viper.New()` instance (no global state). MergeInConfig with graceful missing-file handling. Source tracking via configSource helper. Validate() with errors (port, log level/format) and warnings (path existence). MaskSecrets with pattern-based detection. 29 total tests pass covering priority chain, env mapping, merging, resilience, validation, masking, source tracking.
- Task 4: Cobra-Viper integration in root.go — `cobra.OnInitialize(initConfig)`, `StringVar(&cfgFile)` for config path capture, `--verbose` overrides log level to debug. `GetConfig()` exported for subcommands.
- Task 5: `config show` and `config validate` subcommands in config_cmd.go. Show displays all config values with source annotations and masked secrets. Validate reports errors (hard failures) and warnings (path existence).
- Task 6: config.example.yaml fully documented with all fields, default values, env var mappings, security notes for API keys.
- Task 7: All verification gates passed — `go build`, `go test` (config + domain + store), `go vet`, `config show`, `config validate`.
- Code Review Follow-up (2026-03-08): Addressed 4 findings from code review:
  - ✅ H1: Fixed global config source tracking bug — snapshot now taken BEFORE merge, not after
  - ✅ H2: Replaced `os.Exit(2)` with `return fmt.Errorf()` in `runConfigValidate` for proper Cobra error handling
  - ✅ M2: Removed unused `IsSecretField` function (masking is handled directly by `MaskSecrets`)
  - ✅ M4: Removed dead `sortedKeys` function and `var _ = sortedKeys` suppression hack
  - ✅ L2: Added 3 new source tracking tests (global config, project config, env override) to prevent regression
  - Total tests: 30 config + all domain/store tests pass. Zero regressions.

### File List

- internal/config/config.go (new — replaces doc.go)
- internal/config/types.go (new)
- internal/config/config_test.go (new)
- internal/config/types_test.go (new)
- internal/config/doc.go (deleted)
- internal/cli/root.go (modified — Viper integration)
- internal/cli/config_cmd.go (new)
- config.example.yaml (modified — full documentation)
- go.mod (modified — added viper)
- go.sum (modified)

## Change Log

- 2026-03-08: Story 1.2 implemented — Viper 5-level config priority chain, structured config types, config show/validate CLI commands, config source tracking, secret masking, comprehensive config.example.yaml documentation
- 2026-03-08: Addressed code review findings — fixed source tracking bug (H1), replaced os.Exit with error return (H2), removed dead code (M2, M4), added source tracking regression tests (L2)
