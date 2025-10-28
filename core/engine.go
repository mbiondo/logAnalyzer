package core

import (
	"context"
	"fmt"
	"log"
	"sync"
)

// OutputPipeline represents an output with its own filters and source restrictions
type OutputPipeline struct {
	Name    string         // Optional name for this output
	Output  OutputPlugin   // The output plugin
	Buffer  *OutputBuffer  // Optional output buffer with retry logic
	Filters []FilterPlugin // Filters specific to this output
	Sources []string       // Input sources to accept (empty = all)
}

// Engine represents the core log processing engine
type Engine struct {
	inputCh      chan *Log
	inputs       map[string]InputPlugin // Map of input name -> plugin
	filters      []FilterPlugin         // Global filters (deprecated, but kept for backward compatibility)
	pipelines    []*OutputPipeline      // Output pipelines with their own filters
	persistence  *Persistence           // Persistence layer for WAL
	bufferConfig OutputBufferConfig     // Output buffer configuration
	wg           sync.WaitGroup
	ctx          context.Context
	cancel       context.CancelFunc
	stopped      bool       // Flag to prevent multiple stops
	mu           sync.Mutex // Protects stopped flag
}

// InputPlugin interface for log input sources
type InputPlugin interface {
	Start() error
	Stop() error
	SetLogChannel(ch chan<- *Log)
}

// FilterPlugin interface for log filtering/processing
type FilterPlugin interface {
	Process(log *Log) bool // Returns true if log should be kept
}

// OutputPlugin interface for log output destinations
type OutputPlugin interface {
	Write(log *Log) error
	Close() error
}

// NewEngine creates a new log processing engine
func NewEngine() *Engine {
	ctx, cancel := context.WithCancel(context.Background())
	return &Engine{
		inputCh:   make(chan *Log, 100), // Buffered channel for inputs
		inputs:    make(map[string]InputPlugin),
		filters:   []FilterPlugin{},
		pipelines: []*OutputPipeline{},
		ctx:       ctx,
		cancel:    cancel,
	}
}

// SetPersistence configures the persistence layer for the engine
func (e *Engine) SetPersistence(config PersistenceConfig) error {
	p, err := NewPersistence(config)
	if err != nil {
		return fmt.Errorf("failed to initialize persistence: %w", err)
	}
	e.persistence = p
	return nil
}

// SetOutputBufferConfig configures output buffering for all outputs
func (e *Engine) SetOutputBufferConfig(config OutputBufferConfig) {
	e.bufferConfig = config
}

// AddInput adds an input plugin to the engine with a name
func (e *Engine) AddInput(name string, input InputPlugin) {
	input.SetLogChannel(e.inputCh)
	e.inputs[name] = input
}

// AddInputAnonymous adds an input plugin without a specific name (for backward compatibility)
func (e *Engine) AddInputAnonymous(input InputPlugin) {
	name := fmt.Sprintf("input-%d", len(e.inputs))
	e.AddInput(name, input)
}

// AddFilter adds a global filter plugin to the engine (deprecated)
func (e *Engine) AddFilter(filter FilterPlugin) {
	e.filters = append(e.filters, filter)
}

// AddOutput adds an output plugin to the engine (deprecated - use AddOutputPipeline)
func (e *Engine) AddOutput(output OutputPlugin) {
	pipeline := &OutputPipeline{
		Name:    fmt.Sprintf("output-%d", len(e.pipelines)),
		Output:  output,
		Filters: []FilterPlugin{},
		Sources: []string{}, // Accept from all sources
	}
	e.pipelines = append(e.pipelines, pipeline)
}

