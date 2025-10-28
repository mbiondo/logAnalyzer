package json

import (
	"encoding/json"
	"fmt"

	"github.com/mbiondo/logAnalyzer/core"
)

func init() {
	// Auto-register this plugin
	core.RegisterFilterPlugin("json", NewJsonFilterFromConfig)
}

// Config represents JSON filter configuration
type Config struct {
	Field        string `yaml:"field"`         // Field to parse (default: "message")
	Flatten      bool   `yaml:"flatten"`       // Flatten nested objects
	IgnoreErrors bool   `yaml:"ignore_errors"` // Ignore parsing errors
}

// NewJsonFilterFromConfig creates a JSON filter from configuration map
func NewJsonFilterFromConfig(config map[string]any) (any, error) {
	var cfg Config
	if err := core.GetPluginConfig(config, &cfg); err != nil {
		return nil, err
	}

	if cfg.Field == "" {
		cfg.Field = "message"
	}

	return NewJsonFilter(cfg), nil
}

// JsonFilter parses JSON content and adds fields to metadata
type JsonFilter struct {
	config Config
}

// NewJsonFilter creates a new JSON filter
func NewJsonFilter(config Config) *JsonFilter {
	return &JsonFilter{
		config: config,
	}
}

// Process parses JSON from the specified field and adds parsed fields to metadata
func (f *JsonFilter) Process(log *core.Log) bool {
	// Get data from specified field
	var data string
	switch f.config.Field {
	case "message":
		data = log.Message
	default:
		if val, ok := log.Metadata[f.config.Field]; ok {
			data = val
		} else {
			return true // Field not found, pass through
		}
	}

	// Parse JSON
	var parsed map[string]any
	if err := json.Unmarshal([]byte(data), &parsed); err != nil {
		if f.config.IgnoreErrors {
			return true
		}
		return false // Block on parse error
	}

	// Add parsed fields to metadata
	for k, v := range parsed {
		if f.config.Flatten {
			f.flatten(k, v, log.Metadata)
		} else {
			log.Metadata[k] = fmt.Sprintf("%v", v)
		}
	}

	return true
}

// flatten recursively flattens nested maps with underscore-separated keys
func (f *JsonFilter) flatten(prefix string, value any, target map[string]string) {
	switch v := value.(type) {
	case map[string]any:
		for k, val := range v {
			f.flatten(prefix+"_"+k, val, target)
		}
	default:
		target[prefix] = fmt.Sprintf("%v", v)
	}
}
