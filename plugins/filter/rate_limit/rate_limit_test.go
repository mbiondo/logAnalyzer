package rate_limit

import (
	"testing"
	"time"

	"github.com/mbiondo/logAnalyzer/core"
)

func TestNewRateLimitFilter(t *testing.T) {
	rate := 10.0
	burst := 5
	filter := NewRateLimitFilter(rate, burst)

	if filter.rate != rate {
		t.Errorf("Expected rate %f, got %f", rate, filter.rate)
	}

	if filter.burst != burst {
		t.Errorf("Expected burst %d, got %d", burst, filter.burst)
	}

	if filter.tokens != float64(burst) {
		t.Errorf("Expected tokens %f, got %f", float64(burst), filter.tokens)
	}
}

func TestRateLimitFilterProcess(t *testing.T) {
	rate := 0.5 // 0.5 logs per second
	burst := 3
	filter := NewRateLimitFilter(rate, burst)

	log := core.NewLog("info", "test message")

	// Should allow burst number of logs initially
	for i := 0; i < burst; i++ {
		if !filter.Process(log) {
			t.Errorf("Should allow log %d within burst", i+1)
		}
	}

	// Next should be blocked (no time passed)
	if filter.Process(log) {
		t.Error("Should block log after burst is exhausted")
	}

	// Simulate time passing (set lastRefill to past)
	filter.mu.Lock()
	filter.lastRefill = time.Now().Add(-2 * time.Second) // 2 seconds ago
	filter.mu.Unlock()

	// Should allow one more log (0.5 * 2 = 1 token refilled)
	if !filter.Process(log) {
		t.Error("Should allow log after time has passed")
	}

	// Now tokens should be 0, next should block
	if filter.Process(log) {
		t.Error("Should block again after consuming the refilled token")
	}
}

func TestRateLimitFilterConfig(t *testing.T) {
	config := map[string]any{
		"rate":  5.0,
		"burst": 10,
	}

	filter, err := NewRateLimitFilterFromConfig(config)
	if err != nil {
		t.Fatalf("Failed to create filter from config: %v", err)
	}

	rateLimitFilter, ok := filter.(*RateLimitFilter)
	if !ok {
		t.Fatal("Filter is not of type *RateLimitFilter")
	}

	if rateLimitFilter.rate != 5.0 {
		t.Errorf("Expected rate 5.0, got %f", rateLimitFilter.rate)
	}

	if rateLimitFilter.burst != 10 {
		t.Errorf("Expected burst 10, got %d", rateLimitFilter.burst)
	}
}