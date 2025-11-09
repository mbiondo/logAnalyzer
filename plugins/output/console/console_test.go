package console

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/mbiondo/logAnalyzer/core"
)

func TestNewConsoleOutput(t *testing.T) {
	tests := []struct {
		name        string
		config      Config
		expectError bool
	}{
		{
			name:        "default config",
			config:      Config{},
			expectError: false,
		},
		{
			name: "stdout target",
			config: Config{
				Target: "stdout",
				Format: "text",
			},
			expectError: false,
		},
		{
			name: "stderr target",
			config: Config{
				Target: "stderr",
				Format: "json",
			},
			expectError: false,
		},
		{
			name: "invalid target",
			config: Config{
				Target: "invalid",
			},
			expectError: true,
		},
		{
			name: "invalid format",
			config: Config{
				Format: "invalid",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output, err := NewConsoleOutput(tt.config)
			if tt.expectError {
				if err == nil {
					t.Error("expected error but got none")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if output == nil {
				t.Error("expected output but got nil")
			}
		})
	}
}

func TestNewConsoleOutputWithDefaults(t *testing.T) {
	output, err := NewConsoleOutputWithDefaults()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check defaults
	if output.config.Target != "stdout" {
		t.Errorf("expected target 'stdout', got '%s'", output.config.Target)
	}
	if output.config.Format != "text" {
		t.Errorf("expected format 'text', got '%s'", output.config.Format)
	}
}

func TestConsoleOutputWrite(t *testing.T) {
	tests := []struct {
		name     string
		config   Config
		log      *core.Log
		expected string
	}{
		{
			name: "text format stdout",
			config: Config{
				Target: "stdout",
				Format: "text",
			},
			log: &core.Log{
				Timestamp: time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC),
				Level:     "error",
				Message:   "test message",
			},
			expected: "[2023-01-01 12:00:00] error: test message\n",
		},
		{
			name: "json format",
			config: Config{
				Format: "json",
			},
			log: &core.Log{
				Timestamp: time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC),
				Level:     "info",
				Message:   "json test",
			},
			expected: `{"timestamp":"2023-01-01T12:00:00Z","level":"info","message":"json test"}` + "\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			output := &ConsoleOutput{
				config: tt.config,
				writer: &buf,
			}

			err := output.Write(tt.log)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			actual := buf.String()
			if actual != tt.expected {
				t.Errorf("expected output %q, got %q", tt.expected, actual)
			}
		})
	}
}

func TestConsoleOutputClose(t *testing.T) {
	output, err := NewConsoleOutputWithDefaults()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Close should work
	err = output.Close()
	if err != nil {
		t.Errorf("unexpected error on close: %v", err)
	}

	// Second close should also work
	err = output.Close()
	if err != nil {
		t.Errorf("unexpected error on second close: %v", err)
	}

	// Writing after close should fail
	log := &core.Log{
		Timestamp: time.Now(),
		Level:     "info",
		Message:   "test",
	}
	err = output.Write(log)
	if err == nil {
		t.Error("expected error when writing after close")
	}
}

func TestConsoleOutputConcurrency(t *testing.T) {
	var buf bytes.Buffer
	output := &ConsoleOutput{
		config: Config{Format: "text"},
		writer: &buf,
	}

	// Test concurrent writes
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(id int) {
			log := &core.Log{
				Timestamp: time.Now(),
				Level:     "info",
				Message:   "message " + string(rune(id+'0')),
			}
			err := output.Write(log)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	_ = output.Close()

	// Check that we got some output
	outputStr := buf.String()
	if len(outputStr) == 0 {
		t.Error("expected some output but got none")
	}

	// Count lines (should be 10)
	lines := strings.Split(strings.TrimSpace(outputStr), "\n")
	if len(lines) != 10 {
		t.Errorf("expected 10 lines, got %d", len(lines))
	}
}
