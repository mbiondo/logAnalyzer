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

func TestRateLimiter(t *testing.T) {
	rate := 2.0 // 2 requests per second
	burst := 3
	limiter := NewRateLimiter(rate, burst)

	// Should allow burst number of requests initially
	for i := 0; i < burst; i++ {
		if !limiter.Allow() {
			t.Errorf("Should allow request %d within burst", i+1)
		}
	}

	// Next should be blocked (no time passed)
	if limiter.Allow() {
		t.Error("Should block request after burst is exhausted")
	}

	// Simulate time passing (set lastRefill to past)
	limiter.mu.Lock()
	limiter.lastRefill = time.Now().Add(-1 * time.Second) // 1 second ago
	limiter.mu.Unlock()

	// Should allow requests based on refilled tokens (2.0 * 1 = 2 tokens refilled)
	for i := 0; i < 2; i++ {
		if !limiter.Allow() {
			t.Errorf("Should allow refilled request %d", i+1)
		}
	}

	// Now tokens should be exhausted, next should block
	if limiter.Allow() {
		t.Error("Should block again after consuming all refilled tokens")
	}
}

func TestHTTPInputWithRateLimit(t *testing.T) {
	config := Config{
		Port: "8080",
		RateLimit: RateLimitConfig{
			Enabled: true,
			Rate:    1.0, // 1 request per second
			Burst:   2,   // burst of 2
		},
	}
	input := NewHTTPInputWithConfig(config)

	if input.rateLimiter == nil {
		t.Error("Expected rate limiter to be initialized")
	}

	if input.rateLimiter.rate != 1.0 {
		t.Errorf("Expected rate 1.0, got %f", input.rateLimiter.rate)
	}

	if input.rateLimiter.burst != 2 {
		t.Errorf("Expected burst 2, got %d", input.rateLimiter.burst)
	}
}

func TestHTTPInputWithoutRateLimit(t *testing.T) {
	config := Config{
		Port: "8080",
		RateLimit: RateLimitConfig{
			Enabled: false,
		},
	}
	input := NewHTTPInputWithConfig(config)

	if input.rateLimiter != nil {
		t.Error("Expected rate limiter to be nil when disabled")
	}
}

func TestHTTPInputRateLimitDefaults(t *testing.T) {
	config := Config{
		Port: "8080",
		RateLimit: RateLimitConfig{
			Enabled: true,
			// Rate and Burst not set, should use defaults
		},
	}
	input := NewHTTPInputWithConfig(config)

	if input.rateLimiter == nil {
		t.Error("Expected rate limiter to be initialized")
	}

	if input.rateLimiter.rate != 10.0 {
		t.Errorf("Expected default rate 10.0, got %f", input.rateLimiter.rate)
	}

	if input.rateLimiter.burst != 20 {
		t.Errorf("Expected default burst 20, got %d", input.rateLimiter.burst)
	}
}

