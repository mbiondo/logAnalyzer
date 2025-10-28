package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

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
	// Use plugin registry to create plugin dynamically
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

func createOutputPipeline(name string, outputDef core.PluginDefinition, engine *core.Engine) {
	// Create output plugin
	outputPlugin, err := core.CreateOutputPlugin(outputDef.Type, outputDef.Config)
	if err != nil {
		log.Fatalf("Error creating output plugin %s (%s): %v", outputDef.Type, name, err)
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

	engine.AddOutputPipeline(pipeline)
	log.Printf("Using %s output plugin as '%s' (sources: %v, filters: %d)",
		outputDef.Type, name, outputDef.Sources, len(filters))
}

func createInputPluginWrapper(pluginType string, name string, config map[string]any, engine *core.Engine) {
	createInputPlugin(pluginType, name, config, engine)
}

func createOutputPipelineWrapper(name string, outputDef core.PluginDefinition, engine *core.Engine) {
	createOutputPipeline(name, outputDef, engine)
}
