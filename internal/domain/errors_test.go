package domain

import (
	"errors"
	"fmt"
	"testing"
)

func TestNotFoundError_Error(t *testing.T) {
	err := &NotFoundError{Resource: "project", ID: "123"}
	expected := "project not found: 123"
	if err.Error() != expected {
		t.Errorf("got %q, want %q", err.Error(), expected)
	}
}

func TestValidationError_Error(t *testing.T) {
	err := &ValidationError{Field: "scp_id", Message: "cannot be empty"}
	expected := "validation error on scp_id: cannot be empty"
	if err.Error() != expected {
		t.Errorf("got %q, want %q", err.Error(), expected)
	}
}

func TestPluginError_Error(t *testing.T) {
	inner := fmt.Errorf("connection refused")
	err := &PluginError{Plugin: "openai", Operation: "generate", Err: inner}
	if err.Error() != "plugin openai failed during generate: connection refused" {
		t.Errorf("unexpected error: %s", err.Error())
	}
}

func TestPluginError_Unwrap(t *testing.T) {
	inner := fmt.Errorf("timeout")
	err := &PluginError{Plugin: "tts", Operation: "synthesize", Err: inner}
	if !errors.Is(err, inner) {
		t.Error("expected errors.Is to match inner error")
	}
}

func TestTransitionError_Error(t *testing.T) {
	err := &TransitionError{
		Current:   StatusPending,
		Requested: StatusComplete,
		Allowed:   []string{StatusScenarioReview},
	}
	expected := "cannot transition from pending to complete (allowed: [scenario_review])"
	if err.Error() != expected {
		t.Errorf("got %q, want %q", err.Error(), expected)
	}
}

func TestErrorTypes_ErrorsAs(t *testing.T) {
	var notFound *NotFoundError
	err := error(&NotFoundError{Resource: "job", ID: "456"})
	if !errors.As(err, &notFound) {
		t.Error("expected errors.As to work for NotFoundError")
	}

	var validationErr *ValidationError
	err = error(&ValidationError{Field: "name", Message: "too long"})
	if !errors.As(err, &validationErr) {
		t.Error("expected errors.As to work for ValidationError")
	}

	var pluginErr *PluginError
	err = error(&PluginError{Plugin: "img", Operation: "gen", Err: fmt.Errorf("fail")})
	if !errors.As(err, &pluginErr) {
		t.Error("expected errors.As to work for PluginError")
	}

	var transErr *TransitionError
	err = error(&TransitionError{Current: "a", Requested: "b", Allowed: []string{"c"}})
	if !errors.As(err, &transErr) {
		t.Error("expected errors.As to work for TransitionError")
	}
}
