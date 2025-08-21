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

// BenchmarkStdCatcherCreation benchmarks the creation of StdCatcher instances
func BenchmarkStdCatcherCreation(b *testing.B) {
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		catcher := NewStdCatcher()
		_ = catcher
	}
}

// BenchmarkStartStop benchmarks the start and stop operations
func BenchmarkStartStop(b *testing.B) {
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		catcher := NewStdCatcher()
		err := catcher.Start()
		if err != nil {
			b.Fatalf("Failed to start catcher: %v", err)
		}
		
		// Give time for goroutines to start properly
		time.Sleep(10 * time.Millisecond)
		
		err = catcher.ShutdownGracefully()
		if err != nil {
			b.Fatalf("Failed to shutdown catcher: %v", err)
		}
		
		// Ensure complete shutdown before next iteration
		time.Sleep(10 * time.Millisecond)
	}
}

// BenchmarkStdoutCapture benchmarks stdout capture performance
func BenchmarkStdoutCapture(b *testing.B) {
	catcher := NewStdCatcher()
	
	var captureCount int64
	var mu sync.Mutex
	
	catcher.StdoutWriteFunc = func(s string) {
		mu.Lock()
		captureCount++
		mu.Unlock()
	}
	
	err := catcher.Start()
	if err != nil {
		b.Fatalf("Failed to start catcher: %v", err)
	}
	defer catcher.ShutdownGracefully()
	
	// Give time for setup
	time.Sleep(50 * time.Millisecond)
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		fmt.Print("benchmark test message")
	}
	
	// Give time for all captures to complete
	time.Sleep(100 * time.Millisecond)
}

// BenchmarkStderrCapture benchmarks stderr capture performance
func BenchmarkStderrCapture(b *testing.B) {
	catcher := NewStdCatcher()
	
	var captureCount int64
	var mu sync.Mutex
	
	catcher.StderrWriteFunc = func(s string) {
		mu.Lock()
		captureCount++
		mu.Unlock()
	}
	
	err := catcher.Start()
	if err != nil {
		b.Fatalf("Failed to start catcher: %v", err)
	}
	defer catcher.ShutdownGracefully()
	
	// Give time for setup
	time.Sleep(50 * time.Millisecond)
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		fmt.Fprint(os.Stderr, "benchmark test message")
	}
	
	// Give time for all captures to complete
	time.Sleep(100 * time.Millisecond)
}

// BenchmarkConcurrentOutput benchmarks concurrent stdout/stderr writes
func BenchmarkConcurrentOutput(b *testing.B) {
	catcher := NewStdCatcher()
	
	var stdoutCount, stderrCount int64
	var stdoutMu, stderrMu sync.Mutex
	
	catcher.StdoutWriteFunc = func(s string) {
		stdoutMu.Lock()
		stdoutCount++
		stdoutMu.Unlock()
	}
	
	catcher.StderrWriteFunc = func(s string) {
		stderrMu.Lock()
		stderrCount++
		stderrMu.Unlock()
	}
	
	err := catcher.Start()
	if err != nil {
		b.Fatalf("Failed to start catcher: %v", err)
	}
	defer catcher.ShutdownGracefully()
	
	// Give time for setup
	time.Sleep(50 * time.Millisecond)
	
	b.ResetTimer()
	b.ReportAllocs()
	
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			// Alternate between stdout and stderr
			if stdoutCount%2 == 0 {
				fmt.Print("stdout message")
			} else {
				fmt.Fprint(os.Stderr, "stderr message")
			}
		}
	})
	
	// Give time for all captures to complete
	time.Sleep(200 * time.Millisecond)
}

// BenchmarkLargeThroughput benchmarks handling of large amounts of data
func BenchmarkLargeThroughput(b *testing.B) {
	catcher := NewStdCatcher()
	
	var totalBytes int64
	var mu sync.Mutex
	
	catcher.StdoutWriteFunc = func(s string) {
		mu.Lock()
		totalBytes += int64(len(s))
		mu.Unlock()
	}
	
	err := catcher.Start()
	if err != nil {
		b.Fatalf("Failed to start catcher: %v", err)
	}
	defer catcher.ShutdownGracefully()
	
	// Give time for setup
	time.Sleep(50 * time.Millisecond)
	
	// Create large data blocks
	largeData := strings.Repeat("A", 1024) // 1KB blocks
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		fmt.Print(largeData)
	}
	
	// Give time for all captures to complete
	time.Sleep(200 * time.Millisecond)
	
	b.StopTimer()
	mu.Lock()
	b.ReportMetric(float64(totalBytes), "bytes/captured")
	mu.Unlock()
}

