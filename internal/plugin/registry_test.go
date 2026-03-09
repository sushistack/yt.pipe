package plugin

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegistry_RegisterAndCreate(t *testing.T) {
	r := NewRegistry()

	called := false
	err := r.Register(PluginTypeLLM, "openai", func(cfg map[string]interface{}) (interface{}, error) {
		called = true
		return "mock-llm-instance", nil
	})
	require.NoError(t, err)

	result, err := r.Create(PluginTypeLLM, "openai", nil)
	require.NoError(t, err)
	assert.True(t, called)
	assert.Equal(t, "mock-llm-instance", result)
}

func TestRegistry_UnknownProvider(t *testing.T) {
	r := NewRegistry()

	require.NoError(t, r.Register(PluginTypeLLM, "openai", func(cfg map[string]interface{}) (interface{}, error) {
		return nil, nil
	}))
	require.NoError(t, r.Register(PluginTypeLLM, "anthropic", func(cfg map[string]interface{}) (interface{}, error) {
		return nil, nil
	}))

	_, err := r.Create(PluginTypeLLM, "unknown", nil)
	require.Error(t, err)

	var pluginErr *ProviderNotFoundError
	require.True(t, errors.As(err, &pluginErr))
	assert.Equal(t, PluginTypeLLM, pluginErr.PluginType)
	assert.Equal(t, "unknown", pluginErr.Provider)
	assert.Contains(t, pluginErr.Available, "openai")
	assert.Contains(t, pluginErr.Available, "anthropic")
	// Error message should list available providers
	assert.Contains(t, pluginErr.Error(), "openai")
	assert.Contains(t, pluginErr.Error(), "anthropic")
}

func TestRegistry_Providers(t *testing.T) {
	r := NewRegistry()

	require.NoError(t, r.Register(PluginTypeTTS, "google", func(cfg map[string]interface{}) (interface{}, error) {
		return nil, nil
	}))
	require.NoError(t, r.Register(PluginTypeTTS, "azure", func(cfg map[string]interface{}) (interface{}, error) {
		return nil, nil
	}))

	providers := r.Providers(PluginTypeTTS)
	assert.Equal(t, []string{"azure", "google"}, providers) // sorted
}

func TestRegistry_ProvidersEmpty(t *testing.T) {
	r := NewRegistry()
	providers := r.Providers(PluginTypeLLM)
	assert.Nil(t, providers)
}

func TestRegistry_MultipleTypes(t *testing.T) {
	r := NewRegistry()

	require.NoError(t, r.Register(PluginTypeLLM, "openai", func(cfg map[string]interface{}) (interface{}, error) {
		return "llm-instance", nil
	}))
	require.NoError(t, r.Register(PluginTypeTTS, "google", func(cfg map[string]interface{}) (interface{}, error) {
		return "tts-instance", nil
	}))
	require.NoError(t, r.Register(PluginTypeImageGen, "dalle", func(cfg map[string]interface{}) (interface{}, error) {
		return "imagegen-instance", nil
	}))

	llm, err := r.Create(PluginTypeLLM, "openai", nil)
	require.NoError(t, err)
	assert.Equal(t, "llm-instance", llm)

	tts, err := r.Create(PluginTypeTTS, "google", nil)
	require.NoError(t, err)
	assert.Equal(t, "tts-instance", tts)

	img, err := r.Create(PluginTypeImageGen, "dalle", nil)
	require.NoError(t, err)
	assert.Equal(t, "imagegen-instance", img)

	// Cross-type lookup should fail
	_, err = r.Create(PluginTypeLLM, "google", nil)
	assert.Error(t, err)
}

func TestRegistry_FactoryError(t *testing.T) {
	r := NewRegistry()

	expectedErr := errors.New("initialization failed")
	require.NoError(t, r.Register(PluginTypeLLM, "broken", func(cfg map[string]interface{}) (interface{}, error) {
		return nil, expectedErr
	}))

	_, err := r.Create(PluginTypeLLM, "broken", nil)
	assert.ErrorIs(t, err, expectedErr)
}

func TestRegistry_UnregisteredType(t *testing.T) {
	r := NewRegistry()

	_, err := r.Create(PluginTypeOutput, "capcut", nil)
	require.Error(t, err)

	var pluginErr *ProviderNotFoundError
	require.True(t, errors.As(err, &pluginErr))
	assert.Nil(t, pluginErr.Available)
}

func TestRegistry_ConfigPassthrough(t *testing.T) {
	r := NewRegistry()

	var receivedCfg map[string]interface{}
	require.NoError(t, r.Register(PluginTypeLLM, "openai", func(cfg map[string]interface{}) (interface{}, error) {
		receivedCfg = cfg
		return "instance", nil
	}))

	cfg := map[string]interface{}{
		"api_key": "test-key",
		"model":   "gpt-4",
	}

	_, err := r.Create(PluginTypeLLM, "openai", cfg)
	require.NoError(t, err)
	assert.Equal(t, cfg, receivedCfg)
}

func TestRegistry_DuplicateRegistration(t *testing.T) {
	r := NewRegistry()

	factory := func(cfg map[string]interface{}) (interface{}, error) {
		return "instance", nil
	}

	require.NoError(t, r.Register(PluginTypeLLM, "openai", factory))

	// Second registration with same type+provider should fail
	err := r.Register(PluginTypeLLM, "openai", factory)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already registered")

	// Different provider for same type should succeed
	require.NoError(t, r.Register(PluginTypeLLM, "anthropic", factory))

	// Same provider for different type should succeed
	require.NoError(t, r.Register(PluginTypeTTS, "openai", factory))
}
