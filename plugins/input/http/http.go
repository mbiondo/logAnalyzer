package httpinput

import (
	"crypto/subtle"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/mbiondo/logAnalyzer/core"
	"github.com/mbiondo/logAnalyzer/pkg/tlsconfig"
)

const (
	// Default rate limiting values
	DefaultRateLimit = 10.0 // default requests per second
	DefaultBurst     = 20   // default burst size
)

func init() {
	// Auto-register this plugin
	core.RegisterInputPlugin("http", NewHTTPInputFromConfig)
}

// Config represents HTTP input configuration
type Config struct {
	Port     string           `yaml:"port,omitempty"`
	TLS      tlsconfig.Config `yaml:"tls,omitempty"`       // TLS configuration for HTTPS
	CertFile string           `yaml:"cert_file,omitempty"` // Server certificate file (for HTTPS)
	KeyFile  string           `yaml:"key_file,omitempty"`  // Server key file (for HTTPS)

	// Authentication configuration
	Auth AuthConfig `yaml:"auth,omitempty"`

	// Rate limiting configuration
	RateLimit RateLimitConfig `yaml:"rate_limit,omitempty"`
}

// AuthConfig represents authentication configuration for HTTP input
type AuthConfig struct {
	// Basic authentication
	Username string `yaml:"username,omitempty"`
	Password string `yaml:"password,omitempty"`

	// Bearer token authentication
	BearerToken string `yaml:"bearer_token,omitempty"`

	// API key authentication
	APIKey       string `yaml:"api_key,omitempty"`
	APIKeyHeader string `yaml:"api_key_header,omitempty"` // Default: "X-API-Key"

	// Client certificate authentication (mTLS)
	ClientCertRequired bool `yaml:"client_cert_required,omitempty"` // Require client certificates
}

// RateLimitConfig represents rate limiting configuration for HTTP input
type RateLimitConfig struct {
	Enabled bool    `yaml:"enabled,omitempty"` // Whether rate limiting is enabled
	Rate    float64 `yaml:"rate,omitempty"`    // Requests per second
	Burst   int     `yaml:"burst,omitempty"`   // Maximum burst size
}

// Validate validates the authentication configuration
func (a *AuthConfig) Validate() error {
	authMethods := 0
	if a.Username != "" || a.Password != "" {
		authMethods++
		if a.Username == "" || a.Password == "" {
			return fmt.Errorf("both username and password must be provided for basic authentication")
		}
	}
	if a.BearerToken != "" {
		authMethods++
	}
	if a.APIKey != "" {
		authMethods++
	}

	if authMethods > 1 {
		return fmt.Errorf("only one authentication method can be configured at a time")
	}

	return nil
}

// NewHTTPInputFromConfig creates an HTTP input from configuration map
func NewHTTPInputFromConfig(config map[string]any) (any, error) {
	var cfg Config
	if err := core.GetPluginConfig(config, &cfg); err != nil {
		return nil, err
	}

	// Set defaults
	if cfg.Port == "" {
		cfg.Port = "8080"
	}

	// Set default API key header if API key is configured
	if cfg.Auth.APIKey != "" && cfg.Auth.APIKeyHeader == "" {
		cfg.Auth.APIKeyHeader = "X-API-Key"
	}

	// Validate authentication configuration
	if err := cfg.Auth.Validate(); err != nil {
		return nil, fmt.Errorf("invalid auth config: %w", err)
	}

	// Validate rate limit configuration
	if cfg.RateLimit.Enabled {
		if cfg.RateLimit.Rate < 0 {
			return nil, fmt.Errorf("rate limit rate must be non-negative")
		}
		if cfg.RateLimit.Burst < 0 {
			return nil, fmt.Errorf("rate limit burst must be non-negative")
		}
		// If rate or burst are explicitly set to 0, that's a warning-level issue
		// (defaults will be applied in NewHTTPInputWithConfig)
		if cfg.RateLimit.Rate == 0 && cfg.RateLimit.Burst == 0 {
			// Both are 0, using defaults is fine - this is expected behavior
		}
	}

	// Validate TLS config
	if err := cfg.TLS.Validate(); err != nil {
		return nil, err
	}

	return NewHTTPInputWithConfig(cfg), nil
}

