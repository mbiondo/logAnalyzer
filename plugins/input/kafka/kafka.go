package kafkainput

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mbiondo/logAnalyzer/core"
	"github.com/mbiondo/logAnalyzer/pkg/tlsconfig"
	"github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/sasl/plain"
)

func init() {
	core.RegisterInputPlugin("kafka", NewKafkaInputFromConfig)
}

// Config represents Kafka input configuration values supplied via YAML.
type Config struct {
	Brokers     []string         `yaml:"brokers"`
	Topic       string           `yaml:"topic"`
	GroupID     string           `yaml:"group_id,omitempty"`
	StartOffset string           `yaml:"start_offset,omitempty"`
	MinBytes    int              `yaml:"min_bytes,omitempty"`
	MaxBytes    int              `yaml:"max_bytes,omitempty"`
	ClientID    string           `yaml:"client_id,omitempty"`
	Username    string           `yaml:"username,omitempty"`
	Password    string           `yaml:"password,omitempty"`
	TLS         tlsconfig.Config `yaml:"tls,omitempty"` // TLS configuration
}

// NewKafkaInputFromConfig builds a Kafka input plugin from generic configuration.
func NewKafkaInputFromConfig(config map[string]any) (any, error) {
	var cfg Config
	if err := core.GetPluginConfig(config, &cfg); err != nil {
		return nil, err
	}

	if len(cfg.Brokers) == 0 {
		return nil, fmt.Errorf("kafka input requires at least one broker")
	}
	if cfg.Topic == "" {
		return nil, fmt.Errorf("kafka input requires a topic")
	}

	// Validate TLS config
	if err := cfg.TLS.Validate(); err != nil {
		return nil, err
	}

	startOffset, err := parseStartOffset(cfg.StartOffset)
	if err != nil {
		return nil, err
	}

	minBytes := cfg.MinBytes
	if minBytes <= 0 {
		minBytes = 1
	}

	maxBytes := cfg.MaxBytes
	if maxBytes <= 0 {
		maxBytes = 10 * 1024 * 1024 // 10 MiB reasonable default batch size
	}

	readerCfg := kafka.ReaderConfig{
		Brokers:     cfg.Brokers,
		Topic:       cfg.Topic,
		GroupID:     cfg.GroupID,
		StartOffset: startOffset,
		MinBytes:    minBytes,
		MaxBytes:    maxBytes,
	}

	dialer := &kafka.Dialer{
		Timeout:   10 * time.Second,
		DualStack: true,
	}

	if cfg.ClientID != "" {
		dialer.ClientID = cfg.ClientID
	}

	// Configure TLS
	if cfg.TLS.Enabled {
		tlsConfig, err := cfg.TLS.NewTLSConfig()
		if err != nil {
			return nil, fmt.Errorf("failed to create TLS config: %w", err)
		}
		dialer.TLS = tlsConfig
	}

	// Configure SASL
	if cfg.Username != "" && cfg.Password != "" {
		mechanism := plain.Mechanism{
			Username: cfg.Username,
			Password: cfg.Password,
		}
		dialer.SASLMechanism = mechanism
	}

	readerCfg.Dialer = dialer

	reader := kafka.NewReader(readerCfg)

	return &KafkaInput{
		brokers: cfg.Brokers,
		topic:   cfg.Topic,
		groupID: cfg.GroupID,
		reader:  reader,
	}, nil
}

// KafkaInput consumes records from Kafka topics and forwards them to the engine.
type KafkaInput struct {
	name    string
	logCh   chan<- *core.Log
	reader  *kafka.Reader
	brokers []string
	topic   string
	groupID string

	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	stopped bool
}

// SetName assigns a logical name to this plugin instance.
func (k *KafkaInput) SetName(name string) {
	k.name = name
}

// SetLogChannel stores the channel used to send logs to the engine.
func (k *KafkaInput) SetLogChannel(ch chan<- *core.Log) {
	k.logCh = ch
}

// Start launches the background goroutine that reads from Kafka.
func (k *KafkaInput) Start() error {
	if k.reader == nil {
		return fmt.Errorf("kafka reader is not initialized")
	}

	if k.ctx != nil {
		return fmt.Errorf("kafka input already started")
	}

	k.ctx, k.cancel = context.WithCancel(context.Background())
	k.wg.Add(1)
	go k.consumeLoop()

	log.Printf("Kafka input started (topic=%s, brokers=%v, group=%s)", k.topic, k.brokers, k.groupID)
	return nil
}

// Stop cancels consumption and waits for the goroutine to finish.
func (k *KafkaInput) Stop() error {
	if k.stopped {
		return nil
	}
	k.stopped = true

	if k.cancel != nil {
		k.cancel()
	}

	k.wg.Wait()

	if k.reader != nil {
		if err := k.reader.Close(); err != nil {
			log.Printf("Kafka input close error: %v", err)
		}
	}

	log.Printf("Kafka input stopped")
	k.ctx = nil
	return nil
}

// CheckHealth implements HealthChecker interface
func (k *KafkaInput) CheckHealth(ctx context.Context) error {
	if k.reader == nil {
		return fmt.Errorf("kafka reader not initialized")
	}

	// Check if context is cancelled (indicates connection issues)
	if k.ctx != nil && k.ctx.Err() != nil {
		return fmt.Errorf("kafka connection lost: %w", k.ctx.Err())
	}

	return nil
}

func (k *KafkaInput) consumeLoop() {
	defer k.wg.Done()

	for {
		msg, err := k.reader.FetchMessage(k.ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return
			}

			if k.ctx.Err() != nil {
				return
			}

			log.Printf("Kafka input fetch error: %v", err)
			time.Sleep(time.Second)
			continue
		}

		logEntry := buildLogFromMessage(msg, k.name)

		select {
		case k.logCh <- logEntry:
		case <-k.ctx.Done():
			return
		}

		if k.groupID != "" {
			if err := k.reader.CommitMessages(k.ctx, msg); err != nil {
				if k.ctx.Err() != nil {
					return
				}
				log.Printf("Kafka input commit error: %v", err)
			}
		}
	}
}

func buildLogFromMessage(msg kafka.Message, source string) *core.Log {
	level := "info"
	metadata := map[string]string{
		"source":    "kafka",
		"topic":     msg.Topic,
		"partition": strconv.Itoa(msg.Partition),
		"offset":    strconv.FormatInt(msg.Offset, 10),
	}

	if len(msg.Key) > 0 {
		metadata["key"] = string(msg.Key)
	}

	for _, header := range msg.Headers {
		if strings.EqualFold(header.Key, "level") {
			level = strings.ToLower(string(header.Value))
		}
		metadata["header."+strings.ToLower(header.Key)] = string(header.Value)
	}

	logEntry := core.NewLogWithMetadata(level, string(msg.Value), metadata)
	logEntry.Source = source
	return logEntry
}

func parseStartOffset(raw string) (int64, error) {
	if raw == "" {
		return kafka.LastOffset, nil
	}

	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "earliest", "first", "beginning":
		return kafka.FirstOffset, nil
	case "latest", "last", "end":
		return kafka.LastOffset, nil
	}

	offset, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid start_offset value: %s", raw)
	}

	return offset, nil
}
