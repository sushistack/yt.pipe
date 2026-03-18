package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/sushistack/yt.pipe/internal/domain"
	"github.com/sushistack/yt.pipe/internal/plugin/imagegen"
	"github.com/sushistack/yt.pipe/internal/plugin/llm"
	"github.com/sushistack/yt.pipe/internal/store"
)

// RegenerateFn is a callback that regenerates the current image.
// It is used by ValidateAndRegenerate to avoid circular dependency
// between ImageValidatorService and ImageGenService.
type RegenerateFn func(ctx context.Context) error

// ImageValidatorService evaluates generated images via multimodal LLM.
type ImageValidatorService struct {
	llm    llm.LLM
	store  *store.Store
	logger *slog.Logger
}

// NewImageValidatorService creates a new ImageValidatorService.
// The store parameter is optional (may be nil) — when provided, final
// validation scores are persisted to shot_manifests.
func NewImageValidatorService(l llm.LLM, s *store.Store, logger *slog.Logger) *ImageValidatorService {
	return &ImageValidatorService{llm: l, store: s, logger: logger}
}

// validationResponse is the expected JSON structure from the LLM.
type validationResponse struct {
	PromptMatch    int      `json:"prompt_match"`
	CharacterMatch int      `json:"character_match"`
	TechnicalScore int      `json:"technical_score"`
	Reasons        []string `json:"reasons"`
}

// ValidateImage evaluates a generated image against its prompt and character references.
// Returns nil, nil when the LLM does not support vision (ErrNotSupported).
func (s *ImageValidatorService) ValidateImage(
	ctx context.Context,
	imagePath string,
	originalPrompt string,
	characterRefs []imagegen.CharacterRef,
) (*domain.ValidationResult, error) {
	// Check image file exists before calling LLM
	if _, err := os.Stat(imagePath); err != nil {
		return nil, fmt.Errorf("image file not found: %s", imagePath)
	}

	// Read and base64-encode image
	data, err := os.ReadFile(imagePath)
	if err != nil {
		return nil, fmt.Errorf("read image file: %w", err)
	}

	mime := detectMIME(filepath.Ext(imagePath))
	dataURI := fmt.Sprintf("data:%s;base64,%s", mime, base64.StdEncoding.EncodeToString(data))

	// Build character description
	charDesc := "None"
	hasCharacter := len(characterRefs) > 0
	if hasCharacter {
		var parts []string
		for _, cr := range characterRefs {
			parts = append(parts, fmt.Sprintf("- %s: %s", cr.Name, cr.VisualDescriptor))
		}
		charDesc = strings.Join(parts, "\n")
	}

	// Build evaluation prompt
	systemMsg := llm.VisionMessage{
		Role: "system",
		Content: []llm.ContentPart{
			{Type: "text", Text: `You are an image quality evaluator for SCP content.
Evaluate the image against the following criteria:
1. Prompt consistency (0-100): Does the image match the visual description?
2. Character appearance (0-100): Does the character match the reference? (-1 if no character)
3. Technical quality (0-100): Are there distortions, artifacts, or rendering errors?

Return ONLY valid JSON: {"prompt_match": N, "character_match": N, "technical_score": N, "reasons": ["..."]}`},
		},
	}

	userMsg := llm.VisionMessage{
		Role: "user",
		Content: []llm.ContentPart{
			{Type: "text", Text: fmt.Sprintf("Original prompt: %s\nCharacter references: %s", originalPrompt, charDesc)},
			{Type: "image_url", ImageURL: dataURI},
		},
	}

	result, err := s.llm.CompleteWithVision(ctx, []llm.VisionMessage{systemMsg, userMsg}, llm.CompletionOptions{
		Temperature: 0.1,
		MaxTokens:   512,
	})
	if err != nil {
		if errors.Is(err, llm.ErrNotSupported) {
			s.logger.Warn("vision not supported by LLM provider, skipping validation")
			return nil, nil
		}
		return nil, fmt.Errorf("llm vision call: %w", err)
	}

	// Parse JSON response
	cleaned := extractValidationJSON(result.Content)
	var resp validationResponse
	if err := json.Unmarshal([]byte(cleaned), &resp); err != nil {
		return nil, fmt.Errorf("parse validation response: %w (raw: %s)", err, result.Content)
	}

	vr := &domain.ValidationResult{
		PromptMatch:    resp.PromptMatch,
		CharacterMatch: resp.CharacterMatch,
		TechnicalScore: resp.TechnicalScore,
		Reasons:        resp.Reasons,
	}

	// Override character match for no-character scenes
	if !hasCharacter {
		vr.CharacterMatch = -1
	}

	return vr, nil
}

