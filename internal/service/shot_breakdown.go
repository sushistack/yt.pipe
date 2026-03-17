package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/sushistack/yt.pipe/internal/domain"
	"github.com/sushistack/yt.pipe/internal/plugin/llm"
)

// DefaultDangerousTerms are removed from prompts during sanitization.
var DefaultDangerousTerms = []string{
	"gore", "blood", "violent", "gruesome",
	"mutilation", "decapitation", "dismemberment",
}

var multiSpaceRe = regexp.MustCompile(`\s{2,}`)

// ShotBreakdownStage identifies a stage in the 2-stage image prompt pipeline.
type ShotBreakdownStage string

const (
	StageShotBreakdown ShotBreakdownStage = "01_shot_breakdown"
	StageShotToPrompt  ShotBreakdownStage = "02_shot_to_prompt"
)

// maxCutsPerSentence is the guard limit for split cuts.
const maxCutsPerSentence = 3

// CutDescription holds the structured output from Stage 1 (scene-batched cut decomposition).
type CutDescription struct {
	SentenceStart int    `json:"sentence_start"`
	SentenceEnd   int    `json:"sentence_end"`
	CutNum        int    `json:"cut_num"`
	VisualBeat    string `json:"visual_beat"`
	Role          string `json:"role"`
	CameraType    string `json:"camera_type"`
	EntityVisible bool   `json:"entity_visible"`
	Subject       string `json:"subject"`
	Lighting      string `json:"lighting"`
	Mood          string `json:"mood"`
	Motion        string `json:"motion"`
}

// CutPromptResult holds the structured output from Stage 2 (cut-to-prompt).
type CutPromptResult struct {
	Prompt         string `json:"prompt"`
	NegativePrompt string `json:"negative_prompt"`
	EntityVisible  bool   `json:"entity_visible"`
}

// CutOutput holds the pipeline output for a single cut.
type CutOutput struct {
	SentenceStart  int
	SentenceEnd    int
	CutNum         int
	CutDesc        *CutDescription
	PromptResult   *CutPromptResult
	FinalPrompt    string
	NegativePrompt string
	SentenceText   string // narration text covered by this cut
}

// SceneCutOutput holds the complete output from the pipeline for a scene.
type SceneCutOutput struct {
	SceneNum int
	Cuts     []CutOutput
}

// SceneCutInput holds all input data needed to generate cut prompts for a scene.
type SceneCutInput struct {
	SceneNum             int
	Narration            string
	Mood                 string
	Location             string
	CharactersPresent    []string
	ColorPalette         string
	Atmosphere           string
	EntityVisualIdentity string
	FrozenDescriptor     string
	StyleGuide           string
	StyleConfig          domain.StyleConfig
	PreviousLastCutCtx   string
}

// Legacy types kept for backward compatibility with existing callers.

// ShotDescription holds the structured output from legacy Stage 1 (shot breakdown).
type ShotDescription struct {
	ShotNumber    int    `json:"shot_number"`
	Role          string `json:"role"`
	CameraType    string `json:"camera_type"`
	EntityVisible bool   `json:"entity_visible"`
	Subject       string `json:"subject"`
	Lighting      string `json:"lighting"`
	Mood          string `json:"mood"`
	Motion        string `json:"motion"`
}

// ShotPromptResult holds the structured output from legacy Stage 2 (shot-to-prompt).
type ShotPromptResult struct {
	Prompt         string `json:"prompt"`
	NegativePrompt string `json:"negative_prompt"`
	EntityVisible  bool   `json:"entity_visible"`
}

// ShotBreakdownConfig configures the 2-stage image prompt pipeline.
type ShotBreakdownConfig struct {
	TemplatesDir string
}

// ShotBreakdownPipeline orchestrates the 2-stage image prompt generation:
// Stage 1: Scene-batched bidirectional cut decomposition
// Stage 2: Per-cut prompt generation
type ShotBreakdownPipeline struct {
	llm       llm.LLM
	templates map[ShotBreakdownStage]string
}

