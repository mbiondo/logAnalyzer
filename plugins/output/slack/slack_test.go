package slack

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mbiondo/logAnalyzer/core"
)

func TestNewSlackOutput(t *testing.T) {
	tests := []struct {
		name        string
		config      Config
		expectError bool
	}{
		{
			name: "valid config",
			config: Config{
				WebhookURL: "https://hooks.slack.com/services/xxx",
				Username:   "LogBot",
				Channel:    "#logs",
				Timeout:    10,
			},
			expectError: false,
		},
		{
			name:        "missing webhook URL",
			config:      Config{},
			expectError: true,
		},
		{
			name: "default timeout",
			config: Config{
				WebhookURL: "https://hooks.slack.com/services/xxx",
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output, err := NewSlackOutput(tt.config)
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

func TestNewSlackOutputWithDefaults(t *testing.T) {
	output, err := NewSlackOutputWithDefaults()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output == nil {
		t.Fatal("expected output but got nil")
	}
}

func TestSlackOutputWrite(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST request, got %s", r.Method)
		}

		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected Content-Type application/json, got %s", r.Header.Get("Content-Type"))
		}

		var message SlackMessage
		if err := json.NewDecoder(r.Body).Decode(&message); err != nil {
			t.Errorf("failed to decode message: %v", err)
			return
		}

		// Verify message structure
		if len(message.Attachments) != 1 {
			t.Errorf("expected 1 attachment, got %d", len(message.Attachments))
			return
		}

		attachment := message.Attachments[0]
		if attachment.Title != "Log Entry - error" {
			t.Errorf("expected title 'Log Entry - error', got '%s'", attachment.Title)
		}

		if attachment.Color != "danger" {
			t.Errorf("expected color 'danger', got '%s'", attachment.Color)
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Create Slack output
	config := Config{
		WebhookURL: server.URL,
		Username:   "TestBot",
		Channel:    "#test",
		IconEmoji:  ":robot:",
	}

	output, err := NewSlackOutput(config)
	if err != nil {
		t.Fatalf("failed to create Slack output: %v", err)
	}

	// Create test log
	log := &core.Log{
		Timestamp: time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC),
		Level:     "error",
		Message:   "Test error message",
	}

	// Write log
	err = output.Write(log)
	if err != nil {
		t.Errorf("unexpected error writing log: %v", err)
	}
}

func TestSlackOutputWriteHTTPError(t *testing.T) {
	// Create a test server that returns an error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	config := Config{
		WebhookURL: server.URL,
	}

	output, err := NewSlackOutput(config)
	if err != nil {
		t.Fatalf("failed to create Slack output: %v", err)
	}

	log := &core.Log{
		Timestamp: time.Now(),
		Level:     "info",
		Message:   "test message",
	}

	err = output.Write(log)
	if err == nil {
		t.Error("expected error due to HTTP 500 response")
	}
}

func TestSlackOutputClose(t *testing.T) {
	config := Config{
		WebhookURL: "https://hooks.slack.com/services/xxx",
	}

	output, err := NewSlackOutput(config)
	if err != nil {
		t.Fatalf("failed to create Slack output: %v", err)
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

func TestGetColorForLevel(t *testing.T) {
	config := Config{
		WebhookURL: "https://hooks.slack.com/services/xxx",
	}

	output, err := NewSlackOutput(config)
	if err != nil {
		t.Fatalf("failed to create Slack output: %v", err)
	}

	tests := []struct {
		level string
		color string
	}{
		{"error", "danger"},
		{"ERROR", "danger"},
		{"warn", "warning"},
		{"WARNING", "warning"},
		{"info", "good"},
		{"debug", "#808080"},
		{"unknown", "#808080"},
	}

	for _, tt := range tests {
		t.Run(tt.level, func(t *testing.T) {
			color := output.getColorForLevel(tt.level)
			if color != tt.color {
				t.Errorf("expected color %s for level %s, got %s", tt.color, tt.level, color)
			}
		})
	}
}

func TestCreateSlackMessage(t *testing.T) {
	config := Config{
		WebhookURL: "https://hooks.slack.com/services/xxx",
		Username:   "TestBot",
		Channel:    "#test",
		IconEmoji:  ":robot:",
	}

	output, err := NewSlackOutput(config)
	if err != nil {
		t.Fatalf("failed to create Slack output: %v", err)
	}

	log := &core.Log{
		Timestamp: time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC),
		Level:     "error",
		Message:   "Test message",
	}

	message := output.createSlackMessage(log)

	// Verify message structure
	if message.Username != "TestBot" {
		t.Errorf("expected username 'TestBot', got '%s'", message.Username)
	}
	if message.Channel != "#test" {
		t.Errorf("expected channel '#test', got '%s'", message.Channel)
	}
	if message.IconEmoji != ":robot:" {
		t.Errorf("expected icon emoji ':robot:', got '%s'", message.IconEmoji)
	}

	if len(message.Attachments) != 1 {
		t.Fatalf("expected 1 attachment, got %d", len(message.Attachments))
	}

	attachment := message.Attachments[0]
	if attachment.Color != "danger" {
		t.Errorf("expected color 'danger', got '%s'", attachment.Color)
	}
	if attachment.Title != "Log Entry - error" {
		t.Errorf("expected title 'Log Entry - error', got '%s'", attachment.Title)
	}
	if attachment.Text != "Test message" {
		t.Errorf("expected text 'Test message', got '%s'", attachment.Text)
	}
	if len(attachment.Fields) != 2 {
		t.Errorf("expected 2 fields, got %d", len(attachment.Fields))
	}
}

func TestSlackOutputConcurrency(t *testing.T) {
	// Create a test server that counts requests
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	config := Config{
		WebhookURL: server.URL,
	}

	output, err := NewSlackOutput(config)
	if err != nil {
		t.Fatalf("failed to create Slack output: %v", err)
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

	output.Close()

	// Verify all requests were sent
	if requestCount != 10 {
		t.Errorf("expected 10 requests, got %d", requestCount)
	}
}
