# LogAnalyzer - Project Information

## 📦 Project Structure

```
logAnalyzer/
├── cmd/                        # Application entry point
│   └── main.go                 # Main application
├── core/                       # Core engine and types
│   ├── config.go               # Configuration structures
│   ├── config_test.go          # Configuration tests
│   ├── engine.go               # Pipeline processing engine with API server
│   ├── engine_test.go          # Engine tests
│   ├── log.go                  # Log data structure
│   ├── log_test.go             # Log tests
│   ├── registry.go             # Plugin registration system
│   ├── registry_test.go        # Registry tests
│   ├── persistence.go          # Write-Ahead Logging (WAL)
│   ├── output_buffer.go        # Output buffering with retry & DLQ
│   ├── plugin_resilience.go    # Plugin resilience framework
│   ├── plugin_resilience_test.go  # Resilience tests (14 tests + benchmark)
│   ├── plugin_wrappers.go      # Resilient plugin wrappers
│   ├── plugin_wrappers_test.go # Wrapper tests (12 tests + benchmark)
│   └── config_watcher.go       # Hot reload functionality
├── plugins/                    # Plugin implementations
│   ├── input/                  # Input plugins (Docker, HTTP, File, Kafka)
│   │   ├── all.go              # Input plugin aggregator
│   │   ├── docker/             # Docker input plugin
│   │   ├── file/               # File input plugin
│   │   ├── http/               # HTTP input plugin
│   │   └── kafka/              # Kafka input plugin
│   ├── output/                 # Output plugins
│   │   ├── all.go              # Output plugin aggregator
│   │   ├── console/            # Console output plugin
│   │   ├── elasticsearch/      # Elasticsearch output plugin
│   │   ├── file/               # File output plugin
│   │   ├── prometheus/         # Prometheus output plugin
│   │   └── slack/              # Slack output plugin
│   └── filter/                 # Filter plugins
│       ├── all.go              # Filter plugin aggregator
│       ├── json/               # JSON filter plugin
│       ├── level/              # Level filter plugin
│       ├── rate_limit/         # Rate limit filter plugin
│       └── regex/              # Regex filter plugin
├── examples/                   # Complete working example
│   ├── docker-compose.yml      # All services configuration
│   ├── loganalyzer.yaml        # Pipeline configuration with resilience
│   ├── prometheus.yml          # Prometheus scrape config
│   ├── grafana/                # Grafana dashboards & datasources
│   │   ├── dashboards/
│   │   │   ├── loganalyzer-dashboard.json
│   │   │   └── kafka-logs-dashboard.json
│   │   └── provisioning/
│   │       ├── datasources/
│   │       │   └── datasources.yaml
│   │       └── dashboards/
│   │           └── dashboard-provider.yaml
│   ├── scripts/
│   │   ├── test-data.sh        # Generate test data (Linux/Mac)
│   │   └── test-data.ps1       # Generate test data (Windows)
│   └── README.md               # Example setup guide
├── .github/                    # GitHub workflows
│   └── workflows/
│       ├── ci.yml              # Continuous integration
│       ├── docker.yml          # Docker image builds
│       └── release.yml         # Release automation
├── build.sh                    # Build script (Linux/Mac)
├── build.ps1                   # Build script (Windows)
├── start-example.sh            # Quick start script (Linux/Mac)
├── start-example.ps1           # Quick start script (Windows)
├── Dockerfile                  # Container image definition
├── go.mod                      # Go module definition
├── go.sum                      # Go module checksums
├── README.md                   # Main documentation
├── OUTPUT_BUFFERING.md         # Output buffering documentation
├── TESTING_REPORT.md           # Comprehensive test report with race condition analysis
├── config.example.yaml         # Example configuration
├── LICENSE                     # MIT License
├── CODE_OF_CONDUCT.md          # Community guidelines
├── CONTRIBUTING.md             # Contribution guidelines
└── SECURITY.md                 # Security policy
```

## 🚀 Quick Start Commands

### Building

```bash
# Linux/Mac
./build.sh --test --clean

# Windows
.\build.ps1 -Test -Clean
```

### Running Examples

```bash
# Linux/Mac
./start-example.sh

# Windows
.\start-example.ps1
```

### Testing

```bash
go test ./...
```

## 🏗️ Architecture

### Pipeline System

LogAnalyzer uses a sophisticated pipeline architecture where:

1. **Named Inputs**: Each input plugin has a unique name
2. **Source Tracking**: Logs carry their source identifier
3. **Output Pipelines**: Each output has its own:
   - Source filter (which inputs to accept)
   - Filter chain (level, regex, etc.)
   - Output plugin configuration

### Flow

```
Input Plugins → Engine (Router) → Output Pipelines
   ↓                  ↓                   ↓
Docker, HTTP     Source Filter      Elasticsearch
File Input       Apply Filters      Prometheus
                                   Console, Slack
```

## 🔌 Available Plugins

### Input Plugins
- **docker**: Monitor Docker container logs
- **http**: HTTP endpoint for external logs
- **file**: Read from log files

