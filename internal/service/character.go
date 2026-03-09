package service

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/sushistack/yt.pipe/internal/domain"
	"github.com/sushistack/yt.pipe/internal/plugin/imagegen"
	"github.com/sushistack/yt.pipe/internal/store"
)

// CharacterService manages character ID card lifecycle and scene text matching.
type CharacterService struct {
	store *store.Store
}

// NewCharacterService creates a new CharacterService.
func NewCharacterService(s *store.Store) *CharacterService {
	return &CharacterService{store: s}
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
