package dockerinput

import (
	"bufio"
	"log"
	"os/exec"
	"strings"
	"sync"

	"github.com/mbiondo/logAnalyzer/core"
)

func init() {
	// Auto-register this plugin
	core.RegisterInputPlugin("docker", NewDockerInputFromConfig)
}

// ContainerFilterValue can be either a string or []string
type ContainerFilterValue any

// Config represents docker input configuration
type Config struct {
	ContainerIDs    []string             `yaml:"container_ids,omitempty"`
	ContainerFilter ContainerFilterValue `yaml:"container_filter,omitempty"` // Filter by name pattern (string or []string)
	Labels          map[string]string    `yaml:"labels,omitempty"`
	Stream          string               `yaml:"stream,omitempty"` // "stdout", "stderr", or "both"
}

// NewDockerInputFromConfig creates a docker input from configuration map
func NewDockerInputFromConfig(config map[string]any) (any, error) {
	var cfg Config
	if err := core.GetPluginConfig(config, &cfg); err != nil {
		return nil, err
	}

	// Set defaults
	if cfg.Stream == "" {
		cfg.Stream = "stdout"
	}

	// Convert ContainerFilter to []string
	var containerFilters []string
	if cfg.ContainerFilter != nil {
		switch v := cfg.ContainerFilter.(type) {
		case string:
			containerFilters = []string{v}
		case []any:
			for _, item := range v {
				if str, ok := item.(string); ok {
					containerFilters = append(containerFilters, str)
				}
			}
		case []string:
			containerFilters = v
		}
	}

	return NewDockerInput(cfg.ContainerIDs, containerFilters, cfg.Labels, cfg.Stream), nil
}

// DockerInput reads logs from Docker containers using docker logs command
type DockerInput struct {
	name             string // Name for this input instance
	containerIDs     []string
	containerFilters []string // Filter by name patterns (multiple patterns supported)
	labels           map[string]string
	stream           string // "stdout", "stderr", or "both"
	logCh            chan<- *core.Log
	stopCh           chan struct{}
	wg               sync.WaitGroup
	stopped          bool
}

// NewDockerInput creates a new Docker input plugin
func NewDockerInput(containerIDs []string, containerFilters []string, labels map[string]string, stream string) *DockerInput {
	if stream == "" {
		stream = "stdout"
	}

	return &DockerInput{
		name:             "docker",
		containerIDs:     containerIDs,
		containerFilters: containerFilters,
		labels:           labels,
		stream:           stream,
		stopCh:           make(chan struct{}),
	}
}

// SetName sets the name for this input instance
func (d *DockerInput) SetName(name string) {
	d.name = name
}

// Start begins reading from Docker containers
func (d *DockerInput) Start() error {
	// Get containers to monitor
	containers, err := d.getContainersToMonitor()
	if err != nil {
		return err
	}

	if len(containers) == 0 {
		log.Printf("Docker input: No containers found to monitor")
		return nil
	}

	log.Printf("Docker input started, monitoring %d containers", len(containers))

	// Start monitoring each container
	for _, container := range containers {
		d.wg.Add(1)
		go d.monitorContainer(container)
	}

	return nil
}

// Stop stops reading from Docker containers
func (d *DockerInput) Stop() error {
	if d.stopped {
		return nil
	}
	d.stopped = true
	close(d.stopCh)
	d.wg.Wait()
	log.Printf("Docker input stopped")
	return nil
}

// SetLogChannel sets the channel to send logs to
func (d *DockerInput) SetLogChannel(ch chan<- *core.Log) {
	d.logCh = ch
}

// getContainersToMonitor gets container IDs to monitor
func (d *DockerInput) getContainersToMonitor() ([]string, error) {
	var containers []string

	if len(d.containerIDs) > 0 {
		// Use specified container IDs
		containers = d.containerIDs
	} else if len(d.containerFilters) > 0 {
		// Get containers matching any of the name filters
		containerMap := make(map[string]bool) // Use map to avoid duplicates

		for _, filter := range d.containerFilters {
			cmd := exec.Command("docker", "ps", "--filter", "name="+filter, "--format", "{{.ID}}")
			output, err := cmd.Output()
			if err != nil {
				return nil, err
			}

			lines := strings.Split(strings.TrimSpace(string(output)), "\n")
			for _, line := range lines {
				if line != "" {
					containerID := strings.TrimSpace(line)
					containerMap[containerID] = true
				}
			}
		}

		// Convert map to slice
		for containerID := range containerMap {
			containers = append(containers, containerID)
		}
	} else {
		// Get all running containers
		cmd := exec.Command("docker", "ps", "--format", "{{.ID}}")
		output, err := cmd.Output()
		if err != nil {
			return nil, err
		}

		lines := strings.Split(strings.TrimSpace(string(output)), "\n")
		for _, line := range lines {
			if line != "" {
				containers = append(containers, strings.TrimSpace(line))
			}
		}
	}

	// Filter by labels if specified
	if len(d.labels) > 0 {
		filtered := []string{}
		for _, containerID := range containers {
			if d.containerMatchesLabels(containerID) {
				filtered = append(filtered, containerID)
			}
		}
		containers = filtered
	}

	return containers, nil
}

