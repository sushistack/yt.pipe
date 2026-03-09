package plugin

import (
	"fmt"
	"sort"
	"sync"
)

// PluginType identifies the category of plugin.
type PluginType string

const (
	PluginTypeLLM      PluginType = "llm"
	PluginTypeTTS      PluginType = "tts"
	PluginTypeImageGen PluginType = "imagegen"
	PluginTypeOutput   PluginType = "output"
)

// Factory creates a plugin instance from configuration.
// The returned interface{} must be type-asserted to the specific plugin interface.
type Factory func(cfg map[string]interface{}) (interface{}, error)

// Registry manages plugin provider registration and creation.
// Registry is NOT a singleton — create in main/bootstrap, pass via DI.
type Registry struct {
	mu        sync.RWMutex
	factories map[PluginType]map[string]Factory
}

// NewRegistry creates a new empty plugin registry.
func NewRegistry() *Registry {
	return &Registry{
		factories: make(map[PluginType]map[string]Factory),
	}
}

// Register adds a factory for the given plugin type and provider name.
// Returns an error if a factory is already registered for the same type and provider.
func (r *Registry) Register(pluginType PluginType, provider string, factory Factory) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.factories[pluginType] == nil {
		r.factories[pluginType] = make(map[string]Factory)
	}
	if _, exists := r.factories[pluginType][provider]; exists {
		return fmt.Errorf("plugin %s: provider %q already registered for type %q", provider, provider, pluginType)
	}
	r.factories[pluginType][provider] = factory
	return nil
}

// Create instantiates a plugin using the registered factory for the given type and provider.
// Returns a ProviderNotFoundError with available providers listed if the provider is not found.
func (r *Registry) Create(pluginType PluginType, provider string, cfg map[string]interface{}) (interface{}, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	providers, ok := r.factories[pluginType]
	if !ok || providers == nil {
		return nil, &ProviderNotFoundError{
			PluginType: pluginType,
			Provider:   provider,
			Available:  nil,
		}
	}

	factory, ok := providers[provider]
	if !ok {
		return nil, &ProviderNotFoundError{
			PluginType: pluginType,
			Provider:   provider,
			Available:  r.providersLocked(pluginType),
		}
	}

	return factory(cfg)
}

// Providers returns a sorted list of registered provider names for the given plugin type.
func (r *Registry) Providers(pluginType PluginType) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.providersLocked(pluginType)
}

// providersLocked returns providers without locking (caller must hold lock).
func (r *Registry) providersLocked(pluginType PluginType) []string {
	providers, ok := r.factories[pluginType]
	if !ok {
		return nil
	}
	names := make([]string, 0, len(providers))
	for name := range providers {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// ProviderNotFoundError indicates that a requested plugin provider was not found.
type ProviderNotFoundError struct {
	PluginType PluginType
	Provider   string
	Available  []string
}

func (e *ProviderNotFoundError) Error() string {
	if len(e.Available) == 0 {
		return fmt.Sprintf("plugin %s: provider %q not found, no providers registered", e.PluginType, e.Provider)
	}
	return fmt.Sprintf("plugin %s: unknown provider %q (available: %v)", e.PluginType, e.Provider, e.Available)
}
