package core

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/mbiondo/logAnalyzer/pkg/auth"
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

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name        string
		config      Config
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid minimal config",
			config: Config{
				Inputs: []PluginDefinition{
					{
						Type: "file",
						Config: map[string]any{
							"path": "/var/log/app.log",
						},
					},
				},
				Outputs: []PluginDefinition{
					{
						Type: "console",
						Config: map[string]any{
							"format": "json",
						},
					},
				},
			},
			expectError: false,
		},
		{
			name: "empty inputs",
			config: Config{
				Inputs:  []PluginDefinition{},
				Outputs: []PluginDefinition{{Type: "console", Config: map[string]any{}}},
			},
			expectError: true,
			errorMsg:    "Inputs: cannot be blank",
		},
		{
			name: "too many inputs",
			config: Config{
				Inputs:  generateManyPlugins(101, "file"),
				Outputs: []PluginDefinition{{Type: "console", Config: map[string]any{}}},
			},
			expectError: true,
			errorMsg:    "Inputs: the length must be between 1 and 100",
		},
		{
			name: "empty outputs",
			config: Config{
				Inputs:  []PluginDefinition{{Type: "file", Config: map[string]any{}}},
				Outputs: []PluginDefinition{},
			},
			expectError: true,
			errorMsg:    "Outputs: cannot be blank",
		},
		{
			name: "invalid plugin type",
			config: Config{
				Inputs: []PluginDefinition{
					{
						Type:   "invalid_type",
						Config: map[string]any{"path": "/tmp/test.log"},
					},
				},
				Outputs: []PluginDefinition{{Type: "console", Config: map[string]any{"format": "json"}}},
			},
			expectError: true,
			errorMsg:    "Type: must be a valid value",
		},
		{
			name: "missing plugin config",
			config: Config{
				Inputs: []PluginDefinition{
					{
						Type:   "file",
						Config: nil,
					},
				},
				Outputs: []PluginDefinition{{Type: "console", Config: map[string]any{}}},
			},
			expectError: true,
			errorMsg:    "Config: cannot be blank",
		},
		{
			name: "plugin name too long",
			config: Config{
				Inputs: []PluginDefinition{
					{
						Type:   "file",
						Name:   strings.Repeat("a", 101),
						Config: map[string]any{"path": "/tmp/test.log"},
					},
				},
				Outputs: []PluginDefinition{{Type: "console", Config: map[string]any{}}},
			},
			expectError: true,
			errorMsg:    "Name: the length must be no more than 100",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.expectError {
				if err == nil {
					t.Errorf("expected validation error but got none")
				} else if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("expected error containing '%s', got '%s'", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("expected no validation error but got: %v", err)
				}
			}
		})
	}
}

func TestAPIConfigValidation(t *testing.T) {
	tests := []struct {
		name        string
		config      APIConfig
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid API config disabled",
			config: APIConfig{
				Enabled: false,
				Port:    8080,
			},
			expectError: false,
		},
		{
			name: "valid API config enabled",
			config: APIConfig{
				Enabled: true,
				Port:    9090,
				Auth: APIAuthConfig{
					Enabled: true,
					APIKeys: []auth.APIKeyConfig{
						{
							ID:          "test-key",
							Secret:      "secret123",
							Permissions: []string{"health"},
							Name:        "Test Key",
						},
					},
				},
			},
			expectError: false,
		},
		{
			name: "invalid port too low",
			config: APIConfig{
				Enabled: true,
				Port:    0,
			},
			expectError: true,
			errorMsg:    "Port: cannot be blank",
		},
		{
			name: "invalid port too high",
			config: APIConfig{
				Enabled: true,
				Port:    70000,
			},
			expectError: true,
			errorMsg:    "Port: must be no greater than 65535",
		},
		{
			name: "API key without ID",
			config: APIConfig{
				Enabled: true,
				Port:    9090,
				Auth: APIAuthConfig{
					Enabled: true,
					APIKeys: []auth.APIKeyConfig{
						{
							Secret:      "secret123",
							Permissions: []string{"health"},
						},
					},
				},
			},
			expectError: true,
			errorMsg:    "API key ID cannot be empty",
		},
		{
			name: "API key without secret",
			config: APIConfig{
				Enabled: true,
				Port:    9090,
				Auth: APIAuthConfig{
					Enabled: true,
					APIKeys: []auth.APIKeyConfig{
						{
							ID:          "test-key",
							Permissions: []string{"health"},
						},
					},
				},
			},
			expectError: true,
			errorMsg:    "API key secret cannot be empty",
		},
		{
			name: "API key without permissions",
			config: APIConfig{
				Enabled: true,
				Port:    9090,
				Auth: APIAuthConfig{
					Enabled: true,
					APIKeys: []auth.APIKeyConfig{
						{
							ID:     "test-key",
							Secret: "secret123",
						},
					},
				},
			},
			expectError: true,
			errorMsg:    "API key must have at least one permission",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.expectError {
				if err == nil {
					t.Errorf("expected validation error but got none")
				} else if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("expected error containing '%s', got '%s'", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("expected no validation error but got: %v", err)
				}
			}
		})
	}
}

