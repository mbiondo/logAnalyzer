package httpinput

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"

	"github.com/mbiondo/logAnalyzer/core"
)

func init() {
	// Auto-register this plugin
	core.RegisterInputPlugin("http", NewHTTPInputFromConfig)
}

// Config represents HTTP input configuration
type Config struct {
	Port string `yaml:"port,omitempty"`
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

	return NewHTTPInput(cfg.Port), nil
}

// HTTPInput receives logs via HTTP POST requests
type HTTPInput struct {
	port    string
	server  *http.Server
	logCh   chan<- *core.Log
	stopCh  chan struct{}
	wg      sync.WaitGroup
	stopped bool // Flag to prevent multiple stops
	name    string // Name of this input instance
}

// NewHTTPInput creates a new HTTP input plugin
func NewHTTPInput(port string) *HTTPInput {
	if port == "" {
		port = "8080"
	}

	return &HTTPInput{
		port:   port,
		stopCh: make(chan struct{}),
	}
}

// Start begins the HTTP server
func (h *HTTPInput) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/logs", h.handleLogs)
	mux.HandleFunc("/health", h.handleHealth)

	h.server = &http.Server{
		Addr:    ":" + h.port,
		Handler: mux,
	}

	h.wg.Add(1)
	go func() {
		defer h.wg.Done()
		log.Printf("HTTP input server starting on port %s", h.port)
		if err := h.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("HTTP server error: %v", err)
		}
	}()

	log.Printf("HTTP input started on port %s", h.port)
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
	level := "info" // default
	message := ""
	timestamp := ""
	metadata := make(map[string]string)

	metadata["source"] = "http"
	metadata["content_type"] = "json"

	// Extract common fields
	if l, ok := entry["level"].(string); ok {
		level = strings.ToLower(l)
	}
	if m, ok := entry["message"].(string); ok {
		message = m
	}
	if t, ok := entry["timestamp"].(string); ok {
		timestamp = t
	}
	if ts, ok := entry["time"].(string); ok {
		timestamp = ts
	}

	// Add all other fields as metadata
	for k, v := range entry {
		if k != "level" && k != "message" && k != "timestamp" && k != "time" {
			if str, ok := v.(string); ok {
				metadata[k] = str
			} else {
				// Convert to JSON string for complex types
				if jsonBytes, err := json.Marshal(v); err == nil {
					metadata[k] = string(jsonBytes)
				}
			}
		}
	}

	if message == "" {
		// If no message field, use the entire JSON as message
		if jsonBytes, err := json.Marshal(entry); err == nil {
			message = string(jsonBytes)
		}
	}

	logEntry := core.NewLogWithMetadata(level, message, metadata)
	if timestamp != "" {
		// Note: In a real implementation, you'd parse and set the timestamp
		// For now, we'll just add it to metadata
		logEntry.Metadata["timestamp"] = timestamp
	}

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
