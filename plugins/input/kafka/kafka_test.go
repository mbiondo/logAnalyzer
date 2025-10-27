package kafkainput

import (
	"testing"

	"github.com/segmentio/kafka-go"
)

func TestParseStartOffsetKeywords(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int64
	}{
		{name: "default empty", input: "", expected: kafka.LastOffset},
		{name: "latest keyword", input: "latest", expected: kafka.LastOffset},
		{name: "earliest keyword", input: "earliest", expected: kafka.FirstOffset},
		{name: "mixed case", input: "BeGiNnInG", expected: kafka.FirstOffset},
		{name: "numeric", input: "42", expected: 42},
	}

	for _, tt := range tests {
		result, err := parseStartOffset(tt.input)
		if err != nil {
			t.Fatalf("%s: expected no error, got %v", tt.name, err)
		}

		if result != tt.expected {
			t.Errorf("%s: expected %d, got %d", tt.name, tt.expected, result)
		}
	}
}

func TestParseStartOffsetInvalid(t *testing.T) {
	if _, err := parseStartOffset("not-a-number"); err == nil {
		t.Fatal("expected error for invalid offset, got nil")
	}
}

func TestBuildLogFromMessage(t *testing.T) {
	msg := kafka.Message{
		Topic:     "logs",
		Partition: 3,
		Offset:    101,
		Key:       []byte("order-42"),
		Headers: []kafka.Header{
			{Key: "Level", Value: []byte("ERROR")},
			{Key: "Env", Value: []byte("prod")},
		},
		Value: []byte("service failed"),
	}

	entry := buildLogFromMessage(msg, "kafka-input-1")

	if entry.Level != "error" {
		t.Fatalf("expected level 'error', got %s", entry.Level)
	}

	if entry.Message != "service failed" {
		t.Fatalf("expected message 'service failed', got %s", entry.Message)
	}

	if entry.Source != "kafka-input-1" {
		t.Fatalf("expected source set, got %s", entry.Source)
	}

	metadata := entry.Metadata
	if metadata["topic"] != "logs" {
		t.Errorf("expected topic metadata, got %s", metadata["topic"])
	}
	if metadata["partition"] != "3" {
		t.Errorf("expected partition '3', got %s", metadata["partition"])
	}
	if metadata["offset"] != "101" {
		t.Errorf("expected offset '101', got %s", metadata["offset"])
	}
	if metadata["key"] != "order-42" {
		t.Errorf("expected key metadata, got %s", metadata["key"])
	}
	if metadata["header.env"] != "prod" {
		t.Errorf("expected header metadata for env, got %s", metadata["header.env"])
	}
}

func TestNewKafkaInputFromConfigValidation(t *testing.T) {
	_, err := NewKafkaInputFromConfig(map[string]any{
		"topic": "logs",
	})
	if err == nil {
		t.Fatal("expected error when brokers are missing")
	}

	_, err = NewKafkaInputFromConfig(map[string]any{
		"brokers": []string{"localhost:9092"},
	})
	if err == nil {
		t.Fatal("expected error when topic is missing")
	}
}

func TestNewKafkaInputFromConfigSuccess(t *testing.T) {
	plugin, err := NewKafkaInputFromConfig(map[string]any{
		"brokers":      []string{"localhost:9092"},
		"topic":        "logs",
		"group_id":     "log-analyzer",
		"start_offset": "earliest",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	input, ok := plugin.(*KafkaInput)
	if !ok {
		t.Fatalf("expected *KafkaInput, got %T", plugin)
	}

	if input.topic != "logs" {
		t.Errorf("expected topic 'logs', got %s", input.topic)
	}
	if input.groupID != "log-analyzer" {
		t.Errorf("expected group 'log-analyzer', got %s", input.groupID)
	}
	if len(input.brokers) != 1 || input.brokers[0] != "localhost:9092" {
		t.Errorf("unexpected brokers slice: %v", input.brokers)
	}
	if input.reader == nil {
		t.Fatal("expected reader to be initialized")
	}
}
