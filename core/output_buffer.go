package core

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	validation "github.com/go-ozzo/ozzo-validation/v4"
)

// OutputBufferConfig defines output buffer configuration
type OutputBufferConfig struct {
	Enabled       bool          `yaml:"enabled"`         // Enable/disable output buffering
	Dir           string        `yaml:"dir"`             // Directory for buffer files
	MaxQueueSize  int           `yaml:"max_queue_size"`  // Max logs in memory queue
	MaxRetries    int           `yaml:"max_retries"`     // Max retry attempts
	RetryInterval time.Duration `yaml:"retry_interval"`  // Initial retry interval
	MaxRetryDelay time.Duration `yaml:"max_retry_delay"` // Max backoff delay
	FlushInterval time.Duration `yaml:"flush_interval"`  // How often to flush to disk
	DLQEnabled    bool          `yaml:"dlq_enabled"`     // Enable Dead Letter Queue
	DLQPath       string        `yaml:"dlq_path"`        // Path for DLQ file
}

// Validate validates the OutputBufferConfig
func (o OutputBufferConfig) Validate() error {
	// If output buffering is not enabled and all fields are zero/default, skip validation
	if !o.Enabled && o.Dir == "" && o.MaxQueueSize == 0 && o.MaxRetries == 0 && o.RetryInterval == 0 && o.MaxRetryDelay == 0 && o.FlushInterval == 0 && !o.DLQEnabled && o.DLQPath == "" {
		return nil
	}
	return validation.ValidateStruct(&o,
		validation.Field(&o.Dir, validation.Length(0, 500).Error("the length must be no more than 500")),
		validation.Field(&o.MaxQueueSize, validation.By(func(value interface{}) error {
			v := value.(int)
			if v < 1 {
				return fmt.Errorf("must be no less than 1")
			}
			if v > 100000 {
				return fmt.Errorf("must be no greater than 100000")
			}
			return nil
		})),
		validation.Field(&o.MaxRetries, validation.Min(0).Error("must be no less than 0"), validation.Max(100).Error("must be no greater than 100")),
		validation.Field(&o.RetryInterval, validation.Min(time.Millisecond).Error("must be no less than 1ms"), validation.Max(time.Hour).Error("must be no greater than 1h0m0s")),
		validation.Field(&o.MaxRetryDelay, validation.Min(time.Millisecond).Error("must be no less than 1ms"), validation.Max(24*time.Hour).Error("must be no greater than 24h0m0s")),
		validation.Field(&o.FlushInterval, validation.Min(time.Millisecond).Error("must be no less than 1ms"), validation.Max(time.Hour).Error("must be no greater than 1h0m0s")),
		validation.Field(&o.DLQPath, validation.Length(0, 500).Error("the length must be no more than 500")),
	)
}

// DefaultOutputBufferConfig returns default output buffer configuration
func DefaultOutputBufferConfig() OutputBufferConfig {
	return OutputBufferConfig{
		Enabled:       false,
		Dir:           "./data/buffers",
		MaxQueueSize:  1000,
		MaxRetries:    3,
		RetryInterval: 5 * time.Second,
		MaxRetryDelay: 60 * time.Second,
		FlushInterval: 10 * time.Second,
		DLQEnabled:    true,
		DLQPath:       "./data/dlq",
	}
}

// BufferedLog represents a log with retry metadata
type BufferedLog struct {
	Log         *Log      `json:"log"`
	Attempts    int       `json:"attempts"`
	LastAttempt time.Time `json:"last_attempt"`
	OutputName  string    `json:"output_name"`
	EnqueuedAt  time.Time `json:"enqueued_at"`
}

// OutputBuffer manages output buffering with persistence and retry logic
type OutputBuffer struct {
	config      OutputBufferConfig
	outputName  string
	queue       chan *BufferedLog
	retryQueue  []*BufferedLog
	retryMu     sync.Mutex
	output      OutputPlugin
	stopCh      chan struct{}
	wg          sync.WaitGroup
	dlqFile     *os.File
	dlqMu       sync.Mutex
	flushTicker *time.Ticker
	stats       BufferStats
	statsMu     sync.RWMutex
}

