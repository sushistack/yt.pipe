package plugin

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultPluginConfig(t *testing.T) {
	cfg := DefaultPluginConfig("test-plugin")

	assert.Equal(t, "test-plugin", cfg.Name)
	assert.Equal(t, 120*time.Second, cfg.Timeout)
	assert.Equal(t, 3, cfg.MaxRetries)
	assert.Equal(t, 1*time.Second, cfg.BaseDelay)
}

func TestWithTimeout_Success(t *testing.T) {
	err := WithTimeout(context.Background(), 1*time.Second, func(ctx context.Context) error {
		// Quick operation that completes within timeout
		return nil
	})
	assert.NoError(t, err)
}

func TestWithTimeout_Exceeds(t *testing.T) {
	err := WithTimeout(context.Background(), 50*time.Millisecond, func(ctx context.Context) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(5 * time.Second):
			return nil
		}
	})

	require.Error(t, err)
	assert.True(t, errors.Is(err, context.DeadlineExceeded), "expected DeadlineExceeded, got: %v", err)
}

func TestWithTimeout_PropagatesError(t *testing.T) {
	expectedErr := errors.New("operation failed")
	err := WithTimeout(context.Background(), 1*time.Second, func(ctx context.Context) error {
		return expectedErr
	})

	assert.ErrorIs(t, err, expectedErr)
}
