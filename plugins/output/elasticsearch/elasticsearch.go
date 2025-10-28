package elasticsearch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/mbiondo/logAnalyzer/core"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"
)

func init() {
	// Auto-register this plugin
	core.RegisterOutputPlugin("elasticsearch", NewElasticsearchOutputFromConfig)
}

// Config represents Elasticsearch output configuration
type Config struct {
	Addresses []string `yaml:"addresses"`            // Elasticsearch addresses
	Username  string   `yaml:"username,omitempty"`   // Basic auth username
	Password  string   `yaml:"password,omitempty"`   // Basic auth password
	APIKey    string   `yaml:"api_key,omitempty"`    // API key authentication
	Index     string   `yaml:"index"`                // Index name (supports date templates)
	Timeout   int      `yaml:"timeout,omitempty"`    // Request timeout in seconds
	BatchSize int      `yaml:"batch_size,omitempty"` // Batch size for bulk operations
}

// ElasticsearchOutput sends logs to Elasticsearch
type ElasticsearchOutput struct {
	config     Config
	client     *elasticsearch.Client
	batch      []core.Log
	batchMutex sync.Mutex
	closeMutex sync.Mutex
	closed     bool
	ctx        context.Context
	cancel     context.CancelFunc
}

// NewElasticsearchOutputFromConfig creates an Elasticsearch output from configuration
func NewElasticsearchOutputFromConfig(config map[string]any) (any, error) {
	var cfg Config
	if err := core.GetPluginConfig(config, &cfg); err != nil {
		return nil, err
	}

	return NewElasticsearchOutput(cfg)
}

// NewElasticsearchOutput creates a new Elasticsearch output plugin
func NewElasticsearchOutput(config Config) (*ElasticsearchOutput, error) {
	// Validate required fields
	if len(config.Addresses) == 0 {
		config.Addresses = []string{"http://localhost:9200"}
	}
	if config.Index == "" {
		return nil, fmt.Errorf("index is required")
	}
	if config.Timeout == 0 {
		config.Timeout = 30
	}
	if config.BatchSize == 0 {
		config.BatchSize = 100
	}

	// Configure Elasticsearch client
	esCfg := elasticsearch.Config{
		Addresses: config.Addresses,
		Username:  config.Username,
		Password:  config.Password,
		APIKey:    config.APIKey,
		Transport: nil, // Use default transport
	}

	client, err := elasticsearch.NewClient(esCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create Elasticsearch client: %w", err)
	}

	// Test connection (non-blocking - just log if fails)
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(config.Timeout)*time.Second)
	res, err := client.Info(client.Info.WithContext(ctx))

	if err != nil {
		cancel()
		log.Printf("[ELASTICSEARCH] Initial connection test failed: %v (will retry in background)", err)
		// Don't fail initialization - resilience layer will handle reconnection
	} else {
		defer func() {
			_ = res.Body.Close()
		}()

		if res.IsError() {
			cancel()
			log.Printf("[ELASTICSEARCH] Initial connection returned error: %s (will retry in background)", res.String())
			// Don't fail initialization - resilience layer will handle reconnection
		} else {
			cancel()
			log.Printf("[ELASTICSEARCH] Successfully connected to Elasticsearch")
		}
	}

	ctx, cancel = context.WithCancel(context.Background())

	output := &ElasticsearchOutput{
		config: config,
		client: client,
		batch:  make([]core.Log, 0, config.BatchSize),
		closed: false,
		ctx:    ctx,
		cancel: cancel,
	}

	// Start background flusher
	go output.periodicFlush()

	return output, nil
}

// Write writes a log entry to Elasticsearch
func (e *ElasticsearchOutput) Write(logEntry *core.Log) error {
	e.closeMutex.Lock()
	if e.closed {
		e.closeMutex.Unlock()
		return fmt.Errorf("elasticsearch output is closed")
	}
	e.closeMutex.Unlock()

	e.batchMutex.Lock()
	e.batch = append(e.batch, *logEntry)
	currentSize := len(e.batch)
	shouldFlush := currentSize >= e.config.BatchSize
	e.batchMutex.Unlock()

	log.Printf("[ELASTICSEARCH] Received log (batch size: %d/%d): %s - %s", currentSize, e.config.BatchSize, logEntry.Level, logEntry.Message)

	if shouldFlush {
		log.Printf("[ELASTICSEARCH] Batch full, flushing...")
		return e.flush()
	}

	return nil
}

