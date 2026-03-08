package tts

import (
	"fmt"
	"net/http"
)

// APIError represents an error from a TTS API provider.
type APIError struct {
	Provider   string
	StatusCode int
	Message    string
	Err        error
}

func (e *APIError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("tts %s: HTTP %d: %s: %v", e.Provider, e.StatusCode, e.Message, e.Err)
	}
	return fmt.Sprintf("tts %s: HTTP %d: %s", e.Provider, e.StatusCode, e.Message)
}

func (e *APIError) Unwrap() error {
	return e.Err
}

// IsRetryable returns true for transient errors that should be retried.
func (e *APIError) IsRetryable() bool {
	switch e.StatusCode {
	case http.StatusTooManyRequests,
		http.StatusInternalServerError,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
		0: // network error
		return true
	default:
		return false
	}
}
