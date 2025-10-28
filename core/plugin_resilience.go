package core

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

// PluginHealth represents the health status of a plugin
type PluginHealth int

const (
	HealthUnknown PluginHealth = iota
	HealthHealthy
	HealthUnhealthy
	HealthRecovering
)

func (h PluginHealth) String() string {
	switch h {
	case HealthHealthy:
		return "healthy"
	case HealthUnhealthy:
		return "unhealthy"
	case HealthRecovering:
		return "recovering"
	default:
		return "unknown"
	}
}

// HealthChecker is an optional interface that plugins can implement
// to provide custom health check logic
type HealthChecker interface {
	CheckHealth(ctx context.Context) error
}

// ResilientPlugin wraps any plugin with resilience capabilities
type ResilientPlugin struct {
	name           string
	pluginType     string
	factory        PluginFactory
	config         map[string]any
	plugin         any
	health         PluginHealth
	lastError      error
	lastHealthy    time.Time
	retryInterval  time.Duration
	maxRetries     int
	currentRetries int
	mu             sync.RWMutex
	ctx            context.Context
	cancel         context.CancelFunc
	wg             sync.WaitGroup
}

// ResilientPluginConfig configures resilient plugin behavior
type ResilientPluginConfig struct {
	RetryInterval time.Duration // Time between retry attempts
	MaxRetries    int           // Maximum retries before giving up (0 = infinite)
	HealthCheck   time.Duration // Health check interval (0 = disabled)
}

// DefaultResilientPluginConfig returns default configuration
func DefaultResilientPluginConfig() ResilientPluginConfig {
	return ResilientPluginConfig{
		RetryInterval: 10 * time.Second,
		MaxRetries:    0, // Infinite retries
		HealthCheck:   30 * time.Second,
	}
}

// NewResilientPlugin creates a new resilient plugin wrapper
func NewResilientPlugin(name, pluginType string, factory PluginFactory, config map[string]any, resilientConfig ResilientPluginConfig) *ResilientPlugin {
	ctx, cancel := context.WithCancel(context.Background())

	rp := &ResilientPlugin{
		name:          name,
		pluginType:    pluginType,
		factory:       factory,
		config:        config,
		health:        HealthUnknown,
		retryInterval: resilientConfig.RetryInterval,
		maxRetries:    resilientConfig.MaxRetries,
		ctx:           ctx,
		cancel:        cancel,
	}

	// Try initial connection (non-blocking)
	rp.wg.Add(1)
	go rp.initialize()

	// Start health checker if configured
	if resilientConfig.HealthCheck > 0 {
		rp.wg.Add(1)
		go rp.healthCheckLoop(resilientConfig.HealthCheck)
	}

	return rp
}

// initialize attempts to create the plugin with retries
func (rp *ResilientPlugin) initialize() {
	defer rp.wg.Done()

	backoff := rp.retryInterval

	for {
		select {
		case <-rp.ctx.Done():
			return
		default:
		}

		log.Printf("[RESILIENCE:%s] Attempting to initialize %s plugin (attempt %d)",
			rp.name, rp.pluginType, rp.currentRetries+1)

		plugin, err := rp.factory(rp.config)
		if err != nil {
			rp.mu.Lock()
			rp.health = HealthUnhealthy
			rp.lastError = err
			rp.currentRetries++
			rp.mu.Unlock()

			log.Printf("[RESILIENCE:%s] Failed to initialize: %v", rp.name, err)

			// Check if max retries reached
			if rp.maxRetries > 0 && rp.currentRetries >= rp.maxRetries {
				log.Printf("[RESILIENCE:%s] Max retries (%d) reached, giving up", rp.name, rp.maxRetries)
				return
			}

			// Wait before retry with exponential backoff (capped at 2 minutes)
			log.Printf("[RESILIENCE:%s] Retrying in %v...", rp.name, backoff)
			select {
			case <-time.After(backoff):
				// Exponential backoff with cap
				backoff = backoff * 2
				if backoff > 2*time.Minute {
					backoff = 2 * time.Minute
				}
			case <-rp.ctx.Done():
				return
			}
			continue
		}

		// Success!
		rp.mu.Lock()
		rp.plugin = plugin
		rp.health = HealthHealthy
		rp.lastError = nil
		rp.lastHealthy = time.Now()
		rp.currentRetries = 0
		rp.mu.Unlock()

		log.Printf("[RESILIENCE:%s] Successfully initialized %s plugin", rp.name, rp.pluginType)

		// If it's an input plugin, start it
		if inputPlugin, ok := plugin.(InputPlugin); ok {
			if err := inputPlugin.Start(); err != nil {
				log.Printf("[RESILIENCE:%s] Failed to start input plugin: %v", rp.name, err)
				rp.mu.Lock()
				rp.health = HealthUnhealthy
				rp.lastError = err
				rp.plugin = nil
				rp.mu.Unlock()
				continue // Retry
			}
			log.Printf("[RESILIENCE:%s] Input plugin started", rp.name)
		}

		return
	}
}

