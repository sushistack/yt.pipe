package domain

import "time"

// Glossary suggestion status constants
const (
	SuggestionPending  = "pending"
	SuggestionApproved = "approved"
	SuggestionRejected = "rejected"
)

// allowedSuggestionTransitions defines valid status transitions.
var allowedSuggestionTransitions = map[string][]string{
	SuggestionPending:  {SuggestionApproved, SuggestionRejected},
	SuggestionApproved: {},
	SuggestionRejected: {},
}

// GlossarySuggestion represents a proposed glossary entry awaiting approval.
type GlossarySuggestion struct {
	ID            int       `json:"id"`
	ProjectID     string    `json:"project_id"`
	Term          string    `json:"term"`
	Pronunciation string    `json:"pronunciation"`
	Definition    string    `json:"definition"`
	Category      string    `json:"category"`
	Status        string    `json:"status"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// CanSuggestionTransition checks if a suggestion status transition is valid.
func CanSuggestionTransition(current, requested string) bool {
	allowed, ok := allowedSuggestionTransitions[current]
	if !ok {
		return false
	}
	for _, s := range allowed {
		if s == requested {
			return true
		}
	}
	return false
}

// ValidateGlossarySuggestion validates required fields for a new suggestion.
func ValidateGlossarySuggestion(projectID, term, pronunciation string) error {
	if projectID == "" {
		return &ValidationError{Field: "project_id", Message: "must not be empty"}
	}
	if term == "" {
		return &ValidationError{Field: "term", Message: "must not be empty"}
	}
	if pronunciation == "" {
		return &ValidationError{Field: "pronunciation", Message: "must not be empty"}
	}
	return nil
}
