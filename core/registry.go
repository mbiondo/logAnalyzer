package core

import (
	"fmt"
	"sync"
)

// PluginFactory is a function that creates a plugin instance from configuration
type PluginFactory func(config map[string]interface{}) (interface{}, error)

// PluginRegistry manages plugin registration and instantiation
type PluginRegistry struct {
	inputs  map[string]PluginFactory
	outputs map[string]PluginFactory
	filters map[string]PluginFactory
	mu      sync.RWMutex
}

var (
	// Global plugin registry
	registry = &PluginRegistry{
		inputs:  make(map[string]PluginFactory),
		outputs: make(map[string]PluginFactory),
		filters: make(map[string]PluginFactory),
	}
)

// RegisterInputPlugin registers an input plugin factory
func RegisterInputPlugin(name string, factory PluginFactory) {
	registry.mu.Lock()
	defer registry.mu.Unlock()
	registry.inputs[name] = factory
}

// RegisterOutputPlugin registers an output plugin factory
func RegisterOutputPlugin(name string, factory PluginFactory) {
	registry.mu.Lock()
	defer registry.mu.Unlock()
	registry.outputs[name] = factory
}

// RegisterFilterPlugin registers a filter plugin factory
func RegisterFilterPlugin(name string, factory PluginFactory) {
	registry.mu.Lock()
	defer registry.mu.Unlock()
	registry.filters[name] = factory
}

// CreateInputPlugin creates an input plugin instance
func CreateInputPlugin(pluginType string, config map[string]interface{}) (InputPlugin, error) {
	registry.mu.RLock()
	factory, exists := registry.inputs[pluginType]
	registry.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("unknown input plugin type: %s", pluginType)
	}

	plugin, err := factory(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create input plugin %s: %w", pluginType, err)
	}

	inputPlugin, ok := plugin.(InputPlugin)
	if !ok {
		return nil, fmt.Errorf("plugin %s does not implement InputPlugin interface", pluginType)
	}

	return inputPlugin, nil
}

// CreateOutputPlugin creates an output plugin instance
func CreateOutputPlugin(pluginType string, config map[string]interface{}) (OutputPlugin, error) {
	registry.mu.RLock()
	factory, exists := registry.outputs[pluginType]
	registry.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("unknown output plugin type: %s", pluginType)
	}

	plugin, err := factory(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create output plugin %s: %w", pluginType, err)
	}

	outputPlugin, ok := plugin.(OutputPlugin)
	if !ok {
		return nil, fmt.Errorf("plugin %s does not implement OutputPlugin interface", pluginType)
	}

	return outputPlugin, nil
}

// CreateFilterPlugin creates a filter plugin instance
func CreateFilterPlugin(pluginType string, config map[string]interface{}) (FilterPlugin, error) {
	registry.mu.RLock()
	factory, exists := registry.filters[pluginType]
	registry.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("unknown filter plugin type: %s", pluginType)
	}

	plugin, err := factory(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create filter plugin %s: %w", pluginType, err)
	}

	filterPlugin, ok := plugin.(FilterPlugin)
	if !ok {
		return nil, fmt.Errorf("plugin %s does not implement FilterPlugin interface", pluginType)
	}

	return filterPlugin, nil
}

// ListInputPlugins returns all registered input plugin names
func ListInputPlugins() []string {
	registry.mu.RLock()
	defer registry.mu.RUnlock()

	names := make([]string, 0, len(registry.inputs))
	for name := range registry.inputs {
		names = append(names, name)
	}
	return names
}

// ListOutputPlugins returns all registered output plugin names
func ListOutputPlugins() []string {
	registry.mu.RLock()
	defer registry.mu.RUnlock()

	names := make([]string, 0, len(registry.outputs))
	for name := range registry.outputs {
		names = append(names, name)
	}
	return names
}

// ListFilterPlugins returns all registered filter plugin names
func ListFilterPlugins() []string {
	registry.mu.RLock()
	defer registry.mu.RUnlock()

	names := make([]string, 0, len(registry.filters))
	for name := range registry.filters {
		names = append(names, name)
	}
	return names
}
