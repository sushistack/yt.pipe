package imagegen

import (
	"fmt"
	"net/http"
)

// APIError represents an error from an image generation API call.
type APIError struct {
	Provider   string
	StatusCode int
	Message    string
	Err        error
}

func (e *APIError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s ImageGen API error (HTTP %d): %s: %v", e.Provider, e.StatusCode, e.Message, e.Err)
	}
	return fmt.Sprintf("%s ImageGen API error (HTTP %d): %s", e.Provider, e.StatusCode, e.Message)
}

func (e *APIError) Unwrap() error {
	return e.Err
}

// IsRetryable implements retry.RetryableError.
// Retryable: 429 (rate limit), 500, 502, 503 (server errors), 0 (network error).
// Non-retryable: 400, 401, 403 (client errors).
func (e *APIError) IsRetryable() bool {
	switch e.StatusCode {
	case 0, // network error
		http.StatusTooManyRequests,
		http.StatusInternalServerError,
		http.StatusBadGateway,
		http.StatusServiceUnavailable:
		return true
	default:
		return false
	}
}