// BufferStats tracks buffer statistics
type BufferStats struct {
	TotalEnqueued   int64
	TotalDelivered  int64
	TotalRetried    int64
	TotalFailed     int64
	TotalDLQ        int64
	CurrentQueued   int
	CurrentRetrying int
}

// NewOutputBuffer creates a new output buffer
func NewOutputBuffer(outputName string, output OutputPlugin, config OutputBufferConfig) (*OutputBuffer, error) {
	if !config.Enabled {
		return &OutputBuffer{
			config:     config,
			outputName: outputName,
			output:     output,
		}, nil
	}

	// Create buffer directory
	bufferDir := filepath.Join(config.Dir, outputName)
	if err := os.MkdirAll(bufferDir, 0750); err != nil {
		return nil, fmt.Errorf("failed to create buffer directory: %w", err)
	}

	// Create DLQ directory if enabled
	if config.DLQEnabled {
		dlqDir := config.DLQPath
		if err := os.MkdirAll(dlqDir, 0750); err != nil {
			return nil, fmt.Errorf("failed to create DLQ directory: %w", err)
		}
	}

	ob := &OutputBuffer{
		config:      config,
		outputName:  outputName,
		output:      output,
		queue:       make(chan *BufferedLog, config.MaxQueueSize),
		retryQueue:  make([]*BufferedLog, 0),
		stopCh:      make(chan struct{}),
		flushTicker: time.NewTicker(config.FlushInterval),
	}

	// Open DLQ file if enabled
	if config.DLQEnabled {
		dlqPath := filepath.Join(config.DLQPath, fmt.Sprintf("%s-dlq.jsonl", outputName))
		file, err := os.OpenFile(dlqPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600) // #nosec G304 - path constructed from controlled inputs
		if err != nil {
			return nil, fmt.Errorf("failed to open DLQ file: %w", err)
		}
		ob.dlqFile = file
	}

	// Load persisted logs from disk
	if err := ob.loadPersistedLogs(); err != nil {
		log.Printf("[BUFFER:%s] Error loading persisted logs: %v", outputName, err)
	}

	// Start worker goroutines
	ob.wg.Add(2)
	go ob.deliveryWorker()
	go ob.retryWorker()

	log.Printf("[BUFFER:%s] Output buffer initialized: queue=%d, retries=%d, dlq=%v",
		outputName, config.MaxQueueSize, config.MaxRetries, config.DLQEnabled)

	return ob, nil
}

// Enqueue adds a log to the buffer
func (ob *OutputBuffer) Enqueue(logEntry *Log) error {
	if !ob.config.Enabled {
		// Direct delivery if buffering is disabled
		return ob.output.Write(logEntry)
	}

	bufferedLog := &BufferedLog{
		Log:         logEntry,
		Attempts:    0,
		LastAttempt: time.Time{},
		OutputName:  ob.outputName,
		EnqueuedAt:  time.Now(),
	}

	ob.statsMu.Lock()
	ob.stats.TotalEnqueued++
	ob.stats.CurrentQueued++
	ob.statsMu.Unlock()

	select {
	case ob.queue <- bufferedLog:
		return nil
	case <-time.After(100 * time.Millisecond):
		// Queue is full or blocked, persist to disk
		ob.statsMu.Lock()
		ob.stats.CurrentQueued--
		ob.statsMu.Unlock()
		return ob.persistLog(bufferedLog)
	}
}

