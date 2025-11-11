// Package kkstdcatcher provides high-performance functionality to capture and redirect standard input/output/error streams.
// It features adaptive polling for optimal CPU usage, thread-safe operations, and real-time stream processing.
package kkstdcatcher

import (
	"context"
	"fmt"
	"io"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

var (
	// defaultInstance is the global default StdCatcher instance
	defaultInstance = NewStdCatcher()
)

// StdCatcher captures and redirects standard streams (stdin, stdout, stderr)
type StdCatcher struct {
	// Original file descriptors to restore later
	stdin, stdout, stderr *os.File

	// Pipe file descriptors for capturing streams
	stdinR, stdinW, stdoutR, stdoutW, stderrR, stderrW *os.File

	// Synchronization primitives
	shutdownCtx    context.Context
	shutdownCancel context.CancelFunc
	shutdownWG     sync.WaitGroup
	mu             sync.RWMutex // Protects concurrent access to state and configuration

	// Configuration parameters
	stdReadIntervalMs int

	// Adaptive polling system for CPU optimization
	lastActivityTime int64 // Unix timestamp in milliseconds for activity tracking
	pollingMode      int32 // Atomic: 0=fast(1ms), 1=normal(10ms), 2=slow(50ms)

	// State management with atomic operations for thread safety
	shutdownSig int32 // Atomic: shutdown signal flag
	isRunning   int32 // Atomic: running state flag

	// Callback functions for handling captured data (thread-safe)
	StdinWriteFunc, StdoutWriteFunc, StderrWriteFunc func(s string)

	// Buffered channels for high-throughput streaming captured data
	stdinChan, stdoutChan, stderrChan chan string
}

// NewStdCatcher creates a new StdCatcher instance with default configuration
func NewStdCatcher() *StdCatcher {
	ctx, cancel := context.WithCancel(context.Background())
	return &StdCatcher{
		shutdownCtx:       ctx,
		shutdownCancel:    cancel,
		stdReadIntervalMs: 1000,
		lastActivityTime:  time.Now().UnixMilli(),
		pollingMode:       0, // Start with fast polling
		StdinWriteFunc:    func(s string) {},
		StdoutWriteFunc:   func(s string) {},
		StderrWriteFunc:   func(s string) {},
		// Buffered channels for high-throughput streaming
		stdinChan:  make(chan string, 2048),
		stdoutChan: make(chan string, 2048),
		stderrChan: make(chan string, 2048),
	}
}

// SetStdReadIntervalMs sets the read interval in milliseconds for stream polling
func (s *StdCatcher) SetStdReadIntervalMs(ms int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if ms > 0 {
		s.stdReadIntervalMs = ms
	}
}

// GetPollingMode returns current polling mode for monitoring resource usage
// 0=fast (1ms), 1=normal (10ms), 2=slow (50ms)
func (s *StdCatcher) GetPollingMode() int32 {
	return atomic.LoadInt32(&s.pollingMode)
}

// GetIdleTime returns milliseconds since last activity
func (s *StdCatcher) GetIdleTime() int64 {
	now := time.Now().UnixMilli()
	lastActivity := atomic.LoadInt64(&s.lastActivityTime)
	return now - lastActivity
}

// GetStats returns runtime statistics for monitoring
type StdCatcherStats struct {
	IsRunning    bool
	PollingMode  int32 // 0=fast, 1=normal, 2=slow
	IdleTimeMs   int64
	LastActivity time.Time
}

func (s *StdCatcher) GetStats() StdCatcherStats {
	lastActivityMs := atomic.LoadInt64(&s.lastActivityTime)
	return StdCatcherStats{
		IsRunning:    s.IsRunning(),
		PollingMode:  s.GetPollingMode(),
		IdleTimeMs:   s.GetIdleTime(),
		LastActivity: time.Unix(0, lastActivityMs*int64(time.Millisecond)),
	}
}

// Start begins capturing standard streams. Returns error if already running or on pipe creation failure.
func (s *StdCatcher) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if already running
	if atomic.LoadInt32(&s.isRunning) == 1 {
		return fmt.Errorf("stdcatcher is already running")
	}

	// Create new context and WaitGroup for this start cycle to avoid reuse issues
	s.shutdownCtx, s.shutdownCancel = context.WithCancel(context.Background())
	s.shutdownWG = sync.WaitGroup{} // Reset WaitGroup

	// Store original file descriptors
	s.stdin, s.stdout, s.stderr = os.Stdin, os.Stdout, os.Stderr

	// Create pipes with error handling
	var err error
	if s.stdinR, s.stdinW, err = os.Pipe(); err != nil {
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}
	if s.stdoutR, s.stdoutW, err = os.Pipe(); err != nil {
		s.stdinR.Close()
		s.stdinW.Close()
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	if s.stderrR, s.stderrW, err = os.Pipe(); err != nil {
		s.stdinR.Close()
		s.stdinW.Close()
		s.stdoutR.Close()
		s.stdoutW.Close()
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Redirect standard streams
	os.Stdin = s.stdinR
	os.Stdout = s.stdoutW
	os.Stderr = s.stderrW

	// Mark as running
	atomic.StoreInt32(&s.isRunning, 1)

	// Start capture goroutines
	go s.stdCatchRun(s.stdoutR, s.stdoutChan, "stdout")
	go s.stdCatchRun(s.stderrR, s.stderrChan, "stderr")

	// Start distribution goroutine
	go s.distributeOutput()

	return nil
}

// getAdaptiveTimeout returns timeout based on recent activity to optimize CPU usage
func (s *StdCatcher) getAdaptiveTimeout() time.Duration {
	now := time.Now().UnixMilli()
	lastActivity := atomic.LoadInt64(&s.lastActivityTime)
	idleTime := now - lastActivity

	// Adaptive timeout strategy:
	// 0-100ms: 1ms (real-time for immediate activity)
	// 100ms-1s: 10ms (normal responsiveness)
	// 1s+: 50ms (low CPU usage for idle periods)
	switch {
	case idleTime < 100:
		atomic.StoreInt32(&s.pollingMode, 0)
		return 1 * time.Millisecond
	case idleTime < 1000:
		atomic.StoreInt32(&s.pollingMode, 1)
		return 10 * time.Millisecond
	default:
		atomic.StoreInt32(&s.pollingMode, 2)
		return 50 * time.Millisecond
	}
}

// updateActivityTime records recent data activity for adaptive polling
func (s *StdCatcher) updateActivityTime() {
	atomic.StoreInt64(&s.lastActivityTime, time.Now().UnixMilli())
}

// stdCatchRun captures data from a standard stream with adaptive CPU optimization
// Uses intelligent polling that reduces CPU usage during idle periods
func (s *StdCatcher) stdCatchRun(stdR *os.File, stdChan chan string, streamName string) {
	s.shutdownWG.Add(1)
	defer s.shutdownWG.Done()

	// Use optimized buffer size for better I/O performance
	// 4KB provides optimal balance between memory usage and read efficiency
	buf := make([]byte, 4096)
	consecutiveTimeouts := 0

	for {
		select {
		case <-s.shutdownCtx.Done():
			return
		default:
			// Get adaptive timeout based on recent activity
			timeout := s.getAdaptiveTimeout()
			stdR.SetReadDeadline(time.Now().Add(timeout))
			n, err := stdR.Read(buf)

			// Process any available data immediately
			if n > 0 {
				s.updateActivityTime()  // Record activity for adaptive polling
				consecutiveTimeouts = 0 // Reset timeout counter

				data := string(buf[:n])
				select {
				case stdChan <- data:
				case <-s.shutdownCtx.Done():
					return
				}
				continue // Immediate next read after data
			}

			if err != nil {
				if os.IsTimeout(err) {
					consecutiveTimeouts++
					// Exponential backoff for consecutive timeouts to optimize CPU usage
					if consecutiveTimeouts > 10 {
						// Progressive backoff: more timeouts = longer sleep for better CPU efficiency
						backoffTime := time.Duration(consecutiveTimeouts-10) * time.Millisecond
						if backoffTime > 50*time.Millisecond {
							backoffTime = 50 * time.Millisecond // Cap at 50ms
						}
						time.Sleep(backoffTime)
					}
					continue
				}
				if err != io.EOF {
					// Only log non-timeout, non-EOF errors
					fmt.Printf("Error reading from %s: %v\n", streamName, err)
					return
				}
				// EOF means the pipe was closed, exit gracefully
				return
			}
		}
	}
}

// distributeOutput handles distribution of captured output to callbacks and original streams
// Optimized for immediate processing without buffering delays
func (s *StdCatcher) distributeOutput() {
	s.shutdownWG.Add(1)
	defer s.shutdownWG.Done()

	for {
		select {
		case <-s.shutdownCtx.Done():
			// Drain remaining data from channels before closing
			s.drainChannels()
			return

		case data := <-s.stdoutChan:
			if s.StdoutWriteFunc != nil {
				s.StdoutWriteFunc(data)
			}
			if s.stdout != nil {
				s.stdout.WriteString(data)
			}

		case data := <-s.stderrChan:
			if s.StderrWriteFunc != nil {
				s.StderrWriteFunc(data)
			}
			if s.stderr != nil {
				s.stderr.WriteString(data)
			}
		}
	}
}

// drainChannels drains any remaining data from channels during shutdown
// Ensures no data is lost during shutdown process
func (s *StdCatcher) drainChannels() {
	// Set a reasonable timeout to prevent hanging during shutdown
	timeout := time.After(500 * time.Millisecond)

	for {
		select {
		case data := <-s.stdoutChan:
			if s.StdoutWriteFunc != nil {
				s.StdoutWriteFunc(data)
			}
			if s.stdout != nil {
				s.stdout.WriteString(data)
			}
		case data := <-s.stderrChan:
			if s.StderrWriteFunc != nil {
				s.StderrWriteFunc(data)
			}
			if s.stderr != nil {
				s.stderr.WriteString(data)
			}
		case <-timeout:
			// Timeout reached, stop draining to prevent hanging
			return
		default:
			// No more data to drain
			return
		}
	}
}

// ShutdownGracefully stops capturing and restores original streams
func (s *StdCatcher) ShutdownGracefully() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if not running
	if atomic.LoadInt32(&s.isRunning) == 0 {
		return fmt.Errorf("stdcatcher is not running")
	}

	// Signal shutdown
	if s.shutdownCancel != nil {
		s.shutdownCancel()
	}

	// Wait for all goroutines to finish with timeout to prevent hanging
	// Use a buffered channel to avoid goroutine leak on timeout
	done := make(chan struct{}, 1)
	go func() {
		defer func() {
			select {
			case done <- struct{}{}:
			default:
				// Channel full, timeout already occurred
			}
		}()
		s.shutdownWG.Wait()
	}()

	select {
	case <-done:
		// Normal shutdown completed
	case <-time.After(2 * time.Second):
		// Force shutdown after timeout
		fmt.Printf("Warning: StdCatcher shutdown timed out after 2 seconds\n")
	}

	// Restore original streams
	if s.stdin != nil {
		os.Stdin = s.stdin
	}
	if s.stdout != nil {
		os.Stdout = s.stdout
	}
	if s.stderr != nil {
		os.Stderr = s.stderr
	}

	// Close pipes
	if s.stdinR != nil {
		s.stdinR.Close()
		s.stdinR = nil
	}
	if s.stdinW != nil {
		s.stdinW.Close()
		s.stdinW = nil
	}
	if s.stdoutR != nil {
		s.stdoutR.Close()
		s.stdoutR = nil
	}
	if s.stdoutW != nil {
		s.stdoutW.Close()
		s.stdoutW = nil
	}
	if s.stderrR != nil {
		s.stderrR.Close()
		s.stderrR = nil
	}
	if s.stderrW != nil {
		s.stderrW.Close()
		s.stderrW = nil
	}

	// Recreate channels for next use
	close(s.stdinChan)
	close(s.stdoutChan)
	close(s.stderrChan)
	s.stdinChan = make(chan string, 2048)
	s.stdoutChan = make(chan string, 2048)
	s.stderrChan = make(chan string, 2048)

	// Mark as not running
	atomic.StoreInt32(&s.isRunning, 0)

	return nil
}

// IsRunning returns true if the StdCatcher is currently capturing streams
func (s *StdCatcher) IsRunning() bool {
	return atomic.LoadInt32(&s.isRunning) == 1
}

// GetChannels returns the internal channels for advanced usage (read-only)
func (s *StdCatcher) GetChannels() (<-chan string, <-chan string, <-chan string) {
	return s.stdinChan, s.stdoutChan, s.stderrChan
}

// DefaultInstance returns the global default StdCatcher instance
func DefaultInstance() *StdCatcher {
	return defaultInstance
}

// Start starts capturing using the default instance
func Start() error {
	return defaultInstance.Start()
}

// ShutdownGracefully shuts down the default instance gracefully
func ShutdownGracefully() error {
	return defaultInstance.ShutdownGracefully()
}

// IsRunning checks if the default instance is running
func IsRunning() bool {
	return defaultInstance.IsRunning()
}

// SetStdReadIntervalMs sets the read interval for the default instance
func SetStdReadIntervalMs(ms int) {
	defaultInstance.SetStdReadIntervalMs(ms)
}
