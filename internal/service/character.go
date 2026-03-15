package service

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/sushistack/yt.pipe/internal/domain"
	"github.com/sushistack/yt.pipe/internal/plugin/imagegen"
	"github.com/sushistack/yt.pipe/internal/plugin/llm"
	"github.com/sushistack/yt.pipe/internal/store"
)

// CandidateResult holds one generated character candidate.
type CandidateResult struct {
	Index       int    `json:"index"`
	Description string `json:"description"`
	ImagePath   string `json:"image_path"`
	TextPath    string `json:"text_path"`
}

// CharacterService manages character ID card lifecycle and scene text matching.
type CharacterService struct {
	store    *store.Store
	llm      llm.LLM
	imageGen imagegen.ImageGen
}

// NewCharacterService creates a new CharacterService.
func NewCharacterService(s *store.Store) *CharacterService {
	return &CharacterService{store: s}
}

// SetLLM sets the LLM provider for character description generation.
func (cs *CharacterService) SetLLM(l llm.LLM) {
	cs.llm = l
}

// SetImageGen sets the image generation provider for character candidate images.
func (cs *CharacterService) SetImageGen(ig imagegen.ImageGen) {
	cs.imageGen = ig
}

// CreateCharacter validates inputs, generates UUID, and creates a character.
func (cs *CharacterService) CreateCharacter(scpID, canonicalName string, aliases []string, visualDescriptor, styleGuide, imagePromptBase string) (*domain.Character, error) {
	if scpID == "" {
		return nil, &domain.ValidationError{Field: "scp_id", Message: "must not be empty"}
	}
	if canonicalName == "" {
		return nil, &domain.ValidationError{Field: "canonical_name", Message: "must not be empty"}
	}
	if err := domain.ValidateAliases(aliases); err != nil {
		return nil, &domain.ValidationError{Field: "aliases", Message: err.Error()}
	}

	c := &domain.Character{
		ID:               uuid.New().String(),
		SCPID:            scpID,
		CanonicalName:    canonicalName,
		Aliases:          aliases,
		VisualDescriptor: visualDescriptor,
		StyleGuide:       styleGuide,
		ImagePromptBase:  imagePromptBase,
	}
	if err := cs.store.CreateCharacter(c); err != nil {
		return nil, fmt.Errorf("service: create character: %w", err)
	}
	return c, nil
}

// GetCharacter retrieves a character by ID.
func (cs *CharacterService) GetCharacter(id string) (*domain.Character, error) {
	return cs.store.GetCharacter(id)
}

// ListCharacters returns characters filtered by SCP ID, or all if scpID is empty.
func (cs *CharacterService) ListCharacters(scpID string) ([]*domain.Character, error) {
	if scpID == "" {
		return cs.store.ListAllCharacters()
	}
	return cs.store.ListCharactersBySCPID(scpID)
}

// UpdateCharacter updates a character's fields.
func (cs *CharacterService) UpdateCharacter(id string, canonicalName string, aliases []string, visualDescriptor, styleGuide, imagePromptBase string) (*domain.Character, error) {
	c, err := cs.store.GetCharacter(id)
	if err != nil {
		return nil, err
	}

	if canonicalName != "" {
		c.CanonicalName = canonicalName
	}
	if aliases != nil {
		if err := domain.ValidateAliases(aliases); err != nil {
			return nil, &domain.ValidationError{Field: "aliases", Message: err.Error()}
		}
		c.Aliases = aliases
	}
	if visualDescriptor != "" {
		c.VisualDescriptor = visualDescriptor
	}
	if styleGuide != "" {
		c.StyleGuide = styleGuide
	}
	if imagePromptBase != "" {
		c.ImagePromptBase = imagePromptBase
	}

	if err := cs.store.UpdateCharacter(c); err != nil {
		return nil, fmt.Errorf("service: update character: %w", err)
	}
	return c, nil
}

