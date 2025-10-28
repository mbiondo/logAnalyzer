package core

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// Mock output that can fail
type MockOutput struct {
	logs       []*Log
	mu         sync.Mutex
	shouldFail bool
	failCount  int
	writeCount int
	closed     bool
}

func (m *MockOutput) Write(log *Log) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.writeCount++

	if m.shouldFail && m.failCount > 0 {
		m.failCount--
		return errors.New("simulated output failure")
	}

	m.logs = append(m.logs, log)
	return nil
}

func (m *MockOutput) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	return nil
}

func (m *MockOutput) GetLogs() []*Log {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]*Log, len(m.logs))
	copy(result, m.logs)
	return result
}

func (m *MockOutput) GetWriteCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.writeCount
}

func (m *MockOutput) SetShouldFail(fail bool, count int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.shouldFail = fail
	m.failCount = count
}

func TestOutputBuffer_Disabled(t *testing.T) {
	output := &MockOutput{}
	config := OutputBufferConfig{Enabled: false}

	buffer, err := NewOutputBuffer("test", output, config)
	if err != nil {
		t.Fatalf("Failed to create buffer: %v", err)
	}
	defer buffer.Close()

	log := NewLog("INFO", "test message")
	if err := buffer.Enqueue(log); err != nil {
		t.Errorf("Enqueue failed: %v", err)
	}

	// Should write directly without buffering
	time.Sleep(100 * time.Millisecond)
	logs := output.GetLogs()
	if len(logs) != 1 {
		t.Errorf("Expected 1 log, got %d", len(logs))
	}
}

func TestOutputBuffer_BasicDelivery(t *testing.T) {
	tmpDir := t.TempDir()
	output := &MockOutput{}

	config := OutputBufferConfig{
		Enabled:       true,
		Dir:           tmpDir,
		MaxQueueSize:  10,
		MaxRetries:    3,
		RetryInterval: 100 * time.Millisecond,
		MaxRetryDelay: 1 * time.Second,
		FlushInterval: 500 * time.Millisecond,
		DLQEnabled:    true,
		DLQPath:       tmpDir,
	}

	buffer, err := NewOutputBuffer("test", output, config)
	if err != nil {
		t.Fatalf("Failed to create buffer: %v", err)
	}
	defer buffer.Close()

	// Enqueue some logs
	for i := 0; i < 5; i++ {
		log := NewLog("INFO", "test message")
		if err := buffer.Enqueue(log); err != nil {
			t.Errorf("Enqueue failed: %v", err)
		}
	}

	// Wait for delivery
	time.Sleep(500 * time.Millisecond)

	logs := output.GetLogs()
	if len(logs) != 5 {
		t.Errorf("Expected 5 logs delivered, got %d", len(logs))
	}

	stats := buffer.GetStats()
	if stats.TotalEnqueued != 5 {
		t.Errorf("Expected 5 enqueued, got %d", stats.TotalEnqueued)
	}
	if stats.TotalDelivered != 5 {
		t.Errorf("Expected 5 delivered, got %d", stats.TotalDelivered)
	}
}

func TestOutputBuffer_RetryOnFailure(t *testing.T) {
	tmpDir := t.TempDir()
	output := &MockOutput{}
	output.SetShouldFail(true, 2) // Fail first 2 attempts

	config := OutputBufferConfig{
		Enabled:       true,
		Dir:           tmpDir,
		MaxQueueSize:  10,
		MaxRetries:    5,
		RetryInterval: 100 * time.Millisecond,
		MaxRetryDelay: 1 * time.Second,
		FlushInterval: 500 * time.Millisecond,
		DLQEnabled:    true,
		DLQPath:       tmpDir,
	}

	buffer, err := NewOutputBuffer("test", output, config)
	if err != nil {
		t.Fatalf("Failed to create buffer: %v", err)
	}
	defer buffer.Close()

	// Enqueue a log
	log := NewLog("INFO", "test message")
	if err := buffer.Enqueue(log); err != nil {
		t.Errorf("Enqueue failed: %v", err)
	}

	// Wait for retries (backoff: 100ms, 200ms, then success)
	// Need extra time for retry worker to process
	time.Sleep(3 * time.Second)

	// Should eventually succeed
	logs := output.GetLogs()
	if len(logs) != 1 {
		t.Errorf("Expected 1 log after retries, got %d", len(logs))
	}

	stats := buffer.GetStats()
	if stats.TotalRetried == 0 {
		t.Error("Expected retries to occur")
	}
	if stats.TotalDelivered != 1 {
		t.Errorf("Expected 1 delivered, got %d", stats.TotalDelivered)
	}
}

