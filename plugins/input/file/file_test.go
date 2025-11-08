package fileinput

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mbiondo/logAnalyzer/core"
)

func TestNewFileInput(t *testing.T) {
	filePath := "/tmp/test.log"
	input := NewFileInput(filePath)

	if input.filePath != filePath {
		t.Errorf("Expected filePath %s, got %s", filePath, input.filePath)
	}

	if input.logCh != nil {
		t.Error("logCh should be nil initially")
	}

	if input.stopCh == nil {
		t.Error("stopCh should be initialized")
	}
}

func TestFileInputSetLogChannel(t *testing.T) {
	input := NewFileInput("test.log")
	logCh := make(chan *core.Log, 1)

	input.SetLogChannel(logCh)

	if input.logCh != logCh {
		t.Error("SetLogChannel did not set the channel correctly")
	}
}

func TestParseLogLine(t *testing.T) {
	input := NewFileInput("test.log")

	tests := []struct {
		name            string
		line            string
		expectedLevel   string
		expectedMessage string
		hasMetadata     bool
	}{
		{
			name:            "error log",
			line:            "[ERROR] Database connection failed",
			expectedLevel:   "error",
			expectedMessage: "Database connection failed",
			hasMetadata:     true,
		},
		{
			name:            "warn log",
			line:            "[WARN] High memory usage",
			expectedLevel:   "warn",
			expectedMessage: "High memory usage",
			hasMetadata:     true,
		},
		{
			name:            "warning log alternative",
			line:            "[WARNING] Deprecated function used",
			expectedLevel:   "warn",
			expectedMessage: "Deprecated function used",
			hasMetadata:     true,
		},
		{
			name:            "info log",
			line:            "[INFO] Application started",
			expectedLevel:   "info",
			expectedMessage: "Application started",
			hasMetadata:     true,
		},
		{
			name:            "debug log",
			line:            "[DEBUG] Processing request",
			expectedLevel:   "debug",
			expectedMessage: "Processing request",
			hasMetadata:     true,
		},
		{
			name:            "plain log without brackets",
			line:            "Plain log message",
			expectedLevel:   "info",
			expectedMessage: "Plain log message",
			hasMetadata:     true,
		},
		{
			name:            "empty line",
			line:            "",
			expectedLevel:   "",
			expectedMessage: "",
			hasMetadata:     false,
		},
		{
			name:            "whitespace only",
			line:            "   \t   ",
			expectedLevel:   "",
			expectedMessage: "",
			hasMetadata:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			log := input.ParseLogLine(tt.line, "test.log")

			if tt.expectedMessage == "" {
				if log != nil {
					t.Error("Expected nil log for empty/whitespace line")
				}
				return
			}

			if log.Level != tt.expectedLevel {
				t.Errorf("Expected level %s, got %s", tt.expectedLevel, log.Level)
			}

			if log.Message != tt.expectedMessage {
				t.Errorf("Expected message %s, got %s", tt.expectedMessage, log.Message)
			}

			if tt.hasMetadata {
				if log.Metadata == nil {
					t.Error("Expected metadata to be set")
				} else {
					if source, exists := log.Metadata["source"]; !exists || source != "file" {
						t.Errorf("Expected metadata source=file, got %s", source)
					}
					if file, exists := log.Metadata["file"]; !exists || file != "test.log" {
						t.Errorf("Expected metadata file=test.log, got %s", file)
					}
				}
			}
		})
	}
}

func TestFileInputIntegration(t *testing.T) {
	// Create a temporary file with test content
	tempDir := t.TempDir()
	tempFile := filepath.Join(tempDir, "test.log")

	content := `[ERROR] Database connection failed
[WARN] High memory usage detected
[INFO] Application started successfully
[DEBUG] Processing user request
`

	err := os.WriteFile(tempFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	// Create input plugin
	input := NewFileInput(tempFile)
	logCh := make(chan *core.Log, 10)
	input.SetLogChannel(logCh)

	// Start the input
	err = input.Start()
	if err != nil {
		t.Fatalf("Failed to start file input: %v", err)
	}

	// Wait a bit for processing
	time.Sleep(100 * time.Millisecond)

	// Stop the input
	_ = input.Stop()

	// Collect received logs
	var receivedLogs []*core.Log
	timeout := time.After(1 * time.Second)

	for {
		select {
		case log := <-logCh:
			receivedLogs = append(receivedLogs, log)
		case <-timeout:
			goto done
		}
	}

done:
	// Verify we received the expected logs
	expectedLevels := []string{"error", "warn", "info", "debug"}
	if len(receivedLogs) != len(expectedLevels) {
		t.Errorf("Expected %d logs, got %d", len(expectedLevels), len(receivedLogs))
		return
	}

	for i, log := range receivedLogs {
		if log.Level != expectedLevels[i] {
			t.Errorf("Log %d: expected level %s, got %s", i, expectedLevels[i], log.Level)
		}
	}
}

func TestFileInputNonexistentFile(t *testing.T) {
	input := NewFileInput("/nonexistent/file.log")
	logCh := make(chan *core.Log, 1)
	input.SetLogChannel(logCh)

	err := input.Start()
	if err == nil {
		t.Error("Expected error when starting with nonexistent file")
		_ = input.Stop()
	}
}

func TestFileInputStopBeforeStart(t *testing.T) {
	input := NewFileInput("test.log")

	// Should not panic
	_ = input.Stop()
}

func TestFileInputDoubleStop(t *testing.T) {
	tempDir := t.TempDir()
	tempFile := filepath.Join(tempDir, "test.log")

	err := os.WriteFile(tempFile, []byte("test log"), 0644)
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	input := NewFileInput(tempFile)
	logCh := make(chan *core.Log, 1)
	input.SetLogChannel(logCh)

	err = input.Start()
	if err != nil {
		t.Fatalf("Failed to start: %v", err)
	}

	// First stop should work
	_ = input.Stop()

	// Second stop should not panic
	_ = input.Stop()
}

func TestParseLogLineCaseInsensitive(t *testing.T) {
	input := NewFileInput("test.log")

	tests := []struct {
		input    string
		expected string
	}{
		{"[ERROR] test", "error"},
		{"[Error] test", "error"},
		{"[error] test", "error"},
		{"[WARN] test", "warn"},
		{"[Warning] test", "warn"},
		{"[WARNING] test", "warn"},
		{"[INFO] test", "info"},
		{"[DEBUG] test", "debug"},
	}

	for _, tt := range tests {
		log := input.ParseLogLine(tt.input, "test.log")
		if log.Level != tt.expected {
			t.Errorf("Input %s: expected level %s, got %s", tt.input, tt.expected, log.Level)
		}
	}
}

func TestParseLogLineComplexMessages(t *testing.T) {
	input := NewFileInput("test.log")

	tests := []struct {
		input    string
		expected string
	}{
		{"[ERROR] Failed to connect to database: connection timeout", "Failed to connect to database: connection timeout"},
		{"[WARN] Memory usage at 85% for container web-123", "Memory usage at 85% for container web-123"},
		{"[INFO] User authentication successful for user@example.com", "User authentication successful for user@example.com"},
	}

	for _, tt := range tests {
		log := input.ParseLogLine(tt.input, "test.log")
		if log.Message != tt.expected {
			t.Errorf("Expected message %q, got %q", tt.expected, log.Message)
		}
	}
}
