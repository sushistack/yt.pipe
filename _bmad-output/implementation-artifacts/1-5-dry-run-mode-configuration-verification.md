# Story 1.5: Dry-Run Mode & Configuration Verification

Status: ready-for-dev

## Story

As a creator,
I want to verify my pipeline configuration and flow without making real API calls,
So that I can catch configuration errors before spending API credits.

## Acceptance Criteria

1. **Dry-Run Pipeline Execution** (AC:1)
   - Given the creator runs `yt-pipe run <scp-id> --dry-run`
   - When the pipeline executes in dry-run mode
   - Then every pipeline stage is invoked using mock plugin implementations instead of real API calls
   - And the mock plugins return deterministic sample data (placeholder image, sample audio, fixed timing)
   - And the output shows each stage's expected inputs/outputs and timing
   - And exit code 0 indicates the pipeline flow is valid, non-zero indicates configuration or flow errors
   - And this satisfies FR26

2. **Configuration Verification via Dry-Run** (AC:2)
   - Given the creator has changed configuration settings
   - When they run `yt-pipe run <scp-id> --dry-run` to verify
   - Then the new config values are loaded and applied throughout the dry-run
   - And any invalid config values (missing API keys, unreachable paths) are reported with specific error messages
   - And this satisfies FR36

3. **Dry-Run Results Output** (AC:3)
   - Given a dry-run completes successfully
   - When results are displayed
   - Then JSON output on stdout includes: stages executed, config values used (keys masked), plugin selections, data paths verified
   - And exit code follows the convention: 0=success, 2=config error

## Tasks / Subtasks