func TestOutputBuffer_DLQAfterMaxRetries(t *testing.T) {
	tmpDir := t.TempDir()
	output := &MockOutput{}
	output.SetShouldFail(true, 100) // Fail always

	config := OutputBufferConfig{
		Enabled:       true,
		Dir:           tmpDir,
		MaxQueueSize:  10,
		MaxRetries:    3,
		RetryInterval: 50 * time.Millisecond,
		MaxRetryDelay: 200 * time.Millisecond,
		FlushInterval: 500 * time.Millisecond,
		DLQEnabled:    true,
		DLQPath:       tmpDir,
	}

	buffer, err := NewOutputBuffer("test", output, config)
	if err != nil {
		t.Fatalf("Failed to create buffer: %v", err)
	}
	defer buffer.Close()

	// Enqueue a log
	log := NewLog("INFO", "test message")
	if err := buffer.Enqueue(log); err != nil {
		t.Errorf("Enqueue failed: %v", err)
	}

	// Wait for all retries to fail (50ms, 100ms, 200ms = ~350ms + processing)
	time.Sleep(3 * time.Second)

	// Should not be delivered
	logs := output.GetLogs()
	if len(logs) != 0 {
		t.Errorf("Expected 0 logs delivered, got %d", len(logs))
	}

	stats := buffer.GetStats()
	if stats.TotalDLQ != 1 {
		t.Errorf("Expected 1 log in DLQ, got %d", stats.TotalDLQ)
	}

	// Check DLQ file exists
	dlqFile := filepath.Join(tmpDir, "test-dlq.jsonl")
	if _, err := os.Stat(dlqFile); os.IsNotExist(err) {
		t.Error("DLQ file should exist")
	}
}

func TestOutputBuffer_ExponentialBackoff(t *testing.T) {
	tmpDir := t.TempDir()
	output := &MockOutput{}
	output.SetShouldFail(true, 100)

	config := OutputBufferConfig{
		Enabled:       true,
		Dir:           tmpDir,
		MaxQueueSize:  10,
		MaxRetries:    5,
		RetryInterval: 100 * time.Millisecond,
		MaxRetryDelay: 1 * time.Second,
		FlushInterval: 500 * time.Millisecond,
		DLQEnabled:    true,
		DLQPath:       tmpDir,
	}

	buffer, err := NewOutputBuffer("test", output, config)
	if err != nil {
		t.Fatalf("Failed to create buffer: %v", err)
	}
	defer buffer.Close()

	// Test backoff calculation
	tests := []struct {
		attempts int
		expected time.Duration
	}{
		{0, 100 * time.Millisecond},
		{1, 100 * time.Millisecond},
		{2, 200 * time.Millisecond},
		{3, 400 * time.Millisecond},
		{4, 800 * time.Millisecond},
		{5, 1 * time.Second},  // Capped at MaxRetryDelay
		{10, 1 * time.Second}, // Still capped
	}

	for _, tt := range tests {
		backoff := buffer.calculateBackoff(tt.attempts)
		if backoff != tt.expected {
			t.Errorf("For attempt %d, expected backoff %v, got %v", tt.attempts, tt.expected, backoff)
		}
	}
}

