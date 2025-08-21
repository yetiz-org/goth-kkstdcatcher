package kkstdcatcher

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestNewStdCatcher tests the creation of a new StdCatcher instance
func TestNewStdCatcher(t *testing.T) {
	catcher := NewStdCatcher()
	
	if catcher == nil {
		t.Fatal("NewStdCatcher returned nil")
	}
	
	if catcher.stdReadIntervalMs != 1000 {
		t.Errorf("Expected default stdReadIntervalMs to be 1000, got %d", catcher.stdReadIntervalMs)
	}
	
	if catcher.IsRunning() {
		t.Error("New StdCatcher should not be running initially")
	}
	
	if catcher.StdinWriteFunc == nil || catcher.StdoutWriteFunc == nil || catcher.StderrWriteFunc == nil {
		t.Error("Write functions should be initialized")
	}
}

// TestSetStdReadIntervalMs tests setting the read interval
func TestSetStdReadIntervalMs(t *testing.T) {
	catcher := NewStdCatcher()
	
	// Test valid interval
	catcher.SetStdReadIntervalMs(500)
	if catcher.stdReadIntervalMs != 500 {
		t.Errorf("Expected stdReadIntervalMs to be 500, got %d", catcher.stdReadIntervalMs)
	}
	
	// Test invalid interval (should not change)
	catcher.SetStdReadIntervalMs(-100)
	if catcher.stdReadIntervalMs != 500 {
		t.Errorf("Expected stdReadIntervalMs to remain 500, got %d", catcher.stdReadIntervalMs)
	}
	
	catcher.SetStdReadIntervalMs(0)
	if catcher.stdReadIntervalMs != 500 {
		t.Errorf("Expected stdReadIntervalMs to remain 500, got %d", catcher.stdReadIntervalMs)
	}
}

// TestStartAndShutdown tests the basic start and shutdown functionality
func TestStartAndShutdown(t *testing.T) {
	catcher := NewStdCatcher()
	
	// Test start
	err := catcher.Start()
	if err != nil {
		t.Fatalf("Failed to start catcher: %v", err)
	}
	
	if !catcher.IsRunning() {
		t.Error("Catcher should be running after start")
	}
	
	// Test double start (should fail)
	err = catcher.Start()
	if err == nil {
		t.Error("Expected error when starting already running catcher")
	}
	
	// Give some time for goroutines to start
	time.Sleep(100 * time.Millisecond)
	
	// Test shutdown
	err = catcher.ShutdownGracefully()
	if err != nil {
		t.Fatalf("Failed to shutdown catcher: %v", err)
	}
	
	if catcher.IsRunning() {
		t.Error("Catcher should not be running after shutdown")
	}
	
	// Test double shutdown (should fail)
	err = catcher.ShutdownGracefully()
	if err == nil {
		t.Error("Expected error when shutting down already stopped catcher")
	}
}

// TestStdoutCapture tests capturing stdout
func TestStdoutCapture(t *testing.T) {
	catcher := NewStdCatcher()
	
	var capturedOutput []string
	var mu sync.Mutex
	
	catcher.StdoutWriteFunc = func(s string) {
		mu.Lock()
		capturedOutput = append(capturedOutput, s)
		mu.Unlock()
	}
	
	err := catcher.Start()
	if err != nil {
		t.Fatalf("Failed to start catcher: %v", err)
	}
	defer catcher.ShutdownGracefully()
	
	// Give some time for setup
	time.Sleep(50 * time.Millisecond)
	
	// Write to stdout
	testMessage := "Hello, stdout!"
	fmt.Print(testMessage)
	
	// Give some time for capture
	time.Sleep(100 * time.Millisecond)
	
	mu.Lock()
	found := false
	for _, output := range capturedOutput {
		if strings.Contains(output, testMessage) {
			found = true
			break
		}
	}
	mu.Unlock()
	
	if !found {
		t.Errorf("Expected to capture stdout message '%s', but didn't find it in: %v", testMessage, capturedOutput)
	}
}

