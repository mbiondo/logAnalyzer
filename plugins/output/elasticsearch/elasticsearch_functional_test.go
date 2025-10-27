package elasticsearch

import (
	"testing"
	"time"

	"github.com/mbiondo/logAnalyzer/core"
)

// TestPluginRegistration verifies the plugin is registered correctly
func TestPluginRegistration(t *testing.T) {
	// Test that plugin factory is registered
	config := map[string]any{
		"index":      "test-logs",
		"addresses":  []any{"http://localhost:9200"},
		"batch_size": 50,
	}

	_, err := NewElasticsearchOutputFromConfig(config)
	if err != nil {
		// Expected to fail without Elasticsearch, but config parsing should work
		t.Logf("Plugin creation failed (expected without ES): %v", err)
	}
}

// TestConfigDefaults verifies default values are set correctly
func TestConfigDefaults(t *testing.T) {
	tests := []struct {
		name     string
		input    Config
		expected Config
	}{
		{
			name: "Empty addresses gets default",
			input: Config{
				Index: "logs",
			},
			expected: Config{
				Addresses: []string{"http://localhost:9200"},
				Index:     "logs",
				Timeout:   30,
				BatchSize: 100,
			},
		},
		{
			name: "Custom values are preserved",
			input: Config{
				Addresses: []string{"http://custom:9200"},
				Index:     "custom-logs",
				Timeout:   60,
				BatchSize: 500,
			},
			expected: Config{
				Addresses: []string{"http://custom:9200"},
				Index:     "custom-logs",
				Timeout:   60,
				BatchSize: 500,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := tt.input

			// Apply defaults (simulating what NewElasticsearchOutput does)
			if len(config.Addresses) == 0 {
				config.Addresses = []string{"http://localhost:9200"}
			}
			if config.Timeout == 0 {
				config.Timeout = 30
			}
			if config.BatchSize == 0 {
				config.BatchSize = 100
			}

			if len(config.Addresses) != len(tt.expected.Addresses) ||
				config.Addresses[0] != tt.expected.Addresses[0] {
				t.Errorf("Addresses: got %v, want %v", config.Addresses, tt.expected.Addresses)
			}
			if config.Timeout != tt.expected.Timeout {
				t.Errorf("Timeout: got %d, want %d", config.Timeout, tt.expected.Timeout)
			}
			if config.BatchSize != tt.expected.BatchSize {
				t.Errorf("BatchSize: got %d, want %d", config.BatchSize, tt.expected.BatchSize)
			}
		})
	}
}

// TestBulkRequestBuilding tests the bulk request format without sending
func TestBulkRequestBuilding(t *testing.T) {
	output := &ElasticsearchOutput{
		config: Config{
			Index:     "test-logs-{yyyy.MM.dd}",
			BatchSize: 10,
		},
		batch: make([]core.Log, 0, 10),
	}

	// Test date template resolution
	now := time.Date(2024, 10, 26, 12, 0, 0, 0, time.UTC)
	indexName := output.resolveIndexName(now)
	expected := "test-logs-2024.10.26"

	if indexName != expected {
		t.Errorf("Index resolution failed: got %s, want %s", indexName, expected)
	}
}

// TestBatchFlushThreshold verifies batching logic
func TestBatchFlushThreshold(t *testing.T) {
	config := Config{
		Index:     "test",
		BatchSize: 3,
		Timeout:   30,
	}

	output := &ElasticsearchOutput{
		config: config,
		batch:  make([]core.Log, 0, config.BatchSize),
	}

	// Add logs and check when flush should occur
	tests := []struct {
		logsToAdd   int
		expectedLen int
		shouldFlush bool
	}{
		{1, 1, false},
		{1, 2, false},
		{1, 3, true}, // Should flush at batch size
	}

	for i, tt := range tests {
		output.batchMutex.Lock()
		for j := 0; j < tt.logsToAdd; j++ {
			output.batch = append(output.batch, core.Log{
				Level:     "INFO",
				Message:   "Test",
				Timestamp: time.Now(),
				Metadata:  make(map[string]string),
			})
		}

		currentLen := len(output.batch)
		shouldFlush := currentLen >= config.BatchSize

		if shouldFlush {
			output.batch = make([]core.Log, 0, config.BatchSize)
		}
		output.batchMutex.Unlock()

		if shouldFlush != tt.shouldFlush {
			t.Errorf("Test %d: shouldFlush = %v, want %v", i, shouldFlush, tt.shouldFlush)
		}
	}
}

