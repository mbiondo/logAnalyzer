# Contributing to LogAnalyzer

Thank you for your interest in contributing to LogAnalyzer! 🎉

## Ways to Contribute

- 🐛 Report bugs
- 💡 Suggest new features
- 🔌 Create new plugins
- 📖 Improve documentation
- ✅ Write tests
- 🎨 Improve code quality

## Getting Started

### Prerequisites

- Go 1.21 or higher
- Docker (for integration tests)
- Git

### Development Setup

1. **Fork and clone the repository**
   ```bash
   git clone https://github.com/mbiondo/logAnalyzer.git
   cd logAnalyzer
   ```

2. **Install dependencies**
   ```bash
   go mod download
   ```

3. **Run tests**
   ```bash
   go test ./...
   ```

4. **Build the project**
   ```bash
   go build -o loganalyzer ./cmd
   ```

## Project Structure

```
logAnalyzer/
├── cmd/                    # Application entry point
├── core/                   # Core engine and framework
│   ├── engine.go          # Main processing engine
│   ├── config.go          # Configuration management
│   ├── registry.go        # Plugin registry system
│   └── log.go             # Log data structures
├── plugins/               # Plugin implementations
│   ├── input/            # Input plugins
│   ├── filter/           # Filter plugins
│   └── output/           # Output plugins
├── config/               # Example configurations
├── docs/                 # Documentation
└── .github/              # GitHub templates and workflows
```

## Creating a New Plugin

LogAnalyzer's plugin system is **completely dynamic** - no core code changes needed!

### 1. Create Plugin Directory

```bash
mkdir -p plugins/output/myplugin
cd plugins/output/myplugin
```

### 2. Implement the Interface

```go
package myplugin

import "github.com/mbiondo/logAnalyzer/core"

// Config struct for type-safe configuration
type Config struct {
    Host string `yaml:"host"`
    Port int    `yaml:"port"`
}

// MyPlugin implements core.OutputPlugin interface
type MyPlugin struct {
    config Config
}

func (p *MyPlugin) Write(log *core.Log) error {
    // Your implementation
    return nil
}

func (p *MyPlugin) Close() error {
    return nil
}

// Factory function for the registry
func NewMyPluginFromConfig(config map[string]any) (any, error) {
    var cfg Config
    if err := core.GetPluginConfig(config, &cfg); err != nil {
        return nil, err
    }
    return &MyPlugin{config: cfg}, nil
}

// Auto-register on import
func init() {
    core.RegisterOutputPlugin("myplugin", NewMyPluginFromConfig)
}
```

### 3. Import in main.go

```go
import (
    _ "github.com/mbiondo/logAnalyzer/plugins/output/myplugin"  // Auto-registers!
)
```

### 4. Add Tests

Create `myplugin_test.go`:
```go
package myplugin

import "testing"

func TestMyPlugin(t *testing.T) {
    // Your tests
}
```

### 5. Update Documentation

Add your plugin to the README.md plugin list.

## Code Style Guidelines

### Go Style

- Follow [Effective Go](https://go.dev/doc/effective_go)
- Use `gofmt` for formatting
- Run `go vet` before committing
- Keep functions small and focused
- Write descriptive comments

### Naming Conventions

- **Plugins**: lowercase package names (e.g., `slack`, `elasticsearch`)
- **Config structs**: Always named `Config` within the plugin package
- **Factories**: Always named `New[PluginName]FromConfig`
- **Tests**: Use `TestFunctionName` pattern

### Error Handling

```go
// Good: Descriptive errors with context
if err != nil {
    return fmt.Errorf("failed to connect to %s: %w", host, err)
}

// Bad: Generic errors
if err != nil {
    return err
}
```

## Testing

### Unit Tests

```bash
# Run all tests
go test ./...

# Run tests for specific package
go test ./plugins/output/slack

# Run with coverage
go test -cover ./...

# Verbose output
go test -v ./...
```

### Integration Tests

```bash
# Build and test with Docker
docker build -t loganalyzer:test .
docker run --rm loganalyzer:test
```

## Pull Request Process

1. **Create a branch**
   ```bash
   git checkout -b feature/my-new-plugin
   ```

2. **Make your changes**
   - Write clean, documented code
   - Add tests for new functionality
   - Update documentation

3. **Verify everything works**
   ```bash
   go test ./...
   go build ./cmd
   ```

4. **Commit with clear messages**
   ```bash
   git commit -m "feat: add Elasticsearch output plugin"
   ```
   
   Use conventional commits:
   - `feat:` New feature
   - `fix:` Bug fix
   - `docs:` Documentation changes
   - `test:` Adding tests
   - `refactor:` Code refactoring
   - `chore:` Maintenance tasks

5. **Push and create PR**
   ```bash
   git push origin feature/my-new-plugin
   ```

6. **PR Checklist**
   - [ ] Tests pass (`go test ./...`)
   - [ ] Code follows style guidelines
   - [ ] Documentation updated
   - [ ] No breaking changes (or clearly documented)
   - [ ] Commit messages are clear

## Plugin Development Best Practices

### Configuration

- Always provide sensible defaults
- Validate configuration in the factory function
- Use type-safe config structs

```go
func NewMyPluginFromConfig(config map[string]any) (any, error) {
    var cfg Config
    if err := core.GetPluginConfig(config, &cfg); err != nil {
        return nil, err
    }
    
    // Validate required fields
    if cfg.Host == "" {
        return nil, fmt.Errorf("host is required")
    }
    
    // Set defaults
    if cfg.Port == 0 {
        cfg.Port = 8080
    }
    
    return NewMyPlugin(cfg), nil
}
```

### Error Handling

- Return descriptive errors
- Don't panic unless absolutely necessary
- Log errors appropriately

### Resource Management

- Clean up resources in `Close()`
- Use mutexes for concurrent access
- Handle graceful shutdown

### Testing

- Test both success and failure cases
- Mock external dependencies
- Test concurrent usage if applicable

## Documentation

- Add docstrings to exported functions
- Update README.md with new plugins
- Provide configuration examples
- Include troubleshooting tips

## Community Guidelines

- Be respectful and inclusive
- Help others learn and grow
- Provide constructive feedback
- Celebrate contributions

## Questions?

- 💬 Open a [Discussion](https://github.com/mbiondo/logAnalyzer/discussions)
- 🐛 Report issues on [GitHub Issues](https://github.com/mbiondo/logAnalyzer/issues)

Thank you for contributing! 🚀
