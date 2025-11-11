package kkstdcatcher_test

import (
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yetiz-org/goth-kkstdcatcher"
)

// TestRapidOutput tests data integrity under extreme rapid output conditions
func TestRapidOutput(t *testing.T) {
	catcher := kkstdcatcher.NewStdCatcher()
	
	// Track captured output - accumulate all data then parse
	var mu sync.Mutex
	var capturedData strings.Builder
	
	catcher.StdoutWriteFunc = func(s string) {
		mu.Lock()
		capturedData.WriteString(s)
		mu.Unlock()
	}
	
	if err := catcher.Start(); err != nil {
		t.Fatalf("Failed to start catcher: %v", err)
	}
	defer catcher.ShutdownGracefully()
	
	// Test parameters
	goroutines := 10
	linesPerGoroutine := 1000
	totalExpected := goroutines * linesPerGoroutine
	
	var wg sync.WaitGroup
	start := time.Now()
	
	// Launch multiple goroutines for concurrent rapid output
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < linesPerGoroutine; j++ {
				fmt.Printf("goroutine_%d_line_%d\n", id, j)
			}
		}(i)
	}
	
	wg.Wait()
	elapsed := time.Since(start)
	
	// Wait for all data to be processed
	time.Sleep(500 * time.Millisecond)
	
	// Parse captured data
	mu.Lock()
	allData := capturedData.String()
	mu.Unlock()
	
	// Count lines and build map
	capturedLines := make(map[string]bool)
	lines := 0
	for _, line := range strings.Split(allData, "\n") {
		if line != "" {
			capturedLines[line+"\n"] = true
			lines++
		}
	}
	
	t.Logf("Performance Stats:")
	t.Logf("  Total lines: %d", totalExpected)
	t.Logf("  Captured: %d", lines)
	t.Logf("  Duration: %v", elapsed)
	t.Logf("  Throughput: %.0f lines/sec", float64(totalExpected)/elapsed.Seconds())
	t.Logf("  Polling mode: %d", catcher.GetPollingMode())
	
	// Check for data loss
	if lines < totalExpected {
		lossRate := float64(totalExpected-lines) / float64(totalExpected) * 100
		t.Errorf("Data loss detected: expected %d, captured %d (%.2f%% loss)", 
			totalExpected, lines, lossRate)
	}
	
	// Verify each line was captured
	missingLines := 0
	for i := 0; i < goroutines; i++ {
		for j := 0; j < linesPerGoroutine; j++ {
			line := fmt.Sprintf("goroutine_%d_line_%d\n", i, j)
			if !capturedLines[line] {
				missingLines++
				if missingLines <= 10 { // Only log first 10 missing
					t.Errorf("Missing line: %s", line[:len(line)-1])
				}
			}
		}
	}
	
	if missingLines > 0 {
		t.Errorf("Total missing lines: %d/%d", missingLines, totalExpected)
	}
}

// TestBurstOutput tests handling of burst output patterns
func TestBurstOutput(t *testing.T) {
	catcher := kkstdcatcher.NewStdCatcher()
	
	var capturedCount int64
	catcher.StdoutWriteFunc = func(s string) {
		// Count actual lines in the captured data
		lines := int64(0)
		for i := 0; i < len(s); i++ {
			if s[i] == '\n' {
				lines++
			}
		}
		atomic.AddInt64(&capturedCount, lines)
	}
	
	if err := catcher.Start(); err != nil {
		t.Fatalf("Failed to start catcher: %v", err)
	}
	defer catcher.ShutdownGracefully()
	
	// Simulate burst patterns: rapid output followed by idle periods
	bursts := 5
	linesPerBurst := 500
	
	for burst := 0; burst < bursts; burst++ {
		// Rapid burst
		for i := 0; i < linesPerBurst; i++ {
			fmt.Printf("burst_%d_line_%d\n", burst, i)
		}
		
		// Idle period
		time.Sleep(100 * time.Millisecond)
	}
	
	// Wait for processing
	time.Sleep(300 * time.Millisecond)
	
	expected := int64(bursts * linesPerBurst)
	captured := atomic.LoadInt64(&capturedCount)
	
	t.Logf("Burst test: expected %d, captured %d", expected, captured)
	
	if captured < expected {
		lossRate := float64(expected-captured) / float64(expected) * 100
		t.Errorf("Data loss in burst test: %.2f%%", lossRate)
	}
}

// TestExtremeSpeed tests maximum throughput limits
func TestExtremeSpeed(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping extreme speed test in short mode")
	}
	
	catcher := kkstdcatcher.NewStdCatcher()
	
	var capturedCount int64
	catcher.StdoutWriteFunc = func(s string) {
		// Count actual lines in the captured data
		lines := int64(0)
		for i := 0; i < len(s); i++ {
			if s[i] == '\n' {
				lines++
			}
		}
		atomic.AddInt64(&capturedCount, lines)
	}
	
	if err := catcher.Start(); err != nil {
		t.Fatalf("Failed to start catcher: %v", err)
	}
	defer catcher.ShutdownGracefully()
	
	// Extreme test: 50 goroutines, 2000 lines each = 100,000 lines
	goroutines := 50
	linesPerGoroutine := 2000
	
	var wg sync.WaitGroup
	start := time.Now()
	
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < linesPerGoroutine; j++ {
				fmt.Println(id * linesPerGoroutine + j)
			}
		}(i)
	}
	
	wg.Wait()
	elapsed := time.Since(start)
	
	// Wait for processing
	time.Sleep(1 * time.Second)
	
	expected := int64(goroutines * linesPerGoroutine)
	captured := atomic.LoadInt64(&capturedCount)
	
	t.Logf("Extreme speed test:")
	t.Logf("  Total lines: %d", expected)
	t.Logf("  Captured: %d", captured)
	t.Logf("  Duration: %v", elapsed)
	t.Logf("  Throughput: %.0f lines/sec", float64(expected)/elapsed.Seconds())
	
	lossRate := float64(expected-captured) / float64(expected) * 100
	t.Logf("  Data completeness: %.2f%%", 100-lossRate)
	
	// Allow up to 0.1% loss under extreme conditions
	if lossRate > 0.1 {
		t.Errorf("Excessive data loss: %.2f%%", lossRate)
	}
}

// BenchmarkThroughput measures throughput performance
func BenchmarkThroughput(b *testing.B) {
	catcher := kkstdcatcher.NewStdCatcher()
	catcher.StdoutWriteFunc = func(s string) {}
	
	if err := catcher.Start(); err != nil {
		b.Fatalf("Failed to start catcher: %v", err)
	}
	defer catcher.ShutdownGracefully()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		fmt.Println(i)
	}
	b.StopTimer()
	
	time.Sleep(100 * time.Millisecond) // Allow processing to complete
}
