package core

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// Mock input plugin for testing
type mockInput struct {
	logs   []*Log
	index  int
	logCh  chan<- *Log
	stopCh chan struct{}
	wg     sync.WaitGroup
}

func newMockInput(logs []*Log) *mockInput {
	return &mockInput{
		logs:   logs,
		index:  0,
		stopCh: make(chan struct{}),
	}
}

func (m *mockInput) Start() error {
	m.wg.Add(1)
	go m.run()
	return nil
}

func (m *mockInput) Stop() error {
	close(m.stopCh)
	m.wg.Wait()
	return nil
}

func (m *mockInput) SetLogChannel(ch chan<- *Log) {
	m.logCh = ch
}

func (m *mockInput) run() {
	defer m.wg.Done()

	for m.index < len(m.logs) {
		select {
		case m.logCh <- m.logs[m.index]:
			m.index++
			time.Sleep(1 * time.Millisecond) // Small delay to prevent overwhelming
		case <-m.stopCh:
			return
		}
	}
}

// Mock filter plugin for testing
type mockFilter struct {
	shouldPass bool
	callCount  int
	mu         sync.Mutex
}

func newMockFilter(shouldPass bool) *mockFilter {
	return &mockFilter{shouldPass: shouldPass}
}

func (m *mockFilter) Process(log *Log) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.callCount++
	return m.shouldPass
}

func (m *mockFilter) getCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.callCount
}

// Mock output plugin for testing
type mockOutput struct {
	logs      []*Log
	callCount int
	mu        sync.Mutex
}

func newMockOutput() *mockOutput {
	return &mockOutput{
		logs: make([]*Log, 0),
	}
}

func (m *mockOutput) Write(log *Log) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.logs = append(m.logs, log)
	m.callCount++
	return nil
}

func (m *mockOutput) Close() error {
	return nil
}

func (m *mockOutput) getLogs() []*Log {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]*Log, len(m.logs))
	copy(result, m.logs)
	return result
}

func (m *mockOutput) getCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.callCount
}

func TestNewEngine(t *testing.T) {
	engine := NewEngine()

	if engine.inputCh == nil {
		t.Error("inputCh should be initialized")
	}

	if engine.inputs == nil {
		t.Error("inputs map should be initialized")
	}

	if engine.filters == nil {
		t.Error("filters slice should be initialized")
	}

	if engine.pipelines == nil {
		t.Error("pipelines slice should be initialized")
	}

	if engine.ctx == nil {
		t.Error("context should be initialized")
	}

	if engine.cancel == nil {
		t.Error("cancel function should be initialized")
	}
}

func TestEngineAddInput(t *testing.T) {
	engine := NewEngine()
	input := newMockInput([]*Log{})

	engine.AddInput("test-input", input)

	if len(engine.inputs) != 1 {
		t.Errorf("Expected 1 input, got %d", len(engine.inputs))
	}

	if engine.inputs["test-input"] != input {
		t.Error("Input not added correctly")
	}
}

func TestEngineAddFilter(t *testing.T) {
	engine := NewEngine()
	filter := newMockFilter(true)

	engine.AddFilter(filter)

	if len(engine.filters) != 1 {
		t.Errorf("Expected 1 filter, got %d", len(engine.filters))
	}

	if engine.filters[0] != filter {
		t.Error("Filter not added correctly")
	}
}

func TestEngineAddOutput(t *testing.T) {
	engine := NewEngine()
	output := newMockOutput()

	engine.AddOutput(output)

	if len(engine.pipelines) != 1 {
		t.Errorf("Expected 1 pipeline, got %d", len(engine.pipelines))
	}

	if engine.pipelines[0].Output != output {
		t.Error("Output not added correctly")
	}
}

