package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mbiondo/logAnalyzer/core"

	// Import plugins for auto-registration
	_ "github.com/mbiondo/logAnalyzer/plugins/filter"
	_ "github.com/mbiondo/logAnalyzer/plugins/input"
	_ "github.com/mbiondo/logAnalyzer/plugins/output"
)

func main() {
	// Command line flags
	configFile := flag.String("config", "", "Path to configuration file (YAML)")
	hotReload := flag.Bool("hot-reload", false, "Enable hot reload of configuration file")
	flag.Parse()

	// Load configuration
	var config *core.Config
	var err error

	if *configFile != "" {
		config, err = core.LoadConfig(*configFile)
		if err != nil {
			log.Fatalf("Error loading config file: %v", err)
		}
		log.Printf("Loaded configuration from %s", *configFile)
	} else {
		config = core.DefaultConfig()
		log.Println("Using default configuration")
	}

	// Create engine
	engine := core.NewEngine()

	// Configure persistence if enabled
	persistenceConfig := config.Persistence
	if persistenceConfig.Dir == "" {
		// Use default if not configured
		persistenceConfig = core.DefaultPersistenceConfig()
	}
	if err := engine.SetPersistence(persistenceConfig); err != nil {
		log.Fatalf("Error configuring persistence: %v", err)
	}
	if persistenceConfig.Enabled {
		log.Printf("Persistence enabled: dir=%s, buffer=%d, flush=%ds",
			persistenceConfig.Dir, persistenceConfig.BufferSize, persistenceConfig.FlushInterval)
	}

	// Configure output buffering if enabled
	bufferConfig := config.OutputBuffer
	if bufferConfig.Dir == "" {
		// Use default if not configured
		bufferConfig = core.DefaultOutputBufferConfig()
	}
	engine.SetOutputBufferConfig(bufferConfig)
	if bufferConfig.Enabled {
		log.Printf("Output buffering enabled: queue=%d, retries=%d, dlq=%v",
			bufferConfig.MaxQueueSize, bufferConfig.MaxRetries, bufferConfig.DLQEnabled)
	}

	// Configure API if enabled
	apiConfig := config.API
	if apiConfig.Port == 0 {
		apiConfig = core.DefaultAPIConfig()
	}
	if apiConfig.Enabled {
		if err := engine.EnableAPI(apiConfig); err != nil {
			log.Fatalf("Failed to enable API: %v", err)
		}
		log.Printf("API server enabled on port %d", apiConfig.Port)
	}

	// Configure input plugin(s)
	for i, inputDef := range config.Inputs {
		inputName := inputDef.Name
		if inputName == "" {
			inputName = fmt.Sprintf("%s-%d", inputDef.Type, i+1)
		}
		createInputPlugin(inputDef.Type, inputName, inputDef.Config, engine)
	}

	// Configure filter plugin(s) - now handled per output pipeline

	// Configure output plugin(s)
	for i, outputDef := range config.Outputs {
		outputName := outputDef.Name
		if outputName == "" {
			outputName = fmt.Sprintf("%s-%d", outputDef.Type, i+1)
		}
		createOutputPipeline(outputName, outputDef, engine)
	}

	// Start engine
	engine.Start()

	// Initialize hot reload if enabled and config file is specified
	var configWatcher *core.ConfigWatcher
	if *hotReload && *configFile != "" {
		var err error
		configWatcher, err = core.NewConfigWatcher(*configFile, func(newConfig *core.Config) {
			// Reload engine with new configuration
			if err := engine.ReloadConfig(newConfig, createInputPluginWrapper, createOutputPipelineWrapper); err != nil {
				log.Printf("Error reloading configuration: %v", err)
			}
		})
		if err != nil {
			log.Printf("Warning: Failed to initialize config watcher: %v", err)
			log.Println("Continuing without hot reload")
		} else {
			log.Println("Hot reload enabled for config file:", *configFile)
		}
	}

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	// Stop config watcher if running
	if configWatcher != nil {
		configWatcher.Stop()
	}

	// Stop engine
	engine.Stop()
	log.Println("LogAnalyzer shutdown complete")
}

