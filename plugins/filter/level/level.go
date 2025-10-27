package level

import (
	"strings"

	"github.com/mbiondo/logAnalyzer/core"
)

func init() {
	// Auto-register this plugin
	core.RegisterFilterPlugin("level", NewLevelFilterFromConfig)
}

// Config represents level filter configuration
type Config struct {
	Levels []string `yaml:"levels"`
}

// NewLevelFilterFromConfig creates a level filter from configuration map
func NewLevelFilterFromConfig(config map[string]interface{}) (interface{}, error) {
	var cfg Config
	if err := core.GetPluginConfig(config, &cfg); err != nil {
		return nil, err
	}

	return NewLevelFilter(cfg.Levels), nil
}

// LevelFilter filters logs by level
type LevelFilter struct {
	allowedLevels map[string]bool
}

// NewLevelFilter creates a new level filter
func NewLevelFilter(levels []string) *LevelFilter {
	allowed := make(map[string]bool)
	for _, level := range levels {
		allowed[strings.ToLower(level)] = true
	}
	return &LevelFilter{
		allowedLevels: allowed,
	}
}

// Process determines if a log should be kept based on its level
func (f *LevelFilter) Process(log *core.Log) bool {
	return f.allowedLevels[strings.ToLower(log.Level)]
}