// AddOutputPipeline adds an output pipeline with filters and source restrictions
func (e *Engine) AddOutputPipeline(pipeline *OutputPipeline) error {
	// Wrap output with buffer if configured
	if e.bufferConfig.Enabled {
		buffer, err := NewOutputBuffer(pipeline.Name, pipeline.Output, e.bufferConfig)
		if err != nil {
			return fmt.Errorf("failed to create output buffer for %s: %w", pipeline.Name, err)
		}
		pipeline.Buffer = buffer
	}

	e.pipelines = append(e.pipelines, pipeline)
	return nil
}

// InputChannel returns the channel for input plugins to send logs
func (e *Engine) InputChannel() chan<- *Log {
	return e.inputCh
}

// Start begins the log processing
func (e *Engine) Start() {
	// Recover persisted logs if persistence is enabled
	if e.persistence != nil {
		recoveryCh, err := e.persistence.Recover()
		if err != nil {
			log.Printf("Error starting recovery: %v", err)
		} else {
			e.wg.Add(1)
			go e.processRecoveredLogs(recoveryCh)
		}
	}

	// Start all input plugins
	for name, input := range e.inputs {
		if err := input.Start(); err != nil {
			log.Printf("Error starting input plugin %s: %v", name, err)
		}
	}

	e.wg.Add(1)
	go e.processLogs()
	log.Println("LogAnalyzer engine started")
}

// processRecoveredLogs handles logs recovered from persistence
func (e *Engine) processRecoveredLogs(recoveryCh <-chan *Log) {
	defer e.wg.Done()
	for logEntry := range recoveryCh {
		log.Printf("[ENGINE] Recovered log from WAL: %s - %s", logEntry.Level, logEntry.Message)
		// Send recovered logs directly to the processing pipeline
		select {
		case e.inputCh <- logEntry:
		case <-e.ctx.Done():
			return
		}
	}
	log.Println("[ENGINE] Log recovery complete")
}

// Stop gracefully shuts down the engine
func (e *Engine) Stop() {
	e.mu.Lock()
	if e.stopped {
		e.mu.Unlock()
		return // Already stopped
	}
	e.stopped = true
	e.mu.Unlock()

	// Signal context cancellation first
	e.cancel()

	// Stop all inputs first to stop new logs from coming
	for name, input := range e.inputs {
		if err := input.Stop(); err != nil {
			log.Printf("Error stopping input plugin %s: %v", name, err)
		}
	}

	// Close the input channel after inputs are stopped
	close(e.inputCh)

	// Wait for processing goroutine to finish
	e.wg.Wait()

	// Close persistence layer
	if e.persistence != nil {
		if err := e.persistence.Close(); err != nil {
			log.Printf("Error closing persistence: %v", err)
		}
	}

	// Close all outputs
	for _, pipeline := range e.pipelines {
		// Close buffer if exists
		if pipeline.Buffer != nil {
			if err := pipeline.Buffer.Close(); err != nil {
				log.Printf("Error closing buffer for %s: %v", pipeline.Name, err)
			}
		} else {
			// Close output directly if no buffer
			if err := pipeline.Output.Close(); err != nil {
				log.Printf("Error closing output %s: %v", pipeline.Name, err)
			}
		}
	}
	log.Println("LogAnalyzer engine stopped")
}

