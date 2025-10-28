package core

import (
	"fmt"
	"os"

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