func createInputPlugin(pluginType string, name string, config map[string]any, engine *core.Engine) {
	// Check if resilient mode is enabled in config (default: true)
	resilientEnabled := true
	if val, ok := config["resilient"]; ok {
		if enabled, ok := val.(bool); ok {
			resilientEnabled = enabled
		}
	}

	if resilientEnabled {
		// Use resilient plugin wrapper
		log.Printf("Creating resilient %s input plugin as '%s'", pluginType, name)

		resilientConfig := core.DefaultResilientPluginConfig()
		// Override from config if provided
		if retryInterval, ok := config["retry_interval"].(int); ok {
			resilientConfig.RetryInterval = time.Duration(retryInterval) * time.Second
		}
		if maxRetries, ok := config["max_retries"].(int); ok {
			resilientConfig.MaxRetries = maxRetries
		}
		if healthCheck, ok := config["health_check_interval"].(int); ok {
			resilientConfig.HealthCheck = time.Duration(healthCheck) * time.Second
		}

		// Get factory function
		factory := func(cfg map[string]any) (any, error) {
			return core.CreateInputPlugin(pluginType, cfg)
		}

		resilientInput := core.NewResilientInputPlugin(name, pluginType, factory, config, engine.InputChannel(), resilientConfig)
		engine.AddInput(name, resilientInput)
		log.Printf("Resilient %s input plugin '%s' will connect in background", pluginType, name)
	} else {
		// Use direct plugin (original behavior)
		inputPlugin, err := core.CreateInputPlugin(pluginType, config)
		if err != nil {
			log.Fatalf("Error creating input plugin %s (%s): %v", pluginType, name, err)
		}

		// Set name if plugin supports it (duck typing)
		if nameable, ok := inputPlugin.(interface{ SetName(string) }); ok {
			nameable.SetName(name)
		}

		engine.AddInput(name, inputPlugin)
		log.Printf("Using %s input plugin as '%s'", pluginType, name)
	}
}

func createOutputPipeline(name string, outputDef core.PluginDefinition, engine *core.Engine) {
	// Check if resilient mode is enabled in config (default: true)
	resilientEnabled := true
	if val, ok := outputDef.Config["resilient"]; ok {
		if enabled, ok := val.(bool); ok {
			resilientEnabled = enabled
		}
	}

	var outputPlugin core.OutputPlugin
	var err error

	if resilientEnabled {
		// Use resilient plugin wrapper
		log.Printf("Creating resilient %s output plugin as '%s'", outputDef.Type, name)

		resilientConfig := core.DefaultResilientPluginConfig()
		// Override from config if provided
		if retryInterval, ok := outputDef.Config["retry_interval"].(int); ok {
			resilientConfig.RetryInterval = time.Duration(retryInterval) * time.Second
		}
		if maxRetries, ok := outputDef.Config["max_retries"].(int); ok {
			resilientConfig.MaxRetries = maxRetries
		}
		if healthCheck, ok := outputDef.Config["health_check_interval"].(int); ok {
			resilientConfig.HealthCheck = time.Duration(healthCheck) * time.Second
		}

		// Get factory function
		factory := func(cfg map[string]any) (any, error) {
			return core.CreateOutputPlugin(outputDef.Type, cfg)
		}

		resilientOutput := core.NewResilientOutputPlugin(name, outputDef.Type, factory, outputDef.Config, resilientConfig)
		outputPlugin = resilientOutput
		log.Printf("Resilient %s output plugin '%s' will connect in background", outputDef.Type, name)
	} else {
		// Use direct plugin (original behavior)
		outputPlugin, err = core.CreateOutputPlugin(outputDef.Type, outputDef.Config)
		if err != nil {
			log.Fatalf("Error creating output plugin %s (%s): %v", outputDef.Type, name, err)
		}
		log.Printf("Using %s output plugin as '%s'", outputDef.Type, name)
	}

	// Create filters for this output
	var filters []core.FilterPlugin
	for i, filterDef := range outputDef.Filters {
		filterPlugin, err := core.CreateFilterPlugin(filterDef.Type, filterDef.Config)
		if err != nil {
			log.Fatalf("Error creating filter plugin %s for output %s: %v", filterDef.Type, name, err)
		}
		filters = append(filters, filterPlugin)
		log.Printf("  Added %s filter #%d to output '%s'", filterDef.Type, i+1, name)
	}

	// Create pipeline
	pipeline := &core.OutputPipeline{
		Name:    name,
		Output:  outputPlugin,
		Filters: filters,
		Sources: outputDef.Sources,
	}

	if err := engine.AddOutputPipeline(pipeline); err != nil {
		log.Fatalf("Error adding output pipeline '%s': %v", name, err)
	}
	log.Printf("Using %s output plugin as '%s' (sources: %v, filters: %d)",
		outputDef.Type, name, outputDef.Sources, len(filters))
}

func createInputPluginWrapper(pluginType string, name string, config map[string]any, engine *core.Engine) {
	createInputPlugin(pluginType, name, config, engine)
}

func createOutputPipelineWrapper(name string, outputDef core.PluginDefinition, engine *core.Engine) {
	createOutputPipeline(name, outputDef, engine)
}
