package console

import (
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/mbiondo/logAnalyzer/core"
)

func init() {
	// Auto-register this plugin
	core.RegisterOutputPlugin("console", NewConsoleOutputFromConfig)
}

// Config represents console output configuration
type Config struct {
	Target string `yaml:"target,omitempty"` // "stdout" or "stderr"
	Format string `yaml:"format,omitempty"` // "text" or "json"
}

// NewConsoleOutputFromConfig creates a console output from configuration map
func NewConsoleOutputFromConfig(config map[string]interface{}) (interface{}, error) {
	var cfg Config
	if err := core.GetPluginConfig(config, &cfg); err != nil {
		return nil, err
	}

	return NewConsoleOutput(cfg)
}

// ConsoleOutput writes log entries to stdout/stderr
type ConsoleOutput struct {
	config     Config
	writer     io.Writer
	closeMutex sync.Mutex
	closed     bool
}

// NewConsoleOutput creates a new console output plugin
func NewConsoleOutput(config Config) (*ConsoleOutput, error) {
	// Set defaults
	if config.Target == "" {
		config.Target = "stdout"
	}
	if config.Format == "" {
		config.Format = "text"
	}

	// Validate target
	var writer io.Writer
	switch config.Target {
	case "stdout":
		writer = os.Stdout
	case "stderr":
		writer = os.Stderr
	default:
		return nil, fmt.Errorf("invalid target '%s', must be 'stdout' or 'stderr'", config.Target)
	}

	// Validate format
	if config.Format != "text" && config.Format != "json" {
		return nil, fmt.Errorf("invalid format '%s', must be 'text' or 'json'", config.Format)
	}

	return &ConsoleOutput{
		config: config,
		writer: writer,
		closed: false,
	}, nil
}

// NewConsoleOutputWithDefaults creates a console output with default settings
func NewConsoleOutputWithDefaults() (*ConsoleOutput, error) {
	return NewConsoleOutput(Config{})
}

// Write writes a log entry to the console
func (c *ConsoleOutput) Write(log *core.Log) error {
	c.closeMutex.Lock()
	defer c.closeMutex.Unlock()

	if c.closed {
		return fmt.Errorf("console output is closed")
	}

	var output string
	switch c.config.Format {
	case "json":
		// Simple JSON format
		output = fmt.Sprintf(`{"timestamp":"%s","level":"%s","message":"%s"}`+"\n",
			log.Timestamp.Format("2006-01-02T15:04:05Z07:00"),
			log.Level,
			log.Message)
	case "text":
		// Simple text format
		output = fmt.Sprintf("[%s] %s: %s\n",
			log.Timestamp.Format("2006-01-02 15:04:05"),
			log.Level,
			log.Message)
	}

	_, err := c.writer.Write([]byte(output))
	return err
}

// Close closes the console output (no-op for console)
func (c *ConsoleOutput) Close() error {
	c.closeMutex.Lock()
	defer c.closeMutex.Unlock()

	if c.closed {
		return nil
	}

	c.closed = true
	// No actual closing needed for stdout/stderr
	return nil
}
