package pipeline

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHTTPError_IsRetryable(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		retryable  bool
	}{
		{"429 rate limited", 429, true},
		{"500 internal server error", 500, true},
		{"502 bad gateway", 502, true},
		{"503 service unavailable", 503, true},
		{"408 request timeout", 408, true},
		{"400 bad request", 400, false},
		{"401 unauthorized", 401, false},
		{"403 forbidden", 403, false},
		{"404 not found", 404, false},
		{"200 ok", 200, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &HTTPError{StatusCode: tt.statusCode, Message: "test"}
			assert.Equal(t, tt.retryable, err.IsRetryable())
		})
	}
}

func TestHTTPError_Error(t *testing.T) {
	err := &HTTPError{StatusCode: 429, Message: "rate limited"}
	assert.Equal(t, "HTTP 429: rate limited", err.Error())
}

func TestTimeoutError_IsRetryable(t *testing.T) {
	err := &TimeoutError{Operation: "image_generate", Timeout: "120s"}
	assert.True(t, err.IsRetryable())
	assert.Contains(t, err.Error(), "image_generate")
	assert.Contains(t, err.Error(), "120s")
}

func TestHTTPError_ImplementsRetryable(t *testing.T) {
	// Verify our error types implement the retry.RetryableError interface
	var _ interface{ IsRetryable() bool } = &HTTPError{}
	var _ interface{ IsRetryable() bool } = &TimeoutError{}
}
