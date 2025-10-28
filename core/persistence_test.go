package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPersistence_Disabled(t *testing.T) {
	config := PersistenceConfig{
		Enabled: false,
	}

	p, err := NewPersistence(config)
	if err != nil {
		t.Fatalf("Failed to create persistence: %v", err)
	}
	defer func() { _ = p.Close() }()

	// Should not error when disabled
	log := NewLog("INFO", "test message")
	if err := p.Persist(log); err != nil {
		t.Errorf("Persist failed when disabled: %v", err)
	}
}

func TestPersistence_BasicWriteAndRecover(t *testing.T) {
	tmpDir := t.TempDir()

	config := PersistenceConfig{
		Enabled:        true,
		Dir:            tmpDir,
		MaxFileSize:    1024 * 1024,
		BufferSize:     5,
		FlushInterval:  1,
		RetentionHours: 24,
		SyncWrites:     true,
	}

	// Create persistence and write logs
	p, err := NewPersistence(config)
	if err != nil {
		t.Fatalf("Failed to create persistence: %v", err)
	}

	testLogs := []*Log{
		NewLog("INFO", "message 1"),
		NewLog("WARN", "message 2"),
		NewLog("ERROR", "message 3"),
	}

	for _, log := range testLogs {
		if err := p.Persist(log); err != nil {
			t.Errorf("Failed to persist log: %v", err)
		}
	}

	// Close to flush
	if err := p.Close(); err != nil {
		t.Fatalf("Failed to close persistence: %v", err)
	}

	// Create new instance and recover
	p2, err := NewPersistence(config)
	if err != nil {
		t.Fatalf("Failed to create persistence for recovery: %v", err)
	}
	defer func() { _ = p2.Close() }()

	recoveryCh, err := p2.Recover()
	if err != nil {
		t.Fatalf("Failed to start recovery: %v", err)
	}

	recovered := []*Log{}
	for log := range recoveryCh {
		recovered = append(recovered, log)
	}

	if len(recovered) != len(testLogs) {
		t.Fatalf("Expected %d recovered logs, got %d", len(testLogs), len(recovered))
	}

	for i, log := range recovered {
		if log.Level != testLogs[i].Level {
			t.Errorf("Log %d level mismatch: expected %s, got %s", i, testLogs[i].Level, log.Level)
		}
		if log.Message != testLogs[i].Message {
			t.Errorf("Log %d message mismatch: expected %s, got %s", i, testLogs[i].Message, log.Message)
		}
	}
}

func TestPersistence_BufferFlush(t *testing.T) {
	tmpDir := t.TempDir()

	config := PersistenceConfig{
		Enabled:        true,
		Dir:            tmpDir,
		MaxFileSize:    1024 * 1024,
		BufferSize:     3,
		FlushInterval:  1,
		RetentionHours: 24,
		SyncWrites:     false,
	}

	p, err := NewPersistence(config)
	if err != nil {
		t.Fatalf("Failed to create persistence: %v", err)
	}
	defer func() { _ = p.Close() }()

	// Write exactly buffer size logs
	for i := 0; i < config.BufferSize; i++ {
		log := NewLog("INFO", "test message")
		if err := p.Persist(log); err != nil {
			t.Errorf("Failed to persist log: %v", err)
		}
	}

	// Give it a moment to flush
	time.Sleep(100 * time.Millisecond)

	// Check that file was created and has content
	files, err := filepath.Glob(filepath.Join(tmpDir, "wal-*.log"))
	if err != nil {
		t.Fatalf("Failed to list WAL files: %v", err)
	}

	if len(files) == 0 {
		t.Fatal("No WAL files created")
	}

	// Check file has content
	data, err := os.ReadFile(files[0])
	if err != nil {
		t.Fatalf("Failed to read WAL file: %v", err)
	}

	if len(data) == 0 {
		t.Error("WAL file is empty")
	}
}

