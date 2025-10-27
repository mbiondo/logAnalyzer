package core

import (
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

	if engine == nil {
		t.Fatal("NewEngine should not return nil")
	}

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

	engine.AddOutputPipeline(pipeline)

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
	engine.AddOutputPipeline(pipeline)

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
	engine.AddOutputPipeline(pipeline)

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
	engine.AddOutputPipeline(pipeline)

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

	engine.AddOutputPipeline(pipeline1)
	engine.AddOutputPipeline(pipeline2)

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