// NewShotBreakdownPipeline creates a new 2-stage image prompt pipeline.
func NewShotBreakdownPipeline(l llm.LLM, cfg ShotBreakdownConfig) (*ShotBreakdownPipeline, error) {
	sp := &ShotBreakdownPipeline{
		llm:       l,
		templates: make(map[ShotBreakdownStage]string),
	}

	stages := []ShotBreakdownStage{StageShotBreakdown, StageShotToPrompt}
	for _, stage := range stages {
		path := filepath.Join(cfg.TemplatesDir, "image", string(stage)+".md")
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("shot breakdown pipeline: load template %s: %w", stage, err)
		}
		sp.templates[stage] = string(data)
	}

	return sp, nil
}

// SentencePromptInput holds input for generating a shot description for one sentence (legacy).
type SentencePromptInput struct {
	SceneNum             int
	ShotNum              int
	TotalShots           int
	Sentence             string
	EmotionalBeat        string
	EntityVisualIdentity string
	FrozenDescriptor     string
	PreviousShotCtx      string
}

// ScenePromptInput holds all input data needed to generate image prompts for a scene (legacy).
type ScenePromptInput struct {
	SceneNum             int
	Synopsis             string
	EmotionalBeat        string
	EntityVisualIdentity string
	FrozenDescriptor     string
	PreviousLastShotCtx  string
}

// ShotOutput holds the pipeline output for a single shot (legacy).
type ShotOutput struct {
	ShotNum        int              `json:"shot_num"`
	ShotDesc       *ShotDescription `json:"shot_description"`
	PromptResult   *ShotPromptResult `json:"prompt_result"`
	FinalPrompt    string           `json:"final_prompt"`
	NegativePrompt string           `json:"negative_prompt"`
	SentenceText   string           `json:"sentence_text"`
}

// ScenePromptOutput holds the complete output from the legacy pipeline for a scene.
type ScenePromptOutput struct {
	SceneNum int          `json:"scene_num"`
	Shots    []ShotOutput `json:"shots"`
}

// GenerateSceneCuts runs the new 2-stage pipeline for a single scene.
// Stage 1: Single LLM call for all sentences → []CutDescription
// Stage 2: Per-cut LLM call → CutPromptResult
func (sp *ShotBreakdownPipeline) GenerateSceneCuts(ctx context.Context, input SceneCutInput) (*SceneCutOutput, error) {
	start := time.Now()

	sentences := domain.SplitNarrationSentences(input.Narration)
	if len(sentences) == 0 {
		return nil, fmt.Errorf("scene %d: no sentences in narration", input.SceneNum)
	}

	// Stage 1: Scene-batched bidirectional cut decomposition
	cutDescs, err := sp.runSceneCutDecomposition(ctx, input, sentences)
	if err != nil {
		return nil, fmt.Errorf("scene %d: cut decomposition: %w", input.SceneNum, err)
	}

	// Validate and enforce max-cuts-per-sentence guard
	cutDescs = validateCutDescriptions(cutDescs, len(sentences))

	output := &SceneCutOutput{
		SceneNum: input.SceneNum,
		Cuts:     make([]CutOutput, 0, len(cutDescs)),
	}

	// Resolve style suffix for sanitization
	styleSuffix := input.StyleConfig.StyleSuffix
	artStyle := input.StyleConfig.ArtStyle

	_ = input.PreviousLastCutCtx // context continuity tracked at GenerateAllSceneCuts level

	// Stage 2: Per-cut prompt generation
	for _, desc := range cutDescs {
		if err := ctx.Err(); err != nil {
			return output, fmt.Errorf("scene %d cut %d_%d: cancelled: %w",
				input.SceneNum, desc.SentenceStart, desc.CutNum, err)
		}

		sentText := joinSentenceRange(sentences, desc.SentenceStart, desc.SentenceEnd)

		promptResult, err := sp.runCutToPrompt(ctx, &desc, input.FrozenDescriptor, input.StyleGuide, artStyle, input.ColorPalette, input.Atmosphere)
		if err != nil {
			slog.Error("cut-to-prompt failed",
				"scene_num", input.SceneNum,
				"sentence_start", desc.SentenceStart,
				"cut_num", desc.CutNum,
				"err", err)
			descCopy := desc
			output.Cuts = append(output.Cuts, CutOutput{
				SentenceStart: desc.SentenceStart,
				SentenceEnd:   desc.SentenceEnd,
				CutNum:        desc.CutNum,
				CutDesc:       &descCopy,
				SentenceText:  sentText,
			})
			continue
		}

		finalPrompt := sanitizeImagePrompt(promptResult.Prompt, styleSuffix)

		descCopy := desc
		output.Cuts = append(output.Cuts, CutOutput{
			SentenceStart:  desc.SentenceStart,
			SentenceEnd:    desc.SentenceEnd,
			CutNum:         desc.CutNum,
			CutDesc:        &descCopy,
			PromptResult:   promptResult,
			FinalPrompt:    finalPrompt,
			NegativePrompt: promptResult.NegativePrompt,
			SentenceText:   sentText,
		})

	}

	elapsed := time.Since(start)
	slog.Info("cut decomposition pipeline completed",
		"scene_num", input.SceneNum,
		"cuts", len(output.Cuts),
		"sentences", len(sentences),
		"duration_ms", elapsed.Milliseconds(),
	)

	return output, nil
}

