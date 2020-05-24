package logio

import (
	"bytes"
	"fmt"
	"io"
	"sync"
)

// Logger implements a leveled logging facility, around a wrap-able output stream.
type Logger struct {
	sync.Mutex
	output   io.WriteCloser
	fallback io.WriteCloser
	buf      bytes.Buffer
	exitCode int
}

// SetOutput sets the logger's output stream, closing any prior stream, and any
// wrapper.
func (log *Logger) SetOutput(out io.WriteCloser) {
	log.Lock()
	defer log.Unlock()
	if log.fallback != nil {
		log.fallback.Close()
		log.fallback = nil
	}
	if log.output != nil {
		log.output.Close()
	}
	log.output = out
}

// Wrap the output stream through the given pipe function.
func (log *Logger) Wrap(pipe func(wc io.WriteCloser) io.WriteCloser) {
	log.Lock()
	defer log.Unlock()
	wc := log.output
	if log.fallback == nil {
		log.fallback = wc
		wc = writeNoCloser{wc}
	}
	log.output = pipe(wc)
}

// Unwrap closes any pipe output stream, returning to the original output stream.
func (log *Logger) Unwrap() {
	log.Lock()
	defer log.Unlock()
	log.unwrap()
}

func (log *Logger) unwrap() {
	if log.fallback != nil {
		out := log.output
		log.output = log.fallback
		log.fallback = nil
		if err := out.Close(); err != nil {
			log.reportError(err)
		}
	}
}

// ExitCode returns a code to pass to os.Exit, facilitating "exit non-zero if
// any error log" semantics.
func (log *Logger) ExitCode() int {
	log.Lock()
	defer log.Unlock()
	log.unwrap()
	return log.exitCode
}

// Close any pipe wrapper.
// TODO should we close the output stream itself?
func (log *Logger) Close() {
	log.Lock()
	defer log.Unlock()
	log.unwrap()
}

// Leveledf returns a typical printf-style formatting function that logs
// messages with the given level.
func (log *Logger) Leveledf(level string) func(mess string, args ...interface{}) {
	return func(mess string, args ...interface{}) { log.Printf(level, mess, args...) }
}

// ErrorIf logs any non-nil error through Errorf.
func (log *Logger) ErrorIf(err error) {
	if err != nil {
		log.Lock()
		defer log.Unlock()
		log.reportError(err)
	}
}

// Errorf is loie `Printf("ERROR", ...)` but additionally retains state so that
// ExitCode() will return non-zero.
func (log *Logger) Errorf(mess string, args ...interface{}) {
	log.Lock()
	defer log.Unlock()
	log.unwrap()
	log.printf("ERROR", mess, args...)
	log.exitCode = 1
}

// Printf prints a line to the output stream like "level: message...\n".
// Reports any io error as an "ERROR" level log, and retains similar state for ExitCode().
func (log *Logger) Printf(level, mess string, args ...interface{}) {
	log.Lock()
	defer log.Unlock()
	if err := log.printf(level, mess, args...); err != nil {
		log.reportError(err)
	}
}

func (log *Logger) printf(level, mess string, args ...interface{}) error {
	if level != "" {
		log.buf.WriteString(level)
		log.buf.WriteString(": ")
	}
	if len(args) > 0 {
		fmt.Fprintf(&log.buf, mess, args...)
	} else {
		log.buf.WriteString(mess)
	}
	if b := log.buf.Bytes(); len(b) > 0 && b[len(b)-1] != '\n' {
		log.buf.WriteByte('\n')
	}
	_, err := log.buf.WriteTo(log.output)
	return err
}

func (log *Logger) reportError(err error) {
	if log.fallback != nil {
		log.output.Close()
		log.output = log.fallback
		log.fallback = nil
	}
	log.printf("ERROR", "%+v", err)
	log.exitCode = 2
}

type writeNoCloser struct{ io.Writer }

func (writeNoCloser) Close() error { return nil }
