package file

import (
	"bufio"
	"fmt"
	"os"
	"sync"

	"github.com/mbiondo/logAnalyzer/core"
)

func init() {
	// Auto-register this plugin
	core.RegisterOutputPlugin("file", NewFileOutputFromConfig)
}

// Config represents file output configuration
type Config struct {
	FilePath string `yaml:"file_path"`
}

// NewFileOutputFromConfig creates a file output from configuration map
func NewFileOutputFromConfig(config map[string]any) (any, error) {
	var cfg Config
	if err := core.GetPluginConfig(config, &cfg); err != nil {
		return nil, err
	}

	return NewFileOutput(cfg)
}

// FileOutput represents a file output plugin
type FileOutput struct {
	filePath string
	file     *os.File
	writer   *bufio.Writer
	mu       sync.Mutex
}

// NewFileOutput creates a new file output
func NewFileOutput(config Config) (*FileOutput, error) {
	if config.FilePath == "" {
		return nil, fmt.Errorf("file path cannot be empty")
	}

	file, err := os.OpenFile(config.FilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return nil, fmt.Errorf("failed to open file %s: %w", config.FilePath, err)
	}

	writer := bufio.NewWriter(file)

	return &FileOutput{
		filePath: config.FilePath,
		file:     file,
		writer:   writer,
	}, nil
}

// Write writes a log entry to the file
func (f *FileOutput) Write(log *core.Log) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Format log entry
	line := fmt.Sprintf("[%s] %s: %s\n", log.Timestamp.Format("2006-01-02 15:04:05"), log.Level, log.Message)

	// Write to file
	if _, err := f.writer.WriteString(line); err != nil {
		return fmt.Errorf("failed to write to file: %w", err)
	}

	// Flush to ensure data is written
	if err := f.writer.Flush(); err != nil {
		return fmt.Errorf("failed to flush file: %w", err)
	}

	return nil
}

// Close closes the file output
func (f *FileOutput) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.writer != nil {
		if err := f.writer.Flush(); err != nil {
			_ = f.file.Close()
			return fmt.Errorf("failed to flush writer: %w", err)
		}
	}

	if f.file != nil {
		return f.file.Close()
	}

	return nil
}
