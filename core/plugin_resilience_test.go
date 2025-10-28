package core

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

// Mock plugin for testing
type mockPlugin struct {
	failCount     int
	maxFails      int
	mu            sync.Mutex
	started       bool
	stopped       bool
	healthCheckOK bool
}

func (m *mockPlugin) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.started = true
	return nil
}

func (m *mockPlugin) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stopped = true
	return nil
}

func (m *mockPlugin) SetLogChannel(ch chan<- *Log) {
	// Mock implementation
}

func (m *mockPlugin) Write(log *Log) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.healthCheckOK {
		return errors.New("plugin unhealthy")
	}
	return nil
}

func (m *mockPlugin) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stopped = true
	return nil
}

func (m *mockPlugin) CheckHealth(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.healthCheckOK {
		return errors.New("health check failed")
	}
	return nil
}

// Failing plugin factory for testing retry logic
type failingPluginFactory struct {
	attemptCount int
	failUntil    int
	mu           sync.Mutex
}

func (f *failingPluginFactory) create(config map[string]any) (any, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.attemptCount++

	if f.attemptCount <= f.failUntil {
		return nil, fmt.Errorf("simulated failure (attempt %d/%d)", f.attemptCount, f.failUntil)
	}

	return &mockPlugin{healthCheckOK: true}, nil
}

func TestResilientPlugin_SuccessfulInitialization(t *testing.T) {
	factory := func(config map[string]any) (any, error) {
		return &mockPlugin{healthCheckOK: true}, nil
	}

	config := ResilientPluginConfig{
		RetryInterval: 100 * time.Millisecond,
		MaxRetries:    3,
		HealthCheck:   0, // Disabled for this test
	}

	rp := NewResilientPlugin("test-plugin", "test", factory, map[string]any{}, config)
	defer rp.Close()

	// Wait for initialization
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := rp.WaitForHealthy(ctx); err != nil {
		t.Fatalf("Expected plugin to become healthy, got error: %v", err)
	}

	if !rp.IsHealthy() {
		t.Error("Plugin should be healthy after successful initialization")
	}

	plugin, err := rp.GetPlugin()
	if err != nil {
		t.Errorf("GetPlugin should succeed, got error: %v", err)
	}
	if plugin == nil {
		t.Error("Plugin should not be nil")
	}
}

func TestResilientPlugin_RetryOnFailure(t *testing.T) {
	fpf := &failingPluginFactory{failUntil: 2} // Fail first 2 attempts, succeed on 3rd

	config := ResilientPluginConfig{
		RetryInterval: 50 * time.Millisecond,
		MaxRetries:    5,
		HealthCheck:   0,
	}

	rp := NewResilientPlugin("test-plugin", "test", fpf.create, map[string]any{}, config)
	defer rp.Close()

	// Wait for initialization with retries
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := rp.WaitForHealthy(ctx); err != nil {
		t.Fatalf("Expected plugin to become healthy after retries, got error: %v", err)
	}

	fpf.mu.Lock()
	attempts := fpf.attemptCount
	fpf.mu.Unlock()

	if attempts < 3 {
		t.Errorf("Expected at least 3 attempts, got %d", attempts)
	}

	if !rp.IsHealthy() {
		t.Error("Plugin should be healthy after successful retry")
	}
}

func TestResilientPlugin_MaxRetriesReached(t *testing.T) {
	factory := func(config map[string]any) (any, error) {
		return nil, errors.New("permanent failure")
	}

	config := ResilientPluginConfig{
		RetryInterval: 50 * time.Millisecond,
		MaxRetries:    2, // Give up after 2 retries
		HealthCheck:   0,
	}

	rp := NewResilientPlugin("test-plugin", "test", factory, map[string]any{}, config)
	defer rp.Close()

	// Wait and verify it doesn't become healthy
	time.Sleep(500 * time.Millisecond)

	if rp.IsHealthy() {
		t.Error("Plugin should not be healthy after max retries")
	}

	_, err := rp.GetPlugin()
	if err == nil {
		t.Error("GetPlugin should return error when plugin is unhealthy")
	}
}

