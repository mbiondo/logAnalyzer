package rate_limit

import (
	"sync"
	"time"

	"github.com/mbiondo/logAnalyzer/core"
)

func init() {
	// Auto-register this plugin
	core.RegisterFilterPlugin("rate_limit", NewRateLimitFilterFromConfig)
}

// Config represents rate limit filter configuration
type Config struct {
	Rate  float64 `yaml:"rate"`  // logs per second
	Burst int     `yaml:"burst"` // maximum burst size
}

// NewRateLimitFilterFromConfig creates a rate limit filter from configuration map
func NewRateLimitFilterFromConfig(config map[string]any) (any, error) {
	var cfg Config
	if err := core.GetPluginConfig(config, &cfg); err != nil {
		return nil, err
	}

	return NewRateLimitFilter(cfg.Rate, cfg.Burst), nil
}

// RateLimitFilter implements token bucket rate limiting
type RateLimitFilter struct {
	rate       float64       // tokens per second
	burst      int           // max tokens
	tokens     float64       // current tokens
	lastRefill time.Time     // last refill time
	mu         sync.Mutex    // for thread safety
}

// NewRateLimitFilter creates a new rate limit filter
func NewRateLimitFilter(rate float64, burst int) *RateLimitFilter {
	return &RateLimitFilter{
		rate:       rate,
		burst:      burst,
		tokens:     float64(burst), // start with full burst
		lastRefill: time.Now(),
	}
}

// Process determines if a log should be kept based on rate limiting
func (f *RateLimitFilter) Process(log *core.Log) bool {
	f.mu.Lock()
	defer f.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(f.lastRefill).Seconds()

	// Refill tokens based on elapsed time
	f.tokens += elapsed * f.rate
	if f.tokens > float64(f.burst) {
		f.tokens = float64(f.burst)
	}

	f.lastRefill = now

	// Check if we have a token
	if f.tokens >= 1.0 {
		f.tokens -= 1.0
		return true
	}

	return false
}