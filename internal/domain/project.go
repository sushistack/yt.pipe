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
	StatusPending          = "pending"
	StatusScenarioReview   = "scenario_review"
	StatusApproved         = "approved"
	StatusGeneratingAssets = "generating_assets"
	StatusAssembling       = "assembling"
	StatusComplete         = "complete"
)

// AllowedTransitions defines valid state transitions for projects
var AllowedTransitions = map[string][]string{
	StatusPending:          {StatusScenarioReview},
	StatusScenarioReview:   {StatusApproved, StatusPending},
	StatusApproved:         {StatusGeneratingAssets},
	StatusGeneratingAssets: {StatusAssembling},
	StatusAssembling:       {StatusComplete},
	StatusComplete:         {},
}

// CanTransition checks if a state transition is allowed
func CanTransition(current, requested string) bool {
	allowed, ok := AllowedTransitions[current]
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
		allowed := AllowedTransitions[p.Status]
		return &TransitionError{
			Current:   p.Status,
			Requested: newStatus,
			Allowed:   allowed,
		}
	}
	p.Status = newStatus
	p.UpdatedAt = time.Now().UTC()
	return nil
}