func TestEngineAddOutputPipeline(t *testing.T) {
	engine := NewEngine()
	output := newMockOutput()
	filter := newMockFilter(true)

	pipeline := &OutputPipeline{
		Name:    "test-pipeline",
		Output:  output,
		Filters: []FilterPlugin{filter},
		Sources: []string{"test-source"},
	}

	if err := engine.AddOutputPipeline(pipeline); err != nil {
		t.Fatalf("Failed to add output pipeline: %v", err)
	}

	if len(engine.pipelines) != 1 {
		t.Errorf("Expected 1 pipeline, got %d", len(engine.pipelines))
	}

	if engine.pipelines[0].Name != "test-pipeline" {
		t.Errorf("Expected pipeline name 'test-pipeline', got '%s'", engine.pipelines[0].Name)
	}
}

func TestEngineInputChannel(t *testing.T) {
	engine := NewEngine()
	ch := engine.InputChannel()

	if ch == nil {
		t.Error("InputChannel should not return nil")
	}

	// Test that we can send to the channel
	log := NewLog("info", "test")
	log.Source = "test"
	select {
	case ch <- log:
		// Success
	case <-time.After(100 * time.Millisecond):
		t.Error("Could not send to input channel")
	}
}

func TestEngineProcessingPipeline(t *testing.T) {
	engine := NewEngine()

	// Create test logs with source
	logs := []*Log{
		NewLog("error", "Error message"),
		NewLog("warn", "Warning message"),
		NewLog("info", "Info message"),
		NewLog("debug", "Debug message"),
	}
	for _, log := range logs {
		log.Source = "test-input"
	}

	// Setup mock input
	input := newMockInput(logs)
	engine.AddInput("test-input", input)

	// Setup filter that allows all
	filter := newMockFilter(true)

	// Setup mock output with pipeline
	output := newMockOutput()
	pipeline := &OutputPipeline{
		Name:    "test-output",
		Output:  output,
		Filters: []FilterPlugin{filter},
		Sources: []string{}, // Accept all sources
	}
	if err := engine.AddOutputPipeline(pipeline); err != nil {
		t.Fatalf("Failed to add output pipeline: %v", err)
	}

	// Start engine
	engine.Start()

	// Wait for processing to complete
	time.Sleep(100 * time.Millisecond)

	// Stop engine
	engine.Stop()

	// Verify results
	outputLogs := output.getLogs()
	if len(outputLogs) != len(logs) {
		t.Errorf("Expected %d output logs, got %d", len(logs), len(outputLogs))
	}

	// Verify filter was called for each log
	if filter.getCallCount() != len(logs) {
		t.Errorf("Expected filter to be called %d times, got %d", len(logs), filter.getCallCount())
	}

	// Verify output was called for each log
	if output.getCallCount() != len(logs) {
		t.Errorf("Expected output to be called %d times, got %d", len(logs), output.getCallCount())
	}
}

func TestEngineFilterBlocksLogs(t *testing.T) {
	engine := NewEngine()

	logs := []*Log{
		NewLog("error", "Error message"),
		NewLog("info", "Info message"),
	}
	for _, log := range logs {
		log.Source = "test-input"
	}

	// Setup mock input
	input := newMockInput(logs)
	engine.AddInput("test-input", input)

	// Setup filter that blocks all
	filter := newMockFilter(false)

	// Setup mock output with blocking filter
	output := newMockOutput()
	pipeline := &OutputPipeline{
		Name:    "test-output",
		Output:  output,
		Filters: []FilterPlugin{filter},
		Sources: []string{},
	}
	if err := engine.AddOutputPipeline(pipeline); err != nil {
		t.Fatalf("Failed to add output pipeline: %v", err)
	}

	// Start engine
	engine.Start()

	// Wait for processing
	time.Sleep(100 * time.Millisecond)

	// Stop engine
	engine.Stop()

	// Verify no logs reached output
	outputLogs := output.getLogs()
	if len(outputLogs) != 0 {
		t.Errorf("Expected 0 output logs when filter blocks all, got %d", len(outputLogs))
	}

	// Verify filter was still called
	if filter.getCallCount() != len(logs) {
		t.Errorf("Expected filter to be called %d times, got %d", len(logs), filter.getCallCount())
	}
}