func TestPersistenceConfigValidation(t *testing.T) {
	tests := []struct {
		name        string
		config      PersistenceConfig
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid persistence config",
			config: PersistenceConfig{
				Enabled:        true,
				Dir:            "/tmp/wal",
				MaxFileSize:    100 * 1024 * 1024,
				BufferSize:     100,
				FlushInterval:  5,
				RetentionHours: 24,
				SyncWrites:     false,
			},
			expectError: false,
		},
		{
			name: "max file size too small",
			config: PersistenceConfig{
				MaxFileSize: 100,
			},
			expectError: true,
			errorMsg:    "MaxFileSize: must be no less than 1024",
		},
		{
			name: "max file size too large",
			config: PersistenceConfig{
				MaxFileSize: 11 * 1024 * 1024 * 1024, // 11GB
			},
			expectError: true,
			errorMsg:    "MaxFileSize: must be no greater than 10737418240",
		},
		{
			name: "buffer size too small",
			config: PersistenceConfig{
				BufferSize:     0,
				FlushInterval:  5,
				MaxFileSize:    1024,
				RetentionHours: 24,
			},
			expectError: true,
			errorMsg:    "BufferSize: must be no less than 1",
		},
		{
			name: "buffer size too large",
			config: PersistenceConfig{
				BufferSize:     10001,
				FlushInterval:  5,
				MaxFileSize:    1024,
				RetentionHours: 24,
			},
			expectError: true,
			errorMsg:    "BufferSize: must be no greater than 10000",
		},
		{
			name: "flush interval too small",
			config: PersistenceConfig{
				BufferSize:     100,
				FlushInterval:  0,
				MaxFileSize:    1024,
				RetentionHours: 24,
			},
			expectError: true,
			errorMsg:    "FlushInterval: must be no less than 1",
		},
		{
			name: "flush interval too large",
			config: PersistenceConfig{
				BufferSize:     100,
				FlushInterval:  3601,
				MaxFileSize:    1024,
				RetentionHours: 24,
			},
			expectError: true,
			errorMsg:    "FlushInterval: must be no greater than 3600",
		},
		{
			name: "retention hours too small",
			config: PersistenceConfig{
				BufferSize:     100,
				FlushInterval:  5,
				MaxFileSize:    1024,
				RetentionHours: 0,
			},
			expectError: true,
			errorMsg:    "RetentionHours: must be no less than 1",
		},
		{
			name: "retention hours too large",
			config: PersistenceConfig{
				BufferSize:     100,
				FlushInterval:  5,
				MaxFileSize:    1024,
				RetentionHours: 8761,
			},
			expectError: true,
			errorMsg:    "RetentionHours: must be no greater than 8760",
		},
		{
			name: "directory path too long",
			config: PersistenceConfig{
				Dir: strings.Repeat("a", 501),
			},
			expectError: true,
			errorMsg:    "Dir: the length must be no more than 500",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.expectError {
				if err == nil {
					t.Errorf("expected validation error but got none")
				} else if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("expected error containing '%s', got '%s'", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("expected no validation error but got: %v", err)
				}
			}
		})
	}
}

