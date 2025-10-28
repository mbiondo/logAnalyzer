# LogAnalyzer - Dynamic Log Processing Pipeline

[![Go Version](https://img.shields.io/badge/Go-1.23%2B-00ADD8?logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)
[![GitHub Issues](https://img.shields.io/github/issues/mbiondo/logAnalyzer)](https://github.com/mbiondo/logAnalyzer/issues)
[![GitHub Stars](https://img.shields.io/github/stars/mbiondo/logAnalyzer)](https://github.com/mbiondo/logAnalyzer/stargazers)
[![Test Coverage](https://img.shields.io/badge/coverage-71.3%25-brightgreen.svg)](TESTING_REPORT.md)

A flexible, production-ready log processing system with intelligent routing, automatic failover, and zero-downtime operations. Collect logs from multiple sources, filter intelligently, and route to multiple destinations with per-output configuration.

## ✨ Why LogAnalyzer?

- 🔄 **High Availability**: Service starts even when dependencies (Elasticsearch, Kafka) are down
- 🛡️ **Zero Log Loss**: Write-Ahead Logging + automatic retry with exponential backoff  
- 🎯 **Smart Routing**: Route specific inputs to specific outputs with independent filtering
- ⚡ **Hot Reload**: Update configuration without restarting or dropping logs
- 🔌 **Extensible**: Plugin architecture - add custom inputs, outputs, and filters
- 📊 **Production Ready**: 71% test coverage with race condition verification

## 🚀 Quick Start

### Try the Complete Example

```bash
# Start Elasticsearch, Kibana, Prometheus, Grafana, and LogAnalyzer
cd examples
docker-compose up -d

# Access Services:
# - Grafana Dashboards: http://localhost:3000 (admin/admin)
# - Kibana: http://localhost:5601  
# - Prometheus: http://localhost:9090
# - LogAnalyzer Metrics: http://localhost:9091/metrics
# - HTTP Log Endpoint: http://localhost:8080/logs
```

**📖 Full setup guide:** [examples/README.md](examples/README.md)

### Install Binary

```bash
# Using Go
go install github.com/mbiondo/logAnalyzer/cmd@latest

# Or build from source
git clone https://github.com/mbiondo/logAnalyzer.git
cd logAnalyzer
go build -o loganalyzer ./cmd/main.go

# Run with hot reload
./loganalyzer -config config.yaml -hot-reload
```

## 📋 Complete Configuration Example

```yaml
# ============================================
# PERSISTENCE - Write-Ahead Logging
# Prevents log loss during crashes/restarts
# ============================================
persistence:
  enabled: true
  dir: "./data/wal"
  max_file_size: 104857600    # 100MB per file
  buffer_size: 100            # Buffer 100 logs before flush
  flush_interval: 5           # Flush every 5 seconds
  retention_hours: 24         # Keep WAL files for 24 hours
  sync_writes: false          # false = faster, true = more durable

# ============================================
# INPUTS - Log Sources
# Each input has a unique name for routing
# ============================================
inputs:
  # Monitor Docker containers
  - type: docker
    name: "production-app"
    config:
      container_filter: ["nginx-*", "webapp-*"]  # String or array
      stream: "stdout"

  # HTTP endpoint for external logs
  - type: http
    name: "external-api"
    config:
      port: "8080"

  # Kafka consumer
  - type: kafka
    name: "event-stream"
    config:
      brokers: ["kafka:29092"]
      topic: "application-logs"
      group_id: "loganalyzer-group"
      start_offset: "latest"
      # Resilience (optional - enabled by default)
      resilient: true
      retry_interval: 10        # Retry every 10s
      max_retries: 0            # 0 = never give up
      health_check_interval: 30 # Health check every 30s

  # Tail log files
  - type: file
    name: "legacy-logs"
    config:
      path: "/var/log/app.log"
      encoding: "utf-8"

# ============================================
# OUTPUTS - Log Destinations (Pipelines)
# Each output is an independent pipeline
# ============================================
outputs:
  # Pipeline 1: All logs to Elasticsearch
  - type: elasticsearch
    name: "main-index"
    sources: []                   # Empty = accept all sources
    filters:
      - type: level
        config:
          levels: ["INFO", "WARN", "ERROR"]
      - type: json                # Parse JSON from message field
        config:
          field: "message"
          flatten: true
    config:
      addresses: ["http://elasticsearch:9200"]
      index: "logs-{yyyy.MM.dd}"  # Date-based index
      username: "elastic"
      password: "changeme"
      batch_size: 50
      timeout: 30
      # Output buffering (optional - enabled by default)
      buffer:
        enabled: true
        queue_size: 1000
        max_retries: 5
        retry_delay: 1            # Exponential: 1s → 2s → 4s → 8s → 16s
        dlq_enabled: true         # Save failed logs
        dlq_file: "elasticsearch-main-dlq.jsonl"
      # Resilience (optional - enabled by default)
      resilient: true
      retry_interval: 10
      max_retries: 0
      health_check_interval: 30

  # Pipeline 2: Production errors to Slack (rate limited)
  - type: slack
    name: "critical-alerts"
    sources: ["production-app"]   # Only from production-app
    filters:
      - type: level
        config:
          levels: ["ERROR"]
      - type: rate_limit          # Prevent alert spam
        config:
          rate: 5.0               # Max 5 logs/second
          burst: 20               # Allow bursts up to 20
    config:
      webhook_url: "https://hooks.slack.com/services/YOUR/WEBHOOK/URL"
      channel: "#alerts"
      username: "LogBot"
      icon_emoji: ":fire:"
      buffer:
        enabled: true
        queue_size: 100
        max_retries: 3
        dlq_enabled: true

  # Pipeline 3: Kafka events to separate index
  - type: elasticsearch
    name: "kafka-events"
    sources: ["event-stream"]     # Only from Kafka
    filters:
      - type: json
        config:
          field: "message"
          flatten: true
          ignore_errors: false
    config:
      addresses: ["http://elasticsearch:9200"]
      index: "events-{yyyy.MM.dd}"
      batch_size: 100

  # Pipeline 4: Prometheus metrics (all sources, no filters)
  - type: prometheus
    name: "metrics-exporter"
    sources: []
    filters: []
    config:
      port: 9091
      # Exposes: loganalyzer_logs_total{level="debug|info|warn|error"}

  # Pipeline 5: Debug console output
  - type: console
    name: "debug-output"
    sources: ["external-api"]
    filters:
      - type: regex
        config:
          patterns: ["DEBUG", "TRACE"]
          mode: "include"
          field: "message"
    config:
      target: "stdout"            # stdout or stderr
      format: "json"              # json or text

  # Pipeline 6: Archive all logs to file
  - type: file
    name: "archive"
    sources: []                   # All sources
    filters: []                   # No filtering
    config:
      file_path: "/var/log/loganalyzer/archive.log"
      buffer:
        enabled: true
        queue_size: 500
```

## 🏗️ Architecture

LogAnalyzer uses a pipeline architecture where each output is independent:

```
┌─────────────────────────────────────────────────┐
│ INPUTS (Named Sources)                          │
├─────────────────────────────────────────────────┤
│ docker → "production-app"                       │
│ http → "external-api"                           │  
│ kafka → "event-stream"                          │
│ file → "legacy-logs"                            │
└──────────────────┬──────────────────────────────┘
                   ↓
         ┌─────────────────┐
         │  Engine Router  │ ← Routes logs by source name
         │  (Source Filter)│
         └─────────────────┘
                   ↓
┌──────────────────┴───────────────────────────────┐
│ OUTPUTS (Independent Pipelines)                  │
├──────────────────────────────────────────────────┤
│ Pipeline 1: Elasticsearch                        │
│   ├─ sources: [] (all)                           │
│   ├─ filters: [level: INFO+, json]               │
│   └─ buffer + resilience                         │
│                                                   │
│ Pipeline 2: Slack                                │
│   ├─ sources: ["production-app"]                 │
│   ├─ filters: [level: ERROR, rate_limit]         │
│   └─ buffer + resilience                         │
│                                                   │
│ Pipeline 3: Prometheus                           │
│   ├─ sources: [] (all)                           │
│   └─ filters: none                               │
│                                                   │
│ Pipeline 4: Console                              │
│   ├─ sources: ["external-api"]                   │
│   └─ filters: [regex: DEBUG]                     │
└──────────────────────────────────────────────────┘
```

**Key Concepts:**
- **Named Inputs**: Each input has a unique name (source identifier)
- **Source Routing**: Outputs specify which sources to accept (`sources: []` = all)
- **Independent Filters**: Each output applies its own filter chain
- **Parallel Processing**: Matching outputs process the same log simultaneously

## 🔄 Production Features

### 1. Plugin Resilience (High Availability)

**Service starts and operates even when dependencies are unavailable.**

**Configuration** (optional - enabled by default):
```yaml
outputs:
  - type: elasticsearch
    config:
      resilient: true           # Default: true
      retry_interval: 10        # Retry every 10s
      max_retries: 0            # 0 = never give up (default)
      health_check_interval: 30 # Health check every 30s
```

**How it works:**
1. Service starts immediately (non-blocking initialization)
2. Failed plugins retry in background with exponential backoff:
   - 10s → 20s → 40s → 80s → 120s (max)
3. Health checks detect recovery and automatically reconnect
4. Other plugins operate normally during outages

**Example logs:**
```
[RESILIENCE:elasticsearch] Attempting to initialize (attempt 1)
[RESILIENCE:elasticsearch] Failed: connection refused
[RESILIENCE:elasticsearch] Retrying in 10s...
[RESILIENCE:elasticsearch] Successfully initialized
[RESILIENCE:elasticsearch] Health check passed, plugin recovered
```

### 2. Output Buffering (Zero Log Loss)

**Automatic retry with Dead Letter Queue for failed deliveries.**

**Configuration** (optional - enabled by default):
```yaml
outputs:
  - type: elasticsearch
    config:
      buffer:
        enabled: true
        queue_size: 1000        # In-memory queue
        max_retries: 5          # Retry up to 5 times
        retry_delay: 1          # Initial delay (exponential backoff)
        dlq_enabled: true       # Save failed logs
        dlq_file: "failed-logs.jsonl"
```

**How it works:**
1. Delivery fails → Queued in memory
2. Retry with exponential backoff: 1s → 2s → 4s → 8s → 16s
3. After max retries → Saved to Dead Letter Queue file
4. Continue processing new logs without blocking

**📖 Full documentation:** [OUTPUT_BUFFERING.md](OUTPUT_BUFFERING.md)

### 3. Write-Ahead Logging (Crash Recovery)

**Persist logs to disk before processing to prevent loss during crashes.**

**Configuration:**
```yaml
persistence:
  enabled: true
  dir: "./data/wal"
  buffer_size: 100              # Buffer 100 logs before flush
  flush_interval: 5             # Flush every 5 seconds
  retention_hours: 24           # Keep for 24 hours
  sync_writes: false            # false = faster, true = more durable
```

**How it works:**
1. Log arrives → Written to WAL file
2. Process through pipeline
3. On restart → Recover all unprocessed logs from WAL
4. Old WAL files auto-deleted after retention period

### 4. Hot Reload (Zero Downtime Configuration)

**Update configuration without restarting.**

```bash
./loganalyzer -config config.yaml -hot-reload
```

**What happens:**
1. Edit `config.yaml` and save
2. Engine detects change and reloads automatically
3. All plugins gracefully restart with new config
4. No logs dropped during reload

## 🔌 Plugin Reference

### Input Plugins

#### Docker
Monitor Docker container logs with filtering:

```yaml
- type: docker
  name: "my-app"
  config:
    # Single container
    container_filter: "nginx"
    
    # OR multiple containers
    # container_filter: ["nginx-*", "webapp-*", "api-*"]
    
    # OR by container IDs
    # container_ids: ["abc123", "def456"]
    
    # OR by labels
    # labels:
    #   app: "myapp"
    #   env: "production"
    
    stream: "stdout"  # stdout, stderr, or both
```

**Priority:** `container_ids` > `container_filter` > `labels` > all containers

#### HTTP
Accept logs via HTTP POST:

```yaml
- type: http
  name: "api-logs"
  config:
    port: "8080"
```

**Usage:**
```bash
# Plain text
curl -X POST http://localhost:8080/logs \
  -H "Content-Type: text/plain" \
  -d "Error message"

# JSON
curl -X POST http://localhost:8080/logs \
  -H "Content-Type: application/json" \
  -d '{"level":"error","message":"Failed"}'
```

#### Kafka
Consume from Kafka topics:

```yaml
- type: kafka
  name: "events"
  config:
    brokers: ["kafka:29092", "localhost:9092"]
    topic: "application-logs"
    group_id: "loganalyzer-group"    # Consumer group
    start_offset: "latest"           # earliest, latest, or offset number
    min_bytes: 1
    max_bytes: 10485760              # 10MB
    # Optional SASL authentication
    # username: "user"
    # password: "pass"
    # Optional TLS
    # tls: true
    # insecure_skip_verify: false
```

**Metadata added:**
- `topic`: Kafka topic name
- `partition`: Partition number
- `offset`: Message offset
- `key`: Message key (if present)
- `header.*`: Kafka message headers

#### File
Tail log files:

```yaml
- type: file
  name: "app-file"
  config:
    path: "/var/log/app.log"
    encoding: "utf-8"
```

### Output Plugins

#### Elasticsearch
Send to Elasticsearch with bulk indexing:

```yaml
- type: elasticsearch
  name: "logs"
  config:
    addresses: ["http://elasticsearch:9200"]
    index: "logs-{yyyy.MM.dd}"   # Date templates
    username: "elastic"           # Optional
    password: "changeme"          # Optional
    batch_size: 50
    timeout: 30
```

**Index templates:**
- `{yyyy.MM.dd}` → 2024.01.15
- `{yyyy-MM-dd}` → 2024-01-15
- `{yyyy.MM}` → 2024.01
- `{yyyy}` → 2024

#### Prometheus
Expose metrics endpoint:

```yaml
- type: prometheus
  name: "metrics"
  config:
    port: 9091
```

**Metrics exposed:**
- `loganalyzer_logs_total{level="debug|info|warn|error"}`

Access at: `http://localhost:9091/metrics`

#### Slack
Send to Slack webhooks:

```yaml
- type: slack
  name: "alerts"
  config:
    webhook_url: "https://hooks.slack.com/services/YOUR/WEBHOOK/URL"
    channel: "#alerts"
    username: "LogBot"
    icon_emoji: ":fire:"
```

#### Console
Print to stdout/stderr:

```yaml
- type: console
  name: "debug"
  config:
    target: "stdout"  # stdout or stderr
    format: "json"    # json or text
```

#### File
Write to file:

```yaml
- type: file
  name: "archive"
  config:
    file_path: "/var/log/archive.log"
```

### Filter Plugins

#### Level
Filter by log level:

```yaml
- type: level
  config:
    levels: ["DEBUG", "INFO", "WARN", "ERROR"]
```

#### Regex
Filter by regex patterns:

```yaml
- type: regex
  config:
    patterns: ["ERROR.*", "Exception", "CRITICAL"]
    mode: "include"      # include or exclude
    field: "message"     # message, level, or all
```

#### JSON
Parse JSON from log fields:

```yaml
- type: json
  config:
    field: "message"     # Field to parse (default: "message")
    flatten: false       # Flatten nested objects with underscores
    ignore_errors: false # Ignore parsing errors
```

**Examples:**
- Parse: `{"user":"alice","action":"login"}` → metadata: `user=alice, action=login`
- Flatten: `{"user":{"name":"bob"}}` → metadata: `user_name=bob`

#### Rate Limit
Limit logs per second:

```yaml
- type: rate_limit
  config:
    rate: 10.0     # Logs per second (float)
    burst: 50      # Maximum burst size (int)
```

**How it works:**
- Token bucket algorithm
- Bucket holds up to `burst` tokens
- Tokens refill at `rate` per second
- Logs exceeding available tokens are dropped

## 💡 Common Use Cases

### Multi-Environment Logging

```yaml
# Production errors → Slack
# Staging → Console only
inputs:
  - type: docker
    name: "prod"
    config:
      labels: {env: "production"}
  
  - type: docker
    name: "staging"
    config:
      labels: {env: "staging"}

outputs:
  - type: slack
    sources: ["prod"]
    filters:
      - type: level
        config: {levels: ["ERROR"]}
    config:
      webhook_url: "..."
  
  - type: console
    sources: ["staging"]
    config: {format: "json"}
```

### Compliance & Auditing

```yaml
# All logs → Long-term storage
# Errors → Real-time alerts
outputs:
  - type: elasticsearch
    name: "audit"
    sources: []      # Everything
    filters: []      # No filtering
    config:
      index: "audit-{yyyy.MM}"
  
  - type: slack
    name: "alerts"
    sources: []
    filters:
      - type: level
        config: {levels: ["ERROR"]}
    config:
      webhook_url: "..."
```

### Event Streaming & Microservices

```yaml
# Kafka streams → Elasticsearch with JSON parsing
# Application logs → Prometheus metrics
inputs:
  - type: kafka
    name: "events"
    config:
      brokers: ["kafka:29092"]
      topic: "user-events"
  
  - type: docker
    name: "microservices"
    config:
      container_filter: ["auth-*", "payment-*", "notification-*"]

outputs:
  - type: elasticsearch
    name: "events"
    sources: ["events"]
    filters:
      - type: json
        config:
          field: "message"
          flatten: true
    config:
      index: "events-{yyyy.MM.dd}"
  
  - type: prometheus
    name: "metrics"
    sources: ["microservices"]
    config: {port: 9091}
```

## 🛠️ Development

### Creating Custom Plugins

```go
package myplugin

import "github.com/mbiondo/logAnalyzer/core"

func init() {
    // Auto-register plugin
    core.RegisterOutputPlugin("myplugin", NewMyPluginFromConfig)
}

type Config struct {
    Option string `yaml:"option"`
}

func NewMyPluginFromConfig(config map[string]any) (any, error) {
    var cfg Config
    if err := core.GetPluginConfig(config, &cfg); err != nil {
        return nil, err
    }
    return &MyPlugin{config: cfg}, nil
}

type MyPlugin struct {
    config Config
}

func (p *MyPlugin) Write(log *core.Log) error {
    // Access log.Source to identify input
    // Process log
    return nil
}

func (p *MyPlugin) Close() error {
    return nil
}
```

**Add to aggregator file:**
```go
// For output plugins: plugins/output/all.go
import _ "github.com/mbiondo/logAnalyzer/plugins/output/myplugin"

// For input plugins: plugins/input/all.go  
import _ "github.com/mbiondo/logAnalyzer/plugins/input/myplugin"

// For filter plugins: plugins/filter/all.go
import _ "github.com/mbiondo/logAnalyzer/plugins/filter/myplugin"
```

### Plugin Interfaces

```go
// Log structure
type Log struct {
    Timestamp time.Time
    Level     string
    Message   string
    Metadata  map[string]string
    Source    string  // Input name
}

// InputPlugin interface
type InputPlugin interface {
    Start() error
    Stop() error
    SetLogChannel(ch chan<- *Log)
}

// OutputPlugin interface
type OutputPlugin interface {
    Write(log *Log) error
    Close() error
}

// FilterPlugin interface
type FilterPlugin interface {
    Process(log *Log) bool  // true = pass, false = block
}
```

### Build Scripts

```bash
# Linux/Mac
./build.sh              # Build only
./build.sh --test       # Build with tests
./build.sh --clean      # Clean and build

# Windows
.\build.ps1             # Build only
.\build.ps1 -Test       # Build with tests
.\build.ps1 -Clean      # Clean and build
```

### Testing

```bash
# Run all tests
go test ./...

# With race detector (recommended)
go test -race ./...

# With coverage
go test -cover ./...

# Specific package
go test -race -v ./core
```

**Test Results:**
- **Coverage**: 71.3% of statements
- **Total Tests**: 79+ tests
- **Race Conditions**: ✅ None detected

**📖 Full test report:** [TESTING_REPORT.md](TESTING_REPORT.md)

### Docker

```bash
# Build image
docker build -t loganalyzer:latest .

# Run with config
docker run -v $(pwd)/config.yaml:/config.yaml \
  -v /var/run/docker.sock:/var/run/docker.sock \
  loganalyzer:latest -config /config.yaml
```

## 📚 Documentation

- **[OUTPUT_BUFFERING.md](OUTPUT_BUFFERING.md)** - Output buffering, retry, and DLQ guide
- **[TESTING_REPORT.md](TESTING_REPORT.md)** - Test coverage and race condition analysis
- **[PROJECT_INFO.md](PROJECT_INFO.md)** - Project structure and development guide
- **[examples/README.md](examples/README.md)** - Complete Docker example setup

## 📦 Project Structure

```
logAnalyzer/
├── cmd/                        # Application entry point
│   └── main.go
├── core/                       # Core engine
│   ├── config.go               # Configuration
│   ├── engine.go               # Pipeline engine
│   ├── log.go                  # Log structure
│   ├── registry.go             # Plugin registry
│   ├── persistence.go          # Write-Ahead Logging
│   ├── output_buffer.go        # Retry + DLQ
│   ├── plugin_resilience.go    # Resilience framework
│   ├── plugin_wrappers.go      # Resilient wrappers
│   ├── config_watcher.go       # Hot reload
│   └── *_test.go               # Tests (71.3% coverage)
├── plugins/
│   ├── input/                  # Input plugins
│   │   ├── docker/
│   │   ├── http/
│   │   ├── kafka/
│   │   └── file/
│   ├── output/                 # Output plugins
│   │   ├── elasticsearch/
│   │   ├── prometheus/
│   │   ├── slack/
│   │   ├── console/
│   │   └── file/
│   └── filter/                 # Filter plugins
│       ├── level/
│       ├── regex/
│       ├── json/
│       └── rate_limit/
├── examples/                   # Complete Docker setup
│   ├── docker-compose.yml
│   ├── loganalyzer.yaml
│   ├── grafana/                # Pre-configured dashboards
│   └── README.md
├── build.sh / build.ps1        # Build scripts
├── Dockerfile
└── README.md
```

## 📄 License

MIT License - see [LICENSE](LICENSE) file

---

**Built with ❤️ using Go 1.23** • [GitHub](https://github.com/mbiondo/logAnalyzer) • [Issues](https://github.com/mbiondo/logAnalyzer/issues)