func TestHTTPInputRateLimitFromConfig(t *testing.T) {
	tests := []struct {
		name      string
		config    map[string]any
		wantError bool
	}{
		{
			name: "valid rate limit config",
			config: map[string]any{
				"port": "8080",
				"rate_limit": map[string]any{
					"enabled": true,
					"rate":    5.0,
					"burst":   10,
				},
			},
			wantError: false,
		},
		{
			name: "rate limit disabled",
			config: map[string]any{
				"port": "8080",
				"rate_limit": map[string]any{
					"enabled": false,
				},
			},
			wantError: false,
		},
		{
			name: "negative rate",
			config: map[string]any{
				"port": "8080",
				"rate_limit": map[string]any{
					"enabled": true,
					"rate":    -1.0,
					"burst":   10,
				},
			},
			wantError: true,
		},
		{
			name: "negative burst",
			config: map[string]any{
				"port": "8080",
				"rate_limit": map[string]any{
					"enabled": true,
					"rate":    5.0,
					"burst":   -1,
				},
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewHTTPInputFromConfig(tt.config)
			if (err != nil) != tt.wantError {
				t.Errorf("NewHTTPInputFromConfig() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

func TestHTTPInputRateLimitIntegration(t *testing.T) {
	config := Config{
		Port: "8080",
		RateLimit: RateLimitConfig{
			Enabled: true,
			Rate:    0.5, // 0.5 requests per second (need 2 seconds for 1 token)
			Burst:   1,   // burst of 1
		},
	}
	input := NewHTTPInputWithConfig(config)
	logCh := make(chan *core.Log, 10)
	input.SetLogChannel(logCh)

	// Test rate limiting via direct handler calls
	req := httptest.NewRequest("POST", "/logs", bytes.NewReader([]byte("test log")))
	req.Header.Set("Content-Type", "text/plain")

	// First request should be allowed
	w := httptest.NewRecorder()
	input.handleLogs(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("First request: expected status 200, got %d", w.Code)
	}

	// Second request should be rate limited (429)
	w = httptest.NewRecorder()
	input.handleLogs(w, req)
	if w.Code != http.StatusTooManyRequests {
		t.Errorf("Second request: expected status 429, got %d", w.Code)
	}

	// Wait for tokens to refill with generous timeout to handle slow CI/CD environments
	// At 0.5 req/s, we need ~2 seconds for 1 token; adding 1.5s buffer = 3.5s total
	time.Sleep(3500 * time.Millisecond)

	// Third request should be allowed again
	w = httptest.NewRecorder()
	input.handleLogs(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("Third request after refill: expected status 200, got %d", w.Code)
	}
}

func TestAuthConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  AuthConfig
		wantErr bool
	}{
		{
			name:    "no auth configured",
			config:  AuthConfig{},
			wantErr: false,
		},
		{
			name: "basic auth valid",
			config: AuthConfig{
				Username: "user",
				Password: "pass",
			},
			wantErr: false,
		},
		{
			name: "basic auth missing password",
			config: AuthConfig{
				Username: "user",
			},
			wantErr: true,
		},
		{
			name: "basic auth missing username",
			config: AuthConfig{
				Password: "pass",
			},
			wantErr: true,
		},
		{
			name: "bearer token valid",
			config: AuthConfig{
				BearerToken: "token123",
			},
			wantErr: false,
		},
		{
			name: "api key valid",
			config: AuthConfig{
				APIKey: "key123",
			},
			wantErr: false,
		},
		{
			name: "multiple auth methods - basic and bearer",
			config: AuthConfig{
				Username:    "user",
				Password:    "pass",
				BearerToken: "token",
			},
			wantErr: true,
		},
		{
			name: "multiple auth methods - bearer and api key",
			config: AuthConfig{
				BearerToken: "token",
				APIKey:      "key",
			},
			wantErr: true,
		},
		{
			name: "multiple auth methods - all three",
			config: AuthConfig{
				Username:    "user",
				Password:    "pass",
				BearerToken: "token",
				APIKey:      "key",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("AuthConfig.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestHTTPInputAuthenticateRequest(t *testing.T) {
	tests := []struct {
		name           string
		authConfig     AuthConfig
		requestHeaders map[string]string
		wantStatus     int
	}{
		{
			name:       "no auth configured - should pass",
			authConfig: AuthConfig{},
			wantStatus: 200,
		},
		{
			name: "basic auth - valid credentials",
			authConfig: AuthConfig{
				Username: "testuser",
				Password: "testpass",
			},
			requestHeaders: map[string]string{
				"Authorization": "Basic dGVzdHVzZXI6dGVzdHBhc3M=", // base64 of testuser:testpass
			},
			wantStatus: 200,
		},
		{
			name: "basic auth - invalid credentials",
			authConfig: AuthConfig{
				Username: "testuser",
				Password: "testpass",
			},
			requestHeaders: map[string]string{
				"Authorization": "Basic d3Jvbmd1c2VyOndyb25ncGFzcw==", // base64 of wronguser:wrongpass
			},
			wantStatus: 401,
		},
		{
			name: "basic auth - no auth header",
			authConfig: AuthConfig{
				Username: "testuser",
				Password: "testpass",
			},
			wantStatus: 401,
		},
		{
			name: "bearer token - valid token",
			authConfig: AuthConfig{
				BearerToken: "valid-token-123",
			},
			requestHeaders: map[string]string{
				"Authorization": "Bearer valid-token-123",
			},
			wantStatus: 200,
		},
		{
			name: "bearer token - invalid token",
			authConfig: AuthConfig{
				BearerToken: "valid-token-123",
			},
			requestHeaders: map[string]string{
				"Authorization": "Bearer invalid-token",
			},
			wantStatus: 401,
		},
		{
			name: "bearer token - no auth header",
			authConfig: AuthConfig{
				BearerToken: "valid-token-123",
			},
			wantStatus: 401,
		},
		{
			name: "api key - valid key with default header",
			authConfig: AuthConfig{
				APIKey: "valid-api-key",
			},
			requestHeaders: map[string]string{
				"X-API-Key": "valid-api-key",
			},
			wantStatus: 200,
		},
		{
			name: "api key - valid key with custom header",
			authConfig: AuthConfig{
				APIKey:       "valid-api-key",
				APIKeyHeader: "X-Custom-Key",
			},
			requestHeaders: map[string]string{
				"X-Custom-Key": "valid-api-key",
			},
			wantStatus: 200,
		},
		{
			name: "api key - invalid key",
			authConfig: AuthConfig{
				APIKey: "valid-api-key",
			},
			requestHeaders: map[string]string{
				"X-API-Key": "invalid-key",
			},
			wantStatus: 401,
		},
		{
			name: "api key - missing header",
			authConfig: AuthConfig{
				APIKey: "valid-api-key",
			},
			wantStatus: 401,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := Config{
				Port: "8080",
				Auth: tt.authConfig,
			}
			input := NewHTTPInputWithConfig(config)

			// Set up a log channel to prevent blocking
			logCh := make(chan *core.Log, 10)
			input.SetLogChannel(logCh)

			req := httptest.NewRequest("POST", "/logs", bytes.NewReader([]byte("test log")))
			req.Header.Set("Content-Type", "text/plain")
			for key, value := range tt.requestHeaders {
				req.Header.Set(key, value)
			}

			w := httptest.NewRecorder()
			input.handleLogs(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("Expected status %d, got %d", tt.wantStatus, w.Code)
			}
		})
	}
}

func TestHTTPInputFromConfigWithAuth(t *testing.T) {
	tests := []struct {
		name      string
		config    map[string]any
		wantError bool
	}{
		{
			name: "valid basic auth config",
			config: map[string]any{
				"port": "8080",
				"auth": map[string]any{
					"username": "testuser",
					"password": "testpass",
				},
			},
			wantError: false,
		},
		{
			name: "invalid basic auth - missing password",
			config: map[string]any{
				"port": "8080",
				"auth": map[string]any{
					"username": "testuser",
				},
			},
			wantError: true,
		},
		{
			name: "valid bearer token config",
			config: map[string]any{
				"port": "8080",
				"auth": map[string]any{
					"bearer_token": "test-token",
				},
			},
			wantError: false,
		},
		{
			name: "valid api key config",
			config: map[string]any{
				"port": "8080",
				"auth": map[string]any{
					"api_key": "test-key",
				},
			},
			wantError: false,
		},
		{
			name: "invalid - multiple auth methods",
			config: map[string]any{
				"port": "8080",
				"auth": map[string]any{
					"username":     "testuser",
					"password":     "testpass",
					"bearer_token": "test-token",
				},
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewHTTPInputFromConfig(tt.config)
			if (err != nil) != tt.wantError {
				t.Errorf("NewHTTPInputFromConfig() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}