func TestResilientPlugin_HealthCheckDetectsFailure(t *testing.T) {
	mock := &mockPlugin{healthCheckOK: true}

	factory := func(config map[string]any) (any, error) {
		return mock, nil
	}

	config := ResilientPluginConfig{
		RetryInterval: 100 * time.Millisecond,
		MaxRetries:    0,
		HealthCheck:   100 * time.Millisecond, // Fast health checks
	}

	rp := NewResilientPlugin("test-plugin", "test", factory, map[string]any{}, config)
	defer rp.Close()

	// Wait for initial health
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := rp.WaitForHealthy(ctx); err != nil {
		t.Fatalf("Plugin should be initially healthy: %v", err)
	}

	// Simulate health check failure
	mock.mu.Lock()
	mock.healthCheckOK = false
	mock.mu.Unlock()

	// Wait for health check to detect failure
	time.Sleep(300 * time.Millisecond)

	health, _ := rp.GetHealth()
	if health == HealthHealthy {
		t.Error("Health check should detect failure")
	}

	// Recover
	mock.mu.Lock()
	mock.healthCheckOK = true
	mock.mu.Unlock()

	// Wait for health check to detect recovery
	time.Sleep(300 * time.Millisecond)

	health, _ = rp.GetHealth()
	if health != HealthHealthy {
		t.Error("Health check should detect recovery")
	}
}

func TestResilientPlugin_ConcurrentAccess(t *testing.T) {
	factory := func(config map[string]any) (any, error) {
		return &mockPlugin{healthCheckOK: true}, nil
	}

	config := ResilientPluginConfig{
		RetryInterval: 50 * time.Millisecond,
		MaxRetries:    0,
		HealthCheck:   50 * time.Millisecond,
	}

	rp := NewResilientPlugin("test-plugin", "test", factory, map[string]any{}, config)
	defer rp.Close()

	// Wait for initialization
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = rp.WaitForHealthy(ctx)

	// Concurrent access from multiple goroutines
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				// Test concurrent reads
				_ = rp.IsHealthy()
				_, _ = rp.GetHealth()
				_, _ = rp.GetPlugin()
				_ = rp.GetStats()
			}
		}(i)
	}

	wg.Wait()
}

func TestResilientPlugin_ExponentialBackoff(t *testing.T) {
	fpf := &failingPluginFactory{failUntil: 5}
	var retryTimes []time.Time
	var mu sync.Mutex

	wrappedFactory := func(config map[string]any) (any, error) {
		mu.Lock()
		retryTimes = append(retryTimes, time.Now())
		mu.Unlock()
		return fpf.create(config)
	}

	config := ResilientPluginConfig{
		RetryInterval: 100 * time.Millisecond,
		MaxRetries:    0,
		HealthCheck:   0,
	}

	rp := NewResilientPlugin("test-plugin", "test", wrappedFactory, map[string]any{}, config)
	defer rp.Close()

	// Wait for successful initialization
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = rp.WaitForHealthy(ctx)

	mu.Lock()
	times := make([]time.Time, len(retryTimes))
	copy(times, retryTimes)
	mu.Unlock()

	if len(times) < 3 {
		t.Logf("Not enough retries to test backoff, got %d", len(times))
		return
	}

	// Check that delays increase (exponential backoff)
	for i := 1; i < len(times)-1; i++ {
		delay := times[i+1].Sub(times[i])
		prevDelay := times[i].Sub(times[i-1])

		// Allow some tolerance for timing variations
		if delay < prevDelay*95/100 {
			t.Errorf("Expected exponential backoff: delay[%d]=%v should be >= delay[%d]=%v",
				i+1, delay, i, prevDelay)
		}
	}
}