// TestStderrCapture tests capturing stderr
func TestStderrCapture(t *testing.T) {
	catcher := NewStdCatcher()
	
	var capturedOutput []string
	var mu sync.Mutex
	
	catcher.StderrWriteFunc = func(s string) {
		mu.Lock()
		capturedOutput = append(capturedOutput, s)
		mu.Unlock()
	}
	
	err := catcher.Start()
	if err != nil {
		t.Fatalf("Failed to start catcher: %v", err)
	}
	defer catcher.ShutdownGracefully()
	
	// Give some time for setup
	time.Sleep(50 * time.Millisecond)
	
	// Write to stderr
	testMessage := "Hello, stderr!"
	fmt.Fprint(os.Stderr, testMessage)
	
	// Give some time for capture
	time.Sleep(100 * time.Millisecond)
	
	mu.Lock()
	found := false
	for _, output := range capturedOutput {
		if strings.Contains(output, testMessage) {
			found = true
			break
		}
	}
	mu.Unlock()
	
	if !found {
		t.Errorf("Expected to capture stderr message '%s', but didn't find it in: %v", testMessage, capturedOutput)
	}
}

// TestConcurrentCapture tests capturing multiple streams concurrently
func TestConcurrentCapture(t *testing.T) {
	catcher := NewStdCatcher()
	
	var stdoutCaptures, stderrCaptures []string
	var stdoutMu, stderrMu sync.Mutex
	
	catcher.StdoutWriteFunc = func(s string) {
		stdoutMu.Lock()
		stdoutCaptures = append(stdoutCaptures, s)
		stdoutMu.Unlock()
	}
	
	catcher.StderrWriteFunc = func(s string) {
		stderrMu.Lock()
		stderrCaptures = append(stderrCaptures, s)
		stderrMu.Unlock()
	}
	
	err := catcher.Start()
	if err != nil {
		t.Fatalf("Failed to start catcher: %v", err)
	}
	defer catcher.ShutdownGracefully()
	
	// Give some time for setup
	time.Sleep(50 * time.Millisecond)
	
	var wg sync.WaitGroup
	numGoroutines := 10
	
	// Concurrent stdout writes
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			fmt.Printf("stdout-%d\n", id)
		}(i)
	}
	
	// Concurrent stderr writes
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			fmt.Fprintf(os.Stderr, "stderr-%d\n", id)
		}(i)
	}
	
	wg.Wait()
	
	// Give some time for capture
	time.Sleep(200 * time.Millisecond)
	
	stdoutMu.Lock()
	stderrMu.Lock()
	
	if len(stdoutCaptures) == 0 {
		t.Error("Expected to capture stdout messages, but got none")
	}
	
	if len(stderrCaptures) == 0 {
		t.Error("Expected to capture stderr messages, but got none")
	}
	
	stderrMu.Unlock()
	stdoutMu.Unlock()
}

// TestGetChannels tests the GetChannels method
func TestGetChannels(t *testing.T) {
	catcher := NewStdCatcher()
	
	stdin, stdout, stderr := catcher.GetChannels()
	
	if stdin == nil || stdout == nil || stderr == nil {
		t.Error("GetChannels should return non-nil channels")
	}
}

// TestDefaultInstance tests the default instance functionality
func TestDefaultInstance(t *testing.T) {
	instance := DefaultInstance()
	if instance == nil {
		t.Error("DefaultInstance should not return nil")
	}
	
	// Test that it's the same instance
	instance2 := DefaultInstance()
	if instance != instance2 {
		t.Error("DefaultInstance should return the same instance")
	}
}

// TestGlobalFunctions tests the global convenience functions
func TestGlobalFunctions(t *testing.T) {
	// Ensure not running initially
	if IsRunning() {
		ShutdownGracefully()
		time.Sleep(100 * time.Millisecond)
	}
	
	// Test global start
	err := Start()
	if err != nil {
		t.Fatalf("Global Start failed: %v", err)
	}
	
	if !IsRunning() {
		t.Error("Global IsRunning should return true after Start")
	}
	
	// Test global SetStdReadIntervalMs
	SetStdReadIntervalMs(200)
	
	// Test global shutdown
	err = ShutdownGracefully()
	if err != nil {
		t.Fatalf("Global ShutdownGracefully failed: %v", err)
	}
	
	if IsRunning() {
		t.Error("Global IsRunning should return false after ShutdownGracefully")
	}
}

