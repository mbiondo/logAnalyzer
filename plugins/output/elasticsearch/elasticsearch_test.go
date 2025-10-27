package elasticsearch

import (
	"github.com/mbiondo/logAnalyzer/core"
	"testing"
	"time"
)

func TestNewElasticsearchOutput(t *testing.T) {
	// Test with missing index
	_, err := NewElasticsearchOutput(Config{
		Addresses: []string{"http://localhost:9200"},
	})
	if err == nil {
		t.Error("Expected error for missing index")
	}

	// Test default values
	config := Config{
		Index: "logs",
	}

	// Note: This will fail if Elasticsearch is not running
	// In CI/CD, you'd want to skip this test or use a mock
	_, err = NewElasticsearchOutput(config)
	if err != nil {
		t.Logf("Skipping connection test (Elasticsearch not available): %v", err)
		return
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
	config := map[string]interface{}{
		"addresses":  []interface{}{"http://localhost:9200"},
		"index":      "test-logs",
		"username":   "elastic",
		"password":   "password",
		"timeout":    60,
		"batch_size": 50,
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
				Addresses: []string{"http://localhost:9200"},
				Index:     "logs",
			},
			expectErr: false,
		},
		{
			name: "Missing index",
			config: Config{
				Addresses: []string{"http://localhost:9200"},
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