// GenerateAllSceneCuts runs the pipeline for all scenes sequentially.
func (sp *ShotBreakdownPipeline) GenerateAllSceneCuts(ctx context.Context, inputs []SceneCutInput) ([]*SceneCutOutput, error) {
	results := make([]*SceneCutOutput, 0, len(inputs))
	previousCtx := "(first scene - no previous context)"

	for i := range inputs {
		if err := ctx.Err(); err != nil {
			return results, fmt.Errorf("cut decomposition cancelled: %w", err)
		}

		inputs[i].PreviousLastCutCtx = previousCtx

		output, err := sp.GenerateSceneCuts(ctx, inputs[i])
		if err != nil {
			slog.Error("cut decomposition failed for scene", "scene_num", inputs[i].SceneNum, "err", err)
			results = append(results, nil)
			continue
		}

		results = append(results, output)
		if len(output.Cuts) > 0 {
			lastCut := output.Cuts[len(output.Cuts)-1]
			previousCtx = formatCutContext(lastCut.CutDesc)
		}
	}

	return results, nil
}

// GenerateScenePrompt runs the legacy 2-stage pipeline for a single scene.
// Splits narration into sentences and generates one shot per sentence.
func (sp *ShotBreakdownPipeline) GenerateScenePrompt(ctx context.Context, input ScenePromptInput) (*ScenePromptOutput, error) {
	start := time.Now()

	sentences := domain.SplitNarrationSentences(input.Synopsis)
	if len(sentences) == 0 {
		return nil, fmt.Errorf("scene %d: no sentences in narration", input.SceneNum)
	}

	output := &ScenePromptOutput{
		SceneNum: input.SceneNum,
		Shots:    make([]ShotOutput, 0, len(sentences)),
	}

	previousCtx := input.PreviousLastShotCtx

	for i, sentence := range sentences {
		if err := ctx.Err(); err != nil {
			return output, fmt.Errorf("scene %d shot %d: cancelled: %w", input.SceneNum, i+1, err)
		}

		sentInput := SentencePromptInput{
			SceneNum:             input.SceneNum,
			ShotNum:              i + 1,
			TotalShots:           len(sentences),
			Sentence:             sentence,
			EmotionalBeat:        input.EmotionalBeat,
			EntityVisualIdentity: input.EntityVisualIdentity,
			FrozenDescriptor:     input.FrozenDescriptor,
			PreviousShotCtx:      previousCtx,
		}

		// Stage 1: Shot Breakdown
		shotDesc, err := sp.runShotBreakdown(ctx, sentInput)
		if err != nil {
			slog.Error("shot breakdown failed", "scene_num", input.SceneNum, "shot_num", i+1, "err", err)
			output.Shots = append(output.Shots, ShotOutput{
				ShotNum:      i + 1,
				SentenceText: sentence,
			})
			continue
		}

		// Stage 2: Shot-to-Prompt
		promptResult, err := sp.runShotToPrompt(ctx, shotDesc, input.FrozenDescriptor)
		if err != nil {
			slog.Error("shot-to-prompt failed", "scene_num", input.SceneNum, "shot_num", i+1, "err", err)
			output.Shots = append(output.Shots, ShotOutput{
				ShotNum:      i + 1,
				ShotDesc:     shotDesc,
				SentenceText: sentence,
			})
			continue
		}

		finalPrompt := sanitizeImagePrompt(promptResult.Prompt, "")

		output.Shots = append(output.Shots, ShotOutput{
			ShotNum:        i + 1,
			ShotDesc:       shotDesc,
			PromptResult:   promptResult,
			FinalPrompt:    finalPrompt,
			NegativePrompt: promptResult.NegativePrompt,
			SentenceText:   sentence,
		})

		previousCtx = formatShotContext(shotDesc)
	}

	elapsed := time.Since(start)
	slog.Info("shot breakdown pipeline completed",
		"scene_num", input.SceneNum,
		"shots", len(output.Shots),
		"duration_ms", elapsed.Milliseconds(),
	)

	return output, nil
}

