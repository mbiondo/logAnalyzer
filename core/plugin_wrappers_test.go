package core

import (
	"sync"
	"testing"
	"time"
)

func TestResilientInputPlugin_Success(t *testing.T) {
	factory := func(config map[string]any) (any, error) {
		return &mockPlugin{healthCheckOK: true}, nil
	}

	config := ResilientPluginConfig{
		RetryInterval: 50 * time.Millisecond,
		MaxRetries:    3,
		HealthCheck:   0,
	}

	logCh := make(chan *Log, 10)
	rip := NewResilientInputPlugin("test-input", "test", factory, map[string]any{}, logCh, config)
	defer func() { _ = rip.Stop() }()

	// Wait for initialization
	time.Sleep(200 * time.Millisecond)

	if !rip.IsHealthy() {
		t.Error("ResilientInputPlugin should be healthy")
	}

	stats := rip.GetStats()
	if stats["name"] != "test-input" {
		t.Errorf("Expected name 'test-input', got '%v'", stats["name"])
	}
}

func TestResilientInputPlugin_SetLogChannel(t *testing.T) {
	factory := func(config map[string]any) (any, error) {
		return &mockPlugin{healthCheckOK: true}, nil
	}

	config := ResilientPluginConfig{
		RetryInterval: 50 * time.Millisecond,
		MaxRetries:    3,
		HealthCheck:   0,
	}

	logCh1 := make(chan *Log, 10)
	rip := NewResilientInputPlugin("test-input", "test", factory, map[string]any{}, logCh1, config)
	defer func() { _ = rip.Stop() }()

	// Wait for initialization
	time.Sleep(200 * time.Millisecond)

	// Change channel
	logCh2 := make(chan *Log, 10)
	rip.SetLogChannel(logCh2)

	// Should not panic or error
	if !rip.IsHealthy() {
		t.Error("Plugin should remain healthy after channel change")
	}
}

func TestResilientInputPlugin_StartStop(t *testing.T) {
	factory := func(config map[string]any) (any, error) {
		return &mockPlugin{healthCheckOK: true}, nil
	}

	config := ResilientPluginConfig{
		RetryInterval: 50 * time.Millisecond,
		MaxRetries:    3,
		HealthCheck:   0,
	}

	logCh := make(chan *Log, 10)
	rip := NewResilientInputPlugin("test-input", "test", factory, map[string]any{}, logCh, config)

	// Start should not error (handled by resilient plugin)
	err := rip.Start()
	if err != nil {
		t.Errorf("Start should not return error: %v", err)
	}

	// Wait for initialization
	time.Sleep(200 * time.Millisecond)

	// Stop should clean up
	err = rip.Stop()
	if err != nil {
		t.Errorf("Stop should not return error: %v", err)
	}

	// Multiple stops should be safe
	err = rip.Stop()
	if err != nil {
		t.Errorf("Second stop should not return error: %v", err)
	}
}

func TestResilientInputPlugin_ConcurrentAccess(t *testing.T) {
	factory := func(config map[string]any) (any, error) {
		return &mockPlugin{healthCheckOK: true}, nil
	}

	config := ResilientPluginConfig{
		RetryInterval: 50 * time.Millisecond,
		MaxRetries:    3,
		HealthCheck:   50 * time.Millisecond,
	}

	logCh := make(chan *Log, 10)
	rip := NewResilientInputPlugin("test-input", "test", factory, map[string]any{}, logCh, config)
	defer func() { _ = rip.Stop() }()

	// Wait for initialization
	time.Sleep(200 * time.Millisecond)

	// Concurrent access
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				_ = rip.IsHealthy()
				_ = rip.GetStats()
				rip.SetName("test")
			}
		}()
	}

	wg.Wait()
}

func TestResilientOutputPlugin_Success(t *testing.T) {
	factory := func(config map[string]any) (any, error) {
		return &mockPlugin{healthCheckOK: true}, nil
	}

	config := ResilientPluginConfig{
		RetryInterval: 50 * time.Millisecond,
		MaxRetries:    3,
		HealthCheck:   0,
	}

	rop := NewResilientOutputPlugin("test-output", "test", factory, map[string]any{}, config)
	defer func() { _ = rop.Close() }()

	// Wait for initialization
	time.Sleep(200 * time.Millisecond)

	if !rop.IsHealthy() {
		t.Error("ResilientOutputPlugin should be healthy")
	}

	stats := rop.GetStats()
	if stats["name"] != "test-output" {
		t.Errorf("Expected name 'test-output', got '%v'", stats["name"])
	}
}

func TestResilientOutputPlugin_Write(t *testing.T) {
	factory := func(config map[string]any) (any, error) {
		return &mockPlugin{healthCheckOK: true}, nil
	}

	config := ResilientPluginConfig{
		RetryInterval: 50 * time.Millisecond,
		MaxRetries:    3,
		HealthCheck:   0,
	}

	rop := NewResilientOutputPlugin("test-output", "test", factory, map[string]any{}, config)
	defer func() { _ = rop.Close() }()

	// Wait for initialization
	time.Sleep(200 * time.Millisecond)

	// Write should succeed
	log := NewLog("info", "test message")
	err := rop.Write(log)
	if err != nil {
		t.Errorf("Write should succeed when plugin is healthy: %v", err)
	}
}