// flush sends batched logs to Elasticsearch
func (e *ElasticsearchOutput) flush() error {
	e.batchMutex.Lock()
	if len(e.batch) == 0 {
		e.batchMutex.Unlock()
		log.Printf("[ELASTICSEARCH] Flush called but batch is empty")
		return nil
	}

	// Take ownership of current batch
	batch := e.batch
	batchSize := len(batch)
	e.batch = make([]core.Log, 0, e.config.BatchSize)
	e.batchMutex.Unlock()

	log.Printf("[ELASTICSEARCH] Flushing %d logs to Elasticsearch", batchSize)

	// Build bulk request
	var buf bytes.Buffer

	for i, logEntry := range batch {
		// Index directive
		indexName := e.resolveIndexName(logEntry.Timestamp)
		log.Printf("[ELASTICSEARCH] Log %d/%d -> Index: %s", i+1, batchSize, indexName)
		meta := map[string]any{
			"index": map[string]any{
				"_index": indexName,
			},
		}
		metaBytes, _ := json.Marshal(meta)
		buf.Write(metaBytes)
		buf.WriteByte('\n')

		// Document
		doc := map[string]any{
			"@timestamp": logEntry.Timestamp.Format(time.RFC3339),
			"level":      logEntry.Level,
			"message":    logEntry.Message,
		}

		// Add metadata fields if present
		if len(logEntry.Metadata) > 0 {
			doc["metadata"] = logEntry.Metadata
		}
		docBytes, _ := json.Marshal(doc)
		buf.Write(docBytes)
		buf.WriteByte('\n')
	}

	// Send bulk request
	ctx, cancel := context.WithTimeout(e.ctx, time.Duration(e.config.Timeout)*time.Second)
	defer cancel()

	req := esapi.BulkRequest{
		Body: bytes.NewReader(buf.Bytes()),
	}

	log.Printf("[ELASTICSEARCH] Sending bulk request...")
	res, err := req.Do(ctx, e.client)
	if err != nil {
		log.Printf("[ELASTICSEARCH] Bulk request failed: %v", err)
		return fmt.Errorf("bulk request failed: %w", err)
	}
	defer func() {
		_ = res.Body.Close()
	}()

	if res.IsError() {
		log.Printf("[ELASTICSEARCH] Response error status: %s", res.Status())
		var errResp map[string]any
		if err := json.NewDecoder(res.Body).Decode(&errResp); err == nil {
			log.Printf("[ELASTICSEARCH] Error response: %v", errResp)
			return fmt.Errorf("elasticsearch error: %v", errResp)
		}
		return fmt.Errorf("elasticsearch returned status: %s", res.Status())
	}

	// Check for partial failures
	var bulkResp map[string]any
	if err := json.NewDecoder(res.Body).Decode(&bulkResp); err != nil {
		log.Printf("[ELASTICSEARCH] Failed to parse response: %v", err)
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if bulkResp["errors"] == true {
		// Log partial failures but don't fail completely
		log.Printf("[ELASTICSEARCH] Bulk request had partial failures")
	} else {
		log.Printf("[ELASTICSEARCH] Successfully indexed %d logs", batchSize)
	}

	return nil
}

// periodicFlush flushes logs every 5 seconds
func (e *ElasticsearchOutput) periodicFlush() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			_ = e.flush()
		case <-e.ctx.Done():
			return
		}
	}
}

// resolveIndexName resolves index name with date templates
// Supports: logs-{yyyy.MM.dd}, logs-{yyyy-MM}, etc.
func (e *ElasticsearchOutput) resolveIndexName(t time.Time) string {
	indexName := e.config.Index

	// Replace date templates
	replacements := map[string]string{
		"{yyyy.MM.dd}": t.Format("2006.01.02"),
		"{yyyy-MM-dd}": t.Format("2006-01-02"),
		"{yyyy.MM}":    t.Format("2006.01"),
		"{yyyy-MM}":    t.Format("2006-01"),
		"{yyyy}":       t.Format("2006"),
		"{MM}":         t.Format("01"),
		"{dd}":         t.Format("02"),
	}

	for pattern, value := range replacements {
		indexName = strings.ReplaceAll(indexName, pattern, value)
	}

	return indexName
}

// CheckHealth implements HealthChecker interface
func (e *ElasticsearchOutput) CheckHealth(ctx context.Context) error {
	res, err := e.client.Info(e.client.Info.WithContext(ctx))
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}
	defer func() {
		_ = res.Body.Close()
	}()

	if res.IsError() {
		return fmt.Errorf("elasticsearch health check error: %s", res.String())
	}

	return nil
}

// Close closes the Elasticsearch output
func (e *ElasticsearchOutput) Close() error {
	e.closeMutex.Lock()
	if e.closed {
		e.closeMutex.Unlock()
		return nil
	}
	e.closed = true
	e.closeMutex.Unlock()

	// Cancel background tasks
	e.cancel()

	// Flush remaining logs
	return e.flush()
}