// BenchmarkChannelThroughput benchmarks the internal channel performance
func BenchmarkChannelThroughput(b *testing.B) {
	catcher := NewStdCatcher()
	
	err := catcher.Start()
	if err != nil {
		b.Fatalf("Failed to start catcher: %v", err)
	}
	defer catcher.ShutdownGracefully()
	
	// Get channels for direct testing
	_, stdoutChan, _ := catcher.GetChannels()
	
	b.ResetTimer()
	b.ReportAllocs()
	
	// Benchmark channel send performance by writing to stdout
	for i := 0; i < b.N; i++ {
		fmt.Print("test")
	}
	
	// Drain any remaining data
	time.Sleep(100 * time.Millisecond)
	for {
		select {
		case <-stdoutChan:
		default:
			goto done
		}
	}
done:
}

// BenchmarkMemoryUsage benchmarks memory usage patterns
func BenchmarkMemoryUsage(b *testing.B) {
	b.ReportAllocs()
	
	catcher := NewStdCatcher()
	
	err := catcher.Start()
	if err != nil {
		b.Fatalf("Failed to start catcher: %v", err)
	}
	defer catcher.ShutdownGracefully()
	
	// Give time for setup
	time.Sleep(50 * time.Millisecond)
	
	b.ResetTimer()
	
	// Write various sizes of data to test memory allocation patterns
	for i := 0; i < b.N; i++ {
		switch i % 4 {
		case 0:
			fmt.Print("small")
		case 1:
			fmt.Print(strings.Repeat("medium", 10))
		case 2:
			fmt.Print(strings.Repeat("large", 100))
		case 3:
			fmt.Print(strings.Repeat("xl", 1000))
		}
	}
	
	// Give time for processing
	time.Sleep(100 * time.Millisecond)
}

// BenchmarkMultipleInstances benchmarks performance with multiple StdCatcher instances
func BenchmarkMultipleInstances(b *testing.B) {
	const numInstances = 3 // Reduced to minimize timing issues
	catchers := make([]*StdCatcher, numInstances)
	
	// Create multiple instances
	for i := 0; i < numInstances; i++ {
		catchers[i] = NewStdCatcher()
		catchers[i].StdoutWriteFunc = func(s string) {
			// No-op for benchmark
		}
	}
	
	// Only start one at a time since they share global streams
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		instanceIdx := i % numInstances
		catcher := catchers[instanceIdx]
		
		if !catcher.IsRunning() {
			err := catcher.Start()
			if err != nil {
				b.Fatalf("Failed to start instance %d: %v", instanceIdx, err)
			}
			time.Sleep(5 * time.Millisecond) // Give time for setup
		}
		
		fmt.Printf("instance-%d", instanceIdx)
		
		if catcher.IsRunning() {
			err := catcher.ShutdownGracefully()
			if err != nil {
				b.Fatalf("Failed to shutdown instance %d: %v", instanceIdx, err)
			}
			time.Sleep(5 * time.Millisecond) // Give time for cleanup
		}
	}
}

// BenchmarkGlobalFunctions benchmarks the global convenience functions
func BenchmarkGlobalFunctions(b *testing.B) {
	b.ReportAllocs()
	
	// Ensure clean state
	if IsRunning() {
		ShutdownGracefully()
		time.Sleep(100 * time.Millisecond)
	}
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		err := Start()
		if err != nil {
			b.Fatalf("Failed to start global instance: %v", err)
		}
		
		// Give time for setup
		time.Sleep(5 * time.Millisecond)
		
		fmt.Print("global test")
		
		err = ShutdownGracefully()
		if err != nil {
			b.Fatalf("Failed to shutdown global instance: %v", err)
		}
		
		// Ensure complete shutdown between iterations
		time.Sleep(10 * time.Millisecond)
	}
}

