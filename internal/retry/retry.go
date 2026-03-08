package retry

import (
	"context"
	"errors"
	"math"
	"math/rand"
	"time"
)

// maxBackoffDelay caps the exponential backoff to prevent runaway waits.
const maxBackoffDelay = 60 * time.Second

// RetryableError is an interface that errors can implement to indicate
// whether they should be retried.
type RetryableError interface {
	error
	IsRetryable() bool
}

// Do executes fn with retry logic using exponential backoff with jitter.
// It retries up to maxAttempts times with exponential backoff starting from baseDelay.
// Retryable: network timeout, HTTP 429, HTTP 5xx.
// Non-retryable: HTTP 400, 401, 403, context cancellation.
// If error does NOT implement RetryableError, default to retryable (conservative).
// Context cancellation always stops retry immediately.
func Do(ctx context.Context, maxAttempts int, baseDelay time.Duration, fn func() error) error {
	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			if lastErr != nil {
				return lastErr
			}
			return err
		}

		lastErr = fn()
		if lastErr == nil {
			return nil
		}

		// Check if error is retryable
		if !isRetryable(lastErr) {
			return lastErr
		}

		// Don't sleep after the last attempt
		if attempt < maxAttempts-1 {
			delay := backoffDelay(attempt, baseDelay)
			select {
			case <-ctx.Done():
				return lastErr
			case <-time.After(delay):
			}
		}
	}
	return lastErr
}

// isRetryable determines if an error should be retried.
// If the error implements RetryableError, use its IsRetryable() method.
// Otherwise, default to retryable (conservative approach).
// Uses errors.As to correctly handle both single and multi-error wrapping.
func isRetryable(err error) bool {
	var re RetryableError
	if errors.As(err, &re) {
		return re.IsRetryable()
	}
	// Default: unknown errors are retryable (conservative)
	return true
}

// backoffDelay calculates exponential backoff with jitter (0-25%).
// Delay is capped at maxBackoffDelay to prevent runaway waits.
func backoffDelay(attempt int, baseDelay time.Duration) time.Duration {
	exp := math.Pow(2, float64(attempt))
	delay := time.Duration(float64(baseDelay) * exp)
	if delay > maxBackoffDelay {
		delay = maxBackoffDelay
	}
	// Add jitter: random 0-25% of delay
	jitter := time.Duration(rand.Float64() * 0.25 * float64(delay))
	return delay + jitter
}
