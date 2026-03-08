// Package pipeline provides pipeline orchestration for the youtube content pipeline.
package pipeline

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/sushistack/yt.pipe/internal/config"
)

// StageResult captures the outcome of a single pipeline stage during dry-run.
type StageResult struct {
	Name          string `json:"name"`
	Status        string `json:"status"` // "pass", "fail", "skip"
	DurationMs    int64  `json:"duration_ms"`
	InputSummary  string `json:"input_summary"`
	OutputSummary string `json:"output_summary"`
	Error         string `json:"error,omitempty"`
}

// ConfigSummary provides a masked view of the active configuration.
type ConfigSummary struct {
	SCPDataPath      string `json:"scp_data_path"`
	WorkspacePath    string `json:"workspace_path"`
	LLMProvider      string `json:"llm_provider"`
	LLMAPIKey        string `json:"llm_api_key"`
	ImageGenProvider string `json:"imagegen_provider"`
	ImageGenAPIKey   string `json:"imagegen_api_key"`
	TTSProvider      string `json:"tts_provider"`
	TTSAPIKey        string `json:"tts_api_key"`
	OutputProvider   string `json:"output_provider"`
}

// DryRunResult contains the complete dry-run execution results.
type DryRunResult struct {
	SCPID   string        `json:"scp_id"`
	Success bool          `json:"success"`
	Stages  []StageResult `json:"stages"`
	Config  ConfigSummary `json:"config"`
	Errors  []string      `json:"errors,omitempty"`
}

// pipelineStages defines the ordered pipeline stages.
var pipelineStages = []string{
	"scp_load",
	"scenario_generate",
	"image_generate",
	"tts_synthesize",
	"timing_resolve",
	"subtitle_generate",
	"output_assemble",
}

// RunDryRun executes a simulated pipeline run to verify configuration and flow.
// It validates config, checks paths, and simulates each stage with deterministic data.
func RunDryRun(ctx context.Context, cfg *config.Config, scpID string) (*DryRunResult, error) {
	if cfg == nil {
		return nil, fmt.Errorf("dry-run: config is nil")
	}

	masked := config.MaskSecrets(cfg)
	result := &DryRunResult{
		SCPID:   scpID,
		Success: true,
		Config: ConfigSummary{
			SCPDataPath:      cfg.SCPDataPath,
			WorkspacePath:    cfg.WorkspacePath,
			LLMProvider:      cfg.LLM.Provider,
			LLMAPIKey:        masked.LLM.APIKey,
			ImageGenProvider: cfg.ImageGen.Provider,
			ImageGenAPIKey:   masked.ImageGen.APIKey,
			TTSProvider:      cfg.TTS.Provider,
			TTSAPIKey:        masked.TTS.APIKey,
			OutputProvider:   cfg.Output.Provider,
		},
	}

	// Run each stage
	stageRunners := map[string]func(context.Context, *config.Config, string) StageResult{
		"scp_load":          runSCPLoad,
		"scenario_generate": runScenarioGenerate,
		"image_generate":    runImageGenerate,
		"tts_synthesize":    runTTSSynthesize,
		"timing_resolve":    runTimingResolve,
		"subtitle_generate": runSubtitleGenerate,
		"output_assemble":   runOutputAssemble,
	}

	for _, stageName := range pipelineStages {
		if ctx.Err() != nil {
			result.Stages = append(result.Stages, StageResult{
				Name:   stageName,
				Status: "skip",
				Error:  "context cancelled",
			})
			result.Success = false
			result.Errors = append(result.Errors, fmt.Sprintf("%s: context cancelled", stageName))
			continue
		}

		runner := stageRunners[stageName]
		sr := runner(ctx, cfg, scpID)
		result.Stages = append(result.Stages, sr)

		if sr.Status == "fail" {
			result.Success = false
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %s", stageName, sr.Error))
		}
	}

	return result, nil
}

func runSCPLoad(_ context.Context, cfg *config.Config, scpID string) StageResult {
	start := time.Now()
	sr := StageResult{
		Name:         "scp_load",
		InputSummary: fmt.Sprintf("scp_id=%s, path=%s", scpID, cfg.SCPDataPath),
	}

	if cfg.SCPDataPath == "" {
		sr.Status = "fail"
		sr.Error = "scp_data_path is not configured"
		sr.DurationMs = time.Since(start).Milliseconds()
		return sr
	}

	if _, err := os.Stat(cfg.SCPDataPath); os.IsNotExist(err) {
		sr.Status = "fail"
		sr.Error = fmt.Sprintf("scp_data_path does not exist: %s", cfg.SCPDataPath)
		sr.DurationMs = time.Since(start).Milliseconds()
		return sr
	}

	sr.Status = "pass"
	sr.OutputSummary = fmt.Sprintf("[DRY-RUN] SCP data loaded for %s", scpID)
	sr.DurationMs = time.Since(start).Milliseconds()
	return sr
}

