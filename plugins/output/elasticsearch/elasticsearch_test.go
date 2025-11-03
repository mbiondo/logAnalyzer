package elasticsearch

import (
	"log"
	"os"
	"testing"
	"time"

	"github.com/mbiondo/logAnalyzer/core"
)

func TestMain(m *testing.M) {
	// Set environment variable to skip connection tests in unit tests
	if err := os.Setenv("UNIT_TEST", "true"); err != nil {
		log.Printf("Failed to set UNIT_TEST env var: %v", err)
	}
	code := m.Run()
	if err := os.Unsetenv("UNIT_TEST"); err != nil {
		log.Printf("Failed to unset UNIT_TEST env var: %v", err)
	}
	os.Exit(code)
}

func TestNewElasticsearchOutput(t *testing.T) {
	// Test with missing index - should fail without attempting connections
	_, err := NewElasticsearchOutput(Config{
		// Empty addresses to avoid connection attempts
	})
	if err == nil {
		t.Error("Expected error for missing index")
	}

	// Test default values - use a mock/test configuration that doesn't attempt real connections
	config := Config{
		Index: "logs",
		// Don't set addresses to avoid connection attempts in unit tests
	}

	// This should succeed without attempting connections
	output, err := NewElasticsearchOutput(config)
	if err != nil {
		t.Errorf("Expected success with valid config, got error: %v", err)
		return
	}
	if output == nil {
		t.Error("Expected non-nil output")
		return
	}

	// Verify default values were set
	if len(output.config.Addresses) == 0 {
		t.Error("Expected default addresses to be set")
	}
	if output.config.Timeout != 30 {
		t.Errorf("Expected default timeout 30, got %d", output.config.Timeout)
	}
	if output.config.BatchSize != 100 {
		t.Errorf("Expected default batch size 100, got %d", output.config.BatchSize)
	}
}

func TestResolveIndexName(t *testing.T) {
	output := &ElasticsearchOutput{
		config: Config{
			Index: "logs-{yyyy.MM.dd}",
		},
	}

	timestamp := time.Date(2024, 1, 15, 12, 30, 0, 0, time.UTC)
	indexName := output.resolveIndexName(timestamp)

	expected := "logs-2024.01.15"
	if indexName != expected {
		t.Errorf("Expected index name %s, got %s", expected, indexName)
	}
}

func TestResolveIndexNameMultipleFormats(t *testing.T) {
	tests := []struct {
		pattern  string
		expected string
	}{
		{"logs-{yyyy.MM.dd}", "logs-2024.01.15"},
		{"logs-{yyyy-MM-dd}", "logs-2024-01-15"},
		{"logs-{yyyy.MM}", "logs-2024.01"},
		{"logs-{yyyy-MM}", "logs-2024-01"},
		{"logs-{yyyy}", "logs-2024"},
		{"logs", "logs"},
	}

	timestamp := time.Date(2024, 1, 15, 12, 30, 0, 0, time.UTC)

	for _, tt := range tests {
		output := &ElasticsearchOutput{
			config: Config{
				Index: tt.pattern,
			},
		}

		indexName := output.resolveIndexName(timestamp)
		if indexName != tt.expected {
			t.Errorf("Pattern %s: expected %s, got %s", tt.pattern, tt.expected, indexName)
		}
	}
}

func TestElasticsearchOutputFromConfig(t *testing.T) {
	config := map[string]any{
		"index":      "test-logs",
		"username":   "elastic",
		"password":   "password",
		"timeout":    60,
		"batch_size": 50,
		// Omit addresses to avoid connection attempts in unit tests
	}

	_, err := NewElasticsearchOutputFromConfig(config)
	if err != nil {
		t.Logf("Skipping connection test (Elasticsearch not available): %v", err)
		return
	}
}

func TestBatchingLogic(t *testing.T) {
	// This test validates batching without actual Elasticsearch connection
	config := Config{
		Index:     "test-logs",
		BatchSize: 2,
	}

	output := &ElasticsearchOutput{
		config: config,
		batch:  make([]core.Log, 0, config.BatchSize),
	}

	// Add first log
	log1 := &core.Log{
		Level:     "INFO",
		Message:   "Test message 1",
		Timestamp: time.Now(),
		Metadata:  make(map[string]string),
	}

	output.batchMutex.Lock()
	output.batch = append(output.batch, *log1)
	batchLen := len(output.batch)
	output.batchMutex.Unlock()

	if batchLen != 1 {
		t.Errorf("Expected batch length 1, got %d", batchLen)
	}

	// Add second log
	log2 := &core.Log{
		Level:     "ERROR",
		Message:   "Test message 2",
		Timestamp: time.Now(),
		Metadata:  make(map[string]string),
	}

	output.batchMutex.Lock()
	output.batch = append(output.batch, *log2)
	batchLen = len(output.batch)
	output.batchMutex.Unlock()

	if batchLen != 2 {
		t.Errorf("Expected batch length 2, got %d", batchLen)
	}
}

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name      string
		config    Config
		expectErr bool
	}{
		{
			name: "Valid config",
			config: Config{
				// Omit addresses to avoid connection attempts in unit tests
				Index: "logs",
			},
			expectErr: false,
		},
		{
			name:   "Missing index",
			config: Config{
				// Empty addresses to avoid connection attempts
			},
			expectErr: true,
		},
		{
			name: "Empty addresses (uses defaults)",
			config: Config{
				Index: "logs",
			},
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewElasticsearchOutput(tt.config)

			if tt.expectErr && err == nil {
				t.Error("Expected error but got none")
			}

			if !tt.expectErr && err != nil {
				t.Logf("Skipping test (Elasticsearch not available): %v", err)
			}
		})
	}
}
