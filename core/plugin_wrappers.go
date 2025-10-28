package core

import (
	"log"
	"sync"
)

// ResilientInputPlugin wraps an input plugin with resilience
type ResilientInputPlugin struct {
	resilient *ResilientPlugin
	logCh     chan<- *Log
	mu        sync.RWMutex
}

// NewResilientInputPlugin creates a resilient input plugin
func NewResilientInputPlugin(name, pluginType string, factory PluginFactory, config map[string]any, logCh chan<- *Log, resilientConfig ResilientPluginConfig) *ResilientInputPlugin {
	return &ResilientInputPlugin{
		resilient: NewResilientPlugin(name, pluginType, factory, config, resilientConfig),
		logCh:     logCh,
	}
}

// Start starts the input plugin (delegated to ResilientPlugin)
func (r *ResilientInputPlugin) Start() error {
	// ResilientPlugin handles start automatically
	return nil
}

// Stop stops the input plugin
func (r *ResilientInputPlugin) Stop() error {
	return r.resilient.Close()
}

// SetLogChannel sets the log channel
func (r *ResilientInputPlugin) SetLogChannel(ch chan<- *Log) {
	r.mu.Lock()
	r.logCh = ch
	r.mu.Unlock()

	// If plugin is already healthy, update its channel
	if plugin, err := r.resilient.GetPlugin(); err == nil {
		if inputPlugin, ok := plugin.(InputPlugin); ok {
			inputPlugin.SetLogChannel(ch)
		}
	}
}

// SetName sets the plugin name (for compatibility)
func (r *ResilientInputPlugin) SetName(name string) {
	// Name is already set in resilient plugin
}

// IsHealthy returns true if underlying plugin is healthy
func (r *ResilientInputPlugin) IsHealthy() bool {
	return r.resilient.IsHealthy()
}

// GetStats returns statistics
func (r *ResilientInputPlugin) GetStats() map[string]any {
	return r.resilient.GetStats()
}

// ResilientOutputPlugin wraps an output plugin with resilience
type ResilientOutputPlugin struct {
	resilient *ResilientPlugin
}

// NewResilientOutputPlugin creates a resilient output plugin
func NewResilientOutputPlugin(name, pluginType string, factory PluginFactory, config map[string]any, resilientConfig ResilientPluginConfig) *ResilientOutputPlugin {
	return &ResilientOutputPlugin{
		resilient: NewResilientPlugin(name, pluginType, factory, config, resilientConfig),
	}
}

// Write writes a log entry
func (r *ResilientOutputPlugin) Write(logEntry *Log) error {
	plugin, err := r.resilient.GetPlugin()
	if err != nil {
		// Plugin not healthy, log warning but don't fail
		// The output buffer will handle retries
		log.Printf("[RESILIENT-OUTPUT:%s] Plugin not available, buffering will handle retry: %v",
			r.resilient.name, err)
		return err
	}

	outputPlugin, ok := plugin.(OutputPlugin)
	if !ok {
		log.Printf("[RESILIENT-OUTPUT:%s] Invalid plugin type", r.resilient.name)
		return ErrPluginNotAvailable
	}

	return outputPlugin.Write(logEntry)
}

// Close closes the output plugin
func (r *ResilientOutputPlugin) Close() error {
	return r.resilient.Close()
}

// IsHealthy returns true if underlying plugin is healthy
func (r *ResilientOutputPlugin) IsHealthy() bool {
	return r.resilient.IsHealthy()
}

// GetStats returns statistics
func (r *ResilientOutputPlugin) GetStats() map[string]any {
	return r.resilient.GetStats()
}

// ErrPluginNotAvailable is returned when plugin is not available
var ErrPluginNotAvailable = NewError("plugin not available")

// NewError creates a new error with a message
type pluginError struct {
	message string
}

func (e *pluginError) Error() string {
	return e.message
}

// NewError creates a new plugin error
func NewError(message string) error {
	return &pluginError{message: message}
}