func TestOutputBuffer_QueueFullPersistence(t *testing.T) {
	tmpDir := t.TempDir()
	bufferDir := filepath.Join(tmpDir, "test")
	os.MkdirAll(bufferDir, 0755)

	output := &MockOutput{}

	config := OutputBufferConfig{
		Enabled:       true,
		Dir:           tmpDir,
		MaxQueueSize:  2, // Very small queue to force persistence
		MaxRetries:    3,
		RetryInterval: 100 * time.Millisecond,
		MaxRetryDelay: 1 * time.Second,
		FlushInterval: 500 * time.Millisecond,
		DLQEnabled:    true,
		DLQPath:       tmpDir,
	}

	buffer, err := NewOutputBuffer("test", output, config)
	if err != nil {
		t.Fatalf("Failed to create buffer: %v", err)
	}

	// Try to enqueue more logs than the queue can hold very rapidly
	// This should cause some to timeout and persist
	done := make(chan bool)
	go func() {
		for i := 0; i < 10; i++ {
			log := NewLog("INFO", "test message")
			buffer.Enqueue(log)
		}
		done <- true
	}()

	<-done
	time.Sleep(200 * time.Millisecond)

	buffer.Close()

	// Either buffer files were created OR all logs were delivered quickly
	// Both are acceptable outcomes
	bufferFiles, _ := filepath.Glob(filepath.Join(bufferDir, "buffer-*.jsonl"))
	stats := buffer.GetStats()

	if len(bufferFiles) == 0 && stats.TotalDelivered != 10 {
		t.Errorf("Expected either buffer files or all 10 logs delivered, got %d files and %d delivered",
			len(bufferFiles), stats.TotalDelivered)
	}
}

func TestOutputBuffer_LoadPersistedLogs(t *testing.T) {
	tmpDir := t.TempDir()
	bufferDir := filepath.Join(tmpDir, "test")
	os.MkdirAll(bufferDir, 0755)

	// Create a persisted log file
	bufferedLog := &BufferedLog{
		Log:         NewLog("ERROR", "persisted message"),
		Attempts:    0,
		LastAttempt: time.Time{},
		OutputName:  "test",
		EnqueuedAt:  time.Now(),
	}

	data, _ := json.Marshal(bufferedLog)
	persistFile := filepath.Join(bufferDir, "buffer-12345.jsonl")
	os.WriteFile(persistFile, data, 0644)

	// Create buffer - should load persisted logs
	output := &MockOutput{}
	config := OutputBufferConfig{
		Enabled:       true,
		Dir:           tmpDir,
		MaxQueueSize:  10,
		MaxRetries:    3,
		RetryInterval: 100 * time.Millisecond,
		MaxRetryDelay: 1 * time.Second,
		FlushInterval: 500 * time.Millisecond,
		DLQEnabled:    true,
		DLQPath:       tmpDir,
	}

	buffer, err := NewOutputBuffer("test", output, config)
	if err != nil {
		t.Fatalf("Failed to create buffer: %v", err)
	}
	defer buffer.Close()

	// Wait for delivery (should happen quickly as it's loaded into retry queue)
	time.Sleep(2 * time.Second)

	logs := output.GetLogs()
	if len(logs) != 1 {
		t.Errorf("Expected 1 persisted log to be loaded and delivered, got %d", len(logs))
	}

	if len(logs) > 0 && logs[0].Message != "persisted message" {
		t.Errorf("Expected message 'persisted message', got '%s'", logs[0].Message)
	}

	// Persist file should be deleted
	if _, err := os.Stat(persistFile); !os.IsNotExist(err) {
		t.Error("Persist file should be deleted after loading")
	}
}

