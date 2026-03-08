package service

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/sushistack/yt.pipe/internal/glossary"
	"github.com/sushistack/yt.pipe/internal/plugin/llm"
	"github.com/sushistack/yt.pipe/internal/retry"
	"github.com/sushistack/yt.pipe/internal/workspace"
)

// PronunciationService handles Korean pronunciation conversion for TTS.
// Implements a 2-tier approach: deterministic glossary substitution first,
// then LLM for remaining context-dependent conversions.
type PronunciationService struct {
	llm          llm.LLM
	glossary     *glossary.Glossary
	templatePath string
	logger       *slog.Logger
}

// NewPronunciationService creates a new PronunciationService.
func NewPronunciationService(l llm.LLM, g *glossary.Glossary, templatePath string, logger *slog.Logger) *PronunciationService {
	return &PronunciationService{
		llm:          l,
		glossary:     g,
		templatePath: templatePath,
		logger:       logger,
	}
}

// ConvertResult holds the result of pronunciation conversion.
type ConvertResult struct {
	OriginalText  string
	ConvertedText string
	XMLOutput     string
}

// Convert performs 2-tier pronunciation conversion on narration text.
// Tier 1: Deterministic glossary-based substitution.
// Tier 2: LLM-based context-dependent conversion for remaining terms.
func (s *PronunciationService) Convert(ctx context.Context, narration string) (*ConvertResult, error) {
	start := time.Now()

	// Tier 1: Glossary substitution
	glossaryConverted, appliedTerms := s.applyGlossary(narration)

	s.logger.Debug("glossary substitution applied",
		"original_len", len(narration),
		"converted_len", len(glossaryConverted),
		"terms_applied", len(appliedTerms),
	)

	// Tier 2: LLM conversion for remaining terms
	xmlOutput, err := s.llmConvert(ctx, glossaryConverted, appliedTerms)
	if err != nil {
		return nil, fmt.Errorf("pronunciation llm convert: %w", err)
	}

	convertedText := extractNarratorText(xmlOutput)

	elapsed := time.Since(start)
	s.logger.Info("pronunciation conversion complete",
		"original_len", len(narration),
		"converted_len", len(convertedText),
		"glossary_terms", len(appliedTerms),
		"elapsed_ms", elapsed.Milliseconds(),
	)

	return &ConvertResult{
		OriginalText:  narration,
		ConvertedText: convertedText,
		XMLOutput:     xmlOutput,
	}, nil
}

// ConvertAndSave converts narration and saves the XML output to the scene directory.
func (s *PronunciationService) ConvertAndSave(ctx context.Context, narration string, projectPath string, sceneNum int) (*ConvertResult, error) {
	result, err := s.Convert(ctx, narration)
	if err != nil {
		return nil, err
	}

	sceneDir, err := workspace.InitSceneDir(projectPath, sceneNum)
	if err != nil {
		return nil, fmt.Errorf("pronunciation: init scene dir %d: %w", sceneNum, err)
	}

	xmlPath := filepath.Join(sceneDir, "narration_refined.xml")
	if err := workspace.WriteFileAtomic(xmlPath, []byte(result.XMLOutput)); err != nil {
		return nil, fmt.Errorf("pronunciation: save xml %d: %w", sceneNum, err)
	}

	return result, nil
}

// applyGlossary performs tier-1 deterministic substitution using the glossary.
// Returns the converted text and a list of terms that were applied.
func (s *PronunciationService) applyGlossary(text string) (string, []string) {
	if s.glossary == nil {
		return text, nil
	}

	entries := s.glossary.Entries()
	if len(entries) == 0 {
		return text, nil
	}

	result := text
	var applied []string
	for _, e := range entries {
		if e.Pronunciation == "" {
			continue
		}
		if strings.Contains(result, e.Term) {
			result = strings.ReplaceAll(result, e.Term, e.Pronunciation)
			applied = append(applied, fmt.Sprintf("%s → %s", e.Term, e.Pronunciation))
		}
	}
	return result, applied
}

// llmConvert performs tier-2 LLM-based conversion for remaining terms.
func (s *PronunciationService) llmConvert(ctx context.Context, text string, alreadyConverted []string) (string, error) {
	prompt, err := s.buildPrompt(text, alreadyConverted)
	if err != nil {
		return "", fmt.Errorf("build prompt: %w", err)
	}

	var result *llm.CompletionResult
	err = retry.Do(ctx, 3, 1*time.Second, func() error {
		var callErr error
		result, callErr = s.llm.Complete(ctx, []llm.Message{
			{Role: "user", Content: prompt},
		}, llm.CompletionOptions{
			Temperature: 0.1, // Low temperature for deterministic conversion
		})
		return callErr
	})
	if err != nil {
		return "", err
	}

	return result.Content, nil
}

// buildPrompt constructs the LLM prompt from the template.
func (s *PronunciationService) buildPrompt(text string, alreadyConverted []string) (string, error) {
	templateContent, err := s.loadTemplate()
	if err != nil {
		return "", err
	}

	tmpl, err := template.New("scenario_refine").Parse(templateContent)
	if err != nil {
		return "", fmt.Errorf("parse template: %w", err)
	}

	convertedStr := "None"
	if len(alreadyConverted) > 0 {
		convertedStr = strings.Join(alreadyConverted, "\n")
	}

	data := struct {
		Narration        string
		AlreadyConverted string
	}{
		Narration:        text,
		AlreadyConverted: convertedStr,
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute template: %w", err)
	}

	return buf.String(), nil
}

// loadTemplate loads the scenario_refine template from the configured path.
func (s *PronunciationService) loadTemplate() (string, error) {
	path := s.templatePath
	if path == "" {
		path = "templates/tts/scenario_refine.md"
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("load template %s: %w", path, err)
	}
	return string(data), nil
}

// extractNarratorText extracts the narrator text from the XML output.
func extractNarratorText(xml string) string {
	// Extract content between <narrator> tags
	start := strings.Index(xml, "<narrator>")
	end := strings.Index(xml, "</narrator>")
	if start == -1 || end == -1 || end <= start {
		// Fallback: try to extract from <script> tags
		start = strings.Index(xml, "<script>")
		end = strings.Index(xml, "</script>")
		if start == -1 || end == -1 || end <= start {
			return strings.TrimSpace(xml)
		}
		return strings.TrimSpace(xml[start+len("<script>") : end])
	}
	return strings.TrimSpace(xml[start+len("<narrator>") : end])
}
