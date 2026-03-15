package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sushistack/yt.pipe/internal/domain"
	"github.com/sushistack/yt.pipe/internal/glossary"
	"github.com/sushistack/yt.pipe/internal/plugin/llm"
	"github.com/sushistack/yt.pipe/internal/workspace"
)

// ScenarioPipelineStage identifies a stage in the 4-stage pipeline.
type ScenarioPipelineStage string

const (
	StageResearch  ScenarioPipelineStage = "01_research"
	StageStructure ScenarioPipelineStage = "02_structure"
	StageWriting   ScenarioPipelineStage = "03_writing"
	StageReview    ScenarioPipelineStage = "04_review"
)

// ScenarioPipelineConfig configures the 4-stage scenario pipeline.
type ScenarioPipelineConfig struct {
	TemplatesDir          string
	TargetDurationMin     int
	FactCoverageThreshold float64
	MaxAttempts           int
}

// ScenarioPipeline orchestrates the 4-stage scenario generation process:
// Research → Structure → Writing → Review.
type ScenarioPipeline struct {
	llm            llm.LLM
	glossary       *glossary.Glossary
	config         ScenarioPipelineConfig
	templates      map[ScenarioPipelineStage]string
	formatGuide    string
	criticTemplate string
}

// StageResult holds the output of a single pipeline stage.
type StageResult struct {
	Stage        ScenarioPipelineStage `json:"stage"`
	Content      string                `json:"content"`
	InputTokens  int                   `json:"input_tokens"`
	OutputTokens int                   `json:"output_tokens"`
	DurationMs   int64                 `json:"duration_ms"`
}

// PipelineResult holds the complete output of the 4-stage pipeline.
type PipelineResult struct {
	Scenario     *domain.ScenarioOutput `json:"scenario"`
	Stages       []StageResult          `json:"stages"`
	ReviewReport *ReviewReport          `json:"review_report,omitempty"`
	TotalTokens  int                    `json:"total_tokens"`
	TotalMs      int64                  `json:"total_ms"`
	Attempts     int                    `json:"attempts"`
}

// ReviewReport is the structured output from Stage 4.
type ReviewReport struct {
	OverallPass        bool               `json:"overall_pass"`
	CoveragePct        float64            `json:"coverage_pct"`
	Issues             []ReviewIssue      `json:"issues"`
	Corrections        []ReviewCorrection `json:"corrections"`
	StorytellingScore  int                `json:"storytelling_score"`
	StorytellingIssues []ReviewIssue      `json:"storytelling_issues"`
}

// ReviewIssue is a single issue found during review.
type ReviewIssue struct {
	SceneNum    int    `json:"scene_num"`
	Type        string `json:"type"` // fact_error, missing_fact, descriptor_violation, invented_content
	Severity    string `json:"severity"` // critical, warning, info
	Description string `json:"description"`
	Correction  string `json:"correction"`
}

// ReviewCorrection is a specific text correction.
type ReviewCorrection struct {
	SceneNum  int    `json:"scene_num"`
	Field     string `json:"field"` // narration, visual_description
	Original  string `json:"original"`
	Corrected string `json:"corrected"`
}

// NewScenarioPipeline creates a new 4-stage scenario pipeline.
func NewScenarioPipeline(l llm.LLM, g *glossary.Glossary, cfg ScenarioPipelineConfig) (*ScenarioPipeline, error) {
	if cfg.TargetDurationMin <= 0 {
		cfg.TargetDurationMin = 10
	}
	if cfg.FactCoverageThreshold <= 0 {
		cfg.FactCoverageThreshold = 80.0
	}

	sp := &ScenarioPipeline{
		llm:       l,
		glossary:  g,
		config:    cfg,
		templates: make(map[ScenarioPipelineStage]string),
	}

	// Load templates
	stages := []ScenarioPipelineStage{StageResearch, StageStructure, StageWriting, StageReview}
	for _, stage := range stages {
		path := filepath.Join(cfg.TemplatesDir, "scenario", string(stage)+".md")
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("scenario pipeline: load template %s: %w", stage, err)
		}
		sp.templates[stage] = string(data)
	}

	// Load format guide (graceful degradation — empty string if not found)
	fgPath := filepath.Join(cfg.TemplatesDir, "scenario", "format_guide.md")
	if fgData, err := os.ReadFile(fgPath); err == nil {
		sp.formatGuide = string(fgData)
	}

	// Load critic agent template (graceful degradation — empty string if not found)
	criticPath := filepath.Join(cfg.TemplatesDir, "scenario", "critic_agent.md")
	if criticData, err := os.ReadFile(criticPath); err == nil {
		sp.criticTemplate = string(criticData)
	}

	return sp, nil
}