// TestStreamRestoration tests that original streams are properly restored
func TestStreamRestoration(t *testing.T) {
	// Store original streams
	originalStdout := os.Stdout
	originalStderr := os.Stderr
	
	catcher := NewStdCatcher()
	
	err := catcher.Start()
	if err != nil {
		t.Fatalf("Failed to start catcher: %v", err)
	}
	
	// Streams should be redirected
	if os.Stdout == originalStdout {
		t.Error("Stdout should be redirected after start")
	}
	if os.Stderr == originalStderr {
		t.Error("Stderr should be redirected after start")
	}
	
	err = catcher.ShutdownGracefully()
	if err != nil {
		t.Fatalf("Failed to shutdown catcher: %v", err)
	}
	
	// Streams should be restored
	if os.Stdout != originalStdout {
		t.Error("Stdout should be restored after shutdown")
	}
	if os.Stderr != originalStderr {
		t.Error("Stderr should be restored after shutdown")
	}
}

// TestLargeOutput tests handling of large output
func TestLargeOutput(t *testing.T) {
	catcher := NewStdCatcher()
	
	var capturedData []string
	var mu sync.Mutex
	totalExpectedBytes := 0
	
	catcher.StdoutWriteFunc = func(s string) {
		mu.Lock()
		capturedData = append(capturedData, s)
		mu.Unlock()
	}
	
	err := catcher.Start()
	if err != nil {
		t.Fatalf("Failed to start catcher: %v", err)
	}
	defer catcher.ShutdownGracefully()
	
	// Give some time for setup
	time.Sleep(50 * time.Millisecond)
	
	// Generate large output
	largeString := strings.Repeat("A", 10000)
	for i := 0; i < 10; i++ {
		fmt.Print(largeString)
		totalExpectedBytes += len(largeString)
	}
	
	// Give time for capture
	time.Sleep(500 * time.Millisecond)
	
	mu.Lock()
	totalCapturedBytes := 0
	for _, data := range capturedData {
		totalCapturedBytes += len(data)
	}
	mu.Unlock()
	
	if totalCapturedBytes != totalExpectedBytes {
		t.Errorf("Expected to capture %d bytes, but captured %d bytes", totalExpectedBytes, totalCapturedBytes)
	}
}

// TestSingleCharacterOutput tests immediate capture of single characters
func TestSingleCharacterOutput(t *testing.T) {
	catcher := NewStdCatcher()
	
	var capturedChars []string
	var mu sync.Mutex
	
	catcher.StdoutWriteFunc = func(s string) {
		mu.Lock()
		capturedChars = append(capturedChars, s)
		mu.Unlock()
	}
	
	err := catcher.Start()
	if err != nil {
		t.Fatalf("Failed to start catcher: %v", err)
	}
	defer catcher.ShutdownGracefully()
	
	// Give time for setup
	time.Sleep(20 * time.Millisecond)
	
	// Output single characters with small delays
	testChars := []string{"A", "B", "C", "D", "E"}
	for _, char := range testChars {
		fmt.Print(char)
		time.Sleep(5 * time.Millisecond) // Small delay between characters
	}
	
	// Give time for capture
	time.Sleep(100 * time.Millisecond)
	
	mu.Lock()
	totalCaptured := strings.Join(capturedChars, "")
	mu.Unlock()
	
	expected := strings.Join(testChars, "")
	if !strings.Contains(totalCaptured, expected) {
		t.Errorf("Expected to find '%s' in captured output '%s'", expected, totalCaptured)
	}
	
	// Ensure we captured something (not stalled)
	mu.Lock()
	captureCount := len(capturedChars)
	mu.Unlock()
	
	if captureCount == 0 {
		t.Error("No single character output was captured - potential stalling issue")
	}
}

