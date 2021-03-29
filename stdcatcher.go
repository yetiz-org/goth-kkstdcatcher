package kkstdcatcher

import (
	"bytes"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

var (
	gStdout          = os.Stdout
	gStderr          = os.Stderr
	shutdown         = false
	shutdownWG       = sync.WaitGroup{}
	WriteOutInterval = 100 * time.Millisecond
	StdoutWriteFunc  = func(s string) {}
	StderrWriteFunc  = func(s string) {}
)

func _RunCatch() {
	stdoutR, stdoutW, _ := os.Pipe()
	stderrR, stderrW, _ := os.Pipe()
	os.Stdout = stdoutW
	os.Stderr = stderrW
	var stdoutBuf bytes.Buffer
	var stderrBuf bytes.Buffer
	for {
		if shutdown {
			break
		}

		next := time.Now().Add(WriteOutInterval)
		stdoutR.SetReadDeadline(next)
		stderrR.SetReadDeadline(next)
		if wc, err := io.Copy(&stdoutBuf, stdoutR); err != nil && !os.IsTimeout(err) {
			println(err.Error())
			shutdownWG.Add(1)
			shutdown = true
			continue
		} else if wc > 0 {
			if str := stdoutBuf.String(); str != "" {
				stdoutBuf = bytes.Buffer{}
				for _, s := range strings.Split(str, "\n") {
					StdoutWriteFunc(s)
				}

				gStdout.WriteString(str)
			}
		}

		if wc, err := io.Copy(&stderrBuf, stderrR); err != nil && !os.IsTimeout(err) {
			println(err.Error())
			shutdownWG.Add(1)
			shutdown = true
			continue
		} else if wc > 0 {
			if str := stderrBuf.String(); str != "" {
				stderrBuf = bytes.Buffer{}
				for _, s := range strings.Split(str, "\n") {
					StderrWriteFunc(s)
				}

				gStderr.WriteString(str)
			}
		}
	}

	stdoutW.Close()
	stderrW.Close()
	shutdownWG.Done()
}

func Start() {
	go _RunCatch()
}

func ShutdownGracefully() {
	shutdownWG.Add(1)
	shutdown = true
	shutdownWG.Wait()
}
