package core

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/mbiondo/logAnalyzer/pkg/auth"
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
	nextInputID  int        // Monotonic counter for generating unique input names

	// API server
	apiServer      *http.Server
	apiConfig      APIConfig
	apiKeyManager  *auth.APIKeyManager
	authMiddleware *auth.Middleware

	// Metrics
	totalLogsProcessed int64
	metricsMu          sync.RWMutex
	startTime          time.Time
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
		startTime: time.Now(),
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

// EnableAPI enables the metrics API server with the given configuration
func (e *Engine) EnableAPI(config APIConfig) error {
	if config.Port == 0 {
		return fmt.Errorf("API port cannot be 0")
	}
	e.apiConfig = config

	// Initialize API key manager if authentication is enabled
	if config.Auth.Enabled {
		e.apiKeyManager = auth.NewAPIKeyManager()

		// Load API keys from configuration
		if err := e.apiKeyManager.LoadKeys(config.Auth.APIKeys); err != nil {
			return fmt.Errorf("failed to load API keys: %w", err)
		}

		// Initialize authentication middleware
		e.authMiddleware = auth.NewMiddleware(
			e.apiKeyManager,
			config.Auth.RequireKey,
			config.Auth.HealthBypass,
		)

		log.Printf("API authentication enabled with %d API keys", len(config.Auth.APIKeys))
	}

	return nil
}

// EnableAPIDefault enables the metrics API server on the default port (9090)
func (e *Engine) EnableAPIDefault() error {
	return e.EnableAPI(DefaultAPIConfig())
}

// AddInput adds an input plugin to the engine with a name
func (e *Engine) AddInput(name string, input InputPlugin) {
	input.SetLogChannel(e.inputCh)
	e.inputs[name] = input
}

// AddInputAnonymous adds an input plugin without a specific name (for backward compatibility)
func (e *Engine) AddInputAnonymous(input InputPlugin) {
	e.mu.Lock()
	name := fmt.Sprintf("input-%d", e.nextInputID)
	e.nextInputID++
	e.mu.Unlock()
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

	// Start API server if enabled
	if e.apiConfig.Enabled {
		e.startAPIServer()
	}

	e.wg.Add(1)
	go e.processLogs()
	log.Println("LogAnalyzer engine started")
}

// startAPIServer starts the metrics API server
func (e *Engine) startAPIServer() {
	mux := http.NewServeMux()

	// Apply authentication middleware if enabled
	if e.authMiddleware != nil {
		mux.HandleFunc("/health", e.authMiddleware.WrapHandlerFunc(e.handleHealth))
		mux.HandleFunc("/metrics", e.authMiddleware.WrapHandlerFunc(e.handleMetrics))
		mux.HandleFunc("/status", e.authMiddleware.WrapHandlerFunc(e.handleStatus))
	} else {
		mux.HandleFunc("/health", e.handleHealth)
		mux.HandleFunc("/metrics", e.handleMetrics)
		mux.HandleFunc("/status", e.handleStatus)
	}

	e.apiServer = &http.Server{
		Addr:              fmt.Sprintf(":%d", e.apiConfig.Port),
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	log.Printf("Starting API server on port %d", e.apiConfig.Port)
	if e.authMiddleware != nil {
		log.Printf("API authentication is enabled")
	}

	go func() {
		if err := e.apiServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("API server error: %v", err)
		}
	}()
}