// GenerateAllScenePrompts runs the legacy pipeline for all scenes sequentially.
func (sp *ShotBreakdownPipeline) GenerateAllScenePrompts(ctx context.Context, scenario *domain.ScenarioOutput, frozenDescriptor, visualIdentity string) ([]*ScenePromptOutput, error) {
	results := make([]*ScenePromptOutput, 0, len(scenario.Scenes))
	previousCtx := "(first scene - no previous context)"

	for _, scene := range scenario.Scenes {
		if err := ctx.Err(); err != nil {
			return results, fmt.Errorf("shot breakdown cancelled: %w", err)
		}

		input := ScenePromptInput{
			SceneNum:             scene.SceneNum,
			Synopsis:             scene.Narration,
			EmotionalBeat:        scene.Mood,
			EntityVisualIdentity: visualIdentity,
			FrozenDescriptor:     frozenDescriptor,
			PreviousLastShotCtx:  previousCtx,
		}

		output, err := sp.GenerateScenePrompt(ctx, input)
		if err != nil {
			slog.Error("shot breakdown failed for scene", "scene_num", scene.SceneNum, "err", err)
			results = append(results, nil)
			continue
		}

		results = append(results, output)
		if len(output.Shots) > 0 {
			lastShot := output.Shots[len(output.Shots)-1]
			previousCtx = formatShotContext(lastShot.ShotDesc)
		}
	}

	return results, nil
}

// runSceneCutDecomposition executes Stage 1 — a single LLM call for the entire scene.
func (sp *ShotBreakdownPipeline) runSceneCutDecomposition(ctx context.Context, input SceneCutInput, sentences []string) ([]CutDescription, error) {
	sentencesJSON, err := json.Marshal(buildNumberedSentences(sentences))
	if err != nil {
		return nil, fmt.Errorf("marshal sentences: %w", err)
	}

	tmpl := sp.templates[StageShotBreakdown]
	prompt := strings.ReplaceAll(tmpl, "{entity_visual_identity}", input.EntityVisualIdentity)
	prompt = strings.ReplaceAll(prompt, "{frozen_descriptor}", input.FrozenDescriptor)
	prompt = strings.ReplaceAll(prompt, "{scene_number}", fmt.Sprintf("%d", input.SceneNum))
	prompt = strings.ReplaceAll(prompt, "{full_narration}", input.Narration)
	prompt = strings.ReplaceAll(prompt, "{sentences_json}", string(sentencesJSON))
	prompt = strings.ReplaceAll(prompt, "{scene_location}", input.Location)
	prompt = strings.ReplaceAll(prompt, "{scene_characters}", strings.Join(input.CharactersPresent, ", "))
	prompt = strings.ReplaceAll(prompt, "{scene_palette}", input.ColorPalette)
	prompt = strings.ReplaceAll(prompt, "{scene_atmosphere}", input.Atmosphere)
	prompt = strings.ReplaceAll(prompt, "{scene_mood}", input.Mood)
	prompt = strings.ReplaceAll(prompt, "{style_guide}", input.StyleGuide)
	prompt = strings.ReplaceAll(prompt, "{previous_scene_last_cut_context}", input.PreviousLastCutCtx)

	result, err := sp.llm.Complete(ctx, []llm.Message{
		{Role: "user", Content: prompt},
	}, llm.CompletionOptions{})
	if err != nil {
		return nil, err
	}

	cleaned := extractJSONFromContent(result.Content)
	cleaned = repairJSON(cleaned)

	var cuts []CutDescription
	if err := json.Unmarshal([]byte(cleaned), &cuts); err != nil {
		return nil, fmt.Errorf("parse cut decomposition: %w", err)
	}

	return cuts, nil
}

