package core

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// PersistenceConfig defines persistence settings
type PersistenceConfig struct {
	Enabled        bool   `yaml:"enabled"`         // Enable/disable persistence
	Dir            string `yaml:"dir"`             // Directory for WAL files
	MaxFileSize    int64  `yaml:"max_file_size"`   // Max size per WAL file in bytes (default: 100MB)
	BufferSize     int    `yaml:"buffer_size"`     // Buffer size for batch writes (default: 100)
	FlushInterval  int    `yaml:"flush_interval"`  // Flush interval in seconds (default: 5)
	RetentionHours int    `yaml:"retention_hours"` // How long to keep WAL files (default: 24)
	SyncWrites     bool   `yaml:"sync_writes"`     // fsync after each write (slower but safer)
}

// DefaultPersistenceConfig returns default persistence configuration
func DefaultPersistenceConfig() PersistenceConfig {
	return PersistenceConfig{
		Enabled:        false,
		Dir:            "./data/wal",
		MaxFileSize:    100 * 1024 * 1024, // 100MB
		BufferSize:     100,
		FlushInterval:  5,
		RetentionHours: 24,
		SyncWrites:     false,
	}
}

// Persistence handles Write-Ahead Logging for log entries
type Persistence struct {
	config        PersistenceConfig
	currentFile   *os.File
	writer        *bufio.Writer
	currentSize   int64
	buffer        []*Log
	bufferMu      sync.Mutex
	flushTicker   *time.Ticker
	stopCh        chan struct{}
	wg            sync.WaitGroup
	sequenceNum   uint64
	sequenceMu    sync.Mutex
	recoveryQueue chan *Log
}

// WALEntry represents a Write-Ahead Log entry
type WALEntry struct {
	Sequence  uint64    `json:"seq"`
	Timestamp time.Time `json:"ts"`
	Log       *Log      `json:"log"`
}

// NewPersistence creates a new persistence handler
func NewPersistence(config PersistenceConfig) (*Persistence, error) {
	if !config.Enabled {
		return &Persistence{config: config}, nil
	}

	// Create WAL directory if it doesn't exist
	if err := os.MkdirAll(config.Dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create WAL directory: %w", err)
	}

	p := &Persistence{
		config:        config,
		buffer:        make([]*Log, 0, config.BufferSize),
		stopCh:        make(chan struct{}),
		recoveryQueue: make(chan *Log, 1000),
	}

	// Open initial WAL file
	if err := p.rotateFile(); err != nil {
		return nil, fmt.Errorf("failed to create initial WAL file: %w", err)
	}

	// Start flush ticker
	p.flushTicker = time.NewTicker(time.Duration(config.FlushInterval) * time.Second)
	p.wg.Add(1)
	go p.flushLoop()

	// Start cleanup routine
	p.wg.Add(1)
	go p.cleanupLoop()

	log.Printf("Persistence initialized: dir=%s, buffer=%d, flush=%ds",
		config.Dir, config.BufferSize, config.FlushInterval)

	return p, nil
}

// Persist saves a log entry to the WAL
func (p *Persistence) Persist(logEntry *Log) error {
	if !p.config.Enabled {
		return nil
	}

	p.bufferMu.Lock()
	defer p.bufferMu.Unlock()

	// Add to buffer
	p.buffer = append(p.buffer, logEntry)

	// Flush if buffer is full
	if len(p.buffer) >= p.config.BufferSize {
		return p.flushBufferLocked()
	}

	return nil
}

// flushLoop periodically flushes the buffer
func (p *Persistence) flushLoop() {
	defer p.wg.Done()
	for {
		select {
		case <-p.flushTicker.C:
			p.bufferMu.Lock()
			if len(p.buffer) > 0 {
				if err := p.flushBufferLocked(); err != nil {
					log.Printf("Error flushing persistence buffer: %v", err)
				}
			}
			p.bufferMu.Unlock()
		case <-p.stopCh:
			return
		}
	}
}

// flushBufferLocked flushes the buffer to disk (must be called with bufferMu locked)
func (p *Persistence) flushBufferLocked() error {
	if len(p.buffer) == 0 {
		return nil
	}

	// Check if we need to rotate the file
	if p.currentSize > p.config.MaxFileSize {
		if err := p.rotateFile(); err != nil {
			return fmt.Errorf("failed to rotate WAL file: %w", err)
		}
	}

	// Write buffered logs
	for _, logEntry := range p.buffer {
		p.sequenceMu.Lock()
		p.sequenceNum++
		seq := p.sequenceNum
		p.sequenceMu.Unlock()

		entry := WALEntry{
			Sequence:  seq,
			Timestamp: time.Now(),
			Log:       logEntry,
		}

		data, err := json.Marshal(entry)
		if err != nil {
			log.Printf("Error marshaling WAL entry: %v", err)
			continue
		}

		// Write to file
		n, err := p.writer.Write(append(data, '\n'))
		if err != nil {
			return fmt.Errorf("failed to write to WAL: %w", err)
		}

		p.currentSize += int64(n)
	}

	// Flush writer buffer
	if err := p.writer.Flush(); err != nil {
		return fmt.Errorf("failed to flush writer: %w", err)
	}

	// Optionally sync to disk
	if p.config.SyncWrites {
		if err := p.currentFile.Sync(); err != nil {
			return fmt.Errorf("failed to sync file: %w", err)
		}
	}

	// Clear buffer
	p.buffer = p.buffer[:0]

	return nil
}

