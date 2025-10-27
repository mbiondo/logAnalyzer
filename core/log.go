package core

import (
	"time"
)

// Log represents a standardized log entry
type Log struct {
	Timestamp time.Time         `json:"timestamp"`
	Level     string            `json:"level"`
	Message   string            `json:"message"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	Source    string            `json:"source,omitempty"` // Input plugin identifier
}

// NewLog creates a new Log entry
func NewLog(level, message string) *Log {
	return &Log{
		Timestamp: time.Now(),
		Level:     level,
		Message:   message,
		Metadata:  make(map[string]string),
	}
}

// NewLogWithMetadata creates a new Log entry with metadata
func NewLogWithMetadata(level, message string, metadata map[string]string) *Log {
	log := NewLog(level, message)
	log.Metadata = metadata
	return log
}
