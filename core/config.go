package core

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config represents the application configuration
type Config struct {
	Input  InputConfig  `yaml:"input"`
	Filter FilterConfig `yaml:"filter"`
	Output OutputConfig `yaml:"output"`
}

// InputConfig represents input plugin configuration with dynamic plugin configs
type InputConfig struct {
	// Single input (backward compatibility)
	Type   string                 `yaml:"type,omitempty"`
	Config map[string]interface{} `yaml:"config,omitempty"`

	// Multiple inputs (new feature)
	Inputs []PluginDefinition `yaml:"inputs,omitempty"`
}

// PluginDefinition represents a generic plugin definition
type PluginDefinition struct {
	Type   string                 `yaml:"type"`           // Plugin type: "file", "docker", "http", "slack", etc.
	Name   string                 `yaml:"name,omitempty"` // Optional name to identify this plugin instance
	Config map[string]interface{} `yaml:"config"`         // Dynamic configuration for the plugin

	// Output-specific options
	Sources []string           `yaml:"sources,omitempty"` // Input sources to accept logs from (empty = all)
	Filters []PluginDefinition `yaml:"filters,omitempty"` // Filters to apply before this output
}

// FilterConfig represents filter plugin configuration with dynamic plugin configs
type FilterConfig struct {
	// Single filter (backward compatibility)
	Type   string                 `yaml:"type,omitempty"`
	Config map[string]interface{} `yaml:"config,omitempty"`

	// Multiple filters (new feature)
	Filters []PluginDefinition `yaml:"filters,omitempty"`
}

// OutputConfig represents output configuration with dynamic plugin configs
type OutputConfig struct {
	// Single output (backward compatibility)
	Type   string                 `yaml:"type,omitempty"`
	Config map[string]interface{} `yaml:"config,omitempty"`

	// Multiple outputs (new feature)
	Outputs []PluginDefinition `yaml:"outputs,omitempty"`
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
func GetPluginConfig(pluginConfig map[string]interface{}, target interface{}) error {
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
		Input: InputConfig{
			Type: "file",
			Config: map[string]interface{}{
				"path":     "app.log",
				"encoding": "utf-8",
			},
		},
		Filter: FilterConfig{
			Type: "level",
			Config: map[string]interface{}{
				"levels": []string{"error", "warn"},
			},
		},
		Output: OutputConfig{
			Type: "prometheus",
			Config: map[string]interface{}{
				"port": 9090,
			},
		},
	}
}
