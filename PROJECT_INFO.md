# LogAnalyzer - Project Information

## 📦 Project Structure

```
logAnalyzer/
├── cmd/                        # Application entry point
│   └── main.go                 # Main application
├── core/                       # Core engine and types
│   ├── config.go               # Configuration structures
│   ├── engine.go               # Pipeline processing engine
│   ├── log.go                  # Log data structure
│   └── registry.go             # Plugin registration system
├── plugins/                    # Plugin implementations
│   ├── input/                  # Input plugins (Docker, HTTP, File)
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
│   ├── loganalyzer.yaml        # Pipeline configuration
│   ├── prometheus.yml          # Prometheus scrape config
│   ├── grafana/                # Grafana dashboards & datasources
│   │   ├── dashboards/
│   │   │   └── loganalyzer-dashboard.json
│   │   └── provisioning/
│   │       ├── datasources/
│   │       │   └── datasources.yaml
│   │       └── dashboards/
│   │           └── dashboard-provider.yaml
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

All tests are located alongside their respective code files:

```bash
# Run all tests
go test ./...

# Run specific package tests
go test ./core -v
go test ./plugins/input/docker -v

# Run with coverage
go test -cover ./...
```

## 📝 Configuration

Configuration is YAML-based with three main sections:

1. **input.inputs[]**: Array of input plugin configurations
2. **output.outputs[]**: Array of output pipeline configurations
3. **filters**: Per-pipeline filter definitions

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
- **examples/README.md**: Complete example setup guide
- **CODE_OF_CONDUCT.md**: Community guidelines
- **CONTRIBUTING.md**: How to contribute
- **SECURITY.md**: Security policy and reporting

## 🎯 Key Features

- ✅ Pipeline architecture with source-based routing
- ✅ Per-output filtering and configuration
- ✅ Dynamic plugin registration system
- ✅ Container filtering (string or array)
- ✅ Multiple inputs and outputs simultaneously
- ✅ Elasticsearch bulk indexing
- ✅ Prometheus metrics export
- ✅ Complete Docker example with Grafana dashboards
- ✅ Cross-platform support (Windows, Linux, Mac)

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