// rotateFile creates a new WAL file
func (p *Persistence) rotateFile() error {
	// Close current file if open
	if p.currentFile != nil {
		if err := p.writer.Flush(); err != nil {
			log.Printf("Error flushing before rotation: %v", err)
		}
		if err := p.currentFile.Close(); err != nil {
			log.Printf("Error closing WAL file: %v", err)
		}
	}

	// Create new file with timestamp and sequence number for uniqueness.
	// The sequence number is incremented before being used in the filename to prevent
	// collisions in the rare case where two rotations occur within the same second
	// (e.g., under high load). This ensures each WAL file has a unique name even when
	// timestamps are identical.
	p.sequenceMu.Lock()
	p.sequenceNum++
	seq := p.sequenceNum
	p.sequenceMu.Unlock()

	filename := filepath.Join(p.config.Dir, fmt.Sprintf("wal-%s-%d.log", time.Now().Format("20060102-150405"), seq))
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to create WAL file: %w", err)
	}

	p.currentFile = file
	p.writer = bufio.NewWriter(file)
	p.currentSize = 0

	log.Printf("Created new WAL file: %s", filename)
	return nil
}

// Recover reads all WAL files and returns logs that need to be reprocessed
func (p *Persistence) Recover() (<-chan *Log, error) {
	if !p.config.Enabled {
		close(p.recoveryQueue)
		return p.recoveryQueue, nil
	}

	// Start recovery in background
	p.wg.Add(1)
	go p.recoverAsync()

	return p.recoveryQueue, nil
}

// recoverAsync performs recovery in the background
func (p *Persistence) recoverAsync() {
	defer p.wg.Done()
	defer close(p.recoveryQueue)

	files, err := filepath.Glob(filepath.Join(p.config.Dir, "wal-*.log"))
	if err != nil {
		log.Printf("Error listing WAL files: %v", err)
		return
	}

	if len(files) == 0 {
		log.Println("No WAL files found for recovery")
		return
	}

	log.Printf("Found %d WAL files for recovery", len(files))

	recoveredCount := 0
	for _, filename := range files {
		count, err := p.recoverFile(filename)
		if err != nil {
			log.Printf("Error recovering from %s: %v", filename, err)
			continue
		}
		recoveredCount += count
	}

	log.Printf("Recovery complete: %d logs recovered from %d files", recoveredCount, len(files))
}

// recoverFile recovers logs from a single WAL file
func (p *Persistence) recoverFile(filename string) (int, error) {
	file, err := os.Open(filename)
	if err != nil {
		return 0, fmt.Errorf("failed to open WAL file: %w", err)
	}
	defer func() { _ = file.Close() }()

	reader := bufio.NewReader(file)
	count := 0

	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			return count, fmt.Errorf("error reading WAL file: %w", err)
		}

		var entry WALEntry
		if err := json.Unmarshal(line, &entry); err != nil {
			log.Printf("Error unmarshaling WAL entry: %v", err)
			continue
		}

		// Update sequence number
		p.sequenceMu.Lock()
		if entry.Sequence > p.sequenceNum {
			p.sequenceNum = entry.Sequence
		}
		p.sequenceMu.Unlock()

		// Send to recovery queue
		select {
		case p.recoveryQueue <- entry.Log:
			count++
		case <-p.stopCh:
			return count, nil
		}
	}

	return count, nil
}

// cleanupLoop periodically removes old WAL files
func (p *Persistence) cleanupLoop() {
	defer p.wg.Done()

	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			p.cleanup()
		case <-p.stopCh:
			return
		}
	}
}

// cleanup removes WAL files older than retention period
func (p *Persistence) cleanup() {
	if p.config.RetentionHours <= 0 {
		return // No cleanup if retention is 0
	}

	files, err := filepath.Glob(filepath.Join(p.config.Dir, "wal-*.log"))
	if err != nil {
		log.Printf("Error listing WAL files for cleanup: %v", err)
		return
	}

	cutoff := time.Now().Add(-time.Duration(p.config.RetentionHours) * time.Hour)
	removedCount := 0

	for _, filename := range files {
		info, err := os.Stat(filename)
		if err != nil {
			continue
		}

		if info.ModTime().Before(cutoff) {
			// Don't remove the current file
			if p.currentFile != nil && filename == p.currentFile.Name() {
				continue
			}

			if err := os.Remove(filename); err != nil {
				log.Printf("Error removing old WAL file %s: %v", filename, err)
			} else {
				removedCount++
			}
		}
	}

	if removedCount > 0 {
		log.Printf("Cleaned up %d old WAL files", removedCount)
	}
}

// Close shuts down the persistence handler
func (p *Persistence) Close() error {
	if !p.config.Enabled {
		return nil
	}

	log.Println("Shutting down persistence...")

	// Stop background goroutines
	close(p.stopCh)
	if p.flushTicker != nil {
		p.flushTicker.Stop()
	}

	// Final flush
	p.bufferMu.Lock()
	if err := p.flushBufferLocked(); err != nil {
		log.Printf("Error during final flush: %v", err)
	}
	p.bufferMu.Unlock()

	// Close file
	if p.currentFile != nil {
		if err := p.writer.Flush(); err != nil {
			log.Printf("Error flushing writer: %v", err)
		}
		if err := p.currentFile.Close(); err != nil {
			return fmt.Errorf("failed to close WAL file: %w", err)
		}
	}

	// Wait for goroutines
	p.wg.Wait()

	log.Println("Persistence shut down complete")
	return nil
}
