package prometheusoutput

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/mbiondo/logAnalyzer/core"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func init() {
	// Auto-register this plugin
	core.RegisterOutputPlugin("prometheus", NewPrometheusOutputFromConfig)
}

// Config represents prometheus output configuration
type Config struct {
	Port int `yaml:"port,omitempty"`
}

// NewPrometheusOutputFromConfig creates a prometheus output from configuration map
func NewPrometheusOutputFromConfig(config map[string]any) (any, error) {
	var cfg Config
	if err := core.GetPluginConfig(config, &cfg); err != nil {
		return nil, err
	}

	// Set defaults
	if cfg.Port == 0 {
		cfg.Port = 9091
	}

	return NewPrometheusOutputWithPort(cfg.Port), nil
}

// PrometheusOutput sends logs to Prometheus metrics
type PrometheusOutput struct {
	logsTotal     *prometheus.CounterVec
	httpServer    *http.Server
	mutex         sync.RWMutex
	port          int
	serverStarted bool
}

// NewPrometheusOutput creates a new Prometheus output plugin
func NewPrometheusOutput() *PrometheusOutput {
	return NewPrometheusOutputWithPort(9091)
}

// NewPrometheusOutputWithPort creates a new Prometheus output plugin with custom port
func NewPrometheusOutputWithPort(port int) *PrometheusOutput {
	// Create Prometheus metrics
	logsTotal := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "loganalyzer_logs_total",
			Help: "Total number of logs processed by level",
		},
		[]string{"level"},
	)

	// Register metrics
	prometheus.MustRegister(logsTotal)

	p := &PrometheusOutput{
		logsTotal: logsTotal,
		port:      port,
	}

	// Start HTTP server for metrics
	go p.startMetricsServer()

	return p
}

// startMetricsServer starts the HTTP server for Prometheus metrics
func (p *PrometheusOutput) startMetricsServer() {
	p.mutex.Lock()
	if p.serverStarted {
		p.mutex.Unlock()
		return
	}
	p.serverStarted = true
	addr := fmt.Sprintf(":%d", p.port)
	p.mutex.Unlock()

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())

	p.httpServer = &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	log.Printf("Starting Prometheus metrics server on %s", addr)
	if err := p.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Printf("Prometheus metrics server error: %v", err)
	}
}

// Write processes a log entry and updates metrics
func (p *PrometheusOutput) Write(logEntry *core.Log) error {
	level := strings.ToLower(logEntry.Level)

	// Normalize level names
	switch level {
	case "warn", "warning":
		level = "warn"
	}

	// Increment counter for this level
	p.logsTotal.WithLabelValues(level).Inc()

	return nil
}

// Close cleans up resources
func (p *PrometheusOutput) Close() error {
	log.Println("Prometheus output closed")

	// Shutdown HTTP server
	if p.httpServer != nil {
		log.Println("Shutting down Prometheus metrics server")
		if err := p.httpServer.Close(); err != nil {
			log.Printf("Error closing Prometheus server: %v", err)
		}
	}

	return nil
}
