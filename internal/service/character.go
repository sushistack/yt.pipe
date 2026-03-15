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
// When projectID is non-empty, candidates are tracked in the database for dashboard polling.
func (cs *CharacterService) GenerateCandidates(ctx context.Context, projectID, scpID string, count int, workspacePath string) ([]*domain.CharacterCandidate, error) {
	if cs.llm == nil {
		return nil, fmt.Errorf("service: generate candidates: LLM provider not set")
	}
	if cs.imageGen == nil {
		return nil, fmt.Errorf("service: generate candidates: image generation provider not set")
	}

	candidateDir := filepath.Join(workspacePath, scpID, "characters")
	// Remove old candidate files but preserve user-uploaded images
	clearCandidateFiles(candidateDir)
	if err := os.MkdirAll(candidateDir, 0o755); err != nil {
		return nil, fmt.Errorf("service: generate candidates: create dir: %w", err)
	}

	// DB-backed flow: clear old candidates and create pending rows
	if projectID != "" {
		if err := cs.store.DeleteCandidatesByProject(projectID); err != nil {
			return nil, fmt.Errorf("service: generate candidates: clear old: %w", err)
		}
		if err := cs.store.CreateCandidateBatch(projectID, scpID, count); err != nil {
			return nil, fmt.Errorf("service: generate candidates: create batch: %w", err)
		}
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
		// Mark all as failed in DB
		if projectID != "" {
			cs.markAllCandidatesFailed(projectID, err.Error())
		}
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
		if projectID != "" {
			cs.markAllCandidatesFailed(projectID, "parse LLM response: "+err.Error())
		}
		return nil, fmt.Errorf("service: generate candidates: parse LLM response: %w", err)
	}

	// Load DB candidates for status updates
	var dbCandidates []*domain.CharacterCandidate
	if projectID != "" {
		dbCandidates, _ = cs.store.ListCandidatesByProject(projectID)
	}

	// Step 2: Generate images for each candidate
	var results []*domain.CharacterCandidate
	for i, c := range candidates {
		idx := i + 1
		imgPath := filepath.Join(candidateDir, fmt.Sprintf("candidate_%d.png", idx))
		txtPath := filepath.Join(candidateDir, fmt.Sprintf("candidate_%d.txt", idx))

		// Update status to generating
		if idx <= len(dbCandidates) {
			_ = cs.store.UpdateCandidateStatus(dbCandidates[idx-1].ID, "generating", "", "", "")
		}

		// Save description
		descText := fmt.Sprintf("Name: %s\nVisual: %s\nPrompt: %s", c.Name, c.VisualDescriptor, c.ImagePrompt)
		if err := os.WriteFile(txtPath, []byte(descText), 0o644); err != nil {
			if idx <= len(dbCandidates) {
				_ = cs.store.UpdateCandidateStatus(dbCandidates[idx-1].ID, "failed", "", "", err.Error())
			}
			continue
		}

		// Generate image
		imgResult, err := cs.imageGen.Generate(ctx, c.ImagePrompt, imagegen.GenerateOptions{
			Width:  1024,
			Height: 1024,
		})
		if err != nil {
			if idx <= len(dbCandidates) {
				_ = cs.store.UpdateCandidateStatus(dbCandidates[idx-1].ID, "failed", "", descText, err.Error())
			}
			continue
		}

		if err := os.WriteFile(imgPath, imgResult.ImageData, 0o644); err != nil {
			if idx <= len(dbCandidates) {
				_ = cs.store.UpdateCandidateStatus(dbCandidates[idx-1].ID, "failed", "", descText, err.Error())
			}
			continue
		}

		// Update DB with ready status
		if idx <= len(dbCandidates) {
			_ = cs.store.UpdateCandidateStatus(dbCandidates[idx-1].ID, "ready", imgPath, descText, "")
		}

		results = append(results, &domain.CharacterCandidate{
			CandidateNum: idx,
			ImagePath:    imgPath,
			Description:  descText,
			Status:       "ready",
		})
	}

	// Mark any remaining pending/generating candidates as failed (LLM returned fewer than count)
	if projectID != "" {
		cs.markAllCandidatesFailed(projectID, "LLM returned fewer candidates than requested")
	}

	return results, nil
}

// markAllCandidatesFailed marks all pending/generating candidates as failed.
func (cs *CharacterService) markAllCandidatesFailed(projectID, errorDetail string) {
	candidates, err := cs.store.ListCandidatesByProject(projectID)
	if err != nil {
		return
	}
	for _, c := range candidates {
		if c.Status == "pending" || c.Status == "generating" {
			_ = cs.store.UpdateCandidateStatus(c.ID, "failed", "", "", errorDetail)
		}
	}
}