// HTTPInput receives logs via HTTP POST requests
type HTTPInput struct {
	port      string
	config    Config
	server    *http.Server
	logCh     chan<- *core.Log
	stopCh    chan struct{}
	wg        sync.WaitGroup
	stopped   bool   // Flag to prevent multiple stops
	name      string // Name of this input instance
	tlsConfig *tls.Config

	// Rate limiter
	rateLimiter *RateLimiter
}

// RateLimiter implements token bucket rate limiting for HTTP requests.
// Uses a token bucket algorithm where tokens refill at a specified rate per second.
// Note: Token refill uses floating-point arithmetic for sub-second precision and may have minor variations.
type RateLimiter struct {
	rate       float64    // tokens per second
	burst      int        // max tokens
	tokens     float64    // current tokens
	lastRefill time.Time  // last refill time
	mu         sync.Mutex // for thread safety
}

// NewRateLimiter creates a new rate limiter with the given rate (tokens/sec) and burst size.
// Returns nil if rate or burst are invalid (rate must be > 0, burst must be > 0).
func NewRateLimiter(rate float64, burst int) *RateLimiter {
	if rate <= 0 || burst <= 0 {
		return nil
	}
	return &RateLimiter{
		rate:       rate,
		burst:      burst,
		tokens:     float64(burst), // start with full burst
		lastRefill: time.Now(),
	}
}

// Allow checks if a request should be allowed based on rate limiting
func (r *RateLimiter) Allow() bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(r.lastRefill).Seconds()

	// Refill tokens based on elapsed time, but only update lastRefill if tokens were actually added
	if elapsed > 0 {
		newTokens := elapsed * r.rate
		if r.tokens < float64(r.burst) {
			r.tokens += newTokens
			if r.tokens > float64(r.burst) {
				// Only advance lastRefill by the time it would take to fill the bucket
				overfill := r.tokens - float64(r.burst)
				tokensAdded := newTokens - overfill
				r.tokens = float64(r.burst)
				r.lastRefill = r.lastRefill.Add(time.Duration(tokensAdded / r.rate * float64(time.Second)))
			} else {
				r.lastRefill = now
			}
		}
		// If tokens are already at burst, do not update lastRefill
	}

	// Check if we have a token
	if r.tokens >= 1.0 {
		r.tokens -= 1.0
		return true
	}

	return false
}

// NewHTTPInput creates a new HTTP input plugin
func NewHTTPInput(port string) *HTTPInput {
	if port == "" {
		port = "8080"
	}

	return &HTTPInput{
		port:   port,
		config: Config{Port: port},
		stopCh: make(chan struct{}),
	}
}

// NewHTTPInputWithConfig creates a new HTTP input plugin with full configuration
func NewHTTPInputWithConfig(config Config) *HTTPInput {
	if config.Port == "" {
		config.Port = "8080"
	}

	// Set default API key header if API key is configured
	if config.Auth.APIKey != "" && config.Auth.APIKeyHeader == "" {
		config.Auth.APIKeyHeader = "X-API-Key"
	}

	input := &HTTPInput{
		port:   config.Port,
		config: config,
		stopCh: make(chan struct{}),
	}

	// Initialize rate limiter if enabled
	if config.RateLimit.Enabled {
		// Use defaults if not specified
		if config.RateLimit.Rate <= 0 {
			config.RateLimit.Rate = DefaultRateLimit
		}
		if config.RateLimit.Burst <= 0 {
			config.RateLimit.Burst = DefaultBurst
		}
		// NewRateLimiter validates and returns nil if invalid
		input.rateLimiter = NewRateLimiter(config.RateLimit.Rate, config.RateLimit.Burst)
		if input.rateLimiter == nil {
			// This shouldn't happen since we set defaults above, but safeguard
			input.rateLimiter = NewRateLimiter(DefaultRateLimit, DefaultBurst)
		}
	}

	return input
}