// ReloadConfig reloads the engine with new configuration
// This method stops the current engine and recreates it with new config
func (e *Engine) ReloadConfig(newConfig *Config, createInputFunc func(string, string, map[string]any, *Engine), createOutputFunc func(string, PluginDefinition, *Engine)) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	log.Println("Reloading engine configuration...")

	// Stop current engine
	e.cancel()

	// Stop all inputs first to stop new logs from coming
	for name, input := range e.inputs {
		if err := input.Stop(); err != nil {
			log.Printf("Error stopping input plugin %s: %v", name, err)
		}
	}

	// Close the input channel after inputs are stopped
	close(e.inputCh)

	// Wait for processing goroutine to finish
	e.wg.Wait()

	// Close all outputs
	for _, pipeline := range e.pipelines {
		if err := pipeline.Output.Close(); err != nil {
			log.Printf("Error closing output %s: %v", pipeline.Name, err)
		}
	}

	// Recreate engine with new context
	ctx, cancel := context.WithCancel(context.Background())
	e.ctx = ctx
	e.cancel = cancel
	e.inputCh = make(chan *Log, 100)
	e.inputs = make(map[string]InputPlugin)
	e.filters = []FilterPlugin{}
	e.pipelines = []*OutputPipeline{}
	e.stopped = false

	// Reconfigure with new config
	// Configure input plugin(s)
	for i, inputDef := range newConfig.Inputs {
		inputName := inputDef.Name
		if inputName == "" {
			inputName = fmt.Sprintf("%s-%d", inputDef.Type, i+1)
		}
		createInputFunc(inputDef.Type, inputName, inputDef.Config, e)
	}

	// Configure output plugin(s)
	for i, outputDef := range newConfig.Outputs {
		outputName := outputDef.Name
		if outputName == "" {
			outputName = fmt.Sprintf("%s-%d", outputDef.Type, i+1)
		}
		createOutputFunc(outputName, outputDef, e)
	}

	// Start the reloaded engine
	e.Start()

	log.Println("Engine configuration reloaded successfully")
	return nil
}

// processLogs handles incoming logs, applies filters, and sends to outputs
func (e *Engine) processLogs() {
	defer e.wg.Done()
	for {
		select {
		case logEntry, ok := <-e.inputCh:
			if !ok {
				return
			}

			log.Printf("[ENGINE] Received log from '%s': %s - %s", logEntry.Source, logEntry.Level, logEntry.Message)

			// Persist log before processing (Write-Ahead Log)
			if e.persistence != nil {
				if err := e.persistence.Persist(logEntry); err != nil {
					log.Printf("[ENGINE] Error persisting log: %v", err)
					// Continue processing even if persistence fails
				}
			}

			// Apply global filters (deprecated, but kept for backward compatibility)
			passedGlobalFilters := true
			if len(e.filters) > 0 {
				for i, filter := range e.filters {
					result := filter.Process(logEntry)
					log.Printf("[ENGINE] Global Filter #%d result: %t", i+1, result)
					if !result {
						passedGlobalFilters = false
						log.Printf("[ENGINE] Log BLOCKED by global filter #%d", i+1)
						break
					}
				}
			}

			if !passedGlobalFilters {
				continue // Skip this log
			}

			// Send to each output pipeline
			for _, pipeline := range e.pipelines {
				// Check if this pipeline accepts logs from this source
				if len(pipeline.Sources) > 0 {
					accepted := false
					for _, source := range pipeline.Sources {
						if source == logEntry.Source {
							accepted = true
							break
						}
					}
					if !accepted {
						log.Printf("[ENGINE] Output '%s' rejected log from source '%s'", pipeline.Name, logEntry.Source)
						continue
					}
				}

				// Apply pipeline-specific filters
				passedPipelineFilters := true
				for i, filter := range pipeline.Filters {
					result := filter.Process(logEntry)
					log.Printf("[ENGINE] Output '%s' Filter #%d result: %t", pipeline.Name, i+1, result)
					if !result {
						passedPipelineFilters = false
						log.Printf("[ENGINE] Log BLOCKED by output '%s' filter #%d", pipeline.Name, i+1)
						break
					}
				}

				if passedPipelineFilters {
					log.Printf("[ENGINE] Log PASSED filters for output '%s', sending to output", pipeline.Name)

					// Use buffer if available, otherwise direct write
					var err error
					if pipeline.Buffer != nil {
						err = pipeline.Buffer.Enqueue(logEntry)
					} else {
						err = pipeline.Output.Write(logEntry)
					}

					if err != nil {
						log.Printf("[ENGINE] Error writing to output '%s': %v", pipeline.Name, err)
					}
				}
			}

		case <-e.ctx.Done():
			return
		}
	}
}