// Run executes the full 4-stage pipeline with checkpoint support.
func (sp *ScenarioPipeline) Run(ctx context.Context, scpData *workspace.SCPData, workspacePath string) (*PipelineResult, error) {
	totalStart := time.Now()
	stagesDir := filepath.Join(workspacePath, "stages")
	if err := os.MkdirAll(stagesDir, 0o755); err != nil {
		return nil, fmt.Errorf("scenario pipeline: create stages dir: %w", err)
	}

	result := &PipelineResult{}
	glossarySection := sp.buildGlossarySection(scpData)
	factSheet := sp.buildFactSheet(scpData)

	// Stage 1: Research
	researchContent, err := sp.runStageWithCheckpoint(ctx, StageResearch, stagesDir, func() (*StageResult, error) {
		return sp.runResearch(ctx, scpData, factSheet, glossarySection)
	})
	if err != nil {
		return nil, fmt.Errorf("scenario pipeline: research stage: %w", err)
	}
	result.Stages = append(result.Stages, *researchContent)

	// Extract visual identity from research output
	visualRef := extractVisualIdentity(researchContent.Content)

	// Stage 2: Structure
	structureContent, err := sp.runStageWithCheckpoint(ctx, StageStructure, stagesDir, func() (*StageResult, error) {
		return sp.runStructure(ctx, scpData, researchContent.Content, visualRef, glossarySection)
	})
	if err != nil {
		return nil, fmt.Errorf("scenario pipeline: structure stage: %w", err)
	}
	result.Stages = append(result.Stages, *structureContent)

	// Quality gate retry loop wrapping Stages 3 (Writing) + 4 (Review)
	maxAttempts := sp.config.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = 3
	}

	minScene, maxScene := sceneCountRange(sp.config.TargetDurationMin)
	qgConfig := QualityGateConfig{
		MaxAttempts:           maxAttempts,
		FactCoverageThreshold: sp.config.FactCoverageThreshold,
		MinSceneCount:         minScene,
		MaxSceneCount:         maxScene,
		MinImmersionCount:     3,
	}

	var bestAttempt *writingAttempt
	qualityFeedback := ""

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		// Check for context cancellation before each attempt
		if err := ctx.Err(); err != nil {
			if bestAttempt != nil {
				break // Use best attempt so far
			}
			return nil, fmt.Errorf("scenario pipeline: context cancelled: %w", err)
		}

		// Delete previous Stage 3+4+critic checkpoints for retry
		if attempt > 1 {
			for _, cpFile := range []string{"03_writing.json", "04_review.json", "critic_agent.json"} {
				if rmErr := os.Remove(filepath.Join(stagesDir, cpFile)); rmErr != nil && !os.IsNotExist(rmErr) {
					slog.Warn("quality gate: failed to remove checkpoint for retry", "file", cpFile, "err", rmErr)
				}
			}
		}

		// Stage 3: Writing (with quality_feedback injected)
		writingContent, err := sp.runStageWithCheckpoint(ctx, StageWriting, stagesDir, func() (*StageResult, error) {
			return sp.runWriting(ctx, scpData, structureContent.Content, visualRef, glossarySection, qualityFeedback)
		})
		if err != nil {
			return nil, fmt.Errorf("scenario pipeline: writing stage: %w", err)
		}

		// Stage 4: Review
		reviewContent, err := sp.runStageWithCheckpoint(ctx, StageReview, stagesDir, func() (*StageResult, error) {
			return sp.runReview(ctx, scpData, writingContent.Content, visualRef, factSheet, glossarySection)
		})
		if err != nil {
			return nil, fmt.Errorf("scenario pipeline: review stage: %w", err)
		}

		// Parse scenario + review
		scenario, parseErr := parseScenarioFromWriting(writingContent.Content, scpData.SCPID)
		if parseErr != nil {
			return nil, fmt.Errorf("scenario pipeline: parse writing output: %w", parseErr)
		}

		reviewReport, reviewErr := parseReviewReport(reviewContent.Content)
		if reviewErr != nil {
			slog.Warn("scenario pipeline: could not parse review report", "err", reviewErr)
		}
		if reviewReport != nil {
			scenario = applyCorrections(scenario, reviewReport.Corrections)
		}

		thisAttempt := &writingAttempt{
			scenario:     scenario,
			reviewReport: reviewReport,
			writingStage: writingContent,
			reviewStage:  reviewContent,
			attempt:      attempt,
		}

		// Quality Gate Layer 1
		violations := RunLayer1(scenario, reviewReport, qgConfig)
		thisAttempt.violations = violations

		if len(violations) > 0 {
			// Layer 1 fail → skip Critic, retry immediately
			qualityFeedback = BuildFeedbackString(violations, nil, attempt, maxAttempts)
			bestAttempt = selectBest(bestAttempt, thisAttempt)
			slog.Warn("quality gate: Layer 1 failed", "attempt", attempt, "violations", len(violations))
			if attempt < maxAttempts {
				continue
			}
			break
		}

		// Quality Gate Layer 2: Critic Agent
		if sp.criticTemplate != "" {
			verdict, criticErr := RunLayer2(ctx, sp.llm, scenario, sp.formatGuide, sp.criticTemplate)
			if criticErr == nil && verdict != nil {
				thisAttempt.criticVerdict = verdict
				if verdict.Verdict == "pass" || verdict.Verdict == "accept_with_notes" {
					bestAttempt = thisAttempt
					break // Quality gate passed!
				}
				qualityFeedback = BuildFeedbackString(violations, verdict, attempt, maxAttempts)
				bestAttempt = selectBest(bestAttempt, thisAttempt)
				slog.Warn("quality gate: Critic rejected", "attempt", attempt, "verdict", verdict.Verdict)
				if attempt < maxAttempts {
					continue
				}
			} else {
				// Critic error or nil verdict → treat as pass (graceful degradation)
				bestAttempt = thisAttempt
				break
			}
		} else {
			// No critic template → Layer 1 pass is sufficient
			bestAttempt = thisAttempt
			break
		}
	}

	// Use best attempt for final result (nil guard — should never happen due to loop guarantee)
	if bestAttempt == nil {
		return nil, fmt.Errorf("scenario pipeline: no writing attempt completed")
	}
	result.Stages = append(result.Stages, *bestAttempt.writingStage)
	result.Stages = append(result.Stages, *bestAttempt.reviewStage)
	result.Scenario = bestAttempt.scenario
	result.ReviewReport = bestAttempt.reviewReport
	result.Attempts = bestAttempt.attempt

	// Calculate totals
	for _, s := range result.Stages {
		result.TotalTokens += s.InputTokens + s.OutputTokens
	}
	result.TotalMs = time.Since(totalStart).Milliseconds()

	slog.Info("scenario pipeline completed",
		"scp_id", scpData.SCPID,
		"scenes", len(result.Scenario.Scenes),
		"total_tokens", result.TotalTokens,
		"total_ms", result.TotalMs,
	)

	return result, nil
}