// Start begins the HTTP server
func (h *HTTPInput) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/logs", h.handleLogs)
	mux.HandleFunc("/health", h.handleHealth)

	h.server = &http.Server{
		Addr:              ":" + h.port,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	// Configure TLS if enabled
	if h.config.TLS.Enabled {
		tlsConfig, err := h.config.TLS.NewTLSConfig()
		if err != nil {
			return err
		}

		// Configure client certificate verification if required
		if h.config.Auth.ClientCertRequired {
			tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
		}

		h.tlsConfig = tlsConfig
		h.server.TLSConfig = tlsConfig
	}

	h.wg.Add(1)
	go func() {
		defer h.wg.Done()

		var err error
		if h.config.TLS.Enabled {
			log.Printf("HTTPS input server starting on port %s (TLS enabled)", h.port)
			// Use provided certificate files or TLS config
			if h.config.CertFile != "" && h.config.KeyFile != "" {
				err = h.server.ListenAndServeTLS(h.config.CertFile, h.config.KeyFile)
			} else {
				err = fmt.Errorf("TLS enabled but certificate files not provided: cert_file and key_file are required")
				log.Printf("Error: %v", err)
				return
			}
		} else {
			log.Printf("HTTP input server starting on port %s", h.port)
			err = h.server.ListenAndServe()
		}

		if err != nil && err != http.ErrServerClosed {
			log.Printf("HTTP server error: %v", err)
		}
	}()

	if h.config.TLS.Enabled {
		log.Printf("HTTPS input started on port %s (TLS)", h.port)
	} else {
		log.Printf("HTTP input started on port %s", h.port)
	}
	return nil
}

// Stop stops the HTTP server
func (h *HTTPInput) Stop() error {
	if h.stopped {
		return nil // Already stopped
	}
	h.stopped = true

	close(h.stopCh)

	if h.server != nil {
		if err := h.server.Close(); err != nil {
			log.Printf("Error closing HTTP server: %v", err)
		}
	}

	h.wg.Wait()
	log.Printf("HTTP input stopped")
	return nil
}

// SetLogChannel sets the channel to send logs to
func (h *HTTPInput) SetLogChannel(ch chan<- *core.Log) {
	h.logCh = ch
}

// SetName sets the name for this input instance
func (h *HTTPInput) SetName(name string) {
	h.name = name
}

// handleLogs handles POST requests with log data
func (h *HTTPInput) handleLogs(w http.ResponseWriter, r *http.Request) {
	// Check authentication
	if err := h.authenticateRequest(r); err != nil {
		http.Error(w, fmt.Sprintf("Authentication failed: %v", err), http.StatusUnauthorized)
		return
	}

	// Check rate limit if enabled
	// rateLimiter is nil if rate limiting is disabled, so the nil check is safe and acts as a feature flag
	if h.rateLimiter != nil && !h.rateLimiter.Allow() {
		http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("Error reading request body: %v", err)
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	defer func() {
		_ = r.Body.Close()
	}()

	contentType := r.Header.Get("Content-Type")

	// Handle different content types
	switch {
	case strings.Contains(contentType, "application/json"):
		h.handleJSONLogs(body)
	case strings.Contains(contentType, "text/plain"):
		h.handlePlainTextLogs(body)
	default:
		// Default to plain text
		h.handlePlainTextLogs(body)
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("OK"))
}

// handleHealth provides a health check endpoint
func (h *HTTPInput) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("OK"))
}

// handleJSONLogs processes JSON log entries
func (h *HTTPInput) handleJSONLogs(data []byte) {
	// Try to parse as a single log entry
	var logEntry map[string]any
	if err := json.Unmarshal(data, &logEntry); err != nil {
		// Try to parse as an array of log entries
		var logEntries []map[string]any
		if err := json.Unmarshal(data, &logEntries); err != nil {
			log.Printf("Error parsing JSON logs: %v", err)
			return
		}

		for _, entry := range logEntries {
			h.processJSONLogEntry(entry)
		}
		return
	}

	h.processJSONLogEntry(logEntry)
}

