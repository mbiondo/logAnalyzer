package core

import (
	"os"
	"testing"
)

func TestLoadDynamicConfig(t *testing.T) {
	// Create a temporary config file with dynamic format
	configContent := `
inputs:
  - type: file
    config:
      path: "/var/log/app.log"
      encoding: "utf-8"

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
	defer func() {
		_ = os.Remove(tmpFile.Name())
	}()

	if _, err := tmpFile.Write([]byte(configContent)); err != nil {
		t.Fatalf("failed to write to temp file: %v", err)
	}
	if err := tmpFile.Close(); err != nil {
		t.Fatalf("failed to close temp file: %v", err)
	}

	// Load config
	config, err := LoadConfig(tmpFile.Name())
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	// Test input config
	if len(config.Inputs) != 1 {
		t.Fatalf("expected 1 input, got %d", len(config.Inputs))
	}
	if config.Inputs[0].Type != "file" {
		t.Errorf("expected input type 'file', got '%s'", config.Inputs[0].Type)
	}

	if config.Inputs[0].Config["path"] != "/var/log/app.log" {
		t.Errorf("expected path '/var/log/app.log', got '%v'", config.Inputs[0].Config["path"])
	}

	// Test output config
	if len(config.Outputs) != 3 {
		t.Fatalf("expected 3 outputs, got %d", len(config.Outputs))
	}

	// Verify first output (console)
	if config.Outputs[0].Type != "console" {
		t.Errorf("expected output type 'console', got '%s'", config.Outputs[0].Type)
	}

	// Verify second output (slack)
	if config.Outputs[1].Type != "slack" {
		t.Errorf("expected output type 'slack', got '%s'", config.Outputs[1].Type)
	}

	slackConfig := config.Outputs[1].Config
	if slackConfig["channel"] != "#alerts" {
		t.Errorf("expected slack channel '#alerts', got '%v'", slackConfig["channel"])
	}

	// Verify third output (prometheus)
	if config.Outputs[2].Type != "prometheus" {
		t.Errorf("expected output type 'prometheus', got '%s'", config.Outputs[2].Type)
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

	pluginConfig := map[string]any{
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

	pluginConfig := map[string]any{
		"container_ids": []any{"web", "api"},
		"labels": map[string]any{
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

	if len(config.Inputs) != 1 {
		t.Errorf("expected 1 input, got %d", len(config.Inputs))
	}
	if config.Inputs[0].Type != "file" {
		t.Errorf("expected input type 'file', got '%s'", config.Inputs[0].Type)
	}

	if len(config.Outputs) != 1 {
		t.Errorf("expected 1 output, got %d", len(config.Outputs))
	}
	if config.Outputs[0].Type != "prometheus" {
		t.Errorf("expected output type 'prometheus', got '%s'", config.Outputs[0].Type)
	}
}

func TestBackwardCompatibility(t *testing.T) {
	// Test single input/output format still works (now as arrays)
	configContent := `
inputs:
  - type: http
    config:
      port: "8080"

outputs:
  - type: file
    config:
      file_path: "/var/log/output.log"
`

	tmpFile, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer func() {
		_ = os.Remove(tmpFile.Name())
	}()

	if _, err := tmpFile.Write([]byte(configContent)); err != nil {
		t.Fatalf("failed to write to temp file: %v", err)
	}
	if err := tmpFile.Close(); err != nil {
		t.Fatalf("failed to close temp file: %v", err)
	}

	config, err := LoadConfig(tmpFile.Name())
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if len(config.Inputs) != 1 || config.Inputs[0].Type != "http" {
		t.Errorf("expected input type 'http', got '%v'", config.Inputs)
	}

	if len(config.Outputs) != 1 || config.Outputs[0].Type != "file" {
		t.Errorf("expected output type 'file', got '%v'", config.Outputs)
	}
}

func TestMultipleInputs(t *testing.T) {
	configContent := `
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

outputs:
  - type: console
    config:
      target: "stdout"
      format: "text"
`

	tmpFile, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer func() {
		_ = os.Remove(tmpFile.Name())
	}()

	if _, err := tmpFile.Write([]byte(configContent)); err != nil {
		t.Fatalf("failed to write to temp file: %v", err)
	}
	if err := tmpFile.Close(); err != nil {
		t.Fatalf("failed to close temp file: %v", err)
	}

	config, err := LoadConfig(tmpFile.Name())
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if len(config.Inputs) != 3 {
		t.Errorf("expected 3 inputs, got %d", len(config.Inputs))
	}

	// Verify each input type
	if config.Inputs[0].Type != "file" {
		t.Errorf("expected first input type 'file', got '%s'", config.Inputs[0].Type)
	}
	if config.Inputs[1].Type != "docker" {
		t.Errorf("expected second input type 'docker', got '%s'", config.Inputs[1].Type)
	}
	if config.Inputs[2].Type != "http" {
		t.Errorf("expected third input type 'http', got '%s'", config.Inputs[2].Type)
	}
}

func TestMultipleFilters(t *testing.T) {
	configContent := `
inputs:
  - type: file
    config:
      path: "/var/log/app.log"

outputs:
  - type: console
    config:
      target: "stdout"
    filters:
      - type: level
        config:
          levels: ["error", "warn", "info"]
      - type: regex
        config:
          patterns: ["ERROR.*", "WARN.*"]
          mode: "include"
          field: "message"
`

	tmpFile, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer func() {
		_ = os.Remove(tmpFile.Name())
	}()

	if _, err := tmpFile.Write([]byte(configContent)); err != nil {
		t.Fatalf("failed to write to temp file: %v", err)
	}
	if err := tmpFile.Close(); err != nil {
		t.Fatalf("failed to close temp file: %v", err)
	}

	config, err := LoadConfig(tmpFile.Name())
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if len(config.Outputs) != 1 {
		t.Fatalf("expected 1 output, got %d", len(config.Outputs))
	}

	if len(config.Outputs[0].Filters) != 2 {
		t.Errorf("expected 2 filters, got %d", len(config.Outputs[0].Filters))
	}

	// Verify each filter type
	if config.Outputs[0].Filters[0].Type != "level" {
		t.Errorf("expected first filter type 'level', got '%s'", config.Outputs[0].Filters[0].Type)
	}
	if config.Outputs[0].Filters[1].Type != "regex" {
		t.Errorf("expected second filter type 'regex', got '%s'", config.Outputs[0].Filters[1].Type)
	}

	// Verify filter configs
	if levels, ok := config.Outputs[0].Filters[0].Config["levels"].([]any); !ok || len(levels) != 3 {
		t.Errorf("expected 3 levels in first filter, got %v", config.Outputs[0].Filters[0].Config["levels"])
	}

	if patterns, ok := config.Outputs[0].Filters[1].Config["patterns"].([]any); !ok || len(patterns) != 2 {
		t.Errorf("expected 2 patterns in second filter, got %v", config.Outputs[0].Filters[1].Config["patterns"])
	}
}
