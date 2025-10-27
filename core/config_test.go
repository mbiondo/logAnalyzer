package core

import (
	"os"
	"testing"
)

func TestLoadDynamicConfig(t *testing.T) {
	// Create a temporary config file with dynamic format
	configContent := `
input:
  type: file
  config:
    path: "/var/log/app.log"
    encoding: "utf-8"

filter:
  type: level
  config:
    levels: ["error", "warn", "info"]

output:
  outputs:
    - type: console
      config:
        target: "stdout"
        format: "json"
    - type: slack
      config:
        webhook_url: "https://hooks.slack.com/services/xxx"
        channel: "#alerts"
        username: "LogBot"
        timeout: 30
    - type: prometheus
      config:
        port: 9090
`

	tmpFile, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write([]byte(configContent)); err != nil {
		t.Fatalf("failed to write to temp file: %v", err)
	}
	tmpFile.Close()

	// Load config
	config, err := LoadConfig(tmpFile.Name())
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	// Test input config
	if config.Input.Type != "file" {
		t.Errorf("expected input type 'file', got '%s'", config.Input.Type)
	}

	if config.Input.Config["path"] != "/var/log/app.log" {
		t.Errorf("expected path '/var/log/app.log', got '%v'", config.Input.Config["path"])
	}

	// Test filter config
	if config.Filter.Type != "level" {
		t.Errorf("expected filter type 'level', got '%s'", config.Filter.Type)
	}

	if levels, ok := config.Filter.Config["levels"].([]interface{}); !ok || len(levels) != 3 {
		t.Errorf("expected 3 filter levels in config, got %v", config.Filter.Config["levels"])
	}

	// Test output config
	if len(config.Output.Outputs) != 3 {
		t.Fatalf("expected 3 outputs, got %d", len(config.Output.Outputs))
	}

	// Verify first output (console)
	if config.Output.Outputs[0].Type != "console" {
		t.Errorf("expected output type 'console', got '%s'", config.Output.Outputs[0].Type)
	}

	// Verify second output (slack)
	if config.Output.Outputs[1].Type != "slack" {
		t.Errorf("expected output type 'slack', got '%s'", config.Output.Outputs[1].Type)
	}

	slackConfig := config.Output.Outputs[1].Config
	if slackConfig["channel"] != "#alerts" {
		t.Errorf("expected slack channel '#alerts', got '%v'", slackConfig["channel"])
	}

	// Verify third output (prometheus)
	if config.Output.Outputs[2].Type != "prometheus" {
		t.Errorf("expected output type 'prometheus', got '%s'", config.Output.Outputs[2].Type)
	}
}

func TestGetPluginConfig(t *testing.T) {
	// Test with Slack config
	type TestSlackConfig struct {
		WebhookURL string `yaml:"webhook_url"`
		Channel    string `yaml:"channel"`
		Username   string `yaml:"username"`
		Timeout    int    `yaml:"timeout"`
	}

	pluginConfig := map[string]interface{}{
		"webhook_url": "https://hooks.slack.com/services/xxx",
		"channel":     "#alerts",
		"username":    "LogBot",
		"timeout":     30,
	}

	var slackConfig TestSlackConfig
	err := GetPluginConfig(pluginConfig, &slackConfig)
	if err != nil {
		t.Fatalf("failed to get plugin config: %v", err)
	}

	if slackConfig.WebhookURL != "https://hooks.slack.com/services/xxx" {
		t.Errorf("expected webhook_url, got '%s'", slackConfig.WebhookURL)
	}
	if slackConfig.Channel != "#alerts" {
		t.Errorf("expected channel '#alerts', got '%s'", slackConfig.Channel)
	}
	if slackConfig.Username != "LogBot" {
		t.Errorf("expected username 'LogBot', got '%s'", slackConfig.Username)
	}
	if slackConfig.Timeout != 30 {
		t.Errorf("expected timeout 30, got %d", slackConfig.Timeout)
	}
}

func TestGetPluginConfigWithComplexStructs(t *testing.T) {
	// Test with Docker config
	type TestDockerConfig struct {
		ContainerIDs []string          `yaml:"container_ids"`
		Labels       map[string]string `yaml:"labels"`
		Stream       string            `yaml:"stream"`
	}

	pluginConfig := map[string]interface{}{
		"container_ids": []interface{}{"web", "api"},
		"labels": map[string]interface{}{
			"app": "myapp",
			"env": "prod",
		},
		"stream": "stdout",
	}

	var dockerConfig TestDockerConfig
	err := GetPluginConfig(pluginConfig, &dockerConfig)
	if err != nil {
		t.Fatalf("failed to get plugin config: %v", err)
	}

	if len(dockerConfig.ContainerIDs) != 2 {
		t.Errorf("expected 2 container IDs, got %d", len(dockerConfig.ContainerIDs))
	}
	if dockerConfig.Stream != "stdout" {
		t.Errorf("expected stream 'stdout', got '%s'", dockerConfig.Stream)
	}
	if dockerConfig.Labels["app"] != "myapp" {
		t.Errorf("expected label app='myapp', got '%s'", dockerConfig.Labels["app"])
	}
}