func (sp *ScenarioPipeline) runStageWithCheckpoint(ctx context.Context, stage ScenarioPipelineStage, stagesDir string, fn func() (*StageResult, error)) (*StageResult, error) {
	checkpointPath := filepath.Join(stagesDir, string(stage)+".json")

	// Check for existing checkpoint
	if data, err := os.ReadFile(checkpointPath); err == nil {
		var cached StageResult
		if json.Unmarshal(data, &cached) == nil {
			slog.Info("scenario pipeline: resuming from checkpoint",
				"stage", stage,
				"cached_tokens", cached.InputTokens+cached.OutputTokens,
			)
			return &cached, nil
		}
	}

	// Run stage
	result, err := fn()
	if err != nil {
		return nil, err
	}

	// Save checkpoint
	data, _ := json.MarshalIndent(result, "", "  ")
	if writeErr := workspace.WriteFileAtomic(checkpointPath, data); writeErr != nil {
		slog.Warn("scenario pipeline: failed to save checkpoint", "stage", stage, "err", writeErr)
	}

	return result, nil
}

func (sp *ScenarioPipeline) runResearch(ctx context.Context, scpData *workspace.SCPData, factSheet, glossarySection string) (*StageResult, error) {
	tmpl := sp.templates[StageResearch]
	prompt := strings.ReplaceAll(tmpl, "{scp_id}", scpData.SCPID)
	prompt = strings.ReplaceAll(prompt, "{scp_fact_sheet}", factSheet)
	prompt = strings.ReplaceAll(prompt, "{main_text}", scpData.MainText)
	prompt = strings.ReplaceAll(prompt, "{glossary_section}", glossarySection)
	prompt = strings.ReplaceAll(prompt, "{format_guide}", sp.formatGuide)

	return sp.callLLM(ctx, StageResearch, prompt)
}