// runCutToPrompt executes Stage 2 for a single cut.
func (sp *ShotBreakdownPipeline) runCutToPrompt(ctx context.Context, cut *CutDescription, frozenDescriptor, styleGuide, artStyle, palette, atmosphere string) (*CutPromptResult, error) {
	cutJSON, err := json.Marshal(cut)
	if err != nil {
		return nil, fmt.Errorf("marshal cut: %w", err)
	}

	if artStyle == "" {
		artStyle = "dark horror anime illustration"
	}

	tmpl := sp.templates[StageShotToPrompt]
	prompt := strings.ReplaceAll(tmpl, "{shot_json}", string(cutJSON))
	prompt = strings.ReplaceAll(prompt, "{frozen_descriptor}", frozenDescriptor)
	prompt = strings.ReplaceAll(prompt, "{style_guide}", styleGuide)
	prompt = strings.ReplaceAll(prompt, "{art_style}", artStyle)
	prompt = strings.ReplaceAll(prompt, "{scene_palette}", palette)
	prompt = strings.ReplaceAll(prompt, "{scene_atmosphere}", atmosphere)

	result, err := sp.llm.Complete(ctx, []llm.Message{
		{Role: "user", Content: prompt},
	}, llm.CompletionOptions{})
	if err != nil {
		return nil, err
	}

	var promptResult CutPromptResult
	cleaned := extractJSONFromContent(result.Content)
	cleaned = repairJSON(cleaned)
	if err := json.Unmarshal([]byte(cleaned), &promptResult); err != nil {
		return nil, fmt.Errorf("parse cut-to-prompt: %w", err)
	}

	return &promptResult, nil
}

// runShotBreakdown executes legacy Stage 1 per-sentence.
func (sp *ShotBreakdownPipeline) runShotBreakdown(ctx context.Context, input SentencePromptInput) (*ShotDescription, error) {
	tmpl := sp.templates[StageShotBreakdown]
	prompt := strings.ReplaceAll(tmpl, "{entity_visual_identity}", input.EntityVisualIdentity)
	prompt = strings.ReplaceAll(prompt, "{frozen_descriptor}", input.FrozenDescriptor)
	prompt = strings.ReplaceAll(prompt, "{scene_number}", fmt.Sprintf("%d", input.SceneNum))
	prompt = strings.ReplaceAll(prompt, "{shot_number}", fmt.Sprintf("%d", input.ShotNum))
	prompt = strings.ReplaceAll(prompt, "{total_shots}", fmt.Sprintf("%d", input.TotalShots))
	prompt = strings.ReplaceAll(prompt, "{sentence}", input.Sentence)
	prompt = strings.ReplaceAll(prompt, "{emotional_beat}", input.EmotionalBeat)
	prompt = strings.ReplaceAll(prompt, "{previous_shot_context}", input.PreviousShotCtx)

	// Fill new template placeholders with empty values for legacy path
	prompt = strings.ReplaceAll(prompt, "{full_narration}", input.Sentence)
	prompt = strings.ReplaceAll(prompt, "{sentences_json}", "[]")
	prompt = strings.ReplaceAll(prompt, "{scene_location}", "")
	prompt = strings.ReplaceAll(prompt, "{scene_characters}", "")
	prompt = strings.ReplaceAll(prompt, "{scene_palette}", "")
	prompt = strings.ReplaceAll(prompt, "{scene_atmosphere}", "")
	prompt = strings.ReplaceAll(prompt, "{scene_mood}", input.EmotionalBeat)
	prompt = strings.ReplaceAll(prompt, "{style_guide}", "")
	prompt = strings.ReplaceAll(prompt, "{previous_scene_last_cut_context}", input.PreviousShotCtx)

	result, err := sp.llm.Complete(ctx, []llm.Message{
		{Role: "user", Content: prompt},
	}, llm.CompletionOptions{})
	if err != nil {
		return nil, err
	}

	var shot ShotDescription
	cleaned := extractJSONFromContent(result.Content)
	if err := json.Unmarshal([]byte(cleaned), &shot); err != nil {
		return nil, fmt.Errorf("parse shot breakdown: %w", err)
	}

	return &shot, nil
}

