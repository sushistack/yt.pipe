package domain

import "testing"

func TestCanTransition_ValidTransitions(t *testing.T) {
	tests := []struct {
		name     string
		current  string
		next     string
		expected bool
	}{
		{"pending to scenario_review", StatusPending, StatusScenarioReview, true},
		{"scenario_review to approved", StatusScenarioReview, StatusApproved, true},
		{"scenario_review to pending (reject)", StatusScenarioReview, StatusPending, true},
		{"approved to generating_assets", StatusApproved, StatusGeneratingAssets, true},
		{"generating_assets to assembling", StatusGeneratingAssets, StatusAssembling, true},
		{"assembling to complete", StatusAssembling, StatusComplete, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CanTransition(tt.current, tt.next); got != tt.expected {
				t.Errorf("CanTransition(%s, %s) = %v, want %v", tt.current, tt.next, got, tt.expected)
			}
		})
	}
}

func TestCanTransition_InvalidTransitions(t *testing.T) {
	tests := []struct {
		name    string
		current string
		next    string
	}{
		{"pending to approved (skip)", StatusPending, StatusApproved},
		{"complete to pending (terminal)", StatusComplete, StatusPending},
		{"approved to pending (backward)", StatusApproved, StatusPending},
		{"unknown state", "unknown", StatusPending},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if CanTransition(tt.current, tt.next) {
				t.Errorf("CanTransition(%s, %s) should be false", tt.current, tt.next)
			}
		})
	}
}

func TestProject_Transition_Valid(t *testing.T) {
	p := &Project{Status: StatusPending}
	err := p.Transition(StatusScenarioReview)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if p.Status != StatusScenarioReview {
		t.Errorf("expected status %s, got %s", StatusScenarioReview, p.Status)
	}
}

func TestProject_Transition_Invalid(t *testing.T) {
	p := &Project{Status: StatusPending}
	err := p.Transition(StatusComplete)
	if err == nil {
		t.Fatal("expected error for invalid transition")
	}
	te, ok := err.(*TransitionError)
	if !ok {
		t.Fatalf("expected *TransitionError, got %T", err)
	}
	if te.Current != StatusPending || te.Requested != StatusComplete {
		t.Errorf("unexpected error fields: %+v", te)
	}
}
