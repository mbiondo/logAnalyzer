package core

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

// Mock plugins for testing
type mockInputPlugin struct {
	started bool
}

func (m *mockInputPlugin) Start() error {
	m.started = true
	return nil
}

func (m *mockInputPlugin) Stop() error {
	m.started = false
	return nil
}

func (m *mockInputPlugin) SetLogChannel(ch chan<- *Log) {}

type mockOutputPlugin struct {
	logs []*Log
}

func (m *mockOutputPlugin) Write(log *Log) error {
	m.logs = append(m.logs, log)
	return nil
}

func (m *mockOutputPlugin) Close() error {
	return nil
}

type mockFilterPlugin struct {
	shouldPass bool
}

func (m *mockFilterPlugin) Process(log *Log) bool {
	return m.shouldPass
}

// Factory functions for mock plugins
func mockInputFactory(config map[string]any) (any, error) {
	return &mockInputPlugin{}, nil
}

func mockOutputFactory(config map[string]any) (any, error) {
	return &mockOutputPlugin{logs: make([]*Log, 0)}, nil
}

func mockFilterFactory(config map[string]any) (any, error) {
	shouldPass := true
	if pass, ok := config["pass"].(bool); ok {
		shouldPass = pass
	}
	return &mockFilterPlugin{shouldPass: shouldPass}, nil
}

func mockErrorFactory(config map[string]any) (any, error) {
	return nil, fmt.Errorf("mock error")
}

func mockInvalidTypeFactory(config map[string]any) (any, error) {
	return "not a plugin", nil
}

// TestRegisterInputPlugin tests input plugin registration
func TestRegisterInputPlugin(t *testing.T) {
	// Clear registry before test
	registry.mu.Lock()
	registry.inputs = make(map[string]PluginFactory)
	registry.mu.Unlock()

	RegisterInputPlugin("mock-input", mockInputFactory)

	plugins := ListInputPlugins()
	if len(plugins) != 1 {
		t.Errorf("Expected 1 input plugin, got %d", len(plugins))
	}

	if plugins[0] != "mock-input" {
		t.Errorf("Expected plugin name 'mock-input', got '%s'", plugins[0])
	}
}

// TestRegisterOutputPlugin tests output plugin registration
func TestRegisterOutputPlugin(t *testing.T) {
	// Clear registry before test
	registry.mu.Lock()
	registry.outputs = make(map[string]PluginFactory)
	registry.mu.Unlock()

	RegisterOutputPlugin("mock-output", mockOutputFactory)

	plugins := ListOutputPlugins()
	if len(plugins) != 1 {
		t.Errorf("Expected 1 output plugin, got %d", len(plugins))
	}

	if plugins[0] != "mock-output" {
		t.Errorf("Expected plugin name 'mock-output', got '%s'", plugins[0])
	}
}

// TestRegisterFilterPlugin tests filter plugin registration
func TestRegisterFilterPlugin(t *testing.T) {
	// Clear registry before test
	registry.mu.Lock()
	registry.filters = make(map[string]PluginFactory)
	registry.mu.Unlock()

	RegisterFilterPlugin("mock-filter", mockFilterFactory)

	plugins := ListFilterPlugins()
	if len(plugins) != 1 {
		t.Errorf("Expected 1 filter plugin, got %d", len(plugins))
	}

	if plugins[0] != "mock-filter" {
		t.Errorf("Expected plugin name 'mock-filter', got '%s'", plugins[0])
	}
}

// TestCreateInputPlugin tests input plugin creation
func TestCreateInputPlugin(t *testing.T) {
	// Setup
	registry.mu.Lock()
	registry.inputs = make(map[string]PluginFactory)
	registry.mu.Unlock()
	RegisterInputPlugin("mock-input", mockInputFactory)

	// Test successful creation
	plugin, err := CreateInputPlugin("mock-input", map[string]any{})
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if plugin == nil {
		t.Error("Expected plugin instance, got nil")
	}

	mockPlugin, ok := plugin.(*mockInputPlugin)
	if !ok {
		t.Error("Expected mockInputPlugin instance")
	}

	// Test plugin functionality
	err = mockPlugin.Start()
	if err != nil {
		t.Errorf("Expected no error starting plugin, got %v", err)
	}

	if !mockPlugin.started {
		t.Error("Expected plugin to be started")
	}
}

