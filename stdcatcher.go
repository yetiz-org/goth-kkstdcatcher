package kkstdcatcher

import (
	"bytes"
	"io"
	"os"
	"sync"
	"time"
)

var (
	gStdout          = os.Stdout
	gStderr          = os.Stderr
	shutdown         = false
	shutdownWG       = sync.WaitGroup{}
	once             = sync.Once{}
	WriteOutInterval = 10 * time.Millisecond
	StdoutWriteFunc  = func(s string) {}
	StderrWriteFunc  = func(s string) {}
	stdoutChan       = make(chan string, 16)
	stderrChan       = make(chan string, 16)
)

func _RunCatch() {
	stdoutR, stdoutW, _ := os.Pipe()
	stderrR, stderrW, _ := os.Pipe()
	os.Stdout = stdoutW
	os.Stderr = stderrW

	go func() {
		shutdownWG.Add(1)
		var stdoutBuf bytes.Buffer
		for !shutdown {
			stdoutR.SetReadDeadline(time.Now().Add(WriteOutInterval))
			if wc, err := io.Copy(&stdoutBuf, stdoutR); err == nil && wc > 0 {
				stdoutChan <- stdoutBuf.String()
				stdoutBuf = bytes.Buffer{}
			} else if err != nil && !os.IsTimeout(err) {
				println(err.Error())
				shutdown = true
				continue
			}
		}

		stdoutW.Close()
		shutdownWG.Done()
	}()

	go func() {
		for !shutdown {
			select {
			case str := <-stdoutChan:
				StdoutWriteFunc(str)
				gStdout.WriteString(str)
			case str := <-stderrChan:
				StderrWriteFunc(str)
				gStderr.WriteString(str)
			case <-time.After(WriteOutInterval):
				continue
			}
		}

		close(stdoutChan)
		close(stderrChan)
	}()

	go func() {
		shutdownWG.Add(1)
		var stderrBuf bytes.Buffer
		for !shutdown {
			stderrR.SetReadDeadline(time.Now().Add(WriteOutInterval))
			if wc, err := io.Copy(&stderrBuf, stderrR); err == nil && wc > 0 {
				stderrChan <- stderrBuf.String()
				stderrBuf = bytes.Buffer{}
			} else if err != nil && !os.IsTimeout(err) {
				println(err.Error())
				shutdown = true
				continue
			}
		}

		stderrW.Close()
		shutdownWG.Done()
	}()
}

func Start() {
	once.Do(func() {
		_RunCatch()
	})
}

func ShutdownGracefully() {
	shutdown = true
	shutdownWG.Wait()
}
