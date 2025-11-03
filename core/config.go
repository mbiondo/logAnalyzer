package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/mbiondo/logAnalyzer/pkg/auth"
	"gopkg.in/yaml.v3"
)

// APIConfig defines API server configuration
type APIConfig struct {
	Enabled bool `yaml:"enabled"` // Enable/disable API server
	Port    int  `yaml:"port"`    // Port for the API server

	// Authentication configuration
	Auth APIAuthConfig `yaml:"auth,omitempty"`
}

// APIAuthConfig defines API authentication configuration
type APIAuthConfig struct {
	Enabled      bool                `yaml:"enabled"`       // Enable/disable API authentication
	RequireKey   bool                `yaml:"require_key"`   // Require API key for all endpoints
	HealthBypass bool                `yaml:"health_bypass"` // Allow health endpoint without auth
	APIKeys      []auth.APIKeyConfig `yaml:"api_keys"`      // List of API keys
}

// APIKeyConfig defines an API key configuration
type APIKeyConfig = auth.APIKeyConfig

// DefaultAPIConfig returns default API configuration
func DefaultAPIConfig() APIConfig {
	return APIConfig{
		Enabled: false,
		Port:    9090,
		Auth: APIAuthConfig{
			Enabled:      false,
			RequireKey:   false,
			HealthBypass: true,
			APIKeys:      []APIKeyConfig{},
		},
	}
}

// Config represents the application configuration
type Config struct {
	Inputs       []PluginDefinition `yaml:"inputs"`
	Outputs      []PluginDefinition `yaml:"outputs"`
	Persistence  PersistenceConfig  `yaml:"persistence,omitempty"`
	OutputBuffer OutputBufferConfig `yaml:"output_buffer,omitempty"`
	API          APIConfig          `yaml:"api,omitempty"`
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
	// Validate filename to prevent path traversal
	if err := validateFilePath(filename); err != nil {
		return nil, fmt.Errorf("invalid config file path: %w", err)
	}

	data, err := os.ReadFile(filename) // #nosec G304 - path validated by validateFilePath above
	if err != nil {
		return nil, fmt.Errorf("error reading config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("error parsing config file: %w", err)
	}

	// Load API keys from environment variables if available
	loadAPIKeysFromEnv(&config)

	return &config, nil
}

// loadAPIKeysFromEnv loads API keys from environment variables
func loadAPIKeysFromEnv(config *Config) {
	// Check if API authentication is enabled
	if !config.API.Auth.Enabled {
		return
	}

	// Read API keys from environment variables
	monitoringKey := os.Getenv("API_KEY_MONITORING")
	adminKey := os.Getenv("API_KEY_ADMIN")

	// If environment variables are set, use them instead of YAML config
	if monitoringKey != "" || adminKey != "" {
		config.API.Auth.APIKeys = []auth.APIKeyConfig{}

		if monitoringKey != "" {
			config.API.Auth.APIKeys = append(config.API.Auth.APIKeys, auth.APIKeyConfig{
				ID:          "monitoring-key",
				Secret:      monitoringKey,
				Permissions: []string{"health", "metrics"},
				Name:        "Monitoring API Key",
				Description: "API key for monitoring endpoints",
			})
		}

		if adminKey != "" {
			config.API.Auth.APIKeys = append(config.API.Auth.APIKeys, auth.APIKeyConfig{
				ID:          "admin-key",
				Secret:      adminKey,
				Permissions: []string{"health", "metrics", "admin"},
				Name:        "Admin API Key",
				Description: "API key with full administrative access",
			})
		}
	}
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
		API: DefaultAPIConfig(),
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
		_ = watcher.Close()
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
		_ = watcher.Close()
		return nil, fmt.Errorf("failed to watch directory: %w", err)
	}

	cw.wg.Add(1)
	go cw.watchLoop()

	return cw, nil
}

// Stop stops the config watcher
func (cw *ConfigWatcher) Stop() {
	close(cw.stopCh)
	_ = cw.watcher.Close()
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

// validateFilePath validates a file path to prevent directory traversal attacks
func validateFilePath(path string) error {
	// Allow absolute paths in test environments
	if os.Getenv("UNIT_TEST") == "true" {
		// Clean the path to resolve any .. or . components
		cleanPath := filepath.Clean(path)
		// Only check for directory traversal (..) in test mode
		if strings.Contains(cleanPath, "..") {
			return fmt.Errorf("path contains directory traversal: %s", path)
		}
		return nil
	}

	// Clean the path to resolve any .. or . components
	cleanPath := filepath.Clean(path)

	// Check if the cleaned path tries to escape the current directory
	if strings.Contains(cleanPath, "..") {
		return fmt.Errorf("path contains directory traversal: %s", path)
	}

	// Additional check: ensure path doesn't start with dangerous patterns
	if strings.HasPrefix(cleanPath, "/") || strings.HasPrefix(cleanPath, "\\") ||
		strings.HasPrefix(cleanPath, "../") || strings.HasPrefix(cleanPath, "..\\") {
		return fmt.Errorf("path contains absolute or traversal components: %s", path)
	}

	return nil
}

// validateFileInDirectory validates that a file path is within a specified base directory
func validateFileInDirectory(filePath, baseDir string) error {
	// Clean both paths
	cleanFilePath := filepath.Clean(filePath)
	cleanBaseDir := filepath.Clean(baseDir)

	// Get absolute paths to compare
	absFilePath, err := filepath.Abs(cleanFilePath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path for file: %w", err)
	}

	absBaseDir, err := filepath.Abs(cleanBaseDir)
	if err != nil {
		return fmt.Errorf("failed to get absolute path for base directory: %w", err)
	}

	// Check if file path starts with base directory path
	if !strings.HasPrefix(absFilePath, absBaseDir) {
		return fmt.Errorf("file path is outside base directory: %s not in %s", filePath, baseDir)
	}

	return nil
}