func TestOutputBuffer_ConcurrentEnqueue(t *testing.T) {
	tmpDir := t.TempDir()
	output := &MockOutput{}

	config := OutputBufferConfig{
		Enabled:       true,
		Dir:           tmpDir,
		MaxQueueSize:  100,
		MaxRetries:    3,
		RetryInterval: 100 * time.Millisecond,
		MaxRetryDelay: 1 * time.Second,
		FlushInterval: 500 * time.Millisecond,
		DLQEnabled:    true,
		DLQPath:       tmpDir,
	}

	buffer, err := NewOutputBuffer("test", output, config)
	if err != nil {
		t.Fatalf("Failed to create buffer: %v", err)
	}
	defer buffer.Close()

	// Concurrent enqueue from multiple goroutines
	numGoroutines := 10
	logsPerGoroutine := 10
	var wg sync.WaitGroup

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < logsPerGoroutine; j++ {
				log := NewLog("INFO", "concurrent message")
				if err := buffer.Enqueue(log); err != nil {
					t.Errorf("Concurrent enqueue failed: %v", err)
				}
			}
		}(i)
	}

	wg.Wait()

	// Wait for all deliveries
	time.Sleep(1 * time.Second)

	expectedTotal := numGoroutines * logsPerGoroutine
	logs := output.GetLogs()
	if len(logs) != expectedTotal {
		t.Errorf("Expected %d logs, got %d", expectedTotal, len(logs))
	}

	stats := buffer.GetStats()
	if stats.TotalEnqueued != int64(expectedTotal) {
		t.Errorf("Expected %d enqueued, got %d", expectedTotal, stats.TotalEnqueued)
	}
}

func TestOutputBuffer_GracefulShutdown(t *testing.T) {
	tmpDir := t.TempDir()
	output := &MockOutput{}

	config := OutputBufferConfig{
		Enabled:       true,
		Dir:           tmpDir,
		MaxQueueSize:  50,
		MaxRetries:    3,
		RetryInterval: 100 * time.Millisecond,
		MaxRetryDelay: 1 * time.Second,
		FlushInterval: 500 * time.Millisecond,
		DLQEnabled:    true,
		DLQPath:       tmpDir,
	}

	buffer, err := NewOutputBuffer("test", output, config)
	if err != nil {
		t.Fatalf("Failed to create buffer: %v", err)
	}

	// Enqueue logs
	for i := 0; i < 20; i++ {
		log := NewLog("INFO", "test message")
		buffer.Enqueue(log)
	}

	// Close immediately
	if err := buffer.Close(); err != nil {
		t.Errorf("Close failed: %v", err)
	}

	// Should attempt to drain the queue
	logs := output.GetLogs()
	if len(logs) == 0 {
		t.Error("Expected some logs to be delivered during shutdown")
	}

	if !output.closed {
		t.Error("Output should be closed")
	}
}

func TestOutputBuffer_Stats(t *testing.T) {
	tmpDir := t.TempDir()
	output := &MockOutput{}
	output.SetShouldFail(true, 2) // Fail first 2

	config := OutputBufferConfig{
		Enabled:       true,
		Dir:           tmpDir,
		MaxQueueSize:  10,
		MaxRetries:    3,
		RetryInterval: 50 * time.Millisecond,
		MaxRetryDelay: 500 * time.Millisecond,
		FlushInterval: 200 * time.Millisecond,
		DLQEnabled:    true,
		DLQPath:       tmpDir,
	}

	buffer, err := NewOutputBuffer("test", output, config)
	if err != nil {
		t.Fatalf("Failed to create buffer: %v", err)
	}
	defer buffer.Close()

	// Enqueue logs
	for i := 0; i < 3; i++ {
		log := NewLog("INFO", "test message")
		buffer.Enqueue(log)
	}

	// Wait for processing (first 2 fail, need retries: 50ms, 100ms, 200ms)
	time.Sleep(3 * time.Second)

	stats := buffer.GetStats()

	if stats.TotalEnqueued != 3 {
		t.Errorf("Expected 3 enqueued, got %d", stats.TotalEnqueued)
	}

	if stats.TotalRetried == 0 {
		t.Error("Expected some retries")
	}

	if stats.TotalDelivered != 3 {
		t.Errorf("Expected 3 delivered eventually, got %d", stats.TotalDelivered)
	}
}

func TestDefaultOutputBufferConfig(t *testing.T) {
	config := DefaultOutputBufferConfig()

	if config.Enabled {
		t.Error("Default should have buffering disabled")
	}

	if config.MaxQueueSize <= 0 {
		t.Error("Default queue size should be positive")
	}

	if config.MaxRetries <= 0 {
		t.Error("Default max retries should be positive")
	}

	if config.RetryInterval <= 0 {
		t.Error("Default retry interval should be positive")
	}
}
