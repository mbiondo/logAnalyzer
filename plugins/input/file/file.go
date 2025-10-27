package fileinput

import (
	"bufio"
	"log"
	"os"
	"strings"
	"sync"

	"github.com/mbiondo/logAnalyzer/core"
)

func init() {
	// Auto-register this plugin
	core.RegisterInputPlugin("file", NewFileInputFromConfig)
}

// Config represents file input configuration
type Config struct {
	Path     string `yaml:"path"`
	Encoding string `yaml:"encoding,omitempty"`
}

// NewFileInputFromConfig creates a file input from configuration map
func NewFileInputFromConfig(config map[string]interface{}) (interface{}, error) {
	var cfg Config
	if err := core.GetPluginConfig(config, &cfg); err != nil {
		return nil, err
	}

	// Set defaults
	if cfg.Encoding == "" {
		cfg.Encoding = "utf-8"
	}

	return NewFileInput(cfg.Path), nil
}

// FileInput reads logs from a file
type FileInput struct {
	filePath string
	file     *os.File
	scanner  *bufio.Scanner
	logCh    chan<- *core.Log
	stopCh   chan struct{}
	wg       sync.WaitGroup
	stopped  bool // Flag to prevent multiple stops
}

// NewFileInput creates a new file input plugin
func NewFileInput(filePath string) *FileInput {
	return &FileInput{
		filePath: filePath,
		stopCh:   make(chan struct{}),
	}
}

// Start begins reading from the file
func (f *FileInput) Start() error {
	file, err := os.Open(f.filePath)
	if err != nil {
		return err
	}
	f.file = file
	f.scanner = bufio.NewScanner(file)

	f.wg.Add(1)
	go f.readLines()
	log.Printf("File input started for: %s", f.filePath)
	return nil
}

// Stop stops reading from the file
func (f *FileInput) Stop() error {
	if f.stopped {
		return nil // Already stopped
	}
	f.stopped = true

	close(f.stopCh)
	f.wg.Wait()
	if f.file != nil {
		return f.file.Close()
	}
	log.Printf("File input stopped for: %s", f.filePath)
	return nil
}

// SetLogChannel sets the channel to send logs to
func (f *FileInput) SetLogChannel(ch chan<- *core.Log) {
	f.logCh = ch
}

// readLines continuously reads lines from the file
func (f *FileInput) readLines() {
	defer f.wg.Done()

	for f.scanner.Scan() {
		select {
		case <-f.stopCh:
			return
		default:
			line := strings.TrimSpace(f.scanner.Text())
			if line != "" {
				logEntry := f.parseLogLine(line, f.filePath)
				select {
				case f.logCh <- logEntry:
				case <-f.stopCh:
					return
				}
			}
		}
	}

	if err := f.scanner.Err(); err != nil {
		log.Printf("Error reading file %s: %v", f.filePath, err)
	}
}

// ParseLogLine parses a log line into a Log struct (public for testing)
func (f *FileInput) ParseLogLine(line string, filePath string) *core.Log {
	return f.parseLogLine(line, filePath)
}

// parseLogLine parses a log line into a Log struct
func (f *FileInput) parseLogLine(line string, filePath string) *core.Log {
	// Skip empty lines
	line = strings.TrimSpace(line)
	if line == "" {
		return nil
	}

	// Simple parsing - in a real implementation, you'd use regex or structured parsing
	// For now, assume format: [LEVEL] message
	level := "info" // default
	message := line

	// Convert to lowercase for case-insensitive matching
	lowerLine := strings.ToLower(line)

	if strings.HasPrefix(lowerLine, "[error]") || strings.HasPrefix(lowerLine, "[err]") {
		level = "error"
		message = strings.TrimPrefix(line, "[ERROR]")
		message = strings.TrimPrefix(message, "[Error]")
		message = strings.TrimPrefix(message, "[error]")
		message = strings.TrimPrefix(message, "[ERR]")
		message = strings.TrimPrefix(message, "[Err]")
		message = strings.TrimPrefix(message, "[err]")
	} else if strings.HasPrefix(lowerLine, "[warn]") || strings.HasPrefix(lowerLine, "[warning]") {
		level = "warn"
		message = strings.TrimPrefix(line, "[WARN]")
		message = strings.TrimPrefix(message, "[Warning]")
		message = strings.TrimPrefix(message, "[warning]")
		message = strings.TrimPrefix(message, "[WARNING]")
	} else if strings.HasPrefix(lowerLine, "[info]") {
		level = "info"
		message = strings.TrimPrefix(line, "[INFO]")
		message = strings.TrimPrefix(message, "[Info]")
		message = strings.TrimPrefix(message, "[info]")
	} else if strings.HasPrefix(lowerLine, "[debug]") {
		level = "debug"
		message = strings.TrimPrefix(line, "[DEBUG]")
		message = strings.TrimPrefix(message, "[Debug]")
		message = strings.TrimPrefix(message, "[debug]")
	}

	message = strings.TrimSpace(message)

	metadata := map[string]string{
		"source": "file",
		"file":   filePath,
	}

	return core.NewLogWithMetadata(level, message, metadata)
}