// BenchmarkIdleResourceUsage benchmarks CPU/memory usage during idle periods
func BenchmarkIdleResourceUsage(b *testing.B) {
	catcher := NewStdCatcher()
	
	err := catcher.Start()
	if err != nil {
		b.Fatalf("Failed to start catcher: %v", err)
	}
	defer catcher.ShutdownGracefully()
	
	// Let it settle into idle state
	time.Sleep(100 * time.Millisecond)
	
	b.ResetTimer()
	b.ReportAllocs()
	
	// Measure resource usage during idle periods
	for i := 0; i < b.N; i++ {
		// Simulate idle period - no output
		time.Sleep(1 * time.Millisecond)
		
		// Check that it's in slow polling mode after sufficient idle time
		if i%1000 == 0 && i > 0 { // Check after sufficient idle time has passed
			stats := catcher.GetStats()
			// Only check if we've been idle long enough (at least 1100ms)
			if stats.IdleTimeMs > 1100 && stats.PollingMode != 2 {
				b.Errorf("Expected slow polling mode (2) after %dms idle, got mode %d", stats.IdleTimeMs, stats.PollingMode)
			}
		}
	}
}

// BenchmarkAdaptivePollingTransition benchmarks polling mode transitions
func BenchmarkAdaptivePollingTransition(b *testing.B) {
	catcher := NewStdCatcher()
	
	err := catcher.Start()
	if err != nil {
		b.Fatalf("Failed to start catcher: %v", err)
	}
	defer catcher.ShutdownGracefully()
	
	// Start in active state
	fmt.Print("initial activity")
	time.Sleep(50 * time.Millisecond)
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		// Cycle between active and idle to test transitions
		if i%2 == 0 {
			// Active period
			fmt.Printf("active-%d", i)
			time.Sleep(1 * time.Millisecond)
		} else {
			// Idle period
			time.Sleep(200 * time.Millisecond) // Force transition to slower polling
		}
	}
}

// BenchmarkLongRunningIdleState benchmarks extended idle periods
func BenchmarkLongRunningIdleState(b *testing.B) {
	b.ReportAllocs()
	
	catcher := NewStdCatcher()
	err := catcher.Start()
	if err != nil {
		b.Fatalf("Failed to start catcher: %v", err)
	}
	defer catcher.ShutdownGracefully()
	
	// Initial activity to establish baseline
	fmt.Print("baseline")
	time.Sleep(10 * time.Millisecond)
	
	// Record initial memory
	var m1 runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m1)
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		// Extended idle period to test resource stability
		time.Sleep(100 * time.Millisecond)
		
		// Verify adaptive polling is working
		stats := catcher.GetStats()
		if stats.IdleTimeMs < 50 {
			b.Errorf("Expected idle time > 50ms, got %dms", stats.IdleTimeMs)
		}
	}
	
	b.StopTimer()
	
	// Check memory didn't grow significantly during idle
	var m2 runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m2)
	
	memGrowth := int64(m2.Alloc) - int64(m1.Alloc)
	if memGrowth > 1024*1024 { // > 1MB growth is concerning
		b.Errorf("Memory growth during idle: %d bytes (should be minimal)", memGrowth)
	}
	
	b.ReportMetric(float64(memGrowth), "bytes/memory-growth")
}

// BenchmarkCPUUsageDuringIdle measures CPU cycles during idle state
func BenchmarkCPUUsageDuringIdle(b *testing.B) {
	catcher := NewStdCatcher()
	
	err := catcher.Start()
	if err != nil {
		b.Fatalf("Failed to start catcher: %v", err)
	}
	defer catcher.ShutdownGracefully()
	
	// Let it enter idle state
	time.Sleep(200 * time.Millisecond)
	
	b.ResetTimer()
	b.ReportAllocs()
	
	// This benchmark measures the "cost" of maintaining idle state
	// Lower ns/op indicates better CPU efficiency during idle
	for i := 0; i < b.N; i++ {
		// Minimal work - just maintaining idle state
		time.Sleep(10 * time.Millisecond)
		
		// Force some measurement work to ensure benchmark is meaningful
		_ = catcher.GetPollingMode()
	}
}
