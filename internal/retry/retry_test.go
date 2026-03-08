package retry

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testRetryableError is a test helper implementing RetryableError.
type testRetryableError struct {
	msg       string
	retryable bool
}

func (e *testRetryableError) Error() string     { return e.msg }
func (e *testRetryableError) IsRetryable() bool { return e.retryable }

func TestDo_SuccessOnFirstAttempt(t *testing.T) {
	var calls int
	err := Do(context.Background(), 3, 10*time.Millisecond, func() error {
		calls++
		return nil
	})
	assert.NoError(t, err)
	assert.Equal(t, 1, calls)
}

func TestDo_SuccessAfterRetries(t *testing.T) {
	var calls int
	err := Do(context.Background(), 3, 10*time.Millisecond, func() error {
		calls++
		if calls < 3 {
			return errors.New("transient error")
		}
		return nil
	})
	assert.NoError(t, err)
	assert.Equal(t, 3, calls)
}

func TestDo_ExhaustsMaxAttempts(t *testing.T) {
	var calls int
	err := Do(context.Background(), 3, 10*time.Millisecond, func() error {
		calls++
		return errors.New("persistent error")
	})
	assert.Error(t, err)
	assert.Equal(t, "persistent error", err.Error())
	assert.Equal(t, 3, calls)
}

func TestDo_ExponentialBackoff(t *testing.T) {
	// Verify delays increase by recording timestamps
	var timestamps []time.Time
	baseDelay := 50 * time.Millisecond

	_ = Do(context.Background(), 3, baseDelay, func() error {
		timestamps = append(timestamps, time.Now())
		return errors.New("fail")
	})

	require.Len(t, timestamps, 3)

	// First delay should be approximately baseDelay (50ms), second ~100ms
	// Use generous tolerance (±50ms) to avoid flakiness
	delay1 := timestamps[1].Sub(timestamps[0])
	delay2 := timestamps[2].Sub(timestamps[1])

	assert.True(t, delay1 >= 40*time.Millisecond, "first delay %v should be >= 40ms", delay1)
	assert.True(t, delay1 <= 150*time.Millisecond, "first delay %v should be <= 150ms", delay1)
	assert.True(t, delay2 >= 80*time.Millisecond, "second delay %v should be >= 80ms", delay2)
	assert.True(t, delay2 <= 300*time.Millisecond, "second delay %v should be <= 300ms", delay2)
	// Verify exponential growth: second delay should be roughly 2x first
	assert.True(t, delay2 > delay1, "second delay %v should be > first delay %v", delay2, delay1)
}

func TestDo_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	var calls int32

	go func() {
		time.Sleep(25 * time.Millisecond)
		cancel()
	}()

	err := Do(ctx, 10, 50*time.Millisecond, func() error {
		atomic.AddInt32(&calls, 1)
		return errors.New("keep retrying")
	})

	assert.Error(t, err)
	// Should have been cut short by cancellation
	assert.True(t, atomic.LoadInt32(&calls) < 10, "should not exhaust all attempts")
}

func TestDo_NonRetryableError(t *testing.T) {
	var calls int
	nonRetryable := &testRetryableError{msg: "bad request (400)", retryable: false}

	err := Do(context.Background(), 5, 10*time.Millisecond, func() error {
		calls++
		return nonRetryable
	})

	assert.Error(t, err)
	assert.Equal(t, 1, calls, "should stop immediately on non-retryable error")
	assert.Equal(t, "bad request (400)", err.Error())
}

func TestDo_RetryableError(t *testing.T) {
	var calls int
	retryable := &testRetryableError{msg: "rate limited (429)", retryable: true}

	err := Do(context.Background(), 3, 10*time.Millisecond, func() error {
		calls++
		return retryable
	})

	assert.Error(t, err)
	assert.Equal(t, 3, calls, "should retry on retryable error")
}

func TestDo_DefaultRetryable(t *testing.T) {
	// Unknown errors (not implementing RetryableError) default to retryable
	var calls int
	err := Do(context.Background(), 3, 10*time.Millisecond, func() error {
		calls++
		return errors.New("unknown error")
	})

	assert.Error(t, err)
	assert.Equal(t, 3, calls, "unknown errors should be retried (conservative default)")
}

func TestIsRetryable_WithRetryableError(t *testing.T) {
	assert.True(t, isRetryable(&testRetryableError{retryable: true}))
	assert.False(t, isRetryable(&testRetryableError{retryable: false}))
}

func TestIsRetryable_WithPlainError(t *testing.T) {
	// Plain errors default to retryable
	assert.True(t, isRetryable(errors.New("plain error")))
}

func TestBackoffDelay(t *testing.T) {
	base := 100 * time.Millisecond

	d0 := backoffDelay(0, base)
	d1 := backoffDelay(1, base)
	d2 := backoffDelay(2, base)

	// Attempt 0: 100ms + 0-25% jitter = 100-125ms
	assert.True(t, d0 >= 100*time.Millisecond && d0 <= 125*time.Millisecond, "d0=%v", d0)
	// Attempt 1: 200ms + 0-25% jitter = 200-250ms
	assert.True(t, d1 >= 200*time.Millisecond && d1 <= 250*time.Millisecond, "d1=%v", d1)
	// Attempt 2: 400ms + 0-25% jitter = 400-500ms
	assert.True(t, d2 >= 400*time.Millisecond && d2 <= 500*time.Millisecond, "d2=%v", d2)
}

func TestBackoffDelay_CappedAtMax(t *testing.T) {
	base := 1 * time.Second

	// Attempt 20: 2^20 * 1s = ~1048576s without cap
	d := backoffDelay(20, base)
	// Should be capped at maxBackoffDelay (60s) + up to 25% jitter
	assert.True(t, d >= maxBackoffDelay, "delay %v should be >= maxBackoffDelay", d)
	assert.True(t, d <= maxBackoffDelay+maxBackoffDelay/4, "delay %v should be <= maxBackoffDelay + 25%% jitter", d)
}

func TestIsRetryable_WrappedNonRetryable(t *testing.T) {
	// Non-retryable error wrapped with fmt.Errorf should still be detected
	inner := &testRetryableError{msg: "forbidden (403)", retryable: false}
	wrapped := fmt.Errorf("api call failed: %w", inner)

	assert.False(t, isRetryable(wrapped), "wrapped non-retryable error should not be retried")
}

func TestIsRetryable_JoinedErrors(t *testing.T) {
	// errors.Join produces Unwrap() []error — must be handled by errors.As
	nonRetryable := &testRetryableError{msg: "bad request (400)", retryable: false}
	other := errors.New("some context")
	joined := errors.Join(other, nonRetryable)

	assert.False(t, isRetryable(joined), "joined error containing non-retryable should not be retried")
}
