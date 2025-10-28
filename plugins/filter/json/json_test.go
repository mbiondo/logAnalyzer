package json

import (
	"testing"

	"github.com/mbiondo/logAnalyzer/core"
)

func TestJsonFilter_Process(t *testing.T) {
	tests := []struct {
		name         string
		config       Config
		inputLog     *core.Log
		expectedPass bool
		expectedMeta map[string]string
	}{
		{
			name: "parse message JSON",
			config: Config{
				Field: "message",
			},
			inputLog: &core.Log{
				Message:  `{"user": "alice", "action": "login"}`,
				Metadata: map[string]string{},
			},
			expectedPass: true,
			expectedMeta: map[string]string{
				"user":   "alice",
				"action": "login",
			},
		},
		{
			name: "flatten nested JSON",
			config: Config{
				Field:   "message",
				Flatten: true,
			},
			inputLog: &core.Log{
				Message:  `{"user": {"name": "bob", "id": 123}}`,
				Metadata: map[string]string{},
			},
			expectedPass: true,
			expectedMeta: map[string]string{
				"user_name": "bob",
				"user_id":   "123",
			},
		},
		{
			name: "ignore parse errors",
			config: Config{
				Field:        "message",
				IgnoreErrors: true,
			},
			inputLog: &core.Log{
				Message:  "not json",
				Metadata: map[string]string{},
			},
			expectedPass: true,
			expectedMeta: map[string]string{},
		},
		{
			name: "block on parse error",
			config: Config{
				Field:        "message",
				IgnoreErrors: false,
			},
			inputLog: &core.Log{
				Message:  "not json",
				Metadata: map[string]string{},
			},
			expectedPass: false,
			expectedMeta: map[string]string{},
		},
		{
			name: "parse from metadata field",
			config: Config{
				Field: "data",
			},
			inputLog: &core.Log{
				Message: "some message",
				Metadata: map[string]string{
					"data": `{"key": "value"}`,
				},
			},
			expectedPass: true,
			expectedMeta: map[string]string{
				"data": `{"key": "value"}`,
				"key":  "value",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter := NewJsonFilter(tt.config)
			pass := filter.Process(tt.inputLog)

			if pass != tt.expectedPass {
				t.Errorf("Process() = %v, want %v", pass, tt.expectedPass)
			}

			for k, v := range tt.expectedMeta {
				if got, ok := tt.inputLog.Metadata[k]; !ok || got != v {
					t.Errorf("Metadata[%s] = %v, want %v", k, got, v)
				}
			}
		})
	}
}

func TestNewJsonFilterFromConfig(t *testing.T) {
	config := map[string]any{
		"field":         "message",
		"flatten":       true,
		"ignore_errors": false,
	}

	plugin, err := NewJsonFilterFromConfig(config)
	if err != nil {
		t.Fatalf("NewJsonFilterFromConfig() error = %v", err)
	}

	filter, ok := plugin.(*JsonFilter)
	if !ok {
		t.Fatalf("NewJsonFilterFromConfig() returned wrong type")
	}

	if filter.config.Field != "message" || !filter.config.Flatten || filter.config.IgnoreErrors {
		t.Errorf("Config not set correctly: %+v", filter.config)
	}
}