func (sp *ScenarioPipeline) runStructure(ctx context.Context, scpData *workspace.SCPData, researchPacket, visualRef, glossarySection string) (*StageResult, error) {
	tmpl := sp.templates[StageStructure]
	prompt := strings.ReplaceAll(tmpl, "{scp_id}", scpData.SCPID)
	prompt = strings.ReplaceAll(prompt, "{research_packet}", researchPacket)
	prompt = strings.ReplaceAll(prompt, "{scp_visual_reference}", visualRef)
	prompt = strings.ReplaceAll(prompt, "{target_duration}", fmt.Sprintf("%d", sp.config.TargetDurationMin))
	prompt = strings.ReplaceAll(prompt, "{glossary_section}", glossarySection)
	prompt = strings.ReplaceAll(prompt, "{format_guide}", sp.formatGuide)

	return sp.callLLM(ctx, StageStructure, prompt)
}

func (sp *ScenarioPipeline) runWriting(ctx context.Context, scpData *workspace.SCPData, sceneStructure, visualRef, glossarySection, qualityFeedback string) (*StageResult, error) {
	tmpl := sp.templates[StageWriting]
	prompt := strings.ReplaceAll(tmpl, "{scp_id}", scpData.SCPID)
	prompt = strings.ReplaceAll(prompt, "{scene_structure}", sceneStructure)
	prompt = strings.ReplaceAll(prompt, "{scp_visual_reference}", visualRef)
	prompt = strings.ReplaceAll(prompt, "{glossary_section}", glossarySection)
	prompt = strings.ReplaceAll(prompt, "{format_guide}", sp.formatGuide)
	prompt = strings.ReplaceAll(prompt, "{quality_feedback}", qualityFeedback)

	return sp.callLLM(ctx, StageWriting, prompt)
}

func (sp *ScenarioPipeline) runReview(ctx context.Context, scpData *workspace.SCPData, narrationScript, visualRef, factSheet, glossarySection string) (*StageResult, error) {
	tmpl := sp.templates[StageReview]
	prompt := strings.ReplaceAll(tmpl, "{scp_id}", scpData.SCPID)
	prompt = strings.ReplaceAll(prompt, "{narration_script}", narrationScript)
	prompt = strings.ReplaceAll(prompt, "{scp_visual_reference}", visualRef)
	prompt = strings.ReplaceAll(prompt, "{scp_fact_sheet}", factSheet)
	prompt = strings.ReplaceAll(prompt, "{glossary_section}", glossarySection)
	prompt = strings.ReplaceAll(prompt, "{format_guide}", sp.formatGuide)

	return sp.callLLM(ctx, StageReview, prompt)
}

func (sp *ScenarioPipeline) callLLM(ctx context.Context, stage ScenarioPipelineStage, prompt string) (*StageResult, error) {
	start := time.Now()

	slog.Info("scenario pipeline: starting stage", "stage", stage)

	result, err := sp.llm.Complete(ctx, []llm.Message{
		{Role: "user", Content: prompt},
	}, llm.CompletionOptions{})
	if err != nil {
		return nil, fmt.Errorf("stage %s: %w", stage, err)
	}

	elapsed := time.Since(start)
	slog.Info("scenario pipeline: stage completed",
		"stage", stage,
		"input_tokens", result.InputTokens,
		"output_tokens", result.OutputTokens,
		"duration_ms", elapsed.Milliseconds(),
	)

	return &StageResult{
		Stage:        stage,
		Content:      result.Content,
		InputTokens:  result.InputTokens,
		OutputTokens: result.OutputTokens,
		DurationMs:   elapsed.Milliseconds(),
	}, nil
}

// buildGlossarySection creates a glossary reference section for prompt injection (Story 8-5).
func (sp *ScenarioPipeline) buildGlossarySection(scpData *workspace.SCPData) string {
	if sp.glossary == nil || sp.glossary.Len() == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("## SCP Terminology Reference\n\n")
	b.WriteString("Use these terms consistently:\n\n")
	b.WriteString("| Term | Definition | Korean |\n")
	b.WriteString("|------|-----------|--------|\n")

	for _, entry := range sp.glossary.Entries() {
		korean := entry.Pronunciation
		if korean == "" {
			korean = "-"
		}
		b.WriteString(fmt.Sprintf("| %s | %s | %s |\n", entry.Term, entry.Definition, korean))
	}
	b.WriteString("\n")

	return b.String()
}

// buildFactSheet creates a formatted fact sheet from SCP data.
func (sp *ScenarioPipeline) buildFactSheet(scpData *workspace.SCPData) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("SCP ID: %s\n", scpData.SCPID))
	b.WriteString(fmt.Sprintf("Title: %s\n", scpData.Meta.Title))
	b.WriteString(fmt.Sprintf("Object Class: %s\n", scpData.Meta.ObjectClass))
	b.WriteString(fmt.Sprintf("Series: %s\n\n", scpData.Meta.Series))
	b.WriteString("Facts:\n")
	for k, v := range scpData.Facts.Facts {
		b.WriteString(fmt.Sprintf("- [%s] %s\n", k, v))
	}
	return b.String()
}