// containerMatchesLabels checks if a container matches the specified labels
func (d *DockerInput) containerMatchesLabels(containerID string) bool {
	cmd := exec.Command("docker", "inspect", "--format", "{{json .Config.Labels}}", containerID)
	output, err := cmd.Output()
	if err != nil {
		log.Printf("Error inspecting container %s: %v", containerID, err)
		return false
	}

	// Simple check - in a real implementation, you'd parse the JSON properly
	labelsStr := strings.TrimSpace(string(output))
	for key, value := range d.labels {
		expected := `"` + key + `":"` + value + `"`
		if !strings.Contains(labelsStr, expected) {
			return false
		}
	}

	return true
}

// monitorContainer monitors logs from a specific container
func (d *DockerInput) monitorContainer(containerID string) {
	defer d.wg.Done()

	// Build docker logs command
	args := []string{"logs", "-f"} // -f for follow

	// Add stream filter
	switch d.stream {
	case "stdout":
		args = append(args, "--tail", "0") // Start from end
	case "stderr":
		args = append(args, "--tail", "0")
	case "both":
		args = append(args, "--tail", "0")
	}

	args = append(args, containerID)

	cmd := exec.Command("docker", args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Printf("Error creating stdout pipe for container %s: %v", containerID, err)
		return
	}

	if err := cmd.Start(); err != nil {
		log.Printf("Error starting docker logs for container %s: %v", containerID, err)
		return
	}

	scanner := bufio.NewScanner(stdout)
	for {
		select {
		case <-d.stopCh:
			_ = cmd.Process.Kill()
			return
		default:
			if scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())
				if line != "" {
					logEntry := d.ParseLogLine(line, containerID)
					select {
					case d.logCh <- logEntry:
					case <-d.stopCh:
						_ = cmd.Process.Kill()
						return
					}
				}
			} else {
				// Command finished or error occurred
				if err := scanner.Err(); err != nil {
					log.Printf("Error reading logs from container %s: %v", containerID, err)
				}
				return
			}
		}
	}
}

// ParseLogLine parses a log line into a Log struct (public for testing)
func (d *DockerInput) ParseLogLine(line string, containerID string) *core.Log {
	line = strings.TrimSpace(line)
	if line == "" {
		return nil
	}

	// Simple parsing - try to extract level from common patterns
	level := "info"
	message := line

	// Look for common log level patterns
	lowerLine := strings.ToLower(line)
	if strings.Contains(lowerLine, "error") || strings.Contains(lowerLine, "err") {
		level = "error"
	} else if strings.Contains(lowerLine, "warn") || strings.Contains(lowerLine, "warning") {
		level = "warn"
	} else if strings.Contains(lowerLine, "debug") {
		level = "debug"
	}

	metadata := map[string]string{
		"source": "docker",
	}

	// Add container ID (short version if available)
	if len(containerID) >= 12 {
		metadata["container"] = containerID[:12] // Short ID
	} else {
		metadata["container"] = containerID // Full ID if shorter than 12 chars
	}

	// Try to get container name
	if name := d.getContainerName(containerID); name != "" {
		metadata["name"] = name
	}

	logEntry := core.NewLogWithMetadata(level, message, metadata)
	logEntry.Source = d.name // Set the source to the input name
	return logEntry
}

// getContainerName gets the name of a container
func (d *DockerInput) getContainerName(containerID string) string {
	cmd := exec.Command("docker", "inspect", "--format", "{{.Name}}", containerID)
	output, err := cmd.Output()
	if err != nil {
		return ""
	}

	name := strings.TrimSpace(string(output))
	return strings.TrimPrefix(name, "/") // Remove leading slash
}
