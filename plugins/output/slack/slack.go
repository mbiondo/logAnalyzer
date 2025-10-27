package slack

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/mbiondo/logAnalyzer/core"
	"net/http"
	"sync"
	"time"
)

func init() {
	// Auto-register this plugin
	core.RegisterOutputPlugin("slack", NewSlackOutputFromConfig)
}

// Config represents slack output configuration
type Config struct {
	WebhookURL string `yaml:"webhook_url"`          // Required: Slack webhook URL
	Username   string `yaml:"username,omitempty"`   // Optional: Username to post as
	Channel    string `yaml:"channel,omitempty"`    // Optional: Channel to post to
	IconEmoji  string `yaml:"icon_emoji,omitempty"` // Optional: Emoji icon
	IconURL    string `yaml:"icon_url,omitempty"`   // Optional: URL icon
	Timeout    int    `yaml:"timeout,omitempty"`    // Optional: HTTP timeout in seconds
}

// NewSlackOutputFromConfig creates a slack output from configuration map
func NewSlackOutputFromConfig(config map[string]interface{}) (interface{}, error) {
	var cfg Config
	if err := core.GetPluginConfig(config, &cfg); err != nil {
		return nil, err
	}

	return NewSlackOutput(cfg)
}

// SlackOutput sends log entries to Slack via webhooks
type SlackOutput struct {
	config     Config
	client     *http.Client
	closeMutex sync.Mutex
	closed     bool
}

// SlackMessage represents a Slack message payload
type SlackMessage struct {
	Text        string            `json:"text,omitempty"`
	Username    string            `json:"username,omitempty"`
	Channel     string            `json:"channel,omitempty"`
	IconEmoji   string            `json:"icon_emoji,omitempty"`
	IconURL     string            `json:"icon_url,omitempty"`
	Attachments []SlackAttachment `json:"attachments,omitempty"`
}

// SlackAttachment represents a Slack message attachment
type SlackAttachment struct {
	Fallback   string       `json:"fallback"`
	Color      string       `json:"color"`
	Pretext    string       `json:"pretext,omitempty"`
	AuthorName string       `json:"author_name,omitempty"`
	Title      string       `json:"title"`
	Text       string       `json:"text"`
	Fields     []SlackField `json:"fields,omitempty"`
	Timestamp  int64        `json:"ts,omitempty"`
}

// SlackField represents a field in a Slack attachment
type SlackField struct {
	Title string `json:"title"`
	Value string `json:"value"`
	Short bool   `json:"short,omitempty"`
}

// NewSlackOutput creates a new Slack output plugin
func NewSlackOutput(config Config) (*SlackOutput, error) {
	if config.WebhookURL == "" {
		return nil, fmt.Errorf("webhook_url is required")
	}

	// Set defaults
	if config.Timeout == 0 {
		config.Timeout = 30
	}

	return &SlackOutput{
		config: config,
		client: &http.Client{
			Timeout: time.Duration(config.Timeout) * time.Second,
		},
		closed: false,
	}, nil
}

// NewSlackOutputWithDefaults creates a Slack output with default settings
func NewSlackOutputWithDefaults() (*SlackOutput, error) {
	return NewSlackOutput(Config{
		WebhookURL: "https://hooks.slack.com/services/YOUR/SLACK/WEBHOOK",
		Username:   "LogAnalyzer",
		Channel:    "#logs",
		IconEmoji:  ":robot_face:",
		Timeout:    30,
	})
}

// Write sends a log entry to Slack
func (s *SlackOutput) Write(log *core.Log) error {
	s.closeMutex.Lock()
	defer s.closeMutex.Unlock()

	if s.closed {
		return fmt.Errorf("slack output is closed")
	}

	// Create Slack message
	message := s.createSlackMessage(log)

	// Marshal to JSON
	jsonData, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal Slack message: %w", err)
	}

	// Send HTTP request
	req, err := http.NewRequest("POST", s.config.WebhookURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send Slack message: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("slack webhook returned status %d", resp.StatusCode)
	}

	return nil
}

// createSlackMessage creates a Slack message from a log entry
func (s *SlackOutput) createSlackMessage(log *core.Log) SlackMessage {
	// Determine color based on log level
	color := s.getColorForLevel(log.Level)

	// Create attachment
	attachment := SlackAttachment{
		Fallback:  fmt.Sprintf("[%s] %s", log.Level, log.Message),
		Color:     color,
		Title:     fmt.Sprintf("Log Entry - %s", log.Level),
		Text:      log.Message,
		Timestamp: log.Timestamp.Unix(),
		Fields: []SlackField{
			{
				Title: "Level",
				Value: log.Level,
				Short: true,
			},
			{
				Title: "Timestamp",
				Value: log.Timestamp.Format("2006-01-02 15:04:05"),
				Short: true,
			},
		},
	}

	message := SlackMessage{
		Attachments: []SlackAttachment{attachment},
	}

	// Set optional fields if configured
	if s.config.Username != "" {
		message.Username = s.config.Username
	}
	if s.config.Channel != "" {
		message.Channel = s.config.Channel
	}
	if s.config.IconEmoji != "" {
		message.IconEmoji = s.config.IconEmoji
	}
	if s.config.IconURL != "" {
		message.IconURL = s.config.IconURL
	}

	return message
}

// getColorForLevel returns a color string based on log level
func (s *SlackOutput) getColorForLevel(level string) string {
	switch level {
	case "error", "ERROR", "err", "ERR":
		return "danger" // red
	case "warn", "warning", "WARN", "WARNING":
		return "warning" // yellow/orange
	case "info", "INFO":
		return "good" // green
	case "debug", "DEBUG":
		return "#808080" // gray
	default:
		return "#808080" // gray for unknown levels
	}
}

// Close closes the Slack output (no-op for HTTP client)
func (s *SlackOutput) Close() error {
	s.closeMutex.Lock()
	defer s.closeMutex.Unlock()

	if s.closed {
		return nil
	}

	s.closed = true
	// No actual closing needed for HTTP client
	return nil
}
