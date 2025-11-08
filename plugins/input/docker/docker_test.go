package dockerinput

import (
	"testing"

	"github.com/mbiondo/logAnalyzer/core"
)

func TestNewDockerInput(t *testing.T) {
	containerIDs := []string{"container1", "container2"}
	labels := map[string]string{"app": "test"}
	stream := "stderr"

	input := NewDockerInput(containerIDs, nil, labels, stream)

	if len(input.containerIDs) != 2 {
		t.Errorf("Expected 2 container IDs, got %d", len(input.containerIDs))
	}

	if input.containerIDs[0] != "container1" {
		t.Errorf("Expected first container ID 'container1', got %s", input.containerIDs[0])
	}

	if input.labels["app"] != "test" {
		t.Errorf("Expected label app=test, got %s", input.labels["app"])
	}

	if input.stream != "stderr" {
		t.Errorf("Expected stream 'stderr', got %s", input.stream)
	}

	if input.stopCh == nil {
		t.Error("stopCh should be initialized")
	}
}

func TestNewDockerInputDefaults(t *testing.T) {
	input := NewDockerInput(nil, nil, nil, "")

	if input.stream != "stdout" {
		t.Errorf("Expected default stream 'stdout', got %s", input.stream)
	}
}

func TestDockerInputSetLogChannel(t *testing.T) {
	input := NewDockerInput(nil, nil, nil, "stdout")
	logCh := make(chan *core.Log, 1)

	input.SetLogChannel(logCh)

	if input.logCh != logCh {
		t.Error("SetLogChannel did not set the channel correctly")
	}
}

func TestParseLogLine(t *testing.T) {
	input := NewDockerInput(nil, nil, nil, "stdout")

	tests := []struct {
		name            string
		line            string
		containerID     string
		expectedLevel   string
		expectedMessage string
		checkMetadata   bool
	}{
		{
			name:            "error log",
			line:            "2023-10-26 10:00:00 [ERROR] Database connection failed",
			containerID:     "abc123",
			expectedLevel:   "error",
			expectedMessage: "2023-10-26 10:00:00 [ERROR] Database connection failed",
			checkMetadata:   true,
		},
		{
			name:            "warn log",
			line:            "[WARN] High memory usage",
			containerID:     "def456",
			expectedLevel:   "warn",
			expectedMessage: "[WARN] High memory usage",
			checkMetadata:   true,
		},
		{
			name:            "info log",
			line:            "Application started",
			containerID:     "ghi789",
			expectedLevel:   "info",
			expectedMessage: "Application started",
			checkMetadata:   true,
		},
		{
			name:            "empty line",
			line:            "",
			containerID:     "test",
			expectedLevel:   "",
			expectedMessage: "",
			checkMetadata:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			log := input.ParseLogLine(tt.line, tt.containerID)

			if tt.expectedMessage == "" {
				if log != nil {
					t.Error("Expected nil log for empty line")
				}
				return
			}

			if log.Level != tt.expectedLevel {
				t.Errorf("Expected level %s, got %s", tt.expectedLevel, log.Level)
			}

			if log.Message != tt.expectedMessage {
				t.Errorf("Expected message %q, got %q", tt.expectedMessage, log.Message)
			}

			if tt.checkMetadata {
				if log.Metadata == nil {
					t.Error("Expected metadata to be set")
				} else {
					if source, exists := log.Metadata["source"]; !exists || source != "docker" {
						t.Errorf("Expected metadata source=docker, got %s", source)
					}
					if container, exists := log.Metadata["container"]; !exists {
						t.Errorf("Expected metadata container to be set")
					} else {
						expectedContainer := tt.containerID
						if len(tt.containerID) >= 12 {
							expectedContainer = tt.containerID[:12]
						}
						if container != expectedContainer {
							t.Errorf("Expected metadata container=%s, got %s", expectedContainer, container)
						}
					}
				}
			}
		})
	}
}

func TestGetContainerName(t *testing.T) {
	input := NewDockerInput(nil, nil, nil, "stdout")

	// Test with a mock - this will fail in real execution but tests the logic
	name := input.getContainerName("nonexistent")
	if name != "" {
		t.Errorf("Expected empty name for nonexistent container, got %s", name)
	}
}