// runShotToPrompt executes legacy Stage 2 per-shot.
func (sp *ShotBreakdownPipeline) runShotToPrompt(ctx context.Context, shot *ShotDescription, frozenDescriptor string) (*ShotPromptResult, error) {
	shotJSON, err := json.Marshal(shot)
	if err != nil {
		return nil, fmt.Errorf("marshal shot: %w", err)
	}

	tmpl := sp.templates[StageShotToPrompt]
	prompt := strings.ReplaceAll(tmpl, "{shot_json}", string(shotJSON))
	prompt = strings.ReplaceAll(prompt, "{frozen_descriptor}", frozenDescriptor)

	// Fill new template placeholders with defaults for legacy path
	prompt = strings.ReplaceAll(prompt, "{style_guide}", "")
	prompt = strings.ReplaceAll(prompt, "{art_style}", "dark horror anime illustration")
	prompt = strings.ReplaceAll(prompt, "{scene_palette}", "")
	prompt = strings.ReplaceAll(prompt, "{scene_atmosphere}", "")

	result, err := sp.llm.Complete(ctx, []llm.Message{
		{Role: "user", Content: prompt},
	}, llm.CompletionOptions{})
	if err != nil {
		return nil, err
	}

	var promptResult ShotPromptResult
	cleaned := extractJSONFromContent(result.Content)
	if err := json.Unmarshal([]byte(cleaned), &promptResult); err != nil {
		return nil, fmt.Errorf("parse shot-to-prompt: %w", err)
	}

	return &promptResult, nil
}

// sanitizeImagePrompt applies safety sanitization to image prompts.
// styleSuffix replaces the legacy hardcoded anime suffix. If empty, uses DefaultStyleSuffix.
func sanitizeImagePrompt(prompt string, styleSuffix string) string {
	sanitized := prompt
	for _, term := range DefaultDangerousTerms {
		lower := strings.ToLower(sanitized)
		termLower := strings.ToLower(term)
		for {
			idx := strings.Index(lower, termLower)
			if idx < 0 {
				break
			}
			sanitized = sanitized[:idx] + sanitized[idx+len(term):]
			lower = strings.ToLower(sanitized)
		}
	}
	sanitized = multiSpaceRe.ReplaceAllString(sanitized, " ")
	sanitized = strings.TrimSpace(sanitized)

	if styleSuffix == "" {
		styleSuffix = domain.DefaultStyleSuffix
	}

	// Append style suffix if not already present (check first few words)
	suffixKey := strings.Split(styleSuffix, ",")[0]
	if !strings.Contains(strings.ToLower(sanitized), strings.ToLower(strings.TrimSpace(suffixKey))) {
		sanitized = sanitized + ", " + styleSuffix
	}

	return sanitized
}

// formatCutContext formats a cut description for use as previous context.
func formatCutContext(cut *CutDescription) string {
	if cut == nil {
		return "(no previous cut)"
	}
	return fmt.Sprintf("Camera: %s, Subject: %s, Lighting: %s, Mood: %s",
		cut.CameraType, cut.Subject, cut.Lighting, cut.Mood)
}

// formatShotContext formats a shot description for use as previous context (legacy).
func formatShotContext(shot *ShotDescription) string {
	if shot == nil {
		return "(no previous shot)"
	}
	return fmt.Sprintf("Camera: %s, Subject: %s, Lighting: %s, Mood: %s",
		shot.CameraType, shot.Subject, shot.Lighting, shot.Mood)
}

// maxCutsPerMerge is the guard limit for merged-range cuts.
const maxCutsPerMerge = 5

