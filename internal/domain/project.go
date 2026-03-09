package domain

import "time"

// Project represents an SCP YouTube content project
type Project struct {
	ID            string
	SCPID         string
	Status        string
	SceneCount    int
	WorkspacePath string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// Project status constants
const (
	StatusPending        = "pending"
	StatusScenarioReview = "scenario_review"
	StatusApproved       = "approved"
	StatusImageReview    = "image_review"
	StatusTTSReview      = "tts_review"
	StatusAssembling     = "assembling"
	StatusComplete       = "complete"

	// StatusGeneratingAssets is deprecated; kept for backward compatibility
	// with existing checkpoints and database records. New code should use
	// StatusImageReview and StatusTTSReview instead.
	StatusGeneratingAssets = "generating_assets"
)

// allowedTransitions defines valid state transitions for projects.
// The primary flow is: pending → scenario_review → approved → image_review → tts_review → assembling → complete
// With --skip-approval, image_review and tts_review are auto-transitioned.
var allowedTransitions = map[string][]string{
	StatusPending:        {StatusScenarioReview},
	StatusScenarioReview: {StatusApproved, StatusPending},
	StatusApproved:       {StatusImageReview, StatusGeneratingAssets},
	StatusImageReview:    {StatusTTSReview},
	StatusTTSReview:      {StatusAssembling},
	// Keep generating_assets → assembling for backward compat with existing projects
	StatusGeneratingAssets: {StatusAssembling},
	StatusAssembling:       {StatusComplete},
	StatusComplete:         {},
}

// AllowedTransitions returns the allowed target states for the given status.
func AllowedTransitions(status string) []string {
	return allowedTransitions[status]
}

// CanTransition checks if a state transition is allowed
func CanTransition(current, requested string) bool {
	allowed, ok := allowedTransitions[current]
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

// Transition attempts to transition a project to a new state
func (p *Project) Transition(newStatus string) error {
	if !CanTransition(p.Status, newStatus) {
		return &TransitionError{
			Current:   p.Status,
			Requested: newStatus,
			Allowed:   allowedTransitions[p.Status],
		}
	}
	p.Status = newStatus
	p.UpdatedAt = time.Now().UTC()
	return nil
}