func TestEngineSourceFiltering(t *testing.T) {
	engine := NewEngine()

	// Create logs from different sources
	logs1 := []*Log{NewLog("info", "From source1")}
	logs1[0].Source = "source1"

	logs2 := []*Log{NewLog("info", "From source2")}
	logs2[0].Source = "source2"

	// Setup inputs
	input1 := newMockInput(logs1)
	input2 := newMockInput(logs2)
	engine.AddInput("source1", input1)
	engine.AddInput("source2", input2)

	// Setup output that only accepts from source1
	output := newMockOutput()
	pipeline := &OutputPipeline{
		Name:    "selective-output",
		Output:  output,
		Filters: []FilterPlugin{},
		Sources: []string{"source1"}, // Only accept source1
	}
	if err := engine.AddOutputPipeline(pipeline); err != nil {
		t.Fatalf("Failed to add output pipeline: %v", err)
	}

	// Start engine
	engine.Start()

	// Wait for processing
	time.Sleep(100 * time.Millisecond)

	// Stop engine
	engine.Stop()

	// Verify only source1 logs reached output
	outputLogs := output.getLogs()
	if len(outputLogs) != 1 {
		t.Errorf("Expected 1 output log, got %d", len(outputLogs))
	}

	if len(outputLogs) > 0 && outputLogs[0].Source != "source1" {
		t.Errorf("Expected log from source1, got from %s", outputLogs[0].Source)
	}
}

func TestEngineMultipleOutputs(t *testing.T) {
	engine := NewEngine()

	logs := []*Log{NewLog("error", "Error message")}
	logs[0].Source = "test-input"

	// Setup mock input
	input := newMockInput(logs)
	engine.AddInput("test-input", input)

	// Setup multiple outputs
	output1 := newMockOutput()
	output2 := newMockOutput()

	pipeline1 := &OutputPipeline{
		Name:    "output1",
		Output:  output1,
		Filters: []FilterPlugin{},
		Sources: []string{},
	}
	pipeline2 := &OutputPipeline{
		Name:    "output2",
		Output:  output2,
		Filters: []FilterPlugin{},
		Sources: []string{},
	}

	if err := engine.AddOutputPipeline(pipeline1); err != nil {
		t.Fatalf("Failed to add pipeline1: %v", err)
	}
	if err := engine.AddOutputPipeline(pipeline2); err != nil {
		t.Fatalf("Failed to add pipeline2: %v", err)
	}

	// Start engine
	engine.Start()

	// Wait for processing
	time.Sleep(100 * time.Millisecond)

	// Stop engine
	engine.Stop()

	// Verify both outputs received the logs
	if output1.getCallCount() != len(logs) {
		t.Errorf("Expected output1 to receive %d logs, got %d", len(logs), output1.getCallCount())
	}
	if output2.getCallCount() != len(logs) {
		t.Errorf("Expected output2 to receive %d logs, got %d", len(logs), output2.getCallCount())
	}
}

func TestEngineStopBeforeStart(t *testing.T) {
	engine := NewEngine()

	// Should not panic
	engine.Stop()
}

func TestEngineDoubleStop(t *testing.T) {
	engine := NewEngine()

	// Setup minimal components
	input := newMockInput([]*Log{})
	engine.AddInput("test", input)

	output := newMockOutput()
	engine.AddOutput(output)

	// Start and stop
	engine.Start()
	time.Sleep(10 * time.Millisecond)
	engine.Stop()

	// Second stop should not panic
	engine.Stop()
}

func TestEngineEmptyComponents(t *testing.T) {
	engine := NewEngine()

	// No inputs, filters, or outputs added

	// Should start and stop without issues
	engine.Start()
	time.Sleep(10 * time.Millisecond)
	engine.Stop()
}

func TestEngineChannelBuffering(t *testing.T) {
	engine := NewEngine()

	// Verify channels are buffered
	log := NewLog("test", "message")
	log.Source = "test"
	select {
	case engine.inputCh <- log:
		// Should succeed immediately due to buffering
	default:
		t.Error("Input channel should be buffered")
	}
}

