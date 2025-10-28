package core

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"gopkg.in/yaml.v3"
)

// Config represents the application configuration
type Config struct {
	Inputs  []PluginDefinition `yaml:"inputs"`
	Outputs []PluginDefinition `yaml:"outputs"`
}

// PluginDefinition represents a generic plugin definition
type PluginDefinition struct {
	Type   string         `yaml:"type"`           // Plugin type: "file", "docker", "http", "slack", etc.
	Name   string         `yaml:"name,omitempty"` // Optional name to identify this plugin instance
	Config map[string]any `yaml:"config"`         // Dynamic configuration for the plugin

	// Output-specific options
	Sources []string           `yaml:"sources,omitempty"` // Input sources to accept logs from (empty = all)
	Filters []PluginDefinition `yaml:"filters,omitempty"` // Filters to apply before this output
}

// LoadConfig loads configuration from a YAML file
func LoadConfig(filename string) (*Config, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("error reading config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("error parsing config file: %w", err)
	}

	return &config, nil
}

// GetPluginConfig extracts and unmarshals plugin-specific configuration
func GetPluginConfig(pluginConfig map[string]any, target any) error {
	// Convert map to YAML then unmarshal to target struct
	data, err := yaml.Marshal(pluginConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal plugin config: %w", err)
	}

	if err := yaml.Unmarshal(data, target); err != nil {
		return fmt.Errorf("failed to unmarshal plugin config: %w", err)
	}

	return nil
}

// DefaultConfig returns a default configuration
func DefaultConfig() *Config {
	return &Config{
		Inputs: []PluginDefinition{
			{
				Type: "file",
				Config: map[string]any{
					"path":     "app.log",
					"encoding": "utf-8",
				},
			},
		},
		Outputs: []PluginDefinition{
			{
				Type: "prometheus",
				Config: map[string]any{
					"port": 9090,
				},
			},
		},
	}
}

// ConfigWatcher monitors a config file for changes and triggers reloads
type ConfigWatcher struct {
	filename    string
	watcher     *fsnotify.Watcher
	onReload    func(*Config)
	stopCh      chan struct{}
	wg          sync.WaitGroup
	lastModTime time.Time
	mu          sync.Mutex
}

// NewConfigWatcher creates a new config file watcher
func NewConfigWatcher(filename string, onReload func(*Config)) (*ConfigWatcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create file watcher: %w", err)
	}

	// Get initial file modification time
	info, err := os.Stat(filename)
	if err != nil {
		watcher.Close()
		return nil, fmt.Errorf("failed to stat config file: %w", err)
	}

	cw := &ConfigWatcher{
		filename:    filename,
		watcher:     watcher,
		onReload:    onReload,
		stopCh:      make(chan struct{}),
		lastModTime: info.ModTime(),
	}

	// Watch the directory containing the config file
	// This handles cases where the file is replaced atomically
	dir := filename
	if idx := len(filename) - 1; idx >= 0 {
		for i := idx; i >= 0; i-- {
			if filename[i] == '/' || filename[i] == '\\' {
				dir = filename[:i]
				break
			}
		}
	}

	if err := watcher.Add(dir); err != nil {
		watcher.Close()
		return nil, fmt.Errorf("failed to watch directory: %w", err)
	}

	cw.wg.Add(1)
	go cw.watchLoop()

	return cw, nil
}

// Stop stops the config watcher
func (cw *ConfigWatcher) Stop() {
	close(cw.stopCh)
	cw.watcher.Close()
	cw.wg.Wait()
}

// watchLoop runs the file watching loop
func (cw *ConfigWatcher) watchLoop() {
	defer cw.wg.Done()

	for {
		select {
		case event, ok := <-cw.watcher.Events:
			if !ok {
				return
			}

			// Check if the event is for our config file
			if event.Name != cw.filename {
				continue
			}

			// Only react to write events
			if event.Op&fsnotify.Write == fsnotify.Write {
				cw.handleFileChange()
			}

		case err, ok := <-cw.watcher.Errors:
			if !ok {
				return
			}
			fmt.Printf("Config watcher error: %v\n", err)

		case <-cw.stopCh:
			return
		}
	}
}

// handleFileChange handles a config file change event
func (cw *ConfigWatcher) handleFileChange() {
	cw.mu.Lock()
	defer cw.mu.Unlock()

	// Check if file was actually modified (avoid duplicate events)
	info, err := os.Stat(cw.filename)
	if err != nil {
		fmt.Printf("Error checking config file: %v\n", err)
		return
	}

	if info.ModTime().Equal(cw.lastModTime) {
		return // No actual change
	}

	cw.lastModTime = info.ModTime()

	// Small delay to ensure file write is complete
	time.Sleep(100 * time.Millisecond)

	// Load new config
	config, err := LoadConfig(cw.filename)
	if err != nil {
		fmt.Printf("Error reloading config: %v\n", err)
		return
	}

	fmt.Printf("Config file changed, reloading...\n")
	cw.onReload(config)
}