func TestDefaultDynamicConfig(t *testing.T) {
	config := DefaultConfig()

	if config.Input.Type != "file" {
		t.Errorf("expected input type 'file', got '%s'", config.Input.Type)
	}

	if config.Output.Type != "prometheus" {
		t.Errorf("expected output type 'prometheus', got '%s'", config.Output.Type)
	}

	if config.Filter.Type != "level" {
		t.Errorf("expected filter type 'level', got '%s'", config.Filter.Type)
	}

	if levels, ok := config.Filter.Config["levels"].([]string); !ok || len(levels) != 2 {
		t.Errorf("expected 2 filter levels, got %v", config.Filter.Config["levels"])
	}
}

func TestBackwardCompatibility(t *testing.T) {
	// Test single input/output format still works
	configContent := `
input:
  type: http
  config:
    port: "8080"

filter:
  type: level
  config:
    levels: ["error"]

output:
  type: file
  config:
    file_path: "/var/log/output.log"
`

	tmpFile, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write([]byte(configContent)); err != nil {
		t.Fatalf("failed to write to temp file: %v", err)
	}
	tmpFile.Close()

	config, err := LoadConfig(tmpFile.Name())
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if config.Input.Type != "http" {
		t.Errorf("expected input type 'http', got '%s'", config.Input.Type)
	}

	if config.Output.Type != "file" {
		t.Errorf("expected output type 'file', got '%s'", config.Output.Type)
	}
}

func TestMultipleInputs(t *testing.T) {
	configContent := `
input:
  inputs:
    - type: file
      config:
        path: "/var/log/app1.log"
    - type: docker
      config:
        container_ids: ["web"]
        stream: "stdout"
    - type: http
      config:
        port: "8080"

filter:
  type: level
  config:
    levels: ["error", "warn"]

output:
  type: console
  config:
    target: "stdout"
    format: "text"
`

	tmpFile, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write([]byte(configContent)); err != nil {
		t.Fatalf("failed to write to temp file: %v", err)
	}
	tmpFile.Close()

	config, err := LoadConfig(tmpFile.Name())
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if len(config.Input.Inputs) != 3 {
		t.Errorf("expected 3 inputs, got %d", len(config.Input.Inputs))
	}

	// Verify each input type
	if config.Input.Inputs[0].Type != "file" {
		t.Errorf("expected first input type 'file', got '%s'", config.Input.Inputs[0].Type)
	}
	if config.Input.Inputs[1].Type != "docker" {
		t.Errorf("expected second input type 'docker', got '%s'", config.Input.Inputs[1].Type)
	}
	if config.Input.Inputs[2].Type != "http" {
		t.Errorf("expected third input type 'http', got '%s'", config.Input.Inputs[2].Type)
	}
}

func TestMultipleFilters(t *testing.T) {
	configContent := `
input:
  type: file
  config:
    path: "/var/log/app.log"

filter:
  filters:
    - type: level
      config:
        levels: ["error", "warn", "info"]
    - type: regex
      config:
        patterns: ["ERROR.*", "WARN.*"]
        mode: "include"
        field: "message"

output:
  type: console
  config:
    target: "stdout"
`

	tmpFile, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write([]byte(configContent)); err != nil {
		t.Fatalf("failed to write to temp file: %v", err)
	}
	tmpFile.Close()

	config, err := LoadConfig(tmpFile.Name())
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if len(config.Filter.Filters) != 2 {
		t.Errorf("expected 2 filters, got %d", len(config.Filter.Filters))
	}

	// Verify each filter type
	if config.Filter.Filters[0].Type != "level" {
		t.Errorf("expected first filter type 'level', got '%s'", config.Filter.Filters[0].Type)
	}
	if config.Filter.Filters[1].Type != "regex" {
		t.Errorf("expected second filter type 'regex', got '%s'", config.Filter.Filters[1].Type)
	}

	// Verify filter configs
	if levels, ok := config.Filter.Filters[0].Config["levels"].([]interface{}); !ok || len(levels) != 3 {
		t.Errorf("expected 3 levels in first filter, got %v", config.Filter.Filters[0].Config["levels"])
	}

	if patterns, ok := config.Filter.Filters[1].Config["patterns"].([]interface{}); !ok || len(patterns) != 2 {
		t.Errorf("expected 2 patterns in second filter, got %v", config.Filter.Filters[1].Config["patterns"])
	}
}