// extractVisualIdentity extracts the Visual Identity Profile section from research output.
func extractVisualIdentity(researchContent string) string {
	lines := strings.Split(researchContent, "\n")
	var result []string
	inSection := false

	for _, line := range lines {
		lower := strings.ToLower(line)
		if strings.Contains(lower, "visual identity") || strings.Contains(lower, "frozen descriptor") {
			inSection = true
			result = append(result, line)
			continue
		}
		if inSection {
			// Stop at the next major section header
			if strings.HasPrefix(line, "### ") && !strings.Contains(lower, "visual") {
				break
			}
			result = append(result, line)
		}
	}

	if len(result) == 0 {
		return "(No visual identity extracted from research)"
	}
	return strings.TrimSpace(strings.Join(result, "\n"))
}

// parseScenarioFromWriting parses the Stage 3 writing output into a ScenarioOutput.
func parseScenarioFromWriting(content string, scpID string) (*domain.ScenarioOutput, error) {
	cleaned := extractJSONFromContent(content)

	var raw struct {
		SCPID    string `json:"scp_id"`
		Title    string `json:"title"`
		Scenes   []struct {
			SceneNum    int    `json:"scene_num"`
			Narration   string `json:"narration"`
			VisualDesc  string `json:"visual_description"`
			FactTags    []struct {
				Key     string `json:"key"`
				Content string `json:"content"`
			} `json:"fact_tags"`
			Mood string `json:"mood"`
		} `json:"scenes"`
		Metadata map[string]string `json:"metadata"`
	}

	if err := json.Unmarshal([]byte(cleaned), &raw); err != nil {
		return nil, fmt.Errorf("parse writing output: %w", err)
	}

	scenario := &domain.ScenarioOutput{
		SCPID:    scpID,
		Title:    raw.Title,
		Metadata: raw.Metadata,
	}
	if scenario.Metadata == nil {
		scenario.Metadata = map[string]string{}
	}

	for _, s := range raw.Scenes {
		scene := domain.SceneScript{
			SceneNum:          s.SceneNum,
			Narration:         s.Narration,
			VisualDescription: s.VisualDesc,
			Mood:              s.Mood,
		}
		for _, ft := range s.FactTags {
			scene.FactTags = append(scene.FactTags, domain.FactTag{Key: ft.Key, Content: ft.Content})
		}
		scenario.Scenes = append(scenario.Scenes, scene)
	}

	return scenario, nil
}

// parseReviewReport parses the Stage 4 review output.
func parseReviewReport(content string) (*ReviewReport, error) {
	cleaned := extractJSONFromContent(content)
	var report ReviewReport
	if err := json.Unmarshal([]byte(cleaned), &report); err != nil {
		return nil, fmt.Errorf("parse review report: %w", err)
	}
	return &report, nil
}

// applyCorrections applies review corrections to the scenario.
func applyCorrections(scenario *domain.ScenarioOutput, corrections []ReviewCorrection) *domain.ScenarioOutput {
	if len(corrections) == 0 {
		return scenario
	}

	for _, c := range corrections {
		for i, scene := range scenario.Scenes {
			if scene.SceneNum != c.SceneNum {
				continue
			}
			switch c.Field {
			case "narration":
				if c.Original != "" && c.Corrected != "" {
					scenario.Scenes[i].Narration = strings.ReplaceAll(scene.Narration, c.Original, c.Corrected)
				}
			case "visual_description":
				if c.Original != "" && c.Corrected != "" {
					scenario.Scenes[i].VisualDescription = strings.ReplaceAll(scene.VisualDescription, c.Original, c.Corrected)
				}
			}
		}
	}

	slog.Info("scenario pipeline: applied review corrections", "count", len(corrections))
	return scenario
}

// extractJSONFromContent strips markdown code fences from LLM output.
func extractJSONFromContent(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```json") {
		s = strings.TrimPrefix(s, "```json")
		if idx := strings.LastIndex(s, "```"); idx >= 0 {
			s = s[:idx]
		}
	} else if strings.HasPrefix(s, "```") {
		s = strings.TrimPrefix(s, "```")
		if idx := strings.LastIndex(s, "```"); idx >= 0 {
			s = s[:idx]
		}
	}
	return strings.TrimSpace(s)
}