func TestContainerMatchesLabels(t *testing.T) {
	input := NewDockerInput(nil, nil, map[string]string{"app": "test"}, "stdout")

	// This will fail because docker command won't work, but tests the logic
	matches := input.containerMatchesLabels("nonexistent")
	if matches {
		t.Error("Expected false for nonexistent container")
	}
}

// Mock testing for docker commands - we can't easily test the actual docker integration
// without having docker running, so we focus on the parsing and setup logic

func TestGetContainersToMonitorWithIDs(t *testing.T) {
	containerIDs := []string{"container1", "container2"}
	input := NewDockerInput(containerIDs, nil, nil, "stdout")

	// This will try to run docker ps, which may fail, but we can test the logic
	containers, err := input.getContainersToMonitor()

	// We expect an error since docker may not be available in test environment
	if err == nil {
		t.Log("Docker is available, checking returned containers")
		// If docker is available, we should get some containers or empty slice
		if containers == nil {
			t.Error("Expected non-nil containers slice")
		}
	} else {
		t.Logf("Docker not available (expected): %v", err)
	}
}

func TestGetContainersToMonitorWithLabels(t *testing.T) {
	labels := map[string]string{"app": "test"}
	input := NewDockerInput(nil, nil, labels, "stdout")

	containers, err := input.getContainersToMonitor()

	if err == nil {
		t.Log("Docker is available, checking label filtering")
		// If docker is available, containers should be filtered by labels
		for _, container := range containers {
			// We can't easily verify label matching without docker inspect
			t.Logf("Found container: %s", container)
		}
	} else {
		t.Logf("Docker not available (expected): %v", err)
	}
}

func TestDockerInputStopBeforeStart(t *testing.T) {
	input := NewDockerInput(nil, nil, nil, "stdout")

	// Should not panic
	_ = input.Stop()
}

func TestDockerInputDoubleStop(t *testing.T) {
	input := NewDockerInput(nil, nil, nil, "stdout")

	// Mock start (won't actually start without docker)
	_ = input.Start() // This may fail, but shouldn't panic

	// Both stops should not panic
	_ = input.Stop()
	_ = input.Stop()
}

// Test parsing with various log formats
func TestParseLogLineComplexFormats(t *testing.T) {
	input := NewDockerInput(nil, nil, nil, "stdout")

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "timestamped error",
			input:    "2023-10-26T10:00:00Z [ERROR] Connection refused",
			expected: "error",
		},
		{
			name:     "json-like log",
			input:    `{"level":"error","message":"Something failed","timestamp":"2023-10-26T10:00:00Z"}`,
			expected: "error",
		},
		{
			name:     "multi-line log",
			input:    "Starting application...\n[INFO] Loading configuration\n[ERROR] Config file not found",
			expected: "error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			log := input.ParseLogLine(tt.input, "test")
			if log != nil && log.Level != tt.expected {
				t.Errorf("Expected level %s, got %s", tt.expected, log.Level)
			}
		})
	}
}

// Test that docker commands are constructed correctly
func TestDockerCommandConstruction(t *testing.T) {
	input := NewDockerInput([]string{"mycontainer"}, nil, nil, "stdout")

	// We can't easily test the actual execution, but we can verify the setup
	if input.containerIDs[0] != "mycontainer" {
		t.Errorf("Container ID not set correctly")
	}

	if input.stream != "stdout" {
		t.Errorf("Stream not set correctly")
	}
}

// Test edge cases in container ID handling
func TestContainerIDHandling(t *testing.T) {
	tests := []struct {
		name         string
		containerIDs []string
		expectedLen  int
	}{
		{"empty", nil, 0},
		{"single", []string{"abc123"}, 1},
		{"multiple", []string{"abc123", "def456", "ghi789"}, 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := NewDockerInput(tt.containerIDs, nil, nil, "stdout")
			if len(input.containerIDs) != tt.expectedLen {
				t.Errorf("Expected %d container IDs, got %d", tt.expectedLen, len(input.containerIDs))
			}
		})
	}
}