// ValidateAndRegenerate validates an image and regenerates it if quality is
// below threshold, up to maxAttempts times. The best-scoring result is kept.
// Returns nil, nil when the LLM does not support vision (skipped).
//
// The regenerateFn callback is invoked to regenerate the image, avoiding
// a circular dependency between ImageValidatorService and ImageGenService.
// After regeneration, the same imagePath is re-read and re-validated.
//
// The final validation score is persisted to shot_manifests via the store.
func (s *ImageValidatorService) ValidateAndRegenerate(
	ctx context.Context,
	imagePath string,
	originalPrompt string,
	characterRefs []imagegen.CharacterRef,
	threshold int,
	maxAttempts int,
	projectID string,
	sceneNum int,
	sentenceStart int,
	cutNum int,
	regenerateFn RegenerateFn,
) (*domain.ValidationResult, error) {
	if maxAttempts < 1 {
		maxAttempts = 1
	}

	var best *domain.ValidationResult
	var attemptScores []int

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		result, err := s.ValidateImage(ctx, imagePath, originalPrompt, characterRefs)
		if err != nil {
			return nil, fmt.Errorf("validation attempt %d: %w", attempt, err)
		}
		// Vision not supported — skip validation entirely
		if result == nil {
			return nil, nil
		}

		result.Evaluate(threshold)
		attemptScores = append(attemptScores, result.Score)

		s.logger.Info("image validation attempt",
			slog.Int("attempt", attempt),
			slog.Int("max_attempts", maxAttempts),
			slog.Int("score", result.Score),
			slog.Int("threshold", threshold),
			slog.Bool("passed", !result.ShouldRegenerate),
			slog.Any("reasons", result.Reasons),
			slog.String("project_id", projectID),
			slog.Int("scene_num", sceneNum),
			slog.Int("cut_num", cutNum),
		)

		// Track best result
		if best == nil || result.Score > best.Score {
			best = result
		}

		// Passed — no regeneration needed
		if !result.ShouldRegenerate {
			break
		}

		// Not the last attempt — regenerate and retry
		if attempt < maxAttempts {
			s.logger.Info("regenerating image",
				slog.Int("attempt", attempt),
				slog.Int("score", result.Score),
				slog.Int("threshold", threshold),
			)
			if err := regenerateFn(ctx); err != nil {
				return nil, fmt.Errorf("regenerate image (attempt %d): %w", attempt, err)
			}
		}
	}

	// All attempts exhausted but still below threshold
	if best.ShouldRegenerate {
		best.ShouldRegenerate = false // exhausted — accept best
		s.logger.Warn("image validation exhausted all attempts, keeping best score",
			slog.Int("best_score", best.Score),
			slog.Int("threshold", threshold),
			slog.Int("max_attempts", maxAttempts),
			slog.Any("attempt_scores", attemptScores),
			slog.String("project_id", projectID),
			slog.Int("scene_num", sceneNum),
			slog.Int("cut_num", cutNum),
		)
	}

	// Persist final validation score
	if s.store != nil {
		if err := s.store.UpdateValidationScore(projectID, sceneNum, sentenceStart, cutNum, best.Score); err != nil {
			s.logger.Warn("failed to persist validation score",
				slog.String("error", err.Error()),
				slog.String("project_id", projectID),
				slog.Int("scene_num", sceneNum),
				slog.Int("cut_num", cutNum),
			)
		}
	}

	return best, nil
}

// detectMIME returns the MIME type based on file extension.
func detectMIME(ext string) string {
	switch strings.ToLower(ext) {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".webp":
		return "image/webp"
	default:
		return "image/png"
	}
}

// extractValidationJSON strips markdown code fences from LLM output.
func extractValidationJSON(s string) string {
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
