package httpinput

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mbiondo/logAnalyzer/core"
)

func TestNewHTTPInput(t *testing.T) {
	input := NewHTTPInput("9090")
	if input.port != "9090" {
		t.Errorf("Expected port 9090, got %s", input.port)
	}
	if input.stopped != false {
		t.Error("Expected stopped to be false initially")
	}
}

func TestNewHTTPInputDefaults(t *testing.T) {
	input := NewHTTPInput("")
	if input.port != "8080" {
		t.Errorf("Expected default port 8080, got %s", input.port)
	}
}

func TestHTTPInputSetLogChannel(t *testing.T) {
	input := NewHTTPInput("8080")
	ch := make(chan *core.Log, 1)
	input.SetLogChannel(ch)

	// We can't directly test the private field, but we can verify it doesn't panic
	if input.logCh == nil {
		t.Error("Expected logCh to be set")
	}
}

func TestParseLogLine(t *testing.T) {
	input := NewHTTPInput("8080")

	tests := []struct {
		name     string
		line     string
		expected *core.Log
	}{
		{
			name: "error log",
			line: "This is an error message",
			expected: &core.Log{
				Level:   "error",
				Message: "This is an error message",
				Metadata: map[string]string{
					"source":       "http",
					"content_type": "text",
				},
			},
		},
		{
			name: "warn log",
			line: "This is a warning message",
			expected: &core.Log{
				Level:   "warn",
				Message: "This is a warning message",
				Metadata: map[string]string{
					"source":       "http",
					"content_type": "text",
				},
			},
		},
		{
			name: "info log",
			line: "This is an info message",
			expected: &core.Log{
				Level:   "info",
				Message: "This is an info message",
				Metadata: map[string]string{
					"source":       "http",
					"content_type": "text",
				},
			},
		},
		{
			name: "debug log",
			line: "This is a debug message",
			expected: &core.Log{
				Level:   "debug",
				Message: "This is a debug message",
				Metadata: map[string]string{
					"source":       "http",
					"content_type": "text",
				},
			},
		},
		{
			name:     "empty line",
			line:     "",
			expected: nil,
		},
		{
			name:     "whitespace only",
			line:     "   \t   ",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := input.ParseLogLine(tt.line)
			if tt.expected == nil {
				if result != nil {
					t.Errorf("Expected nil, got %v", result)
				}
				return
			}

			if result == nil {
				t.Errorf("Expected log entry, got nil")
				return
			}

			if result.Level != tt.expected.Level {
				t.Errorf("Expected level %s, got %s", tt.expected.Level, result.Level)
			}
			if result.Message != tt.expected.Message {
				t.Errorf("Expected message %s, got %s", tt.expected.Message, result.Message)
			}

			for k, v := range tt.expected.Metadata {
				if result.Metadata[k] != v {
					t.Errorf("Expected metadata[%s] = %s, got %s", k, v, result.Metadata[k])
				}
			}
		})
	}
}

func TestHTTPInputStopBeforeStart(t *testing.T) {
	input := NewHTTPInput("8080")
	err := input.Stop()
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if !input.stopped {
		t.Error("Expected stopped to be true")
	}
}

func TestHTTPInputDoubleStop(t *testing.T) {
	input := NewHTTPInput("8080")

	// First stop
	err1 := input.Stop()
	if err1 != nil {
		t.Errorf("First stop: expected no error, got %v", err1)
	}

	// Second stop should not panic
	err2 := input.Stop()
	if err2 != nil {
		t.Errorf("Second stop: expected no error, got %v", err2)
	}
}