func TestOutputBufferConfigValidation(t *testing.T) {
	tests := []struct {
		name        string
		config      OutputBufferConfig
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid output buffer config",
			config: OutputBufferConfig{
				Enabled:       true,
				Dir:           "/tmp/buffers",
				MaxQueueSize:  1000,
				MaxRetries:    3,
				RetryInterval: 5 * time.Second,
				MaxRetryDelay: 60 * time.Second,
				FlushInterval: 10 * time.Second,
				DLQEnabled:    true,
				DLQPath:       "/tmp/dlq",
			},
			expectError: false,
		},
		{
			name: "max queue size too small",
			config: OutputBufferConfig{
				MaxQueueSize:  0,
				MaxRetries:    3,
				RetryInterval: 5 * time.Second,
				MaxRetryDelay: 60 * time.Second,
				FlushInterval: 10 * time.Second,
			},
			expectError: true,
			errorMsg:    "MaxQueueSize: must be no less than 1",
		},
		{
			name: "max queue size too large",
			config: OutputBufferConfig{
				MaxQueueSize:  100001,
				MaxRetries:    3,
				RetryInterval: 5 * time.Second,
				MaxRetryDelay: 60 * time.Second,
				FlushInterval: 10 * time.Second,
			},
			expectError: true,
			errorMsg:    "MaxQueueSize: must be no greater than 100000",
		},
		{
			name: "max retries negative",
			config: OutputBufferConfig{
				MaxQueueSize:  1000,
				MaxRetries:    -1,
				RetryInterval: 5 * time.Second,
				MaxRetryDelay: 60 * time.Second,
				FlushInterval: 10 * time.Second,
			},
			expectError: true,
			errorMsg:    "MaxRetries: must be no less than 0",
		},
		{
			name: "max retries too large",
			config: OutputBufferConfig{
				MaxQueueSize:  1000,
				MaxRetries:    101,
				RetryInterval: 5 * time.Second,
				MaxRetryDelay: 60 * time.Second,
				FlushInterval: 10 * time.Second,
			},
			expectError: true,
			errorMsg:    "MaxRetries: must be no greater than 100",
		},
		{
			name: "retry interval too small",
			config: OutputBufferConfig{
				MaxQueueSize:  1000,
				MaxRetries:    3,
				RetryInterval: 500 * time.Microsecond,
				MaxRetryDelay: 60 * time.Second,
				FlushInterval: 10 * time.Second,
			},
			expectError: true,
			errorMsg:    "RetryInterval: must be no less than 1ms",
		},
		{
			name: "retry interval too large",
			config: OutputBufferConfig{
				MaxQueueSize:  1000,
				MaxRetries:    3,
				RetryInterval: 2 * time.Hour,
				MaxRetryDelay: 60 * time.Second,
				FlushInterval: 10 * time.Second,
			},
			expectError: true,
			errorMsg:    "RetryInterval: must be no greater than 1h0m0s",
		},
		{
			name: "max retry delay too small",
			config: OutputBufferConfig{
				MaxQueueSize:  1000,
				MaxRetries:    3,
				RetryInterval: 5 * time.Second,
				MaxRetryDelay: 500 * time.Microsecond,
				FlushInterval: 10 * time.Second,
			},
			expectError: true,
			errorMsg:    "MaxRetryDelay: must be no less than 1ms",
		},
		{
			name: "max retry delay too large",
			config: OutputBufferConfig{
				MaxQueueSize:  1000,
				MaxRetries:    3,
				RetryInterval: 5 * time.Second,
				MaxRetryDelay: 25 * time.Hour,
				FlushInterval: 10 * time.Second,
			},
			expectError: true,
			errorMsg:    "MaxRetryDelay: must be no greater than 24h0m0s",
		},
		{
			name: "flush interval too small",
			config: OutputBufferConfig{
				MaxQueueSize:  1000,
				MaxRetries:    3,
				RetryInterval: 5 * time.Second,
				MaxRetryDelay: 60 * time.Second,
				FlushInterval: 500 * time.Microsecond,
			},
			expectError: true,
			errorMsg:    "FlushInterval: must be no less than 1ms",
		},
		{
			name: "flush interval too large",
			config: OutputBufferConfig{
				MaxQueueSize:  1000,
				MaxRetries:    3,
				RetryInterval: 5 * time.Second,
				MaxRetryDelay: 60 * time.Second,
				FlushInterval: 2 * time.Hour,
			},
			expectError: true,
			errorMsg:    "FlushInterval: must be no greater than 1h0m0s",
		},
		{
			name: "directory path too long",
			config: OutputBufferConfig{
				Dir: strings.Repeat("a", 501),
			},
			expectError: true,
			errorMsg:    "Dir: the length must be no more than 500",
		},
		{
			name: "DLQ path too long",
			config: OutputBufferConfig{
				DLQPath: strings.Repeat("a", 501),
			},
			expectError: true,
			errorMsg:    "DLQPath: the length must be no more than 500",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.expectError {
				if err == nil {
					t.Errorf("expected validation error but got none")
				} else if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("expected error containing '%s', got '%s'", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("expected no validation error but got: %v", err)
				}
			}
		})
	}
}