// deliveryWorker processes logs from the main queue
func (ob *OutputBuffer) deliveryWorker() {
	defer ob.wg.Done()

	log.Printf("[BUFFER:%s] Delivery worker started", ob.outputName)

	for {
		select {
		case bufferedLog := <-ob.queue:
			ob.statsMu.Lock()
			ob.stats.CurrentQueued--
			ob.statsMu.Unlock()

			log.Printf("[BUFFER:%s] Attempting delivery (attempt %d)", ob.outputName, bufferedLog.Attempts+1)

			if err := ob.deliverLog(bufferedLog); err != nil {
				log.Printf("[BUFFER:%s] Delivery failed: %v (attempt %d/%d)",
					ob.outputName, err, bufferedLog.Attempts, ob.config.MaxRetries)
				ob.requeueForRetry(bufferedLog)
			} else {
				ob.statsMu.Lock()
				ob.stats.TotalDelivered++
				ob.statsMu.Unlock()
				log.Printf("[BUFFER:%s] Delivery successful", ob.outputName)
			}

		case <-ob.stopCh:
			log.Printf("[BUFFER:%s] Delivery worker stopping", ob.outputName)
			return
		}
	}
}

// retryWorker processes logs that need to be retried
func (ob *OutputBuffer) retryWorker() {
	defer ob.wg.Done()

	log.Printf("[BUFFER:%s] Retry worker started", ob.outputName)

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			ob.processRetries()

		case <-ob.flushTicker.C:
			ob.persistRetryQueue()

		case <-ob.stopCh:
			log.Printf("[BUFFER:%s] Retry worker stopping", ob.outputName)
			return
		}
	}
}

// processRetries attempts to deliver logs from the retry queue
func (ob *OutputBuffer) processRetries() {
	ob.retryMu.Lock()
	queueSize := len(ob.retryQueue)
	ob.retryMu.Unlock()

	if queueSize > 0 {
		log.Printf("[BUFFER:%s] Processing %d logs in retry queue", ob.outputName, queueSize)
	}

	ob.retryMu.Lock()
	defer ob.retryMu.Unlock()

	now := time.Now()
	remaining := make([]*BufferedLog, 0)

	for _, bufferedLog := range ob.retryQueue {
		// Calculate backoff delay
		backoff := ob.calculateBackoff(bufferedLog.Attempts)
		nextAttempt := bufferedLog.LastAttempt.Add(backoff)

		if now.Before(nextAttempt) {
			// Not ready for retry yet
			remaining = append(remaining, bufferedLog)
			continue
		}

		log.Printf("[BUFFER:%s] Retrying log (attempt %d/%d, backoff: %v)",
			ob.outputName, bufferedLog.Attempts, ob.config.MaxRetries, backoff)

		// Try delivery
		if err := ob.deliverLog(bufferedLog); err != nil {
			log.Printf("[BUFFER:%s] Retry failed: %v (attempt %d/%d)",
				ob.outputName, err, bufferedLog.Attempts, ob.config.MaxRetries)

			if bufferedLog.Attempts >= ob.config.MaxRetries {
				// Max retries reached, send to DLQ
				log.Printf("[BUFFER:%s] Max retries reached, sending to DLQ", ob.outputName)
				ob.sendToDLQ(bufferedLog)
			} else {
				// Requeue for another retry
				remaining = append(remaining, bufferedLog)
			}
		} else {
			log.Printf("[BUFFER:%s] Retry successful!", ob.outputName)
			ob.statsMu.Lock()
			ob.stats.TotalDelivered++
			ob.statsMu.Unlock()
		}
	}

	ob.retryQueue = remaining
	ob.statsMu.Lock()
	ob.stats.CurrentRetrying = len(remaining)
	ob.statsMu.Unlock()
}

// deliverLog attempts to deliver a log to the output
func (ob *OutputBuffer) deliverLog(bufferedLog *BufferedLog) error {
	bufferedLog.Attempts++
	bufferedLog.LastAttempt = time.Now()

	return ob.output.Write(bufferedLog.Log)
}

