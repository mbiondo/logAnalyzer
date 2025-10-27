package regex

import (
	"regexp"

	"github.com/mbiondo/logAnalyzer/core"
)

func init() {
	// Auto-register this plugin
	core.RegisterFilterPlugin("regex", NewRegexFilterFromConfig)
}

// Config represents regex filter configuration
type Config struct {
	Patterns []string `yaml:"patterns"`
	Mode     string   `yaml:"mode,omitempty"`  // "include" or "exclude"
	Field    string   `yaml:"field,omitempty"` // "message", "level", or "all"
}

// NewRegexFilterFromConfig creates a regex filter from configuration map
func NewRegexFilterFromConfig(config map[string]any) (any, error) {
	var cfg Config
	if err := core.GetPluginConfig(config, &cfg); err != nil {
		return nil, err
	}

	// Set defaults
	if cfg.Mode == "" {
		cfg.Mode = "include"
	}
	if cfg.Field == "" {
		cfg.Field = "message"
	}

	return NewRegexFilter(cfg.Patterns, cfg.Mode, cfg.Field), nil
}

// RegexFilter filters logs based on regular expressions
type RegexFilter struct {
	patterns []*regexp.Regexp
	mode     string // "include" or "exclude"
	field    string // "message", "level", or "all"
}

// NewRegexFilter creates a new regex filter
func NewRegexFilter(patterns []string, mode string, field string) *RegexFilter {
	if mode == "" {
		mode = "include"
	}
	if field == "" {
		field = "message"
	}

	compiledPatterns := make([]*regexp.Regexp, 0, len(patterns))
	for _, pattern := range patterns {
		if compiled, err := regexp.Compile(pattern); err == nil {
			compiledPatterns = append(compiledPatterns, compiled)
		}
	}

	return &RegexFilter{
		patterns: compiledPatterns,
		mode:     mode,
		field:    field,
	}
}

// Process determines if a log should be kept based on regex matching
func (f *RegexFilter) Process(log *core.Log) bool {
	// Get the text to match against
	var text string
	switch f.field {
	case "level":
		text = log.Level
	case "all":
		text = log.Level + " " + log.Message
	default: // "message"
		text = log.Message
	}

	// Check if any pattern matches
	matches := false
	for _, pattern := range f.patterns {
		if pattern.MatchString(text) {
			matches = true
			break
		}
	}

	// Return based on mode
	if f.mode == "exclude" {
		return !matches
	}
	return matches
}