### Output Plugins
- **elasticsearch**: Send to Elasticsearch with bulk indexing
- **prometheus**: Expose metrics endpoint
- **console**: Print to stdout/stderr
- **file**: Write to file
- **slack**: Send to Slack webhook

### Filter Plugins
- **level**: Filter by log level (DEBUG, INFO, WARN, ERROR)
- **regex**: Filter by regex pattern (include/exclude)

## 📊 Example Services

The `examples/` directory includes a complete setup with:

- **Elasticsearch** (9200): Log storage
- **Kibana** (5601): Log visualization
- **Prometheus** (9090): Metrics collection
- **Grafana** (3000): Unified dashboards
- **LogAnalyzer** (8080, 9091): Log processing
- **Demo App**: Sample log generator

## 🧪 Testing

All tests are located alongside their respective code files.

### Running Tests

```bash
# Run all tests
go test ./...

# Run with race detector (recommended)
go test -race ./...

# Run with coverage
go test -cover ./...

# Run specific package tests
go test ./core -v
go test ./plugins/input/docker -v

# Run with race detector and coverage
go test -race -cover ./core
```

### Test Coverage

- **Core Module**: 71.3% coverage
- **Total Tests**: 79 tests in core + all plugin tests
- **Race Conditions**: ✅ None detected (verified with `-race` flag)

### Plugin Resilience Tests

The resilience framework has comprehensive test coverage:

**`core/plugin_resilience_test.go`** (14 tests + 1 benchmark):
- Successful initialization
- Retry on failure with exponential backoff
- Max retries enforcement
- Health check detection
- **Concurrent access** (10 goroutines × 100 operations)
- Exponential backoff timing
- Close while initializing
- Statistics tracking
- Multiple closes (idempotent)
- Context cancellation

**`core/plugin_wrappers_test.go`** (12 tests + 1 benchmark):
- Input plugin wrapper tests
- Output plugin wrapper tests
- **Concurrent writes** (10 writers + 5 health checkers)
- Write before initialization
- Recovery during active writes
- Unhealthy state handling

See `TESTING_REPORT.md` for detailed test results and race condition analysis.

## 📝 Configuration

Configuration is YAML-based with two main sections:

1. **inputs[]**: Array of input plugin configurations
2. **outputs[]**: Array of output pipeline configurations

See `examples/loganalyzer.yaml` for a complete example.

## 🔧 Development

### Adding a New Plugin

1. Create plugin in appropriate directory (`plugins/input/`, `plugins/output/`, or `plugins/filter/`)
2. Implement the plugin interface
3. Register in `init()` function
4. Add blank import to the corresponding aggregator file (`plugins/input/all.go`, `plugins/output/all.go`, or `plugins/filter/all.go`)

See README.md section "Creating Custom Plugins" for details.

### Building Docker Image

```bash
docker build -t loganalyzer:latest .
```

### Running in Docker

```bash
docker run -v $(pwd)/config.yaml:/config.yaml \
  -v /var/run/docker.sock:/var/run/docker.sock \
  loganalyzer:latest -config /config.yaml
```

## 📚 Documentation

- **README.md**: Main documentation with full plugin reference
- **OUTPUT_BUFFERING.md**: Comprehensive guide to output buffering, retry logic, and DLQ
- **TESTING_REPORT.md**: Complete test report with race condition analysis
- **examples/README.md**: Complete example setup guide
- **CODE_OF_CONDUCT.md**: Community guidelines
- **CONTRIBUTING.md**: How to contribute
- **SECURITY.md**: Security policy and reporting

## 🎯 Key Features

- ✅ Pipeline architecture with source-based routing
- ✅ Per-output filtering and configuration
- ✅ Dynamic plugin registration system
- ✅ **Built-in REST API for service monitoring and metrics**
- ✅ **Plugin resilience with automatic reconnection**
- ✅ **Output buffering with retry logic and DLQ**
- ✅ **Persistent buffers using Write-Ahead Logging (WAL)**
- ✅ Container filtering (string or array)
- ✅ Multiple inputs and outputs simultaneously
- ✅ Elasticsearch bulk indexing
- ✅ Prometheus metrics export
- ✅ Complete Docker example with Grafana dashboards
- ✅ Cross-platform support (Windows, Linux, Mac)
- ✅ Hot reload of configuration files
- ✅ **Comprehensive test coverage (71.3%) with race condition verification**

## 🔒 Security

- Docker socket access required for container monitoring
- Elasticsearch security disabled in examples (enable for production)
- Use environment variables for sensitive configuration
- Follow SECURITY.md for reporting vulnerabilities

## 📦 Dependencies

Major Go dependencies:
- github.com/elastic/go-elasticsearch/v8
- github.com/prometheus/client_golang
- github.com/docker/docker (API client)
- gopkg.in/yaml.v2

See `go.mod` for complete list.

## 🌟 Version

Current version: Development (pre-release)

Check releases page for stable versions.

---

**Built with ❤️ using Go 1.23.0**