// requeueForRetry adds a log to the retry queue
func (ob *OutputBuffer) requeueForRetry(bufferedLog *BufferedLog) {
	ob.retryMu.Lock()
	defer ob.retryMu.Unlock()

	ob.retryQueue = append(ob.retryQueue, bufferedLog)

	ob.statsMu.Lock()
	ob.stats.TotalRetried++
	ob.stats.CurrentRetrying++
	ob.statsMu.Unlock()
}

// calculateBackoff calculates exponential backoff delay
func (ob *OutputBuffer) calculateBackoff(attempts int) time.Duration {
	// For first retry (attempts=1), backoff is 1x base interval.
	// For subsequent retries, backoff doubles: 2x, 4x, 8x, etc.
	if attempts < 1 {
		attempts = 1
	}

	// Cap attempts to prevent excessively large backoff delays (2^10 = 1024x multiplier is already very large)
	if attempts > 10 {
		attempts = 10
	}

	// Exponential backoff: RetryInterval * 2^(attempts-1)
	// Ensure attempts is at least 1
	if attempts < 1 {
		attempts = 1
	}

	// Limit shift to 30 bits to prevent overflow (2^30 = 1,073,741,824)
	shift := attempts - 1
	if shift > 30 {
		shift = 30
	}
	multiplier := int64(1 << uint(shift)) // #nosec G115 - shift is capped at 30 bits, safe for int64

	backoff := ob.config.RetryInterval * time.Duration(multiplier)

	if backoff > ob.config.MaxRetryDelay {
		backoff = ob.config.MaxRetryDelay
	}

	return backoff
}

// sendToDLQ writes a log to the Dead Letter Queue
func (ob *OutputBuffer) sendToDLQ(bufferedLog *BufferedLog) {
	if !ob.config.DLQEnabled || ob.dlqFile == nil {
		ob.statsMu.Lock()
		ob.stats.TotalFailed++
		ob.statsMu.Unlock()
		log.Printf("[BUFFER:%s] Log failed permanently (DLQ disabled)", ob.outputName)
		return
	}

	ob.dlqMu.Lock()
	defer ob.dlqMu.Unlock()

	data, err := json.Marshal(bufferedLog)
	if err != nil {
		log.Printf("[BUFFER:%s] Error marshaling DLQ entry: %v", ob.outputName, err)
		return
	}

	if _, err := ob.dlqFile.Write(append(data, '\n')); err != nil {
		log.Printf("[BUFFER:%s] Error writing to DLQ: %v", ob.outputName, err)
		return
	}

	ob.statsMu.Lock()
	ob.stats.TotalDLQ++
	ob.statsMu.Unlock()

	log.Printf("[BUFFER:%s] Log sent to DLQ after %d failed attempts", ob.outputName, bufferedLog.Attempts)
}

// persistLog saves a log to disk when the queue is full
func (ob *OutputBuffer) persistLog(bufferedLog *BufferedLog) error {
	filename := filepath.Join(ob.config.Dir, ob.outputName, fmt.Sprintf("buffer-%d.jsonl", time.Now().UnixNano()))

	data, err := json.Marshal(bufferedLog)
	if err != nil {
		return fmt.Errorf("failed to marshal log: %w", err)
	}

	if err := os.WriteFile(filename, append(data, '\n'), 0600); err != nil {
		return fmt.Errorf("failed to write buffer file: %w", err)
	}

	return nil
}

// persistRetryQueue saves the retry queue to disk
func (ob *OutputBuffer) persistRetryQueue() {
	ob.retryMu.Lock()
	defer ob.retryMu.Unlock()

	if len(ob.retryQueue) == 0 {
		return
	}

	filename := filepath.Join(ob.config.Dir, ob.outputName, "retry-queue.jsonl")
	file, err := os.Create(filename) // #nosec G304 - path constructed from controlled inputs
	if err != nil {
		log.Printf("[BUFFER:%s] Error creating retry queue file: %v", ob.outputName, err)
		return
	}
	defer func() {
		_ = file.Close()
	}()

	for _, bufferedLog := range ob.retryQueue {
		data, err := json.Marshal(bufferedLog)
		if err != nil {
			log.Printf("[BUFFER:%s] Error marshaling retry log: %v", ob.outputName, err)
			continue
		}
		if _, err := file.Write(append(data, '\n')); err != nil {
			log.Printf("[BUFFER:%s] Error writing retry log to disk: %v", ob.outputName, err)
		}
	}
}