// TestRealTimeStreaming tests time-sensitive real-time output scenarios
func TestRealTimeStreaming(t *testing.T) {
	catcher := NewStdCatcher()
	
	var captureTimestamps []time.Time
	var capturedData []string
	var mu sync.Mutex
	
	catcher.StdoutWriteFunc = func(s string) {
		mu.Lock()
		captureTimestamps = append(captureTimestamps, time.Now())
		capturedData = append(capturedData, s)
		mu.Unlock()
	}
	
	err := catcher.Start()
	if err != nil {
		t.Fatalf("Failed to start catcher: %v", err)
	}
	defer catcher.ShutdownGracefully()
	
	// Give time for setup
	time.Sleep(20 * time.Millisecond)
	
	startTime := time.Now()
	
	// Simulate real-time logging scenario with time-sensitive output
	for i := 0; i < 5; i++ {
		fmt.Printf("Log entry %d\n", i)
		time.Sleep(10 * time.Millisecond) // Simulate real-time logging intervals
	}
	
	// Give time for final captures
	time.Sleep(50 * time.Millisecond)
	
	mu.Lock()
	captureCount := len(captureTimestamps)
	firstCaptureDelay := captureTimestamps[0].Sub(startTime)
	mu.Unlock()
	
	// Ensure we captured output
	if captureCount == 0 {
		t.Fatal("No real-time output was captured")
	}
	
	// Ensure first capture happened quickly (< 50ms delay indicates real-time response)
	if firstCaptureDelay > 50*time.Millisecond {
		t.Errorf("First capture took too long: %v (should be < 50ms for real-time)", firstCaptureDelay)
	}
	
	// Verify we captured all log entries
	mu.Lock()
	allCaptured := strings.Join(capturedData, "")
	mu.Unlock()
	
	for i := 0; i < 5; i++ {
		expected := fmt.Sprintf("Log entry %d", i)
		if !strings.Contains(allCaptured, expected) {
			t.Errorf("Missing log entry: '%s' in captured output", expected)
		}
	}
}

// TestHighFrequencySmallWrites tests rapid small writes without data loss
func TestHighFrequencySmallWrites(t *testing.T) {
	catcher := NewStdCatcher()
	
	var capturedData []string
	var mu sync.Mutex
	
	catcher.StdoutWriteFunc = func(s string) {
		mu.Lock()
		capturedData = append(capturedData, s)
		mu.Unlock()
	}
	
	err := catcher.Start()
	if err != nil {
		t.Fatalf("Failed to start catcher: %v", err)
	}
	defer catcher.ShutdownGracefully()
	
	// Give time for setup
	time.Sleep(20 * time.Millisecond)
	
	// High-frequency small writes simulation
	expectedWrites := 100
	var wg sync.WaitGroup
	
	for i := 0; i < expectedWrites; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			fmt.Printf("%d", id%10) // Write single digit
		}(i)
	}
	
	wg.Wait()
	
	// Give time for all captures
	time.Sleep(200 * time.Millisecond)
	
	mu.Lock()
	totalCapturedChars := 0
	for _, data := range capturedData {
		totalCapturedChars += len(data)
	}
	mu.Unlock()
	
	// We should have captured at least the same number of characters as writes
	// (allowing for potential batching in the capture mechanism)
	if totalCapturedChars < expectedWrites {
		t.Errorf("Data loss detected: expected at least %d characters, got %d", expectedWrites, totalCapturedChars)
	}
	
	// Ensure we actually captured some data (not completely stalled)
	mu.Lock()
	captureEvents := len(capturedData)
	mu.Unlock()
	
	if captureEvents == 0 {
		t.Error("No high-frequency writes were captured - complete stalling detected")
	}
}

// TestMinimalLatencyOutput tests that small outputs are processed with minimal delay
func TestMinimalLatencyOutput(t *testing.T) {
	catcher := NewStdCatcher()
	
	var firstCaptureTime time.Time
	var mu sync.Mutex
	captured := false
	
	catcher.StdoutWriteFunc = func(s string) {
		mu.Lock()
		if !captured {
			firstCaptureTime = time.Now()
			captured = true
		}
		mu.Unlock()
	}
	
	err := catcher.Start()
	if err != nil {
		t.Fatalf("Failed to start catcher: %v", err)
	}
	defer catcher.ShutdownGracefully()
	
	// Give time for setup
	time.Sleep(20 * time.Millisecond)
	
	// Record write time and immediately write small data
	writeTime := time.Now()
	fmt.Print("X") // Single character
	
	// Wait for capture with timeout
	timeout := time.After(100 * time.Millisecond)
	for {
		mu.Lock()
		wasCaptured := captured
		mu.Unlock()
		
		if wasCaptured {
			break
		}
		
		select {
		case <-timeout:
			t.Fatal("Timeout waiting for minimal data capture - stalling detected")
		default:
			time.Sleep(1 * time.Millisecond)
		}
	}
	
	mu.Lock()
	latency := firstCaptureTime.Sub(writeTime)
	mu.Unlock()
	
	// For real-time applications, latency should be very low (< 20ms)
	if latency > 20*time.Millisecond {
		t.Errorf("Latency too high for minimal output: %v (should be < 20ms)", latency)
	}
	
	t.Logf("Minimal output latency: %v", latency)
}