// validateCutDescriptions enforces max-cuts-per-sentence, valid sentence ranges, and no overlaps.
func validateCutDescriptions(cuts []CutDescription, totalSentences int) []CutDescription {
	sentenceCutCount := make(map[int]int)
	var valid []CutDescription

	for _, c := range cuts {
		if c.SentenceStart < 1 {
			c.SentenceStart = 1
		}
		if c.SentenceEnd < c.SentenceStart {
			c.SentenceEnd = c.SentenceStart
		}
		if c.SentenceStart > totalSentences {
			slog.Warn("cut references out-of-range sentence, skipping",
				"sentence_start", c.SentenceStart, "total", totalSentences)
			continue
		}
		if c.SentenceEnd > totalSentences {
			c.SentenceEnd = totalSentences
		}
		if c.CutNum < 1 {
			c.CutNum = 1
		}

		// Max-cuts guard: per-sentence for splits, per-merge for merged ranges
		if c.SentenceStart == c.SentenceEnd {
			sentenceCutCount[c.SentenceStart]++
			if sentenceCutCount[c.SentenceStart] > maxCutsPerSentence {
				slog.Warn("max cuts per sentence exceeded, discarding",
					"sentence", c.SentenceStart, "cut_num", c.CutNum)
				continue
			}
		} else {
			// Count merged cuts by their start sentence
			sentenceCutCount[c.SentenceStart]++
			if sentenceCutCount[c.SentenceStart] > maxCutsPerMerge {
				slog.Warn("max cuts per merge range exceeded, discarding",
					"sentence_start", c.SentenceStart, "sentence_end", c.SentenceEnd, "cut_num", c.CutNum)
				continue
			}
		}

		valid = append(valid, c)
	}

	// Check for sentence gaps and fill with warning
	covered := make(map[int]bool)
	for _, c := range valid {
		for s := c.SentenceStart; s <= c.SentenceEnd; s++ {
			covered[s] = true
		}
	}
	for s := 1; s <= totalSentences; s++ {
		if !covered[s] {
			slog.Warn("sentence not covered by any cut, adding fallback",
				"sentence", s, "total", totalSentences)
			valid = append(valid, CutDescription{
				SentenceStart: s,
				SentenceEnd:   s,
				CutNum:        1,
				VisualBeat:    "fallback",
				Role:          "detail",
				CameraType:    "medium",
			})
		}
	}

	return valid
}

// repairJSON attempts to fix common malformed JSON from LLM output.
// Handles trailing commas and unclosed brackets (string-aware counting).
func repairJSON(s string) string {
	s = strings.TrimSpace(s)

	// Remove trailing commas before ] or }
	s = regexp.MustCompile(`,\s*]`).ReplaceAllString(s, "]")
	s = regexp.MustCompile(`,\s*}`).ReplaceAllString(s, "}")

	// String-aware bracket counting to avoid miscounting brackets inside strings
	openBrackets, openBraces := countUnmatchedBrackets(s)
	for openBrackets > 0 {
		s += "]"
		openBrackets--
	}
	for openBraces > 0 {
		s += "}"
		openBraces--
	}

	return s
}

// countUnmatchedBrackets counts unmatched [ and { outside of JSON strings.
func countUnmatchedBrackets(s string) (brackets, braces int) {
	inString := false
	escaped := false
	for _, r := range s {
		if escaped {
			escaped = false
			continue
		}
		if r == '\\' && inString {
			escaped = true
			continue
		}
		if r == '"' {
			inString = !inString
			continue
		}
		if inString {
			continue
		}
		switch r {
		case '[':
			brackets++
		case ']':
			brackets--
		case '{':
			braces++
		case '}':
			braces--
		}
	}
	if brackets < 0 {
		brackets = 0
	}
	if braces < 0 {
		braces = 0
	}
	return
}

type numberedSentence struct {
	Num  int    `json:"num"`
	Text string `json:"text"`
}

// joinSentenceRange extracts and joins sentences covered by a cut (1-based indices).
func joinSentenceRange(sentences []string, start, end int) string {
	if start < 1 || start > len(sentences) {
		return ""
	}
	if end > len(sentences) {
		end = len(sentences)
	}
	if end < start {
		end = start
	}
	return strings.Join(sentences[start-1:end], " ")
}

func buildNumberedSentences(sentences []string) []numberedSentence {
	result := make([]numberedSentence, len(sentences))
	for i, s := range sentences {
		result[i] = numberedSentence{Num: i + 1, Text: s}
	}
	return result
}