func TestEngineEnableAPI(t *testing.T) {
	engine := NewEngine()

	config := &Config{
		API: APIConfig{
			Enabled: true,
			Port:    9092,
		},
	}

	// Enable API
	err := engine.EnableAPI(config.API)
	if err != nil {
		t.Fatalf("EnableAPI should not return error: %v", err)
	}

	// API server is not started until Start() is called
	if engine.apiServer != nil {
		t.Error("API server should not be initialized until Start() is called")
	}

	if !engine.apiConfig.Enabled {
		t.Error("API config enabled should be true")
	}

	if engine.apiConfig.Port != 9092 {
		t.Error("API config port should be 9092")
	}

	// Start the engine to initialize the API server
	engine.Start()
	defer engine.Stop()

	if engine.apiServer == nil {
		t.Error("API server should be initialized after Start()")
	}
}

func TestEngineEnableAPIDisabled(t *testing.T) {
	engine := NewEngine()

	config := &Config{
		API: APIConfig{
			Enabled: false,
			Port:    9092,
		},
	}

	// Enable API with disabled config
	err := engine.EnableAPI(config.API)
	if err != nil {
		t.Fatalf("EnableAPI should not return error even when disabled: %v", err)
	}

	if engine.apiServer != nil {
		t.Error("API server should not be initialized when disabled")
	}

	if engine.apiConfig.Enabled {
		t.Error("API config should be disabled")
	}

	if engine.apiConfig.Port != 9092 {
		t.Error("API config port should be 9092")
	}
}

func TestEngineEnableAPINilConfig(t *testing.T) {
	engine := NewEngine()

	// Enable API with invalid config (port 0)
	invalidConfig := APIConfig{
		Enabled: true,
		Port:    0,
	}
	err := engine.EnableAPI(invalidConfig)
	if err == nil {
		t.Error("EnableAPI should return error with invalid config (port 0)")
	}
}

func TestEngineHandleHealth(t *testing.T) {
	engine := NewEngine()

	// Create a test HTTP request
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	// Call the handler
	engine.handleHealth(w, req)

	// Check response
	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	body := w.Body.String()
	var healthResp map[string]interface{}
	if err := json.Unmarshal([]byte(body), &healthResp); err != nil {
		t.Fatalf("Failed to parse JSON response: %v", err)
	}

	if healthResp["status"] != "ok" {
		t.Errorf("Expected status 'ok', got '%v'", healthResp["status"])
	}

	if _, exists := healthResp["time"]; !exists {
		t.Error("Response should contain 'time' field")
	}
}

func TestEngineHandleMetrics(t *testing.T) {
	engine := NewEngine()

	// Setup some mock data
	engine.totalLogsProcessed = 42

	// Configure output buffer to enable buffer stats
	bufferConfig := OutputBufferConfig{
		Enabled:       true,
		Dir:           "/tmp/buffer",
		MaxQueueSize:  100,
		MaxRetries:    3,
		RetryInterval: time.Second,
		MaxRetryDelay: time.Minute,
		FlushInterval: time.Minute,
	}
	engine.SetOutputBufferConfig(bufferConfig)

	// Create a test HTTP request
	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()

	// Call the handler
	engine.handleMetrics(w, req)

	// Check response
	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	body := w.Body.String()
	var metricsResp map[string]interface{}
	if err := json.Unmarshal([]byte(body), &metricsResp); err != nil {
		t.Fatalf("Failed to parse JSON response: %v", err)
	}

	if _, exists := metricsResp["buffer_enabled"]; !exists {
		t.Error("Response should contain 'buffer_enabled' field")
	}

	if _, exists := metricsResp["buffer_stats"]; !exists {
		t.Error("Response should contain 'buffer_stats' field")
	}
}