// TestAdaptivePollingBehavior tests that polling modes adjust correctly based on activity
func TestAdaptivePollingBehavior(t *testing.T) {
	catcher := NewStdCatcher()
	
	err := catcher.Start()
	if err != nil {
		t.Fatalf("Failed to start catcher: %v", err)
	}
	defer catcher.ShutdownGracefully()
	
	// Initially should be in fast mode (0)
	stats := catcher.GetStats()
	if stats.PollingMode != 0 {
		t.Errorf("Expected fast polling mode (0) initially, got %d", stats.PollingMode)
	}
	
	// Write some data to maintain fast mode
	fmt.Print("activity")
	time.Sleep(50 * time.Millisecond)
	
	stats = catcher.GetStats()
	if stats.PollingMode != 0 {
		t.Errorf("Expected fast polling mode (0) after recent activity, got %d", stats.PollingMode)
	}
	
	// Wait for transition to normal mode (100ms+ idle)
	time.Sleep(150 * time.Millisecond)
	stats = catcher.GetStats()
	if stats.PollingMode != 1 {
		t.Errorf("Expected normal polling mode (1) after 150ms idle, got %d", stats.PollingMode)
	}
	
	// Wait for transition to slow mode (1s+ idle)
	time.Sleep(1200 * time.Millisecond)
	stats = catcher.GetStats()
	if stats.PollingMode != 2 {
		t.Errorf("Expected slow polling mode (2) after 1200ms idle, got %d", stats.PollingMode)
	}
	
	// Activity should bring it back to fast mode
	fmt.Print("new activity")
	time.Sleep(50 * time.Millisecond)
	stats = catcher.GetStats()
	if stats.PollingMode != 0 {
		t.Errorf("Expected fast polling mode (0) after new activity, got %d", stats.PollingMode)
	}
	
	t.Logf("Adaptive polling test passed - transitions working correctly")
}

// TestIdleStateResourceUsage tests resource usage during extended idle periods
func TestIdleStateResourceUsage(t *testing.T) {
	catcher := NewStdCatcher()
	
	err := catcher.Start()
	if err != nil {
		t.Fatalf("Failed to start catcher: %v", err)
	}
	defer catcher.ShutdownGracefully()
	
	// Initial activity
	fmt.Print("initial")
	time.Sleep(10 * time.Millisecond)
	
	// Record baseline memory
	var m1 runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m1)
	baselineAlloc := m1.Alloc
	
	// Extended idle period (5 seconds)
	t.Log("Starting 5-second idle period...")
	time.Sleep(5 * time.Second)
	
	// Check memory after idle period
	var m2 runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m2)
	finalAlloc := m2.Alloc
	
	memGrowth := int64(finalAlloc) - int64(baselineAlloc)
	
	// Verify polling mode transitioned to slow
	stats := catcher.GetStats()
	if stats.PollingMode != 2 {
		t.Errorf("Expected slow polling mode (2) after 5s idle, got %d", stats.PollingMode)
	}
	
	// Memory growth should be minimal (< 100KB)
	if memGrowth > 100*1024 {
		t.Errorf("Excessive memory growth during idle: %d bytes (should be < 100KB)", memGrowth)
	}
	
	t.Logf("Idle period completed - Mode: %d, Memory growth: %d bytes, Idle time: %dms", 
		stats.PollingMode, memGrowth, stats.IdleTimeMs)
}
