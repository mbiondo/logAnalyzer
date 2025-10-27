package regex

import (
	"testing"

	"github.com/mbiondo/logAnalyzer/core"
)

func TestNewRegexFilter(t *testing.T) {
	filter := NewRegexFilter([]string{"error", "warn"}, "include", "message")
	if filter.mode != "include" {
		t.Errorf("Expected mode 'include', got %s", filter.mode)
	}
	if filter.field != "message" {
		t.Errorf("Expected field 'message', got %s", filter.field)
	}
	if len(filter.patterns) != 2 {
		t.Errorf("Expected 2 patterns, got %d", len(filter.patterns))
	}
}

func TestNewRegexFilterDefaults(t *testing.T) {
	filter := NewRegexFilter([]string{"test"}, "", "")
	if filter.mode != "include" {
		t.Errorf("Expected default mode 'include', got %s", filter.mode)
	}
	if filter.field != "message" {
		t.Errorf("Expected default field 'message', got %s", filter.field)
	}
}

func TestRegexFilterProcessIncludeMode(t *testing.T) {
	filter := NewRegexFilter([]string{"error", "ERROR"}, "include", "message")

	tests := []struct {
		name     string
		log      *core.Log
		expected bool
	}{
		{
			name: "message matches first pattern",
			log: &core.Log{
				Level:   "info",
				Message: "This is an error in the system",
			},
			expected: true,
		},
		{
			name: "message matches second pattern",
			log: &core.Log{
				Level:   "info",
				Message: "ERROR: Database connection failed",
			},
			expected: true,
		},
		{
			name: "message does not match",
			log: &core.Log{
				Level:   "info",
				Message: "This is a normal info message",
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := filter.Process(tt.log)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestRegexFilterProcessExcludeMode(t *testing.T) {
	filter := NewRegexFilter([]string{"debug", "DEBUG"}, "exclude", "message")

	tests := []struct {
		name     string
		log      *core.Log
		expected bool
	}{
		{
			name: "message matches pattern - should be excluded",
			log: &core.Log{
				Level:   "info",
				Message: "This is a debug message",
			},
			expected: false,
		},
		{
			name: "message does not match - should be included",
			log: &core.Log{
				Level:   "info",
				Message: "This is an error message",
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := filter.Process(tt.log)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestRegexFilterProcessLevelField(t *testing.T) {
	filter := NewRegexFilter([]string{"^error$"}, "include", "level")

	tests := []struct {
		name     string
		log      *core.Log
		expected bool
	}{
		{
			name: "level matches",
			log: &core.Log{
				Level:   "error",
				Message: "Some message",
			},
			expected: true,
		},
		{
			name: "level does not match",
			log: &core.Log{
				Level:   "warn",
				Message: "Some message",
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := filter.Process(tt.log)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestRegexFilterProcessAllField(t *testing.T) {
	filter := NewRegexFilter([]string{"error.*failed"}, "include", "all")

	tests := []struct {
		name     string
		log      *core.Log
		expected bool
	}{
		{
			name: "level and message match",
			log: &core.Log{
				Level:   "error",
				Message: "Database connection failed",
			},
			expected: true,
		},
		{
			name: "only level matches",
			log: &core.Log{
				Level:   "error",
				Message: "Success message",
			},
			expected: false,
		},
		{
			name: "only message matches",
			log: &core.Log{
				Level:   "info",
				Message: "Database connection failed",
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := filter.Process(tt.log)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestRegexFilterInvalidPattern(t *testing.T) {
	// Invalid regex pattern should be ignored
	filter := NewRegexFilter([]string{"[invalid"}, "include", "message")

	// Should have 0 patterns since the invalid one was skipped
	if len(filter.patterns) != 0 {
		t.Errorf("Expected 0 patterns for invalid regex, got %d", len(filter.patterns))
	}

	// Should not match anything
	log := &core.Log{
		Level:   "info",
		Message: "test message",
	}
	result := filter.Process(log)
	if result {
		t.Error("Expected false for filter with no valid patterns")
	}
}

func TestRegexFilterMultiplePatterns(t *testing.T) {
	filter := NewRegexFilter([]string{"error", "warn", "fatal"}, "include", "message")

	tests := []struct {
		name     string
		message  string
		expected bool
	}{
		{"matches first pattern", "This is an error", true},
		{"matches second pattern", "This is a warning", true},
		{"matches third pattern", "Fatal error occurred", true},
		{"matches none", "This is info", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			log := &core.Log{
				Level:   "info",
				Message: tt.message,
			}
			result := filter.Process(log)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}