func TestEngineHandleStatus(t *testing.T) {
	engine := NewEngine()

	// Setup some mock data
	engine.totalLogsProcessed = 100
	engine.startTime = time.Now().Add(-time.Hour) // Started 1 hour ago

	// Add some mock inputs and outputs
	input := newMockInput([]*Log{})
	engine.AddInput("test-input", input)

	output := newMockOutput()
	pipeline := &OutputPipeline{
		Name:    "test-output",
		Output:  output,
		Filters: []FilterPlugin{},
		Sources: []string{},
	}
	if err := engine.AddOutputPipeline(pipeline); err != nil {
		t.Fatalf("Failed to add output pipeline: %v", err)
	}

	// Create a test HTTP request
	req := httptest.NewRequest("GET", "/status", nil)
	w := httptest.NewRecorder()

	// Call the handler
	engine.handleStatus(w, req)

	// Check response
	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	body := w.Body.String()
	var statusResp map[string]interface{}
	if err := json.Unmarshal([]byte(body), &statusResp); err != nil {
		t.Fatalf("Failed to parse JSON response: %v", err)
	}

	// Check API section
	if apiSection, exists := statusResp["api"]; exists {
		apiMap := apiSection.(map[string]interface{})
		if apiMap["enabled"] != false { // API not enabled in this test
			t.Error("API should be disabled in this test")
		}
	}

	// Check engine section
	if engineSection, exists := statusResp["engine"]; exists {
		engineMap := engineSection.(map[string]interface{})
		if engineMap["total_logs_processed"] != float64(100) {
			t.Errorf("Expected 100 total logs processed, got %v", engineMap["total_logs_processed"])
		}
		if engineMap["status"] != "running" {
			t.Errorf("Expected status 'running', got '%v'", engineMap["status"])
		}
		if _, exists := engineMap["uptime_seconds"]; !exists {
			t.Error("Response should contain 'uptime_seconds' field")
		}
	}

	// Check inputs section
	if inputsSection, exists := statusResp["inputs"]; exists {
		inputsMap := inputsSection.(map[string]interface{})
		if inputsMap["count"] != float64(1) {
			t.Errorf("Expected 1 input, got %v", inputsMap["count"])
		}
		if inputsSlice, exists := inputsMap["names"]; exists {
			names := inputsSlice.([]interface{})
			if len(names) != 1 || names[0] != "test-input" {
				t.Errorf("Expected input names ['test-input'], got %v", names)
			}
		}
	}

	// Check outputs section
	if outputsSection, exists := statusResp["outputs"]; exists {
		outputsMap := outputsSection.(map[string]interface{})
		if outputsMap["count"] != float64(1) {
			t.Errorf("Expected 1 output, got %v", outputsMap["count"])
		}
		if pipelinesSlice, exists := outputsMap["pipelines"]; exists {
			pipelines := pipelinesSlice.([]interface{})
			if len(pipelines) != 1 {
				t.Errorf("Expected 1 pipeline, got %d", len(pipelines))
			}
		}
	}
}

func TestEngineHandleStatusWithAPIEnabled(t *testing.T) {
	engine := NewEngine()

	// Enable API
	config := &Config{
		API: APIConfig{
			Enabled: true,
			Port:    9092,
		},
	}
	if err := engine.EnableAPI(config.API); err != nil {
		t.Fatalf("Failed to enable API: %v", err)
	}

	// Create a test HTTP request
	req := httptest.NewRequest("GET", "/status", nil)
	w := httptest.NewRecorder()

	// Call the handler
	engine.handleStatus(w, req)

	// Check response
	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	body := w.Body.String()
	var statusResp map[string]interface{}
	if err := json.Unmarshal([]byte(body), &statusResp); err != nil {
		t.Fatalf("Failed to parse JSON response: %v", err)
	}

	// Check API section when enabled
	if apiSection, exists := statusResp["api"]; exists {
		apiMap := apiSection.(map[string]interface{})
		if apiMap["enabled"] != true {
			t.Error("API should be enabled")
		}
		if apiMap["port"] != float64(9092) {
			t.Errorf("Expected port 9092, got %v", apiMap["port"])
		}
	}
}