// DeleteCharacter removes a character by ID.
func (cs *CharacterService) DeleteCharacter(id string) error {
	return cs.store.DeleteCharacter(id)
}

// UpdateSelectedImagePath sets the selected character image path.
func (cs *CharacterService) UpdateSelectedImagePath(characterID, imagePath string) error {
	return cs.store.UpdateSelectedImagePath(characterID, imagePath)
}

// CheckExistingCharacter returns the first character for a given SCP ID, or nil if none exists.
func (cs *CharacterService) CheckExistingCharacter(scpID string) (*domain.Character, error) {
	chars, err := cs.store.ListCharactersBySCPID(scpID)
	if err != nil {
		return nil, fmt.Errorf("service: check existing character: %w", err)
	}
	if len(chars) == 0 {
		return nil, nil
	}
	return chars[0], nil
}

// GenerateCandidates uses LLM to generate character appearance descriptions and images.
func (cs *CharacterService) GenerateCandidates(ctx context.Context, scpID string, count int, workspacePath string) ([]CandidateResult, error) {
	if cs.llm == nil {
		return nil, fmt.Errorf("service: generate candidates: LLM provider not set")
	}
	if cs.imageGen == nil {
		return nil, fmt.Errorf("service: generate candidates: image generation provider not set")
	}

	candidateDir := filepath.Join(workspacePath, scpID, "characters")
	if err := os.MkdirAll(candidateDir, 0o755); err != nil {
		return nil, fmt.Errorf("service: generate candidates: create dir: %w", err)
	}

	// Step 1: Ask LLM for character descriptions
	prompt := fmt.Sprintf(
		"Generate %d distinct visual appearance descriptions for the main character/entity of %s. "+
			"For each candidate, provide a JSON object with fields: "+
			"\"index\" (1-based), \"name\" (canonical name), \"visual_descriptor\" (detailed physical appearance for image generation), "+
			"\"image_prompt\" (concise image generation prompt). "+
			"Output as a JSON array. Be specific about physical features, colors, textures, and proportions.",
		count, scpID,
	)

	result, err := cs.llm.Complete(ctx, []llm.Message{
		{Role: "user", Content: prompt},
	}, llm.CompletionOptions{})
	if err != nil {
		return nil, fmt.Errorf("service: generate candidates: LLM: %w", err)
	}

	// Parse LLM response
	type llmCandidate struct {
		Index            int    `json:"index"`
		Name             string `json:"name"`
		VisualDescriptor string `json:"visual_descriptor"`
		ImagePrompt      string `json:"image_prompt"`
	}

	cleaned := extractJSONArray(result.Content)
	var candidates []llmCandidate
	if err := json.Unmarshal([]byte(cleaned), &candidates); err != nil {
		return nil, fmt.Errorf("service: generate candidates: parse LLM response: %w", err)
	}

	// Step 2: Generate images for each candidate
	var results []CandidateResult
	for i, c := range candidates {
		idx := i + 1
		imgPath := filepath.Join(candidateDir, fmt.Sprintf("candidate_%d.png", idx))
		txtPath := filepath.Join(candidateDir, fmt.Sprintf("candidate_%d.txt", idx))

		// Save description
		descText := fmt.Sprintf("Name: %s\nVisual: %s\nPrompt: %s", c.Name, c.VisualDescriptor, c.ImagePrompt)
		if err := os.WriteFile(txtPath, []byte(descText), 0o644); err != nil {
			return nil, fmt.Errorf("service: generate candidates: save description %d: %w", idx, err)
		}

		// Generate image
		imgResult, err := cs.imageGen.Generate(ctx, c.ImagePrompt, imagegen.GenerateOptions{
			Width:  1024,
			Height: 1024,
		})
		if err != nil {
			return nil, fmt.Errorf("service: generate candidates: image %d: %w", idx, err)
		}

		if err := os.WriteFile(imgPath, imgResult.ImageData, 0o644); err != nil {
			return nil, fmt.Errorf("service: generate candidates: save image %d: %w", idx, err)
		}

		results = append(results, CandidateResult{
			Index:       idx,
			Description: descText,
			ImagePath:   imgPath,
			TextPath:    txtPath,
		})
	}

	return results, nil
}

