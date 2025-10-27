package file

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mbiondo/logAnalyzer/core"
)

func TestNewFileOutput(t *testing.T) {
	// Test with valid config
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "test.log")

	config := Config{FilePath: filePath}
	output, err := NewFileOutput(config)
	if err != nil {
		t.Fatalf("NewFileOutput failed: %v", err)
	}
	defer output.Close()

	if output.filePath != filePath {
		t.Errorf("Expected filePath %s, got %s", filePath, output.filePath)
	}
}

func TestNewFileOutputDefaults(t *testing.T) {
	// Test with empty config
	config := Config{}
	_, err := NewFileOutput(config)
	if err == nil {
		t.Error("Expected error for empty file path, got nil")
	}
}

func TestFileOutputWrite(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "test.log")

	config := Config{FilePath: filePath}
	output, err := NewFileOutput(config)
	if err != nil {
		t.Fatalf("NewFileOutput failed: %v", err)
	}
	defer output.Close()

	testLog := core.Log{
		Timestamp: time.Now(),
		Level:     "error",
		Message:   "Test error message",
	}

	err = output.Write(&testLog)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Read file and verify content
	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, "error") {
		t.Errorf("Expected log level 'error' in file, got: %s", contentStr)
	}
	if !strings.Contains(contentStr, "Test error message") {
		t.Errorf("Expected message 'Test error message' in file, got: %s", contentStr)
	}
}

func TestFileOutputClose(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "test.log")

	config := Config{FilePath: filePath}
	output, err := NewFileOutput(config)
	if err != nil {
		t.Fatalf("NewFileOutput failed: %v", err)
	}

	err = output.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Verify file exists and has content
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Error("File should exist after close")
	}
}

func TestFileOutputMultipleWrites(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "test.log")

	config := Config{FilePath: filePath}
	output, err := NewFileOutput(config)
	if err != nil {
		t.Fatalf("NewFileOutput failed: %v", err)
	}
	defer output.Close()

	logs := []core.Log{
		{Timestamp: time.Now(), Level: "error", Message: "First error"},
		{Timestamp: time.Now(), Level: "warn", Message: "Second warning"},
		{Timestamp: time.Now(), Level: "info", Message: "Third info"},
	}

	for _, log := range logs {
		err := output.Write(&log)
		if err != nil {
			t.Fatalf("Write failed: %v", err)
		}
	}

	// Read file and verify all logs are present
	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	contentStr := string(content)
	lines := strings.Split(strings.TrimSpace(contentStr), "\n")

	if len(lines) != 3 {
		t.Errorf("Expected 3 lines, got %d", len(lines))
	}

	expectedLevels := []string{"error", "warn", "info"}
	expectedMessages := []string{"First error", "Second warning", "Third info"}

	for i, line := range lines {
		if !strings.Contains(line, expectedLevels[i]) {
			t.Errorf("Line %d should contain level %s, got: %s", i+1, expectedLevels[i], line)
		}
		if !strings.Contains(line, expectedMessages[i]) {
			t.Errorf("Line %d should contain message %s, got: %s", i+1, expectedMessages[i], line)
		}
	}
}

func TestFileOutputConcurrency(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "test.log")

	config := Config{FilePath: filePath}
	output, err := NewFileOutput(config)
	if err != nil {
		t.Fatalf("NewFileOutput failed: %v", err)
	}
	defer output.Close()

	// Write logs concurrently
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(id int) {
			log := core.Log{
				Timestamp: time.Now(),
				Level:     "info",
				Message:   fmt.Sprintf("Concurrent message %d", id),
			}
			err := output.Write(&log)
			if err != nil {
				t.Errorf("Concurrent write failed: %v", err)
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify all messages were written
	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	contentStr := string(content)
	lines := strings.Split(strings.TrimSpace(contentStr), "\n")

	if len(lines) != 10 {
		t.Errorf("Expected 10 lines, got %d", len(lines))
	}
}
