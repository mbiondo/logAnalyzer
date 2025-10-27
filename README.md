# LogAnalyzer - Dynamic Plugin System with Pipeline Architecture

[![Go Version](https://img.shields.io/badge/Go-1.23%2B-00ADD8?logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)
[![GitHub Issues](https://img.shields.io/github/issues/mbiondo/logAnalyzer)](https://github.com/mbiondo/logAnalyzer/issues)
[![GitHub Stars](https://img.shields.io/github/stars/mbiondo/logAnalyzer)](https://github.com/mbiondo/logAnalyzer/stargazers)

A flexible, extensible log analysis system with a dynamic plugin architecture and powerful output pipeline system. Process logs from multiple sources with per-output filtering and routing!

## 🚀 Key Features

- **Pipeline Architecture**: Each output can have its own filters and source restrictions
- **Source-Based Routing**: Route logs from specific inputs to specific outputs
- **Per-Output Filtering**: Apply different filters to different outputs
- **Dynamic Plugin Registration**: Plugins auto-register themselves using Go's `init()` function
- **Multiple Inputs/Outputs**: Run multiple input and output plugins simultaneously
- **Container Filtering**: Monitor specific Docker containers by name pattern (string or array)
- **Flexible Configuration**: YAML-based configuration with plugin-specific settings

## 📦 Built-in Plugins

- **Inputs**: File, Docker (with container filtering), HTTP
- **Outputs**: Console, File, Prometheus, Slack, Elasticsearch (with bulk indexing)
- **Filters**: Level-based, Regex pattern matching, JSON parsing

## � Installation

### Using Go Install

```bash
go install github.com/mbiondo/logAnalyzer/cmd@latest
```

### Build from Source

```bash
# Clone the repository
git clone https://github.com/mbiondo/logAnalyzer.git
cd logAnalyzer

# Build binary directly
go build -o loganalyzer ./cmd/main.go

# Or use the build script

# Linux/Mac:
chmod +x build.sh
./build.sh              # Build only
./build.sh --test       # Build with tests
./build.sh --clean      # Clean and build
./build.sh -t -c        # Clean, test, and build

# Windows:
.\build.ps1             # Build only
.\build.ps1 -Test       # Build with tests
.\build.ps1 -Clean      # Clean and build
```

### Using Docker

```bash
docker pull ghcr.io/mbiondo/loganalyzer:latest
# Or build locally
docker build -t loganalyzer .
```

## 📋 Quick Start

### Run

```bash
# With configuration file
./loganalyzer -config config.yaml

# Using example config
./loganalyzer -config examples/loganalyzer.yaml
```

### Quick Example Start

```bash
# Linux/Mac:
chmod +x start-example.sh
./start-example.sh

# Windows:
.\start-example.ps1
```

This will start Elasticsearch, Kibana, Prometheus, Grafana, and LogAnalyzer with a demo app!

### Verify the Stack

After starting the example, verify all services are working:

```bash
# Check all containers are running
docker-compose ps

# Verify Elasticsearch is indexing logs
curl http://localhost:9200/_cat/indices?v
# Windows: (Invoke-WebRequest http://localhost:9200/_cat/indices?v).Content

# View LogAnalyzer metrics
curl http://localhost:9091/metrics | grep loganalyzer_logs_total
# Windows: (Invoke-WebRequest http://localhost:9091/metrics).Content | Select-String "loganalyzer_logs_total"

# Access Grafana Dashboard
# Open: http://localhost:3000/d/loganalyzer-metrics (admin/admin)
```

### Stop

Press `Ctrl+C` to gracefully shutdown

## 🧪 Complete Example with Docker

### Ready-to-Use Environment

All configuration files and a complete Docker setup are in the `examples/` directory:

```powershell
# Start everything
cd examples
docker-compose up -d

# View logs
docker logs loganalyzer-service -f

# Stop everything
docker-compose down
```

**See [examples/README.md](examples/README.md) for detailed setup instructions!**

**Services included:**
- 🔍 **Elasticsearch** (http://localhost:9200) - Log storage and indexing
- 📊 **Kibana** (http://localhost:5601) - Log search and visualization  
- 📈 **Prometheus** (http://localhost:9090) - Metrics collection and monitoring
- 📉 **Grafana** (http://localhost:3000) - Unified dashboards (credentials: admin/admin)
  - **Pre-configured Dashboard**: http://localhost:3000/d/loganalyzer-metrics
- 🚀 **LogAnalyzer** - Log processing with pipeline architecture
  - HTTP input: http://localhost:8080/logs
  - Metrics: http://localhost:9091/metrics
- 🐋 **Demo App** - Generates sample logs for testing

## 🏗️ Pipeline Architecture

### How It Works

Each output operates as an independent pipeline with:
- **Source Filter**: Accept logs only from specified inputs
- **Custom Filters**: Apply level and regex filters per output
- **Independent Config**: Each output has its own settings

```
┌──────────────┐  Source: "docker-app"
│ Docker Input ├────────────┐
└──────────────┘            │
                            ▼
┌──────────────┐      ┌────────────────┐
│  HTTP Input  │─────▶│  Log Router    │
└──────────────┘      │   (Engine)     │
   Source: "http"     └────────────────┘
                            │
                ┌───────────┼───────────┐
                ▼           ▼           ▼
         ┌──────────┐ ┌──────────┐ ┌──────────┐
         │Pipeline 1│ │Pipeline 2│ │Pipeline 3│
         │──────────│ │──────────│ │──────────│
         │Sources:  │ │Sources:  │ │Sources:  │
         │  - All   │ │ - docker │ │  - http  │
         │Filters:  │ │Filters:  │ │Filters:  │
         │ - ERROR  │ │ - INFO+  │ │ - WARN+  │
         │Output:   │ │Output:   │ │Output:   │
         │  Slack   │ │Elastic   │ │Console   │
         └──────────┘ └──────────┘ └──────────┘
```

## ⚙️ Configuration

### Basic Pipeline Example

```yaml
input:
  inputs:
    - type: docker
      name: "app-logs"           # Name for routing
      config:
        container_filter: "my-app"
    
    - type: http
      name: "api-logs"
      config:
        port: "8080"

output:
  outputs:
    # Elasticsearch: All sources, INFO+ levels
    - type: elasticsearch
      name: "main-index"
      sources: []                # Empty = all sources
      filters:
        - type: level
          config:
            levels: ["INFO", "WARN", "ERROR"]
      config:
        addresses:
          - "http://elasticsearch:9200"
        index: "logs-{yyyy.MM.dd}"
        batch_size: 50
    
    # Prometheus: Only docker logs, no filters
    - type: prometheus
      name: "metrics"
      sources: ["app-logs"]      # Only from app-logs
      filters: []
      config:
        port: 9091
    
    # Console: All sources, ERROR only
    - type: console
      name: "errors"
      sources: []
      filters:
        - type: level
          config:
            levels: ["ERROR"]
      config:
        format: "json"
```

### Docker Container Filtering

```yaml
input:
  inputs:
    - type: docker
      name: "my-containers"
      config:
        # Option 1: Single container (string)
        container_filter: "my-app"
        
        # Option 2: Multiple containers (array)
        # container_filter:
        #   - "app-*"
        #   - "service-*"
        #   - "worker-*"
        
        stream: "stdout"
```

### Advanced Multi-Pipeline

```yaml
input:
  inputs:
    - type: docker
      name: "web-app"
      config:
        container_filter: ["nginx", "webapp"]
    
    - type: docker
      name: "api-service"
      config:
        container_filter: "api-*"
    
    - type: http
      name: "external"
      config:
        port: "8080"

output:
  outputs:
    # Critical alerts to Slack (all sources)
    - type: slack
      name: "alerts"
      sources: []
      filters:
        - type: level
          config:
            levels: ["ERROR"]
        - type: regex
          config:
            patterns: ["CRITICAL", "FATAL"]
            mode: "include"
      config:
        webhook_url: "https://hooks.slack.com/..."
        channel: "#alerts"
    
    # Web logs to Elasticsearch
    - type: elasticsearch
      name: "web-index"
      sources: ["web-app"]
      filters:
        - type: level
          config:
            levels: ["INFO", "WARN", "ERROR"]
      config:
        index: "web-{yyyy.MM.dd}"
    
    # API metrics to Prometheus
    - type: prometheus
      name: "api-metrics"
      sources: ["api-service"]
      filters: []
      config:
        port: 9091
```

## 🔌 Plugin Reference

### Docker Input

```yaml
- type: docker
  name: "docker-logs"
  config:
    # Filter by name (string or array)
    container_filter: "my-app"
    # OR
    # container_filter: ["app1", "app2"]
    
    # Filter by IDs
    # container_ids: ["abc123"]
    
    # Filter by labels
    # labels:
    #   app: "myapp"
    
    stream: "stdout"  # stdout, stderr, both
```

**Priority**: container_ids > container_filter > all containers

### Elasticsearch Output

```yaml
- type: elasticsearch
  name: "logs"
  sources: ["docker-logs"]
  filters:
    - type: level
      config:
        levels: ["INFO", "WARN", "ERROR"]
  config:
    addresses:
      - "http://elasticsearch:9200"
    index: "logs-{yyyy.MM.dd}"
    username: "elastic"      # Optional
    password: "changeme"     # Optional
    batch_size: 50           # Bulk size
    timeout: 30              # Seconds
```

**Index Templates**:
- `{yyyy.MM.dd}` → 2024.01.15
- `{yyyy-MM-dd}` → 2024-01-15
- `{yyyy.MM}` → 2024.01
- `{yyyy}` → 2024

### Prometheus Output

```yaml
- type: prometheus
  name: "metrics"
  sources: []
  filters: []
  config:
    port: 9091
```

**Metrics exposed**:
- `loganalyzer_logs_total{level="debug|info|warn|error"}`

### Slack Output

```yaml
- type: slack
  name: "alerts"
  sources: []
  filters:
    - type: level
      config:
        levels: ["ERROR"]
  config:
    webhook_url: "https://hooks.slack.com/..."
    channel: "#alerts"
    username: "LogBot"
    icon_emoji: ":fire:"
```

### Console Output

```yaml
- type: console
  name: "debug"
  sources: []
  filters: []
  config:
    target: "stdout"  # stdout or stderr
    format: "json"    # json or text
```

### File Output

```yaml
- type: file
  name: "archive"
  sources: []
  filters: []
  config:
    file_path: "/var/log/archive.log"
```

### HTTP Input

```yaml
- type: http
  name: "api"
  config:
    port: "8080"
```

**Usage**:
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

### File Input

```yaml
- type: file
  name: "app-file"
  config:
    path: "/var/log/app.log"
    encoding: "utf-8"
```

### Level Filter

```yaml
- type: level
  config:
    levels: ["DEBUG", "INFO", "WARN", "ERROR"]
```

### Regex Filter

```yaml
- type: regex
  config:
    patterns: ["ERROR.*", "Exception"]
    mode: "include"      # include or exclude
    field: "message"     # message, level, or all
```

### JSON Filter

```yaml
- type: json
  config:
    field: "message"     # Field to parse (default: "message")
    flatten: false       # Flatten nested objects with underscores
    ignore_errors: false # Ignore parsing errors instead of blocking
```

**Examples**:
- Parse JSON from message: `{"user":"alice","action":"login"}` → metadata: `user=alice, action=login`
- Flatten nested: `{"user":{"name":"bob"}}` → metadata: `user_name=bob`
- Parse from metadata field: Use `field: "data"` to parse a different metadata field

## 💡 Use Cases

### Use Case 1: Multi-Environment

```yaml
# Production errors → Slack
# Staging → Console only
input:
  inputs:
    - type: docker
      name: "prod"
      config:
        labels: {env: "production"}
    - type: docker
      name: "staging"
      config:
        labels: {env: "staging"}

output:
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
      filters: []
      config: {format: "json"}
```

### Use Case 2: Compliance

```yaml
# All logs → Audit
# Errors → Alerts
output:
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

### Use Case 3: Performance Monitoring

```yaml
# Web → Prometheus
# DB errors → Elasticsearch
input:
  inputs:
    - type: docker
      name: "web"
      config:
        container_filter: ["nginx-*", "webapp-*"]
    - type: docker
      name: "db"
      config:
        container_filter: "postgres-*"

output:
  outputs:
    - type: prometheus
      sources: ["web"]
      filters: []
      config: {port: 9091}
    
    - type: elasticsearch
      sources: ["db"]
      filters:
        - type: level
          config: {levels: ["ERROR", "WARN"]}
      config:
        index: "db-errors-{yyyy.MM.dd}"
```

## 🛠️ Creating Custom Plugins

```go
package myplugin

import "github.com/mbiondo/logAnalyzer/core"

func init() {
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

Then import in `cmd/main.go`:
```go
import _ "github.com/mbiondo/logAnalyzer/plugins/output/myplugin"
```

## 📚 Interfaces

### Log Structure
```go
type Log struct {
    Timestamp time.Time
    Level     string
    Message   string
    Metadata  map[string]string
    Source    string  // Input name
}
```

### InputPlugin
```go
type InputPlugin interface {
    Start() error
    Stop() error
    SetLogChannel(ch chan<- *Log)
}
```

### OutputPlugin
```go
type OutputPlugin interface {
    Write(log *Log) error
    Close() error
}
```

### FilterPlugin
```go
type FilterPlugin interface {
    Process(log *Log) bool
}
```

## 🧪 Testing

```bash
# All tests
go test ./...

# Specific package
go test ./core -v
go test ./plugins/input/docker -v

# Coverage
go test -cover ./...
```

## 📁 Project Structure

```
log-analyzer/
├── cmd/
│   └── main.go                 # Entry point
├── core/
│   ├── config.go               # Config structures
│   ├── engine.go               # Pipeline engine
│   ├── log.go                  # Log structure
│   └── registry.go             # Plugin registry
├── plugins/
│   ├── input/                  # Input plugins
│   ├── output/                 # Output plugins
│   └── filter/                 # Filter plugins
├── examples/                   # Complete working example
│   ├── docker-compose.yml      # All services
│   ├── loganalyzer.yaml        # Pipeline config
│   ├── prometheus.yml          # Prometheus config
│   ├── grafana/                # Dashboards & datasources
│   └── README.md               # Setup guide
└── README.md                   # Main documentation
```

## ✨ Benefits

- ✅ **Flexible Routing**: Logs from specific sources to specific outputs
- ✅ **Per-Output Filtering**: Different rules for different outputs
- ✅ **Container Isolation**: Monitor only specific containers
- ✅ **Efficient Processing**: Logs only processed by relevant pipelines
- ✅ **Extensibility**: Add plugins without core changes
- ✅ **Clear Configuration**: Declarative pipeline definitions

## 📄 License

MIT License

---

Built with ❤️ using Go