// SelectCandidate selects a generated candidate and creates/updates a character record.
func (cs *CharacterService) SelectCandidate(scpID string, candidateNum int, workspacePath string) (*domain.Character, error) {
	candidateDir := filepath.Join(workspacePath, scpID, "characters")
	imgPath := filepath.Join(candidateDir, fmt.Sprintf("candidate_%d.png", candidateNum))
	txtPath := filepath.Join(candidateDir, fmt.Sprintf("candidate_%d.txt", candidateNum))

	if _, err := os.Stat(imgPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("service: select candidate: image not found: %s", imgPath)
	}

	description := ""
	if data, err := os.ReadFile(txtPath); err == nil {
		description = strings.TrimSpace(string(data))
	}

	// Check if character already exists for this SCP
	existing, _ := cs.CheckExistingCharacter(scpID)
	if existing != nil {
		if description != "" {
			existing.VisualDescriptor = description
		}
		existing.SelectedImagePath = imgPath
		if err := cs.store.UpdateCharacter(existing); err != nil {
			return nil, fmt.Errorf("service: select candidate: update: %w", err)
		}
		if err := cs.store.UpdateSelectedImagePath(existing.ID, imgPath); err != nil {
			return nil, fmt.Errorf("service: select candidate: update image path: %w", err)
		}
		return existing, nil
	}

	// Create new
	c := &domain.Character{
		ID:                uuid.New().String(),
		SCPID:             scpID,
		CanonicalName:     scpID,
		Aliases:           []string{},
		VisualDescriptor:  description,
		SelectedImagePath: imgPath,
	}
	if err := cs.store.CreateCharacter(c); err != nil {
		return nil, fmt.Errorf("service: select candidate: create: %w", err)
	}
	return c, nil
}

// extractJSONArray strips markdown fences and extracts JSON array.
func extractJSONArray(s string) string {
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

// MatchCharacters finds characters whose canonical_name or aliases appear in scene text.
// Returns deduplicated CharacterRef slice for ImageGen plugin consumption.
func (cs *CharacterService) MatchCharacters(scpID, sceneText string) ([]imagegen.CharacterRef, error) {
	// Load characters for this SCP ID + all global characters
	scpChars, err := cs.store.ListCharactersBySCPID(scpID)
	if err != nil {
		return nil, fmt.Errorf("service: match characters: load scp chars: %w", err)
	}
	allChars, err := cs.store.ListAllCharacters()
	if err != nil {
		return nil, fmt.Errorf("service: match characters: load all chars: %w", err)
	}

	// Deduplicate by ID (SCP-specific chars may overlap with all chars)
	seen := make(map[string]bool)
	var candidates []*domain.Character
	for _, c := range scpChars {
		if !seen[c.ID] {
			seen[c.ID] = true
			candidates = append(candidates, c)
		}
	}
	for _, c := range allChars {
		if !seen[c.ID] {
			seen[c.ID] = true
			candidates = append(candidates, c)
		}
	}

	lowerText := strings.ToLower(sceneText)
	var refs []imagegen.CharacterRef

	for _, c := range candidates {
		if matchesText(c, lowerText) {
			refs = append(refs, imagegen.CharacterRef{
				Name:             c.CanonicalName,
				VisualDescriptor: c.VisualDescriptor,
				ImagePromptBase:  c.ImagePromptBase,
			})
		}
	}

	return refs, nil
}

// matchesText checks if a character's canonical name or any alias appears in the lowered text.
func matchesText(c *domain.Character, lowerText string) bool {
	if strings.Contains(lowerText, strings.ToLower(c.CanonicalName)) {
		return true
	}
	for _, alias := range c.Aliases {
		if strings.Contains(lowerText, strings.ToLower(alias)) {
			return true
		}
	}
	return false
}