func TestPluginDefinitionValidation(t *testing.T) {
	tests := []struct {
		name        string
		plugin      PluginDefinition
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid file plugin",
			plugin: PluginDefinition{
				Type: "file",
				Config: map[string]any{
					"path": "/var/log/app.log",
				},
			},
			expectError: false,
		},
		{
			name: "valid console plugin with filters",
			plugin: PluginDefinition{
				Type:    "console",
				Name:    "stdout",
				Config:  map[string]any{"format": "json"},
				Sources: []string{"input1", "input2"},
				Filters: []PluginDefinition{
					{
						Type:   "level",
						Config: map[string]any{"levels": []string{"error", "warn"}},
					},
				},
			},
			expectError: false,
		},
		{
			name: "invalid plugin type",
			plugin: PluginDefinition{
				Type:   "unknown",
				Config: map[string]any{},
			},
			expectError: true,
			errorMsg:    "Type: must be a valid value",
		},
		{
			name: "missing config",
			plugin: PluginDefinition{
				Type: "console",
			},
			expectError: true,
			errorMsg:    "Config: cannot be blank",
		},
		{
			name: "name too long",
			plugin: PluginDefinition{
				Type:   "console",
				Name:   strings.Repeat("a", 101),
				Config: map[string]any{},
			},
			expectError: true,
			errorMsg:    "Config: cannot be blank",
		},
		{
			name: "empty source name",
			plugin: PluginDefinition{
				Type:    "console",
				Config:  map[string]any{"format": "json"},
				Sources: []string{"", "valid"},
			},
			expectError: true,
			errorMsg:    "Sources: (0: cannot be blank",
		},
		{
			name: "invalid filter",
			plugin: PluginDefinition{
				Type:   "console",
				Config: map[string]any{"format": "json"},
				Filters: []PluginDefinition{
					{
						Type: "invalid_filter",
					},
				},
			},
			expectError: true,
			errorMsg:    "Filters: (0: (Config: cannot be blank",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.plugin.Validate()
			if tt.expectError {
				if err == nil {
					t.Errorf("expected validation error but got none")
				} else if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("expected error containing '%s', got '%s'", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("expected no validation error but got: %v", err)
				}
			}
		})
	}
}

func TestLoadConfigValidation(t *testing.T) {
	tests := []struct {
		name        string
		configYAML  string
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid config",
			configYAML: `
inputs:
  - type: file
    config:
      path: "/var/log/app.log"
outputs:
  - type: console
    config:
      format: "json"
`,
			expectError: false,
		},
		{
			name: "invalid config - empty inputs",
			configYAML: `
inputs: []
outputs:
  - type: console
    config: {}
`,
			expectError: true,
			errorMsg:    "Inputs: cannot be blank",
		},
		{
			name: "invalid config - invalid plugin type",
			configYAML: `
inputs:
  - type: invalid_plugin
    config: {}
outputs:
  - type: console
    config:
      format: "json"
`,
			expectError: true,
			errorMsg:    "Type: must be a valid value",
		},
		{
			name: "invalid config - API port out of range",
			configYAML: `
inputs:
  - type: file
    config:
      path: "/var/log/app.log"
outputs:
  - type: console
    config: {}
api:
  enabled: true
  port: 70000
`,
			expectError: true,
			errorMsg:    "Port: must be no greater than 65535",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpFile, err := os.CreateTemp("", "config-test-*.yaml")
			if err != nil {
				t.Fatalf("failed to create temp file: %v", err)
			}
			defer func() {
				_ = os.Remove(tmpFile.Name())
			}()

			if _, err := tmpFile.WriteString(tt.configYAML); err != nil {
				t.Fatalf("failed to write config: %v", err)
			}
			if err := tmpFile.Close(); err != nil {
				t.Fatalf("failed to close temp file: %v", err)
			}

			_, err = LoadConfig(tmpFile.Name())
			if tt.expectError {
				if err == nil {
					t.Errorf("expected validation error but got none")
				} else if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("expected error containing '%s', got '%s'", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("expected no validation error but got: %v", err)
				}
			}
		})
	}
}

// Helper function to generate many plugins for testing limits
func generateManyPlugins(count int, pluginType string) []PluginDefinition {
	plugins := make([]PluginDefinition, count)
	for i := 0; i < count; i++ {
		plugins[i] = PluginDefinition{
			Type: pluginType,
			Config: map[string]any{
				"path": fmt.Sprintf("/tmp/test%d.log", i),
			},
		}
	}
	return plugins
}