func TestHandleHealth(t *testing.T) {
	input := NewHTTPInput("8080")

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	input.handleHealth(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	if w.Body.String() != "OK" {
		t.Errorf("Expected body 'OK', got '%s'", w.Body.String())
	}
}

func TestHandleLogsWrongMethod(t *testing.T) {
	input := NewHTTPInput("8080")

	req := httptest.NewRequest("GET", "/logs", nil)
	w := httptest.NewRecorder()

	input.handleLogs(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandlePlainTextLogs(t *testing.T) {
	input := NewHTTPInput("8080")
	logCh := make(chan *core.Log, 10)
	input.SetLogChannel(logCh)

	data := []byte("This is an error message\nThis is a warning message\n")

	input.handlePlainTextLogs(data)

	// Wait a bit for async processing
	time.Sleep(10 * time.Millisecond)

	if len(logCh) != 2 {
		t.Errorf("Expected 2 log entries, got %d", len(logCh))
	}

	log1 := <-logCh
	if log1.Level != "error" {
		t.Errorf("Expected first log level 'error', got '%s'", log1.Level)
	}

	log2 := <-logCh
	if log2.Level != "warn" {
		t.Errorf("Expected second log level 'warn', got '%s'", log2.Level)
	}
}

func TestHandleJSONLogsSingle(t *testing.T) {
	input := NewHTTPInput("8080")
	logCh := make(chan *core.Log, 10)
	input.SetLogChannel(logCh)

	logData := map[string]any{
		"level":     "error",
		"message":   "Test error message",
		"timestamp": "2023-01-01T12:00:00Z",
		"service":   "test-service",
	}

	data, _ := json.Marshal(logData)
	input.handleJSONLogs(data)

	// Wait a bit for async processing
	time.Sleep(10 * time.Millisecond)

	if len(logCh) != 1 {
		t.Errorf("Expected 1 log entry, got %d", len(logCh))
	}

	logEntry := <-logCh
	if logEntry.Level != "error" {
		t.Errorf("Expected level 'error', got '%s'", logEntry.Level)
	}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(logEntry.Message), &parsed); err != nil {
		t.Fatalf("Expected valid JSON message, got error: %v", err)
	}
	if parsed["message"] != "Test error message" {
		t.Errorf("Expected embedded message 'Test error message', got '%v'", parsed["message"])
	}
	if parsed["service"] != "test-service" {
		t.Errorf("Expected embedded service 'test-service', got '%v'", parsed["service"])
	}

	if logEntry.Metadata["content_type"] != "json" {
		t.Errorf("Expected content_type metadata 'json', got '%s'", logEntry.Metadata["content_type"])
	}
}

func TestHandleJSONLogsArray(t *testing.T) {
	input := NewHTTPInput("8080")
	logCh := make(chan *core.Log, 10)
	input.SetLogChannel(logCh)

	logData := []map[string]any{
		{
			"level":   "error",
			"message": "First error",
		},
		{
			"level":   "warn",
			"message": "Second warning",
		},
	}

	data, _ := json.Marshal(logData)
	input.handleJSONLogs(data)

	// Wait a bit for async processing
	time.Sleep(10 * time.Millisecond)

	if len(logCh) != 2 {
		t.Errorf("Expected 2 log entries, got %d", len(logCh))
	}

	log1 := <-logCh
	if log1.Level != "error" {
		t.Errorf("First log: expected level 'error', got %s", log1.Level)
	}
	var first map[string]any
	if err := json.Unmarshal([]byte(log1.Message), &first); err != nil {
		t.Fatalf("First log: invalid JSON message: %v", err)
	}
	if first["message"] != "First error" {
		t.Errorf("First log: expected message 'First error', got '%v'", first["message"])
	}

	log2 := <-logCh
	if log2.Level != "warn" {
		t.Errorf("Second log: expected level 'warn', got %s", log2.Level)
	}
	var second map[string]any
	if err := json.Unmarshal([]byte(log2.Message), &second); err != nil {
		t.Fatalf("Second log: invalid JSON message: %v", err)
	}
	if second["message"] != "Second warning" {
		t.Errorf("Second log: expected message 'Second warning', got '%v'", second["message"])
	}
}

func TestHTTPInputIntegration(t *testing.T) {
	input := NewHTTPInput("0") // Use port 0 for auto-assignment
	logCh := make(chan *core.Log, 10)
	input.SetLogChannel(logCh)

	// Start the input
	err := input.Start()
	if err != nil {
		t.Fatalf("Failed to start HTTP input: %v", err)
	}
	defer func() {
		_ = input.Stop()
	}()

	// Wait a bit for server to start
	time.Sleep(100 * time.Millisecond)

	// Create a test server to get the actual port
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// This is just to get a port, we'll make direct requests
	}))
	defer testServer.Close()

	// For integration testing, we'll test the handlers directly since
	// we can't easily test the actual HTTP server in unit tests
	// In a real scenario, you'd use a test HTTP client

	// Test health endpoint via direct call
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	input.handleHealth(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Health check failed with status %d", w.Code)
	}

	// Test logs endpoint with plain text
	logData := "Test error message"
	req = httptest.NewRequest("POST", "/logs", bytes.NewReader([]byte(logData)))
	req.Header.Set("Content-Type", "text/plain")
	w = httptest.NewRecorder()
	input.handleLogs(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Logs endpoint failed with status %d", w.Code)
	}

	// Wait for processing
	time.Sleep(10 * time.Millisecond)

	if len(logCh) != 1 {
		t.Errorf("Expected 1 log entry, got %d", len(logCh))
	}

	logEntry := <-logCh
	if logEntry.Level != "error" {
		t.Errorf("Expected level 'error', got '%s'", logEntry.Level)
	}
}