- [ ] Task 1: Create `run` command scaffold with `--dry-run` flag (AC: #1)
  - [ ] 1.1 Create `internal/cli/run_cmd.go` with `yt-pipe run <scp-id>` Cobra command
  - [ ] 1.2 Register `runCmd` in rootCmd via `init()` function
  - [ ] 1.3 Add `--dry-run` flag (bool, default false)
  - [ ] 1.4 Validate that `scp-id` positional argument is provided
  - [ ] 1.5 When `--dry-run` is NOT set, print "pipeline execution not yet implemented" and exit 0 (placeholder for Epic 5)

- [ ] Task 2: Create dry-run pipeline executor (AC: #1, #2)
  - [ ] 2.1 Create `internal/pipeline/dryrun.go` in new `internal/pipeline/` package
  - [ ] 2.2 Define `DryRunResult` struct with: Stages []StageResult, Config ConfigSummary, Errors []string, Success bool
  - [ ] 2.3 Define `StageResult` struct with: Name, Status, Duration, InputSummary, OutputSummary
  - [ ] 2.4 Define `ConfigSummary` struct with masked config values, plugin selections, data paths
  - [ ] 2.5 Implement `RunDryRun(ctx, cfg *config.Config, scpID string) (*DryRunResult, error)`
  - [ ] 2.6 DryRun validates config completeness (paths exist, plugins configured) before running stages

- [ ] Task 3: Implement dry-run pipeline stages (AC: #1)
  - [ ] 3.1 Define pipeline stages as ordered slice: ["scp_load", "scenario_generate", "image_generate", "tts_synthesize", "timing_resolve", "subtitle_generate", "output_assemble"]
  - [ ] 3.2 Each stage: validate prerequisites, simulate execution with deterministic timing, record inputs/outputs summary
  - [ ] 3.3 `scp_load` stage: verify `scp_data_path` exists and contains expected files for given SCP ID
  - [ ] 3.4 `scenario_generate` stage: verify LLM config (provider set, model set), simulate with sample ScenarioOutput
  - [ ] 3.5 `image_generate` stage: verify ImageGen config (provider set), simulate with placeholder ImageResult per scene
  - [ ] 3.6 `tts_synthesize` stage: verify TTS config (provider set, voice set), simulate with sample SynthesisResult
  - [ ] 3.7 `timing_resolve` stage: simulate timing resolution from TTS word timings
  - [ ] 3.8 `subtitle_generate` stage: simulate subtitle file generation
  - [ ] 3.9 `output_assemble` stage: verify Output config (provider set), verify workspace_path writable, simulate assembly

- [ ] Task 4: Implement config verification logic (AC: #2)
  - [ ] 4.1 Verify all required paths exist: scp_data_path, workspace_path
  - [ ] 4.2 Verify all plugin providers are configured: llm.provider, imagegen.provider, tts.provider, output.provider
  - [ ] 4.3 Verify API keys are set (via env vars) for non-edge providers: check YTP_LLM_API_KEY, YTP_IMAGEGEN_API_KEY, YTP_TTS_API_KEY
  - [ ] 4.4 Report each verification as pass/fail with actionable error messages
  - [ ] 4.5 Continue checking all items even after first failure (don't short-circuit)

- [ ] Task 5: Implement JSON result output (AC: #3)
  - [ ] 5.1 When `--json-output` flag is set (from root.go), output DryRunResult as JSON to stdout
  - [ ] 5.2 When `--json-output` is NOT set, output human-readable formatted summary
  - [ ] 5.3 Mask all API keys in output using config.MaskSecrets pattern (any field with "key"/"secret"/"token")
  - [ ] 5.4 Include: stages executed with status, config values with sources, plugin selections, verified paths
  - [ ] 5.5 Exit code 0 for success, 2 for config errors

- [ ] Task 6: Write comprehensive tests (AC: #1, #2, #3)
  - [ ] 6.1 `internal/pipeline/dryrun_test.go`: test dry-run with valid config, missing paths, missing providers, missing API keys
  - [ ] 6.2 `internal/cli/run_cmd_test.go`: test run command with --dry-run flag, without flag, missing scp-id
  - [ ] 6.3 Test JSON output format and content
  - [ ] 6.4 Test human-readable output format
  - [ ] 6.5 Test exit codes (0 for success, 2 for config error)
  - [ ] 6.6 Use `t.TempDir()` for workspace/scp paths, `t.Setenv()` for API keys

- [ ] Task 7: Final verification
  - [ ] 7.1 `go build ./...` — zero errors
  - [ ] 7.2 `go test ./...` — all tests pass (including all previous story tests)
  - [ ] 7.3 `go vet ./...` — zero warnings
  - [ ] 7.4 Manual test: `go run ./cmd/yt-pipe run SCP-173 --dry-run` — shows stage-by-stage results
  - [ ] 7.5 Manual test: `go run ./cmd/yt-pipe run SCP-173 --dry-run --json-output` — valid JSON output
  - [ ] 7.6 Verify exit code 0 on success, 2 on config error

## Dev Notes

### Critical Architecture Constraints

**CLI Structure (Architecture-mandated):**
- All CLI commands in `internal/cli/`
- `run.go` is listed in architecture's file structure as `internal/cli/run.go`
- Command registered via `init()` (Cobra's only allowed `init()` exception)
- No business logic in CLI commands — parsing + service calls + output formatting only

**New Package: `internal/pipeline/`:**
- This story introduces `internal/pipeline/` package for pipeline orchestration
- `pipeline/` imports: `config/`, `domain/` — does NOT import `cli/`, `store/`, `service/`
- Keep pipeline logic separate from CLI for API reuse (Epic 7 will use same pipeline)
- The dry-run executor is the first inhabitant of this package; full pipeline executor comes in Epic 5

**Exit Code Convention (PRD-mandated):**
- 0 = success
- 1 = runtime error
- 2 = configuration error
- 3 = validation error

**Config Integration:**
- Use `cli.GetConfig()` to access loaded config (already initialized via `cobra.OnInitialize`)
- Use `config.MaskSecrets()` for display output
- Config is already loaded with 5-level priority by the time run command executes

**Anti-Patterns (FORBIDDEN):**
1. **NO** real API calls in dry-run — this is the entire point
2. **NO** importing `plugin/` sub-packages (llm, tts, imagegen, output) — dry-run uses its own deterministic mock data, NOT the mockery-generated mocks (those are for unit tests)
3. **NO** `os.Exit()` or `log.Fatal()` — return errors to Cobra, use `cmd.SilenceUsage` for clean exit codes
4. **NO** global state — pass config explicitly
5. **NO** importing `store/` — dry-run doesn't touch the database

### Dry-Run Mock Data (NOT mockery mocks)

The dry-run creates its own deterministic sample data inline. Do NOT use `internal/mocks/` (those are testify mocks for unit tests). Instead, create simple struct literals:

```go
// Example deterministic stage output
sampleScenario := &domain.ScenarioOutput{
    SCPID:     scpID,
    Title:     fmt.Sprintf("[DRY-RUN] SCP-%s Scenario", scpID),
    SceneCount: 5,
    Scenes: []domain.SceneScript{
        {SceneNum: 1, Narration: "[DRY-RUN] Sample narration for scene 1", ImagePrompt: "[DRY-RUN] Sample image prompt"},
        // ... 5 scenes
    },
}
```

### Pipeline Stage Design

```go
// internal/pipeline/dryrun.go
package pipeline

type StageResult struct {
    Name          string        `json:"name"`
    Status        string        `json:"status"`  // "pass", "fail", "skip"
    Duration      time.Duration `json:"duration_ms"`
    InputSummary  string        `json:"input_summary"`
    OutputSummary string        `json:"output_summary"`
    Error         string        `json:"error,omitempty"`
}

type ConfigSummary struct {
    SCPDataPath   string `json:"scp_data_path"`
    WorkspacePath string `json:"workspace_path"`
    LLMProvider   string `json:"llm_provider"`
    ImageGenProvider string `json:"imagegen_provider"`
    TTSProvider   string `json:"tts_provider"`
    OutputProvider string `json:"output_provider"`
    LLMAPIKey     string `json:"llm_api_key"`      // masked
    ImageGenAPIKey string `json:"imagegen_api_key"` // masked
    TTSAPIKey     string `json:"tts_api_key"`       // masked
}

type DryRunResult struct {
    SCPID   string         `json:"scp_id"`
    Success bool           `json:"success"`
    Stages  []StageResult  `json:"stages"`
    Config  ConfigSummary  `json:"config"`
    Errors  []string       `json:"errors,omitempty"`
}

func RunDryRun(ctx context.Context, cfg *config.Config, scpID string) (*DryRunResult, error)
```

### Previous Story Intelligence (Stories 1.1-1.4)

**From Story 1.1 (Project Scaffolding):**
- Domain models in `internal/domain/` — use `domain.ScenarioOutput`, `domain.SceneScript`, `domain.Project`
- Project states defined: pending, scenario_review, approved, generating_assets, assembling, complete
- Store uses `modernc.org/sqlite` — dry-run does NOT use store

**From Story 1.2 (Configuration Management):**
- `config.Load()` returns `*LoadResult` with Config + Sources map
- `config.MaskSecrets()` masks fields containing "key"/"secret"/"token"/"password"
- Config validation via field checks (not a Validate() method on Config — validation is in config_cmd.go)
- Viper 5-level priority already handles config resolution

**From Story 1.3 (Plugin Interface Framework):**
- Plugin interfaces: `llm.LLM`, `tts.TTS`, `imagegen.ImageGen`, `output.Assembler`
- Plugin registry maps provider names to factories
- `plugin.PluginConfig` with Timeout, MaxRetries, BaseDelay defaults
- Dry-run does NOT use plugin registry — it simulates directly

**From Story 1.4 (Initial Setup Wizard):**
- `yt-pipe init` creates config at `$HOME/.yt-pipe/config.yaml`
- API keys stored as env var instructions in comments (not plaintext)
- Provider options: LLM=openai, ImageGen=siliconflow, TTS=openai/google/edge, Output=capcut
- `config validate` and `config show` commands exist for post-init verification

**Files that MUST NOT be modified:**
- `internal/config/*` — config loading is stable
- `internal/plugin/*` — plugin interfaces are stable
- `internal/domain/*` — domain models are stable
- `internal/store/*` — store implementation is stable
- `internal/retry/*` — retry helper is stable
- `internal/cli/init_cmd.go` — init wizard is stable
- `internal/cli/config_cmd.go` — config commands are stable

**Files this story modifies:**
- `internal/cli/root.go` — NO changes needed (run_cmd registers via `init()`)

### Git Intelligence

**Recent commits:**
- `5f6ce0d` — chore: add BMAD project artifacts and planning docs
- `79c29b4` — feat: scaffold project foundation (Story 1.1)

**Uncommitted changes:** Stories 1.2, 1.3, 1.4 code (config, plugins, init wizard). All tests pass.

**Conventions:**
- Commit prefix: `feat:`, `chore:`
- All code in `internal/` (Go convention)
- Co-authored-by: Claude Opus 4.6

### Testing Strategy

**Dry-run pipeline tests:**
```go
// internal/pipeline/dryrun_test.go
func TestRunDryRun_ValidConfig(t *testing.T)         // all stages pass
func TestRunDryRun_MissingSCPDataPath(t *testing.T)  // reports path error
func TestRunDryRun_MissingWorkspacePath(t *testing.T) // reports path error
func TestRunDryRun_MissingProvider(t *testing.T)     // reports config error
func TestRunDryRun_MissingAPIKey(t *testing.T)       // reports key warning
func TestRunDryRun_AllStagesExecuted(t *testing.T)   // verifies 7 stages run
func TestRunDryRun_DeterministicOutput(t *testing.T) // same input = same output
```

**CLI run command tests:**
```go
// internal/cli/run_cmd_test.go
func TestRunCmd_DryRunFlag(t *testing.T)        // --dry-run triggers dry-run path
func TestRunCmd_MissingSCPID(t *testing.T)      // error when no positional arg
func TestRunCmd_NoDryRun_Placeholder(t *testing.T) // without --dry-run shows placeholder
func TestRunCmd_JSONOutput(t *testing.T)        // --json-output produces valid JSON
func TestRunCmd_ExitCode0OnSuccess(t *testing.T)
func TestRunCmd_ExitCode2OnConfigError(t *testing.T)
```

**Test helpers:** `t.TempDir()` for paths, `t.Setenv("YTP_LLM_API_KEY", "test-key")` for API keys.

### Dependency Direction (Import Cycle Prevention)

```
cli/run_cmd.go  → pipeline/, config/
pipeline/       → config/, domain/
pipeline/       ✗ cli/, store/, service/, plugin/
```

**CRITICAL:** `pipeline/` is a new package that depends only on `config/` and `domain/`. It does NOT import plugin interfaces — dry-run creates its own deterministic data.

### New Dependencies

**None.** All functionality uses stdlib (`encoding/json`, `fmt`, `os`, `time`, `path/filepath`).

### Architecture Compliance Checklist

- [ ] New file: `internal/cli/run_cmd.go` — matches architecture's `run.go`
- [ ] New package: `internal/pipeline/dryrun.go` — clean pipeline orchestration
- [ ] File names: `snake_case.go`
- [ ] Test naming: `Test{Function}_{Scenario}`
- [ ] No `plugin/`, `service/`, `store/` imports from pipeline dry-run
- [ ] No `os.Exit()` or `log.Fatal()` — return errors to Cobra
- [ ] API keys masked in all output
- [ ] Exit codes: 0=success, 2=config error
- [ ] Context propagation through all functions
- [ ] Error wrapping: `fmt.Errorf("dry-run: %w", err)`

### Project Structure Notes

- `run.go` (called `run_cmd.go` per snake_case convention) listed in architecture
- `internal/pipeline/` is a new package — first code for pipeline orchestration
- Pipeline package is designed to be reused by both CLI (this story) and API (Epic 7)
- Dry-run does NOT create workspace files — it only validates and simulates

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Story 1.5 — Acceptance Criteria, user story]
- [Source: _bmad-output/planning-artifacts/prd.md#FR26 — Dry-run mode pipeline flow verification]
- [Source: _bmad-output/planning-artifacts/prd.md#FR36 — Test pipeline run after config changes]
- [Source: _bmad-output/planning-artifacts/prd.md#CLI Structure — exit code conventions 0/1/2/3]
- [Source: _bmad-output/planning-artifacts/architecture.md#Code Organization — run.go in CLI structure]
- [Source: _bmad-output/planning-artifacts/architecture.md#Architectural Boundaries — dependency direction]
- [Source: _bmad-output/implementation-artifacts/1-2-configuration-management-system.md — Config loading, MaskSecrets]
- [Source: _bmad-output/implementation-artifacts/1-3-plugin-interface-framework.md — Plugin interfaces, registry]
- [Source: _bmad-output/implementation-artifacts/1-4-initial-setup-wizard.md — Init wizard, provider options]

## Dev Agent Record

### Agent Model Used

{{agent_model_name_version}}

### Debug Log References

### Completion Notes List

### File List