// ListCandidates returns all candidates for a project.
func (cs *CharacterService) ListCandidates(projectID string) ([]*domain.CharacterCandidate, error) {
	return cs.store.ListCandidatesByProject(projectID)
}

// GetCandidateGenerationStatus returns aggregate status for a project's candidates.
// Returns: "empty" (no rows), "generating" (any pending/generating), "ready" (all ready), "failed" (any failed, none generating).
func (cs *CharacterService) GetCandidateGenerationStatus(projectID string) (string, error) {
	candidates, err := cs.store.ListCandidatesByProject(projectID)
	if err != nil {
		return "", fmt.Errorf("service: candidate status: %w", err)
	}
	if len(candidates) == 0 {
		return "empty", nil
	}

	hasGenerating := false
	hasFailed := false
	allReady := true
	for _, c := range candidates {
		switch c.Status {
		case "pending", "generating":
			hasGenerating = true
			allReady = false
		case "failed":
			hasFailed = true
			allReady = false
		case "ready":
			// ok
		default:
			allReady = false
		}
	}

	if hasGenerating {
		return "generating", nil
	}
	if allReady {
		return "ready", nil
	}
	if hasFailed {
		return "failed", nil
	}
	return "generating", nil
}

// SelectCandidate selects a generated candidate and creates/updates a character record.
func (cs *CharacterService) SelectCandidate(scpID string, candidateNum int, workspacePath string) (*domain.Character, error) {
	candidateDir := filepath.Join(workspacePath, scpID, "characters")
	imgPath := filepath.Join(candidateDir, fmt.Sprintf("candidate_%d.png", candidateNum))
	txtPath := filepath.Join(candidateDir, fmt.Sprintf("candidate_%d.txt", candidateNum))

	if _, err := os.Stat(imgPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("service: select candidate: candidate %d image not found", candidateNum)
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

// UploadedImagePath returns the path for a user-uploaded character image.
func UploadedImagePath(workspacePath, scpID string) string {
	return filepath.Join(workspacePath, scpID, "characters", "uploaded.png")
}

// UploadCharacterImage saves a user-uploaded image as the selected character image.
// The uploaded file is stored separately from generated candidates and preserved across regeneration.
func (cs *CharacterService) UploadCharacterImage(scpID string, imageData []byte, workspacePath string) (*domain.Character, error) {
	charDir := filepath.Join(workspacePath, scpID, "characters")
	if err := os.MkdirAll(charDir, 0o755); err != nil {
		return nil, fmt.Errorf("service: upload character: create dir: %w", err)
	}

	imgPath := UploadedImagePath(workspacePath, scpID)
	if err := os.WriteFile(imgPath, imageData, 0o644); err != nil {
		return nil, fmt.Errorf("service: upload character: write image: %w", err)
	}

	// Create or update character with uploaded image
	existing, _ := cs.CheckExistingCharacter(scpID)
	if existing != nil {
		existing.SelectedImagePath = imgPath
		if err := cs.store.UpdateCharacter(existing); err != nil {
			return nil, fmt.Errorf("service: upload character: update: %w", err)
		}
		return existing, nil
	}

	c := &domain.Character{
		ID:                uuid.New().String(),
		SCPID:             scpID,
		CanonicalName:     scpID,
		Aliases:           []string{},
		VisualDescriptor:  "User-uploaded character image",
		SelectedImagePath: imgPath,
	}
	if err := cs.store.CreateCharacter(c); err != nil {
		return nil, fmt.Errorf("service: upload character: create: %w", err)
	}
	return c, nil
}

// DeleteUploadedImage removes the user-uploaded character image.
func (cs *CharacterService) DeleteUploadedImage(scpID, workspacePath string) error {
	imgPath := UploadedImagePath(workspacePath, scpID)
	if err := os.Remove(imgPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("service: delete uploaded image: %w", err)
	}

	// If the selected image was the uploaded one, clear selection
	existing, _ := cs.CheckExistingCharacter(scpID)
	if existing != nil && existing.SelectedImagePath == imgPath {
		existing.SelectedImagePath = ""
		_ = cs.store.UpdateCharacter(existing)
	}
	return nil
}

// clearCandidateFiles removes generated candidate files but preserves user-uploaded images.
func clearCandidateFiles(candidateDir string) {
	entries, err := os.ReadDir(candidateDir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		name := entry.Name()
		// Preserve uploaded.png
		if name == "uploaded.png" {
			continue
		}
		_ = os.Remove(filepath.Join(candidateDir, name))
	}
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
