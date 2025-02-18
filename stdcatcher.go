package kkstdcatcher

import (
	"os"
	"sync"
	"sync/atomic"
)

var (
	defaultInstance = NewStdCatcher()
)

type StdCatcher struct {
	stdin, stdout, stderr                              *os.File
	stdinR, stdinW, stdoutR, stdoutW, stderrR, stderrW *os.File
	shutdownChan                                       chan bool
	shutdownWG                                         sync.WaitGroup
	stdReadIntervalMs                                  int
	shutdownSig                                        int32
	StdinWriteFunc, StdoutWriteFunc, StderrWriteFunc   func(s string)
	stdinChan, stdoutChan, stderrChan                  chan string
}

func NewStdCatcher() *StdCatcher {
	return &StdCatcher{
		shutdownChan:      make(chan bool),
		stdReadIntervalMs: 1000,
		StdinWriteFunc:    func(s string) {},
		StdoutWriteFunc:   func(s string) {},
		StderrWriteFunc:   func(s string) {},
		stdinChan:         make(chan string, 64),
		stdoutChan:        make(chan string, 64),
		stderrChan:        make(chan string, 64),
	}
}

func (s *StdCatcher) SetStdReadIntervalMs(ms int) {
	s.stdReadIntervalMs = ms
}

func (s *StdCatcher) Start() {
	s.stdin, s.stdout, s.stderr = os.Stdin, os.Stdout, os.Stderr
	s.stdinR, s.stdinW, _ = os.Pipe()
	s.stdoutR, s.stdoutW, _ = os.Pipe()
	s.stderrR, s.stderrW, _ = os.Pipe()

	os.Stdin = s.stdinW
	os.Stdout = s.stdoutW
	os.Stderr = s.stderrW

	// implement stdin
	go s._StdCatchRun(&s.shutdownWG, s.stdinR, s.stdinChan)

	// implement stdout
	go s._StdCatchRun(&s.shutdownWG, s.stdoutR, s.stdoutChan)

	// implement stderr
	go s._StdCatchRun(&s.shutdownWG, s.stderrR, s.stderrChan)

	go func() {
		for done := false; !done; {
			select {
			case <-s.shutdownChan:
				atomic.AddInt32(&s.shutdownSig, 1)
				done = true
			case str := <-s.stdinChan:
				s.StdinWriteFunc(str)
				s.stdin.WriteString(str)
			case str := <-s.stdoutChan:
				s.StdoutWriteFunc(str)
				s.stdout.WriteString(str)
			case str := <-s.stderrChan:
				s.StderrWriteFunc(str)
				s.stderr.WriteString(str)
			}
		}

		close(s.stdinChan)
		close(s.stdoutChan)
		close(s.stderrChan)
	}()
}

func (s *StdCatcher) _StdCatchRun(wg *sync.WaitGroup, stdR *os.File, stdChan chan string) {
	wg.Add(1)
	for done := false; !done; {
		var buf = make([]byte, 1024)
		if wc, err := stdR.Read(buf); err == nil && wc > 0 {
			if atomic.LoadInt32(&s.shutdownSig) == 0 {
				stdChan <- string(buf[:wc])
			}
		} else if err != nil && !os.IsTimeout(err) {
			println(err.Error())
			done = true
		}
	}

	wg.Done()
}

func (s *StdCatcher) ShutdownGracefully() {
	s.shutdownChan <- true
	s.stdinR.Close()
	s.stdoutR.Close()
	s.stderrR.Close()
	s.shutdownWG.Wait()
}

func DefaultInstance() *StdCatcher {
	return defaultInstance
}

func Start() {
	defaultInstance.Start()
}

func ShutdownGracefully() {
	defaultInstance.ShutdownGracefully()
}