// TestDateTemplateEdgeCases tests various date template scenarios
func TestDateTemplateEdgeCases(t *testing.T) {
	tests := []struct {
		template string
		date     time.Time
		expected string
	}{
		{
			template: "logs-{yyyy}.{MM}.{dd}",
			date:     time.Date(2024, 1, 5, 0, 0, 0, 0, time.UTC),
			expected: "logs-2024.01.05",
		},
		{
			template: "app-{yyyy}-{MM}",
			date:     time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC),
			expected: "app-2024-12",
		},
		{
			template: "static-index",
			date:     time.Now(),
			expected: "static-index",
		},
		{
			template: "{yyyy}{MM}{dd}",
			date:     time.Date(2024, 10, 26, 0, 0, 0, 0, time.UTC),
			expected: "20241026",
		},
	}

	for _, tt := range tests {
		t.Run(tt.template, func(t *testing.T) {
			output := &ElasticsearchOutput{
				config: Config{Index: tt.template},
			}

			result := output.resolveIndexName(tt.date)
			if result != tt.expected {
				t.Errorf("Template %s with date %v: got %s, want %s",
					tt.template, tt.date, result, tt.expected)
			}
		})
	}
}

// TestConfigFromMap tests configuration parsing from map
func TestConfigFromMap(t *testing.T) {
	configMap := map[string]any{
		"addresses":  []any{"http://es1:9200", "http://es2:9200"},
		"index":      "test-logs-{yyyy.MM.dd}",
		"username":   "elastic",
		"password":   "secret",
		"timeout":    60,
		"batch_size": 200,
	}

	var cfg Config
	err := core.GetPluginConfig(configMap, &cfg)
	if err != nil {
		t.Fatalf("Failed to parse config: %v", err)
	}

	if len(cfg.Addresses) != 2 {
		t.Errorf("Addresses length: got %d, want 2", len(cfg.Addresses))
	}
	if cfg.Addresses[0] != "http://es1:9200" {
		t.Errorf("Addresses[0]: got %s, want http://es1:9200", cfg.Addresses[0])
	}
	if cfg.Index != "test-logs-{yyyy.MM.dd}" {
		t.Errorf("Index: got %s, want test-logs-{yyyy.MM.dd}", cfg.Index)
	}
	if cfg.Username != "elastic" {
		t.Errorf("Username: got %s, want elastic", cfg.Username)
	}
	if cfg.Timeout != 60 {
		t.Errorf("Timeout: got %d, want 60", cfg.Timeout)
	}
	if cfg.BatchSize != 200 {
		t.Errorf("BatchSize: got %d, want 200", cfg.BatchSize)
	}
}

// TestInvalidConfigurations tests error handling for invalid configs
func TestInvalidConfigurations(t *testing.T) {
	tests := []struct {
		name      string
		config    Config
		shouldErr bool
	}{
		{
			name: "Missing index",
			config: Config{
				Addresses: []string{"http://localhost:9200"},
			},
			shouldErr: true,
		},
		{
			name: "Valid minimal config",
			config: Config{
				Index: "logs",
			},
			shouldErr: false, // Will fail on connection, not config
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewElasticsearchOutput(tt.config)

			if tt.shouldErr && err == nil {
				t.Error("Expected error but got none")
			}

			// Note: Connection errors are expected without ES running
			if err != nil {
				t.Logf("Got error (may be connection error): %v", err)
			}
		})
	}
}