// TestCreateOutputPlugin tests output plugin creation
func TestCreateOutputPlugin(t *testing.T) {
	// Setup
	registry.mu.Lock()
	registry.outputs = make(map[string]PluginFactory)
	registry.mu.Unlock()
	RegisterOutputPlugin("mock-output", mockOutputFactory)

	// Test successful creation
	plugin, err := CreateOutputPlugin("mock-output", map[string]any{})
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if plugin == nil {
		t.Error("Expected plugin instance, got nil")
	}

	mockPlugin, ok := plugin.(*mockOutputPlugin)
	if !ok {
		t.Error("Expected mockOutputPlugin instance")
	}

	// Test plugin functionality
	testLog := &Log{
		Timestamp: time.Now(),
		Level:     "INFO",
		Message:   "test message",
	}

	err = mockPlugin.Write(testLog)
	if err != nil {
		t.Errorf("Expected no error writing log, got %v", err)
	}

	if len(mockPlugin.logs) != 1 {
		t.Errorf("Expected 1 log, got %d", len(mockPlugin.logs))
	}
}

// TestCreateFilterPlugin tests filter plugin creation
func TestCreateFilterPlugin(t *testing.T) {
	// Setup
	registry.mu.Lock()
	registry.filters = make(map[string]PluginFactory)
	registry.mu.Unlock()
	RegisterFilterPlugin("mock-filter", mockFilterFactory)

	// Test successful creation with pass=true
	plugin, err := CreateFilterPlugin("mock-filter", map[string]any{"pass": true})
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if plugin == nil {
		t.Error("Expected plugin instance, got nil")
	}

	mockPlugin, ok := plugin.(*mockFilterPlugin)
	if !ok {
		t.Error("Expected mockFilterPlugin instance")
	}

	// Test plugin functionality
	testLog := &Log{
		Timestamp: time.Now(),
		Level:     "INFO",
		Message:   "test message",
	}

	if !mockPlugin.Process(testLog) {
		t.Error("Expected filter to pass log")
	}
}

// TestCreatePluginUnknownType tests error handling for unknown plugin types
func TestCreatePluginUnknownType(t *testing.T) {
	// Clear registries
	registry.mu.Lock()
	registry.inputs = make(map[string]PluginFactory)
	registry.outputs = make(map[string]PluginFactory)
	registry.filters = make(map[string]PluginFactory)
	registry.mu.Unlock()

	// Test unknown input plugin
	_, err := CreateInputPlugin("unknown", map[string]any{})
	if err == nil {
		t.Error("Expected error for unknown input plugin")
	}

	// Test unknown output plugin
	_, err = CreateOutputPlugin("unknown", map[string]any{})
	if err == nil {
		t.Error("Expected error for unknown output plugin")
	}

	// Test unknown filter plugin
	_, err = CreateFilterPlugin("unknown", map[string]any{})
	if err == nil {
		t.Error("Expected error for unknown filter plugin")
	}
}

// TestCreatePluginFactoryError tests error handling when factory returns error
func TestCreatePluginFactoryError(t *testing.T) {
	// Setup
	registry.mu.Lock()
	registry.inputs = make(map[string]PluginFactory)
	registry.outputs = make(map[string]PluginFactory)
	registry.filters = make(map[string]PluginFactory)
	registry.mu.Unlock()

	RegisterInputPlugin("error-input", mockErrorFactory)
	RegisterOutputPlugin("error-output", mockErrorFactory)
	RegisterFilterPlugin("error-filter", mockErrorFactory)

	// Test input plugin error
	_, err := CreateInputPlugin("error-input", map[string]any{})
	if err == nil {
		t.Error("Expected error from input plugin factory")
	}

	// Test output plugin error
	_, err = CreateOutputPlugin("error-output", map[string]any{})
	if err == nil {
		t.Error("Expected error from output plugin factory")
	}

	// Test filter plugin error
	_, err = CreateFilterPlugin("error-filter", map[string]any{})
	if err == nil {
		t.Error("Expected error from filter plugin factory")
	}
}

// TestCreatePluginInvalidType tests error handling when plugin doesn't implement interface
func TestCreatePluginInvalidType(t *testing.T) {
	// Setup
	registry.mu.Lock()
	registry.inputs = make(map[string]PluginFactory)
	registry.outputs = make(map[string]PluginFactory)
	registry.filters = make(map[string]PluginFactory)
	registry.mu.Unlock()

	RegisterInputPlugin("invalid-input", mockInvalidTypeFactory)
	RegisterOutputPlugin("invalid-output", mockInvalidTypeFactory)
	RegisterFilterPlugin("invalid-filter", mockInvalidTypeFactory)

	// Test input plugin invalid type
	_, err := CreateInputPlugin("invalid-input", map[string]any{})
	if err == nil {
		t.Error("Expected error for invalid input plugin type")
	}

	// Test output plugin invalid type
	_, err = CreateOutputPlugin("invalid-output", map[string]any{})
	if err == nil {
		t.Error("Expected error for invalid output plugin type")
	}

	// Test filter plugin invalid type
	_, err = CreateFilterPlugin("invalid-filter", map[string]any{})
	if err == nil {
		t.Error("Expected error for invalid filter plugin type")
	}
}

