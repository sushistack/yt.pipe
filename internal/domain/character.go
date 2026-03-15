package domain

import (
	"fmt"
	"strings"
	"time"
)

// Character represents a per-SCP character ID card with visual presets.
type Character struct {
	ID                string
	SCPID             string
	CanonicalName     string
	Aliases           []string
	VisualDescriptor  string
	StyleGuide        string
	ImagePromptBase   string
	SelectedImagePath string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// CharacterCandidate represents a candidate character image generated for selection.
type CharacterCandidate struct {
	ID           string
	ProjectID    string
	SCPID        string
	CandidateNum int
	ImagePath    string
	Description  string
	Status       string // "pending", "generating", "ready", "failed"
	ErrorDetail  string
	CreatedAt    time.Time
}

// ValidateAliases checks that all provided aliases are non-empty strings.
func ValidateAliases(aliases []string) error {
	for i, a := range aliases {
		if strings.TrimSpace(a) == "" {
			return fmt.Errorf("alias at index %d is empty", i)
		}
	}
	return nil
}