func TestPersistence_FileRotation(t *testing.T) {
	tmpDir := t.TempDir()

	config := PersistenceConfig{
		Enabled:        true,
		Dir:            tmpDir,
		MaxFileSize:    500, // Small size to trigger rotation
		BufferSize:     1,
		FlushInterval:  1,
		RetentionHours: 24,
		SyncWrites:     true,
	}

	p, err := NewPersistence(config)
	if err != nil {
		t.Fatalf("Failed to create persistence: %v", err)
	}
	defer func() { _ = p.Close() }()

	// Write enough logs to trigger rotation
	for i := 0; i < 10; i++ {
		log := NewLog("INFO", "This is a longer message to fill up the file faster")
		if err := p.Persist(log); err != nil {
			t.Errorf("Failed to persist log: %v", err)
		}
	}

	// Wait for flushes
	time.Sleep(200 * time.Millisecond)

	// Check that multiple files were created
	files, err := filepath.Glob(filepath.Join(tmpDir, "wal-*.log"))
	if err != nil {
		t.Fatalf("Failed to list WAL files: %v", err)
	}

	if len(files) < 2 {
		t.Errorf("Expected multiple WAL files due to rotation, got %d", len(files))
	}
}

func TestPersistence_Cleanup(t *testing.T) {
	tmpDir := t.TempDir()

	config := PersistenceConfig{
		Enabled:        true,
		Dir:            tmpDir,
		MaxFileSize:    1024 * 1024,
		BufferSize:     1,
		FlushInterval:  1,
		RetentionHours: 0, // Keep for 0 hours (will be in cleanup test)
		SyncWrites:     false,
	}

	p, err := NewPersistence(config)
	if err != nil {
		t.Fatalf("Failed to create persistence: %v", err)
	}
	defer func() { _ = p.Close() }()

	// Create an old file manually
	oldFile := filepath.Join(tmpDir, "wal-20200101-120000.log")
	if err := os.WriteFile(oldFile, []byte("old data\n"), 0644); err != nil {
		t.Fatalf("Failed to create old file: %v", err)
	}

	// Set modification time to old
	oldTime := time.Now().Add(-25 * time.Hour)
	if err := os.Chtimes(oldFile, oldTime, oldTime); err != nil {
		t.Fatalf("Failed to set old time: %v", err)
	}

	// Trigger cleanup
	config.RetentionHours = 24
	p.config.RetentionHours = 24
	p.cleanup()

	// Check that old file was removed
	if _, err := os.Stat(oldFile); !os.IsNotExist(err) {
		t.Error("Old WAL file was not cleaned up")
	}
}

func TestWALEntry_Serialization(t *testing.T) {
	log := NewLogWithMetadata("ERROR", "test error", map[string]string{
		"key1": "value1",
		"key2": "value2",
	})

	entry := WALEntry{
		Sequence:  42,
		Timestamp: time.Now(),
		Log:       log,
	}

	// Marshal
	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("Failed to marshal WAL entry: %v", err)
	}

	// Unmarshal
	var decoded WALEntry
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal WAL entry: %v", err)
	}

	if decoded.Sequence != entry.Sequence {
		t.Errorf("Sequence mismatch: expected %d, got %d", entry.Sequence, decoded.Sequence)
	}

	if decoded.Log.Level != log.Level {
		t.Errorf("Level mismatch: expected %s, got %s", log.Level, decoded.Log.Level)
	}

	if decoded.Log.Message != log.Message {
		t.Errorf("Message mismatch: expected %s, got %s", log.Message, decoded.Log.Message)
	}

	if len(decoded.Log.Metadata) != len(log.Metadata) {
		t.Errorf("Metadata length mismatch: expected %d, got %d", len(log.Metadata), len(decoded.Log.Metadata))
	}
}

func TestDefaultPersistenceConfig(t *testing.T) {
	config := DefaultPersistenceConfig()

	if config.Enabled {
		t.Error("Default config should have persistence disabled")
	}

	if config.BufferSize <= 0 {
		t.Error("Default buffer size should be positive")
	}

	if config.FlushInterval <= 0 {
		t.Error("Default flush interval should be positive")
	}

	if config.MaxFileSize <= 0 {
		t.Error("Default max file size should be positive")
	}
}