func TestResilientOutputPlugin_WriteWhenUnhealthy(t *testing.T) {
	mock := &mockPlugin{healthCheckOK: false} // Unhealthy plugin

	factory := func(config map[string]any) (any, error) {
		return mock, nil
	}

	config := ResilientPluginConfig{
		RetryInterval: 50 * time.Millisecond,
		MaxRetries:    3,
		HealthCheck:   0,
	}

	rop := NewResilientOutputPlugin("test-output", "test", factory, map[string]any{}, config)
	defer func() { _ = rop.Close() }()

	// Wait for initialization
	time.Sleep(200 * time.Millisecond)

	// Write should return error when plugin is unhealthy
	log := NewLog("info", "test message")
	err := rop.Write(log)
	if err == nil {
		t.Error("Write should return error when plugin is unhealthy")
	}
}

func TestResilientOutputPlugin_WriteBeforeInitialization(t *testing.T) {
	factory := func(config map[string]any) (any, error) {
		time.Sleep(500 * time.Millisecond) // Slow initialization
		return &mockPlugin{healthCheckOK: true}, nil
	}

	config := ResilientPluginConfig{
		RetryInterval: 100 * time.Millisecond,
		MaxRetries:    3,
		HealthCheck:   0,
	}

	rop := NewResilientOutputPlugin("test-output", "test", factory, map[string]any{}, config)
	defer func() { _ = rop.Close() }()

	// Try to write before initialization completes
	log := NewLog("info", "test message")
	err := rop.Write(log)
	if err == nil {
		t.Error("Write should return error before plugin is initialized")
	}
}

func TestResilientOutputPlugin_ConcurrentWrites(t *testing.T) {
	factory := func(config map[string]any) (any, error) {
		return &mockPlugin{healthCheckOK: true}, nil
	}

	config := ResilientPluginConfig{
		RetryInterval: 50 * time.Millisecond,
		MaxRetries:    3,
		HealthCheck:   50 * time.Millisecond,
	}

	rop := NewResilientOutputPlugin("test-output", "test", factory, map[string]any{}, config)
	defer func() { _ = rop.Close() }()

	// Wait for initialization
	time.Sleep(200 * time.Millisecond)

	// Concurrent writes
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				log := NewLog("info", "test message")
				_ = rop.Write(log)
			}
		}(i)
	}

	// Concurrent health checks
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				_ = rop.IsHealthy()
				_ = rop.GetStats()
			}
		}()
	}

	wg.Wait()
}

func TestResilientOutputPlugin_Close(t *testing.T) {
	factory := func(config map[string]any) (any, error) {
		return &mockPlugin{healthCheckOK: true}, nil
	}

	config := ResilientPluginConfig{
		RetryInterval: 50 * time.Millisecond,
		MaxRetries:    3,
		HealthCheck:   0,
	}

	rop := NewResilientOutputPlugin("test-output", "test", factory, map[string]any{}, config)

	// Wait for initialization
	time.Sleep(200 * time.Millisecond)

	// Close should clean up
	err := rop.Close()
	if err != nil {
		t.Errorf("Close should not return error: %v", err)
	}

	// Multiple closes should be safe
	err = rop.Close()
	if err != nil {
		t.Errorf("Second close should not return error: %v", err)
	}

	// Writes after close should fail
	log := NewLog("info", "test message")
	err = rop.Write(log)
	if err == nil {
		t.Error("Write should fail after close")
	}
}

func TestResilientOutputPlugin_RecoveryDuringWrites(t *testing.T) {
	mock := &mockPlugin{healthCheckOK: false}

	factory := func(config map[string]any) (any, error) {
		return mock, nil
	}

	config := ResilientPluginConfig{
		RetryInterval: 50 * time.Millisecond,
		MaxRetries:    0,
		HealthCheck:   50 * time.Millisecond,
	}

	rop := NewResilientOutputPlugin("test-output", "test", factory, map[string]any{}, config)
	defer func() { _ = rop.Close() }()

	// Wait for initialization
	time.Sleep(200 * time.Millisecond)

	// Write should fail initially
	log := NewLog("info", "test message")
	err := rop.Write(log)
	if err == nil {
		t.Error("Write should fail when plugin is unhealthy")
	}

	// Recover plugin
	mock.mu.Lock()
	mock.healthCheckOK = true
	mock.mu.Unlock()

	// Wait for health check to detect recovery
	time.Sleep(200 * time.Millisecond)

	// Write should now succeed
	err = rop.Write(log)
	if err != nil {
		t.Errorf("Write should succeed after recovery: %v", err)
	}
}

// Benchmark output plugin writes
func BenchmarkResilientOutputPlugin_Write(b *testing.B) {
	factory := func(config map[string]any) (any, error) {
		return &mockPlugin{healthCheckOK: true}, nil
	}

	config := ResilientPluginConfig{
		RetryInterval: 100 * time.Millisecond,
		MaxRetries:    3,
		HealthCheck:   100 * time.Millisecond,
	}

	rop := NewResilientOutputPlugin("test-output", "test", factory, map[string]any{}, config)
	defer func() { _ = rop.Close() }()

	// Wait for initialization
	time.Sleep(200 * time.Millisecond)

	log := NewLog("info", "benchmark message")

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = rop.Write(log)
		}
	})
}
