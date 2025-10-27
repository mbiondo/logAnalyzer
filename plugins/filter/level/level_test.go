package level

import (
	"testing"

	"github.com/mbiondo/logAnalyzer/core"
)

func TestNewLevelFilter(t *testing.T) {
	levels := []string{"error", "warn", "info"}
	filter := NewLevelFilter(levels)

	if filter.allowedLevels == nil {
		t.Error("allowedLevels map should not be nil")
	}

	if len(filter.allowedLevels) != 3 {
		t.Errorf("Expected 3 allowed levels, got %d", len(filter.allowedLevels))
	}

	for _, level := range levels {
		if !filter.allowedLevels[level] {
			t.Errorf("Level %s should be allowed", level)
		}
	}
}

func TestLevelFilterProcess(t *testing.T) {
	tests := []struct {
		name           string
		allowedLevels  []string
		logLevel       string
		expectedResult bool
	}{
		{
			name:           "allowed level - error",
			allowedLevels:  []string{"error", "warn"},
			logLevel:       "error",
			expectedResult: true,
		},
		{
			name:           "allowed level - warn",
			allowedLevels:  []string{"error", "warn"},
			logLevel:       "warn",
			expectedResult: true,
		},
		{
			name:           "not allowed level - info",
			allowedLevels:  []string{"error", "warn"},
			logLevel:       "info",
			expectedResult: false,
		},
		{
			name:           "not allowed level - debug",
			allowedLevels:  []string{"error", "warn"},
			logLevel:       "debug",
			expectedResult: false,
		},
		{
			name:           "empty allowed levels",
			allowedLevels:  []string{},
			logLevel:       "error",
			expectedResult: false,
		},
		{
			name:           "case insensitive - ERROR",
			allowedLevels:  []string{"error"},
			logLevel:       "ERROR",
			expectedResult: true,
		},
		{
			name:           "case insensitive - Error",
			allowedLevels:  []string{"error"},
			logLevel:       "Error",
			expectedResult: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter := NewLevelFilter(tt.allowedLevels)
			log := core.NewLog(tt.logLevel, "test message")

			result := filter.Process(log)
			if result != tt.expectedResult {
				t.Errorf("Process() = %v, expected %v", result, tt.expectedResult)
			}
		})
	}
}

func TestLevelFilterCaseSensitivity(t *testing.T) {
	filter := NewLevelFilter([]string{"ERROR", "warn"})

	// Test that filter normalizes to lowercase
	log1 := core.NewLog("error", "test") // lowercase
	log2 := core.NewLog("ERROR", "test") // uppercase
	log3 := core.NewLog("Error", "test") // mixed case

	if !filter.Process(log1) {
		t.Error("Lowercase 'error' should be allowed")
	}

	if !filter.Process(log2) {
		t.Error("Uppercase 'ERROR' should be allowed")
	}

	if !filter.Process(log3) {
		t.Error("Mixed case 'Error' should be allowed")
	}
}