// loadPersistedLogs loads logs from disk on startup
func (ob *OutputBuffer) loadPersistedLogs() error {
	bufferDir := filepath.Join(ob.config.Dir, ob.outputName)

	files, err := filepath.Glob(filepath.Join(bufferDir, "*.jsonl"))
	if err != nil {
		return err
	}

	if len(files) == 0 {
		return nil
	}

	log.Printf("[BUFFER:%s] Loading %d persisted buffer files", ob.outputName, len(files))

	loadedCount := 0
	for _, filename := range files {
		// Validate that the file is within our configured directory
		if err := validateFileInDirectory(filename, bufferDir); err != nil {
			log.Printf("[BUFFER:%s] Skipping invalid buffer file path %s: %v", ob.outputName, filename, err)
			continue
		}

		data, err := os.ReadFile(filename) // #nosec G304 - path validated by validateFileInDirectory above
		if err != nil {
			log.Printf("[BUFFER:%s] Error reading buffer file %s: %v", ob.outputName, filename, err)
			continue
		}

		var bufferedLog BufferedLog
		if err := json.Unmarshal(data, &bufferedLog); err != nil {
			log.Printf("[BUFFER:%s] Error unmarshaling buffer file %s: %v", ob.outputName, filename, err)
			continue
		}

		// Reset attempts for persisted logs
		bufferedLog.Attempts = 0
		bufferedLog.LastAttempt = time.Time{}

		// Add to retry queue
		ob.retryQueue = append(ob.retryQueue, &bufferedLog)
		loadedCount++

		// Remove the file after loading
		_ = os.Remove(filename)
	}

	ob.statsMu.Lock()
	ob.stats.CurrentRetrying = len(ob.retryQueue)
	ob.statsMu.Unlock()

	log.Printf("[BUFFER:%s] Loaded %d logs from disk", ob.outputName, loadedCount)
	return nil
}

// GetStats returns current buffer statistics
func (ob *OutputBuffer) GetStats() BufferStats {
	ob.statsMu.RLock()
	defer ob.statsMu.RUnlock()
	return ob.stats
}

// Close shuts down the output buffer
func (ob *OutputBuffer) Close() error {
	if !ob.config.Enabled {
		return ob.output.Close()
	}

	log.Printf("[BUFFER:%s] Shutting down output buffer...", ob.outputName)

	// Stop workers
	close(ob.stopCh)
	if ob.flushTicker != nil {
		ob.flushTicker.Stop()
	}

	// Drain the queue with timeout
	timeout := time.After(10 * time.Second)
drainLoop:
	for {
		select {
		case bufferedLog := <-ob.queue:
			if err := ob.deliverLog(bufferedLog); err != nil {
				ob.requeueForRetry(bufferedLog)
			}
		case <-timeout:
			log.Printf("[BUFFER:%s] Drain timeout reached", ob.outputName)
			break drainLoop
		default:
			break drainLoop
		}
	}

	// Persist remaining logs
	ob.persistRetryQueue()

	// Wait for workers
	ob.wg.Wait()

	// Close DLQ file
	if ob.dlqFile != nil {
		_ = ob.dlqFile.Close()
	}

	// Close underlying output
	if err := ob.output.Close(); err != nil {
		return err
	}

	// Log final stats
	stats := ob.GetStats()
	log.Printf("[BUFFER:%s] Final stats - Enqueued: %d, Delivered: %d, Retried: %d, DLQ: %d, Failed: %d",
		ob.outputName, stats.TotalEnqueued, stats.TotalDelivered, stats.TotalRetried, stats.TotalDLQ, stats.TotalFailed)

	return nil
}
