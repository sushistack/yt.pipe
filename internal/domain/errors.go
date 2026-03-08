package domain

import "fmt"

// NotFoundError indicates a resource was not found (maps to API 404)
type NotFoundError struct {
	Resource string
	ID       string
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("%s not found: %s", e.Resource, e.ID)
}

// ValidationError indicates invalid input (maps to API 400)
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation error on %s: %s", e.Field, e.Message)
}

// PluginError indicates a plugin operation failed (maps to API 500/502)
type PluginError struct {
	Plugin    string
	Operation string
	Err       error
}

func (e *PluginError) Error() string {
	return fmt.Sprintf("plugin %s failed during %s: %v", e.Plugin, e.Operation, e.Err)
}

func (e *PluginError) Unwrap() error {
	return e.Err
}

// TransitionError indicates an invalid state transition (maps to API 409)
type TransitionError struct {
	Current   string
	Requested string
	Allowed   []string
}

func (e *TransitionError) Error() string {
	return fmt.Sprintf("cannot transition from %s to %s (allowed: %v)", e.Current, e.Requested, e.Allowed)
}