// processJSONLogEntry processes a single JSON log entry
func (h *HTTPInput) processJSONLogEntry(entry map[string]any) {
	// For JSON logs, pass the raw JSON as the message so filters can parse it
	jsonBytes, err := json.Marshal(entry)
	if err != nil {
		log.Printf("Error marshaling JSON entry: %v", err)
		return
	}

	message := string(jsonBytes)
	level := "info" // default level
	metadata := make(map[string]string)

	metadata["source"] = "http"
	metadata["content_type"] = "json"

	// Try to extract level from the JSON for initial classification
	if l, ok := entry["level"].(string); ok {
		level = strings.ToLower(l)
	}

	logEntry := core.NewLogWithMetadata(level, message, metadata)
	logEntry.Source = h.name // Set the source to the input name

	select {
	case h.logCh <- logEntry:
	case <-h.stopCh:
		return
	}
}

// handlePlainTextLogs processes plain text log entries
func (h *HTTPInput) handlePlainTextLogs(data []byte) {
	lines := strings.Split(string(data), "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		logEntry := h.parseLogLine(line)
		if logEntry != nil {
			select {
			case h.logCh <- logEntry:
			case <-h.stopCh:
				return
			}
		}
	}
}

// ParseLogLine parses a log line into a Log struct (public for testing)
func (h *HTTPInput) ParseLogLine(line string) *core.Log {
	return h.parseLogLine(line)
}

// parseLogLine parses a log line into a Log struct
func (h *HTTPInput) parseLogLine(line string) *core.Log {
	line = strings.TrimSpace(line)
	if line == "" {
		return nil
	}

	// Simple parsing - try to extract level from common patterns
	level := "info"
	message := line

	// Convert to lowercase for case-insensitive matching
	lowerLine := strings.ToLower(line)

	if strings.Contains(lowerLine, "error") || strings.Contains(lowerLine, "err") {
		level = "error"
	} else if strings.Contains(lowerLine, "warn") || strings.Contains(lowerLine, "warning") {
		level = "warn"
	} else if strings.Contains(lowerLine, "debug") {
		level = "debug"
	}

	metadata := map[string]string{
		"source":       "http",
		"content_type": "text",
	}

	logEntry := core.NewLogWithMetadata(level, message, metadata)
	logEntry.Source = h.name // Set the source to the input name
	return logEntry
}

// authenticateRequest authenticates the incoming HTTP request
func (h *HTTPInput) authenticateRequest(r *http.Request) error {
	// If no authentication is configured, allow all requests
	if h.config.Auth.Username == "" && h.config.Auth.Password == "" &&
		h.config.Auth.BearerToken == "" && h.config.Auth.APIKey == "" {
		return nil
	}

	// Check Basic Authentication
	if h.config.Auth.Username != "" && h.config.Auth.Password != "" {
		username, password, ok := r.BasicAuth()
		if !ok {
			return fmt.Errorf("basic authentication required")
		}
		// Use constant-time comparison to prevent timing attacks
		usernameMatch := subtle.ConstantTimeCompare([]byte(username), []byte(h.config.Auth.Username))
		passwordMatch := subtle.ConstantTimeCompare([]byte(password), []byte(h.config.Auth.Password))
		if usernameMatch != 1 || passwordMatch != 1 {
			return fmt.Errorf("invalid credentials")
		}
		return nil
	}

	// Check Bearer Token Authentication
	if h.config.Auth.BearerToken != "" {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			return fmt.Errorf("bearer token required")
		}
		const bearerPrefix = "Bearer "
		if !strings.HasPrefix(authHeader, bearerPrefix) {
			return fmt.Errorf("invalid authorization header format")
		}
		token := strings.TrimPrefix(authHeader, bearerPrefix)
		// Use constant-time comparison to prevent timing attacks
		if subtle.ConstantTimeCompare([]byte(token), []byte(h.config.Auth.BearerToken)) != 1 {
			return fmt.Errorf("invalid bearer token")
		}
		return nil
	}

	// Check API Key Authentication
	if h.config.Auth.APIKey != "" {
		headerName := h.config.Auth.APIKeyHeader
		if headerName == "" {
			headerName = "X-API-Key"
		}
		apiKey := r.Header.Get(headerName)
		if apiKey == "" {
			return fmt.Errorf("API key required in header %s", headerName)
		}
		// Use constant-time comparison to prevent timing attacks
		if subtle.ConstantTimeCompare([]byte(apiKey), []byte(h.config.Auth.APIKey)) != 1 {
			return fmt.Errorf("invalid API key")
		}
		return nil
	}

	return nil
}
