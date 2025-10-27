package core

import (
	"testing"
	"time"
)

func TestNewLog(t *testing.T) {
	level := "error"
	message := "This is a test error"

	log := NewLog(level, message)

	if log.Level != level {
		t.Errorf("Expected level %s, got %s", level, log.Level)
	}

	if log.Message != message {
		t.Errorf("Expected message %s, got %s", message, log.Message)
	}

	if log.Metadata == nil {
		t.Error("Expected metadata map to be initialized")
	}

	// Check that timestamp is recent (within last second)
	now := time.Now()
	if now.Sub(log.Timestamp) > time.Second {
		t.Error("Timestamp should be recent")
	}
}

func TestNewLogWithMetadata(t *testing.T) {
	level := "warn"
	message := "Warning message"
	metadata := map[string]string{
		"source": "test",
		"user":   "123",
	}

	log := NewLogWithMetadata(level, message, metadata)

	if log.Level != level {
		t.Errorf("Expected level %s, got %s", level, log.Level)
	}

	if log.Message != message {
		t.Errorf("Expected message %s, got %s", message, log.Message)
	}

	if len(log.Metadata) != len(metadata) {
		t.Errorf("Expected %d metadata entries, got %d", len(metadata), len(log.Metadata))
	}

	for key, expectedValue := range metadata {
		if actualValue, exists := log.Metadata[key]; !exists || actualValue != expectedValue {
			t.Errorf("Expected metadata[%s] = %s, got %s", key, expectedValue, actualValue)
		}
	}
}

func TestLogTimestamp(t *testing.T) {
	log := NewLog("info", "test")

	// Ensure timestamp is set
	if log.Timestamp.IsZero() {
		t.Error("Timestamp should not be zero")
	}

	// Ensure it's recent
	if time.Since(log.Timestamp) > time.Minute {
		t.Error("Timestamp should be recent")
	}
}