func TestResilientPlugin_CloseWhileInitializing(t *testing.T) {
	factory := func(config map[string]any) (any, error) {
		time.Sleep(200 * time.Millisecond)
		return &mockPlugin{healthCheckOK: true}, nil
	}

	config := ResilientPluginConfig{
		RetryInterval: 100 * time.Millisecond,
		MaxRetries:    0,
		HealthCheck:   0,
	}

	rp := NewResilientPlugin("test-plugin", "test", factory, map[string]any{}, config)

	// Close immediately
	time.Sleep(50 * time.Millisecond)
	err := rp.Close()
	if err != nil {
		t.Errorf("Close should not return error: %v", err)
	}

	// Verify it stopped
	time.Sleep(500 * time.Millisecond)
	if rp.IsHealthy() {
		t.Error("Plugin should not be healthy after close")
	}
}

func TestResilientPlugin_GetStats(t *testing.T) {
	fpf := &failingPluginFactory{failUntil: 2}

	config := ResilientPluginConfig{
		RetryInterval: 50 * time.Millisecond,
		MaxRetries:    0,
		HealthCheck:   0,
	}

	rp := NewResilientPlugin("test-plugin", "test-type", fpf.create, map[string]any{}, config)
	defer rp.Close()

	// Wait for initialization
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = rp.WaitForHealthy(ctx)

	stats := rp.GetStats()

	if stats["name"] != "test-plugin" {
		t.Errorf("Expected name 'test-plugin', got '%v'", stats["name"])
	}
	if stats["type"] != "test-type" {
		t.Errorf("Expected type 'test-type', got '%v'", stats["type"])
	}
	if stats["health"] != "healthy" {
		t.Errorf("Expected health 'healthy', got '%v'", stats["health"])
	}

	retries, ok := stats["current_retries"].(int)
	if !ok || retries < 0 {
		t.Errorf("Invalid current_retries: %v", stats["current_retries"])
	}
}

func TestResilientPlugin_MultipleCloses(t *testing.T) {
	factory := func(config map[string]any) (any, error) {
		return &mockPlugin{healthCheckOK: true}, nil
	}

	config := ResilientPluginConfig{
		RetryInterval: 100 * time.Millisecond,
		MaxRetries:    0,
		HealthCheck:   0,
	}

	rp := NewResilientPlugin("test-plugin", "test", factory, map[string]any{}, config)

	// Wait for initialization
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = rp.WaitForHealthy(ctx)

	// Close multiple times - should not panic or error
	for i := 0; i < 3; i++ {
		err := rp.Close()
		if err != nil {
			t.Errorf("Close #%d should not return error: %v", i+1, err)
		}
	}
}

func TestResilientPlugin_ContextCancellation(t *testing.T) {
	factory := func(config map[string]any) (any, error) {
		time.Sleep(1 * time.Second) // Never succeeds in test timeframe
		return &mockPlugin{healthCheckOK: true}, nil
	}

	config := ResilientPluginConfig{
		RetryInterval: 500 * time.Millisecond,
		MaxRetries:    0,
		HealthCheck:   0,
	}

	rp := NewResilientPlugin("test-plugin", "test", factory, map[string]any{}, config)
	defer rp.Close()

	// Wait with short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := rp.WaitForHealthy(ctx)
	if err == nil {
		t.Error("WaitForHealthy should return error when context is cancelled")
	}
}

// Benchmark concurrent access
func BenchmarkResilientPlugin_ConcurrentAccess(b *testing.B) {
	factory := func(config map[string]any) (any, error) {
		return &mockPlugin{healthCheckOK: true}, nil
	}

	config := ResilientPluginConfig{
		RetryInterval: 100 * time.Millisecond,
		MaxRetries:    0,
		HealthCheck:   100 * time.Millisecond,
	}

	rp := NewResilientPlugin("test-plugin", "test", factory, map[string]any{}, config)
	defer rp.Close()

	// Wait for initialization
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = rp.WaitForHealthy(ctx)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = rp.IsHealthy()
			_, _ = rp.GetHealth()
			_, _ = rp.GetPlugin()
		}
	})
}