// healthCheckLoop periodically checks plugin health
func (rp *ResilientPlugin) healthCheckLoop(interval time.Duration) {
	defer rp.wg.Done()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			rp.performHealthCheck()
		case <-rp.ctx.Done():
			return
		}
	}
}

// performHealthCheck checks if plugin is still healthy
func (rp *ResilientPlugin) performHealthCheck() {
	rp.mu.Lock()
	plugin := rp.plugin
	currentHealth := rp.health
	rp.mu.Unlock()

	if plugin == nil {
		return // Still initializing
	}

	// If plugin implements HealthChecker, use it
	if checker, ok := plugin.(HealthChecker); ok {
		ctx, cancel := context.WithTimeout(rp.ctx, 10*time.Second)
		defer cancel()

		err := checker.CheckHealth(ctx)
		rp.mu.Lock()
		if err != nil {
			if currentHealth == HealthHealthy {
				log.Printf("[RESILIENCE:%s] Health check failed: %v", rp.name, err)
			}
			rp.health = HealthUnhealthy
			rp.lastError = err
		} else {
			if currentHealth != HealthHealthy {
				log.Printf("[RESILIENCE:%s] Health check passed, plugin recovered", rp.name)
			}
			rp.health = HealthHealthy
			rp.lastHealthy = time.Now()
			rp.lastError = nil
		}
		rp.mu.Unlock()
	}
}

// GetPlugin returns the underlying plugin if healthy
func (rp *ResilientPlugin) GetPlugin() (any, error) {
	rp.mu.RLock()
	defer rp.mu.RUnlock()

	if rp.plugin == nil {
		return nil, fmt.Errorf("plugin not initialized: %w", rp.lastError)
	}

	if rp.health != HealthHealthy {
		return nil, fmt.Errorf("plugin unhealthy: %w", rp.lastError)
	}

	return rp.plugin, nil
}

// GetHealth returns current health status
func (rp *ResilientPlugin) GetHealth() (PluginHealth, error) {
	rp.mu.RLock()
	defer rp.mu.RUnlock()
	return rp.health, rp.lastError
}

// IsHealthy returns true if plugin is healthy
func (rp *ResilientPlugin) IsHealthy() bool {
	rp.mu.RLock()
	defer rp.mu.RUnlock()
	return rp.health == HealthHealthy && rp.plugin != nil
}

// WaitForHealthy blocks until plugin is healthy or context is cancelled
func (rp *ResilientPlugin) WaitForHealthy(ctx context.Context) error {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		if rp.IsHealthy() {
			return nil
		}

		select {
		case <-ticker.C:
			continue
		case <-ctx.Done():
			rp.mu.RLock()
			lastErr := rp.lastError
			rp.mu.RUnlock()
			if lastErr != nil {
				return fmt.Errorf("plugin not healthy: %w", lastErr)
			}
			return ctx.Err()
		case <-rp.ctx.Done():
			return fmt.Errorf("plugin closed")
		}
	}
}

// Close stops the resilient plugin
func (rp *ResilientPlugin) Close() error {
	rp.cancel()
	rp.wg.Wait()

	rp.mu.Lock()
	plugin := rp.plugin
	rp.plugin = nil
	rp.mu.Unlock()

	if plugin == nil {
		return nil
	}

	// Close based on plugin type
	if inputPlugin, ok := plugin.(InputPlugin); ok {
		return inputPlugin.Stop()
	}
	if outputPlugin, ok := plugin.(OutputPlugin); ok {
		return outputPlugin.Close()
	}

	return nil
}

// GetStats returns statistics about the resilient plugin
func (rp *ResilientPlugin) GetStats() map[string]any {
	rp.mu.RLock()
	defer rp.mu.RUnlock()

	stats := map[string]any{
		"name":            rp.name,
		"type":            rp.pluginType,
		"health":          rp.health.String(),
		"current_retries": rp.currentRetries,
	}

	if !rp.lastHealthy.IsZero() {
		stats["last_healthy"] = rp.lastHealthy.Format(time.RFC3339)
		stats["uptime_seconds"] = time.Since(rp.lastHealthy).Seconds()
	}

	if rp.lastError != nil {
		stats["last_error"] = rp.lastError.Error()
	}

	return stats
}