// TestConcurrentRegistration tests thread safety of plugin registration
func TestConcurrentRegistration(t *testing.T) {
	// Clear registry
	registry.mu.Lock()
	registry.inputs = make(map[string]PluginFactory)
	registry.outputs = make(map[string]PluginFactory)
	registry.filters = make(map[string]PluginFactory)
	registry.mu.Unlock()

	var wg sync.WaitGroup
	concurrency := 10

	// Register input plugins concurrently
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			name := fmt.Sprintf("input-%d", index)
			RegisterInputPlugin(name, mockInputFactory)
		}(i)
	}

	// Register output plugins concurrently
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			name := fmt.Sprintf("output-%d", index)
			RegisterOutputPlugin(name, mockOutputFactory)
		}(i)
	}

	// Register filter plugins concurrently
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			name := fmt.Sprintf("filter-%d", index)
			RegisterFilterPlugin(name, mockFilterFactory)
		}(i)
	}

	wg.Wait()

	// Verify all plugins were registered
	inputs := ListInputPlugins()
	outputs := ListOutputPlugins()
	filters := ListFilterPlugins()

	if len(inputs) != concurrency {
		t.Errorf("Expected %d input plugins, got %d", concurrency, len(inputs))
	}

	if len(outputs) != concurrency {
		t.Errorf("Expected %d output plugins, got %d", concurrency, len(outputs))
	}

	if len(filters) != concurrency {
		t.Errorf("Expected %d filter plugins, got %d", concurrency, len(filters))
	}
}

// TestConcurrentCreation tests thread safety of plugin creation
func TestConcurrentCreation(t *testing.T) {
	// Setup
	registry.mu.Lock()
	registry.inputs = make(map[string]PluginFactory)
	registry.outputs = make(map[string]PluginFactory)
	registry.filters = make(map[string]PluginFactory)
	registry.mu.Unlock()

	RegisterInputPlugin("mock-input", mockInputFactory)
	RegisterOutputPlugin("mock-output", mockOutputFactory)
	RegisterFilterPlugin("mock-filter", mockFilterFactory)

	var wg sync.WaitGroup
	concurrency := 100
	errors := make(chan error, concurrency*3)

	// Create plugins concurrently
	for i := 0; i < concurrency; i++ {
		// Input plugins
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := CreateInputPlugin("mock-input", map[string]any{})
			if err != nil {
				errors <- err
			}
		}()

		// Output plugins
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := CreateOutputPlugin("mock-output", map[string]any{})
			if err != nil {
				errors <- err
			}
		}()

		// Filter plugins
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := CreateFilterPlugin("mock-filter", map[string]any{})
			if err != nil {
				errors <- err
			}
		}()
	}

	wg.Wait()
	close(errors)

	// Check for errors
	for err := range errors {
		t.Errorf("Unexpected error during concurrent creation: %v", err)
	}
}

// TestListPlugins tests listing all registered plugins
func TestListPlugins(t *testing.T) {
	// Clear and setup
	registry.mu.Lock()
	registry.inputs = make(map[string]PluginFactory)
	registry.outputs = make(map[string]PluginFactory)
	registry.filters = make(map[string]PluginFactory)
	registry.mu.Unlock()

	RegisterInputPlugin("input1", mockInputFactory)
	RegisterInputPlugin("input2", mockInputFactory)
	RegisterOutputPlugin("output1", mockOutputFactory)
	RegisterOutputPlugin("output2", mockOutputFactory)
	RegisterOutputPlugin("output3", mockOutputFactory)
	RegisterFilterPlugin("filter1", mockFilterFactory)

	inputs := ListInputPlugins()
	outputs := ListOutputPlugins()
	filters := ListFilterPlugins()

	if len(inputs) != 2 {
		t.Errorf("Expected 2 input plugins, got %d", len(inputs))
	}

	if len(outputs) != 3 {
		t.Errorf("Expected 3 output plugins, got %d", len(outputs))
	}

	if len(filters) != 1 {
		t.Errorf("Expected 1 filter plugin, got %d", len(filters))
	}
}

// TestPluginOverwrite tests that registering a plugin with same name overwrites it
func TestPluginOverwrite(t *testing.T) {
	// Clear registry
	registry.mu.Lock()
	registry.inputs = make(map[string]PluginFactory)
	registry.mu.Unlock()

	firstCalled := false
	secondCalled := false

	firstFactory := func(config map[string]any) (any, error) {
		firstCalled = true
		return &mockInputPlugin{}, nil
	}

	secondFactory := func(config map[string]any) (any, error) {
		secondCalled = true
		return &mockInputPlugin{}, nil
	}

	RegisterInputPlugin("test", firstFactory)
	RegisterInputPlugin("test", secondFactory)

	_, err := CreateInputPlugin("test", map[string]any{})
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if firstCalled {
		t.Error("First factory should not have been called (was overwritten)")
	}

	if !secondCalled {
		t.Error("Second factory should have been called")
	}
}