// handleHealth returns a simple health check
func (e *Engine) handleHealth(w http.ResponseWriter, r *http.Request) {
	e.mu.Lock()
	stopped := e.stopped
	e.mu.Unlock()

	status := "ok"
	if stopped {
		status = "stopped"
	}

	response := map[string]string{
		"status": status,
		"time":   time.Now().Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Error encoding health response: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// handleMetrics returns detailed metrics in JSON format
func (e *Engine) handleMetrics(w http.ResponseWriter, r *http.Request) {
	e.metricsMu.RLock()
	totalLogs := e.totalLogsProcessed
	e.metricsMu.RUnlock()

	uptime := time.Since(e.startTime)

	metrics := map[string]interface{}{
		"total_logs_processed": totalLogs,
		"uptime_seconds":       uptime.Seconds(),
		"inputs_count":         len(e.inputs),
		"pipelines_count":      len(e.pipelines),
		"buffer_enabled":       e.bufferConfig.Enabled,
	}

	// Add buffer stats if enabled
	if e.bufferConfig.Enabled {
		bufferStats := make(map[string]interface{})
		for _, pipeline := range e.pipelines {
			if pipeline.Buffer != nil {
				stats := pipeline.Buffer.GetStats()
				bufferStats[pipeline.Name] = map[string]interface{}{
					"total_enqueued":   stats.TotalEnqueued,
					"total_delivered":  stats.TotalDelivered,
					"total_retried":    stats.TotalRetried,
					"total_failed":     stats.TotalFailed,
					"total_dlq":        stats.TotalDLQ,
					"current_queued":   stats.CurrentQueued,
					"current_retrying": stats.CurrentRetrying,
				}
			}
		}
		metrics["buffer_stats"] = bufferStats
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(metrics); err != nil {
		log.Printf("Error encoding metrics response: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// handleStatus returns comprehensive status information
func (e *Engine) handleStatus(w http.ResponseWriter, r *http.Request) {
	e.mu.Lock()
	stopped := e.stopped
	e.mu.Unlock()

	e.metricsMu.RLock()
	totalLogs := e.totalLogsProcessed
	e.metricsMu.RUnlock()

	uptime := time.Since(e.startTime)

	status := map[string]interface{}{
		"engine": map[string]interface{}{
			"status":               map[bool]string{true: "stopped", false: "running"}[stopped],
			"uptime_seconds":       uptime.Seconds(),
			"start_time":           e.startTime.Format(time.RFC3339),
			"total_logs_processed": totalLogs,
		},
		"inputs": map[string]interface{}{
			"count": len(e.inputs),
			"names": func() []string {
				names := make([]string, 0, len(e.inputs))
				for name := range e.inputs {
					names = append(names, name)
				}
				return names
			}(),
		},
		"outputs": map[string]interface{}{
			"count": len(e.pipelines),
			"pipelines": func() []map[string]interface{} {
				pipelines := make([]map[string]interface{}, 0, len(e.pipelines))
				for _, p := range e.pipelines {
					pipeline := map[string]interface{}{
						"name":       p.Name,
						"has_buffer": p.Buffer != nil,
						"filters":    len(p.Filters),
						"sources":    p.Sources,
					}
					if p.Buffer != nil {
						stats := p.Buffer.GetStats()
						pipeline["buffer_stats"] = map[string]interface{}{
							"total_enqueued":   stats.TotalEnqueued,
							"total_delivered":  stats.TotalDelivered,
							"total_retried":    stats.TotalRetried,
							"total_failed":     stats.TotalFailed,
							"total_dlq":        stats.TotalDLQ,
							"current_queued":   stats.CurrentQueued,
							"current_retrying": stats.CurrentRetrying,
						}
					}
					pipelines = append(pipelines, pipeline)
				}
				return pipelines
			}(),
		},
		"persistence": map[string]interface{}{
			"enabled": e.persistence != nil,
		},
		"api": map[string]interface{}{
			"enabled": e.apiConfig.Enabled,
			"port":    e.apiConfig.Port,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(status); err != nil {
		log.Printf("Error encoding status response: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
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
	// Don't set to nil to avoid potential races
	// e.inputCh = nil

	// Wait for processing goroutine to finish
	e.wg.Wait()

	// Close persistence layer
	if e.persistence != nil {
		if err := e.persistence.Close(); err != nil {
			log.Printf("Error closing persistence: %v", err)
		}
	}

	// Close API server
	if e.apiServer != nil {
		log.Println("Shutting down API server")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := e.apiServer.Shutdown(ctx); err != nil {
			log.Printf("Error shutting down API server: %v", err)
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
	if e.inputCh != nil {
		close(e.inputCh)
	}

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

			// Increment total logs processed counter
			e.metricsMu.Lock()
			e.totalLogsProcessed++
			e.metricsMu.Unlock()

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