func runScenarioGenerate(_ context.Context, cfg *config.Config, scpID string) StageResult {
	start := time.Now()
	sr := StageResult{
		Name:         "scenario_generate",
		InputSummary: fmt.Sprintf("scp_id=%s, provider=%s, model=%s", scpID, cfg.LLM.Provider, cfg.LLM.Model),
	}

	if cfg.LLM.Provider == "" {
		sr.Status = "fail"
		sr.Error = "llm.provider is not configured"
		sr.DurationMs = time.Since(start).Milliseconds()
		return sr
	}

	if cfg.LLM.APIKey == "" {
		sr.Status = "fail"
		sr.Error = "llm.api_key is not set (set YTP_LLM_API_KEY environment variable)"
		sr.DurationMs = time.Since(start).Milliseconds()
		return sr
	}

	sr.Status = "pass"
	sr.OutputSummary = "[DRY-RUN] Generated scenario with 5 scenes"
	sr.DurationMs = time.Since(start).Milliseconds()
	return sr
}

func runImageGenerate(_ context.Context, cfg *config.Config, _ string) StageResult {
	start := time.Now()
	sr := StageResult{
		Name:         "image_generate",
		InputSummary: fmt.Sprintf("provider=%s, model=%s, scenes=5", cfg.ImageGen.Provider, cfg.ImageGen.Model),
	}

	if cfg.ImageGen.Provider == "" {
		sr.Status = "fail"
		sr.Error = "imagegen.provider is not configured"
		sr.DurationMs = time.Since(start).Milliseconds()
		return sr
	}

	if cfg.ImageGen.APIKey == "" {
		sr.Status = "fail"
		sr.Error = "imagegen.api_key is not set (set YTP_IMAGEGEN_API_KEY environment variable)"
		sr.DurationMs = time.Since(start).Milliseconds()
		return sr
	}

	sr.Status = "pass"
	sr.OutputSummary = "[DRY-RUN] Generated 5 placeholder images (1024x1024 PNG)"
	sr.DurationMs = time.Since(start).Milliseconds()
	return sr
}

func runTTSSynthesize(_ context.Context, cfg *config.Config, _ string) StageResult {
	start := time.Now()
	sr := StageResult{
		Name:         "tts_synthesize",
		InputSummary: fmt.Sprintf("provider=%s, voice=%s, speed=%.1f, scenes=5", cfg.TTS.Provider, cfg.TTS.Voice, cfg.TTS.Speed),
	}

	if cfg.TTS.Provider == "" {
		sr.Status = "fail"
		sr.Error = "tts.provider is not configured"
		sr.DurationMs = time.Since(start).Milliseconds()
		return sr
	}

	// Edge TTS doesn't require an API key
	if cfg.TTS.Provider != "edge" && cfg.TTS.APIKey == "" {
		sr.Status = "fail"
		sr.Error = "tts.api_key is not set (set YTP_TTS_API_KEY environment variable)"
		sr.DurationMs = time.Since(start).Milliseconds()
		return sr
	}

	sr.Status = "pass"
	sr.OutputSummary = "[DRY-RUN] Synthesized 5 audio segments (~30s each, 150s total)"
	sr.DurationMs = time.Since(start).Milliseconds()
	return sr
}

func runTimingResolve(_ context.Context, _ *config.Config, _ string) StageResult {
	start := time.Now()
	return StageResult{
		Name:          "timing_resolve",
		Status:        "pass",
		InputSummary:  "scenes=5, word_timings=available",
		OutputSummary: "[DRY-RUN] Resolved timing for 5 scenes (image display durations calculated)",
		DurationMs:    time.Since(start).Milliseconds(),
	}
}

func runSubtitleGenerate(_ context.Context, _ *config.Config, _ string) StageResult {
	start := time.Now()
	return StageResult{
		Name:          "subtitle_generate",
		Status:        "pass",
		InputSummary:  "scenes=5, word_timings=available",
		OutputSummary: "[DRY-RUN] Generated SRT subtitles for 5 scenes",
		DurationMs:    time.Since(start).Milliseconds(),
	}
}

func runOutputAssemble(_ context.Context, cfg *config.Config, _ string) StageResult {
	start := time.Now()
	sr := StageResult{
		Name:         "output_assemble",
		InputSummary: fmt.Sprintf("provider=%s, workspace=%s", cfg.Output.Provider, cfg.WorkspacePath),
	}

	if cfg.Output.Provider == "" {
		sr.Status = "fail"
		sr.Error = "output.provider is not configured"
		sr.DurationMs = time.Since(start).Milliseconds()
		return sr
	}

	if cfg.WorkspacePath == "" {
		sr.Status = "fail"
		sr.Error = "workspace_path is not configured"
		sr.DurationMs = time.Since(start).Milliseconds()
		return sr
	}

	if _, err := os.Stat(cfg.WorkspacePath); os.IsNotExist(err) {
		sr.Status = "fail"
		sr.Error = fmt.Sprintf("workspace_path does not exist: %s", cfg.WorkspacePath)
		sr.DurationMs = time.Since(start).Milliseconds()
		return sr
	}

	sr.Status = "pass"
	sr.OutputSummary = "[DRY-RUN] Assembled CapCut project structure"
	sr.DurationMs = time.Since(start).Milliseconds()
	return sr
}
