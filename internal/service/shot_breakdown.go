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

// ShotDescription holds the structured output from Stage 1 (shot breakdown).
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

// ShotPromptResult holds the structured output from Stage 2 (shot-to-prompt).
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
// Shot Breakdown → Shot-to-Prompt.
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

// SentencePromptInput holds input for generating a shot description for one sentence.
type SentencePromptInput struct {
	SceneNum             int
	ShotNum              int    // 1-based
	TotalShots           int    // total sentences in this scene
	Sentence             string // the specific narration sentence
	EmotionalBeat        string
	EntityVisualIdentity string
	FrozenDescriptor     string
	PreviousShotCtx      string // previous shot's context (within same scene or last shot of prev scene)
}

// ScenePromptInput holds all input data needed to generate image prompts for a scene.
type ScenePromptInput struct {
	SceneNum             int
	Synopsis             string // narration text — split into sentences internally
	EmotionalBeat        string // mood
	EntityVisualIdentity string // full visual identity profile from research
	FrozenDescriptor     string // locked descriptor text
	PreviousLastShotCtx  string // previous scene's last shot context for continuity
}

// ShotOutput holds the pipeline output for a single shot (sentence).
type ShotOutput struct {
	ShotNum        int              `json:"shot_num"`
	ShotDesc       *ShotDescription `json:"shot_description"`
	PromptResult   *ShotPromptResult `json:"prompt_result"`
	FinalPrompt    string           `json:"final_prompt"`
	NegativePrompt string           `json:"negative_prompt"`
	SentenceText   string           `json:"sentence_text"`
}

// ScenePromptOutput holds the complete output from the 2-stage pipeline for a scene.
type ScenePromptOutput struct {
	SceneNum int          `json:"scene_num"`
	Shots    []ShotOutput `json:"shots"`
}

// GenerateScenePrompt runs the 2-stage pipeline for a single scene.
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

		finalPrompt := sanitizeImagePrompt(promptResult.Prompt)

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

// GenerateAllScenePrompts runs the 2-stage pipeline for all scenes sequentially,
// carrying previous shot context for visual continuity.
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
		// Carry last shot's context to next scene
		if len(output.Shots) > 0 {
			lastShot := output.Shots[len(output.Shots)-1]
			previousCtx = formatShotContext(lastShot.ShotDesc)
		}
	}

	return results, nil
}

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

func (sp *ShotBreakdownPipeline) runShotToPrompt(ctx context.Context, shot *ShotDescription, frozenDescriptor string) (*ShotPromptResult, error) {
	shotJSON, err := json.Marshal(shot)
	if err != nil {
		return nil, fmt.Errorf("marshal shot: %w", err)
	}

	tmpl := sp.templates[StageShotToPrompt]
	prompt := strings.ReplaceAll(tmpl, "{shot_json}", string(shotJSON))
	prompt = strings.ReplaceAll(prompt, "{frozen_descriptor}", frozenDescriptor)

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

// sanitizeImagePrompt applies safety sanitization to image prompts for SCP content.
func sanitizeImagePrompt(prompt string) string {
	sanitized := prompt
	for _, term := range DefaultDangerousTerms {
		// Case-insensitive removal without lowercasing the entire prompt
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

	// Ensure anime style suffix is present
	suffix := "anime illustration, dark horror anime style, highly detailed, vibrant colors, cel shading, sharp lines, dramatic lighting, 16:9 aspect ratio"
	if !strings.Contains(strings.ToLower(sanitized), "anime") {
		sanitized = sanitized + ", " + suffix
	}

	return sanitized
}

// formatShotContext formats a shot description for use as previous context.
func formatShotContext(shot *ShotDescription) string {
	if shot == nil {
		return "(no previous shot)"
	}
	return fmt.Sprintf("Camera: %s, Subject: %s, Lighting: %s, Mood: %s",
		shot.CameraType, shot.Subject, shot.Lighting, shot.Mood)
}
