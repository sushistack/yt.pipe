package plugin

import (
	"context"
	"time"
)

// PluginConfig holds common configuration for all plugins.
type PluginConfig struct {
	Name       string
	Timeout    time.Duration
	MaxRetries int
	BaseDelay  time.Duration
}

// DefaultPluginConfig returns sensible defaults.
func DefaultPluginConfig(name string) PluginConfig {
	return PluginConfig{
		Name:       name,
		Timeout:    120 * time.Second, // NFR10: configurable, default 120s
		MaxRetries: 3,                 // NFR6: max 3 retries
		BaseDelay:  1 * time.Second,
	}
}

// WithTimeout executes fn with a context timeout derived from the given duration.
func WithTimeout(ctx context.Context, timeout time.Duration, fn func(ctx context.Context) error) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return fn(ctx)
}
