package logio

import (
	"bytes"
	"sync"
)

// Writer implements an io.Writer around a formatted logging function.
type Writer struct {
	Logf func(string, ...interface{})

	mu  sync.Mutex
	buf bytes.Buffer
}

// Write writes the given bytes into an internal buffer, then flushes any
// completed lines through Logf. This is all done while holding a lock, so that
// writing is safe from multiple goroutines.
// Returns any io error.
func (lw *Writer) Write(p []byte) (n int, err error) {
	lw.mu.Lock()
	defer lw.mu.Unlock()
	lw.buf.Write(p)
	lw.flushLines(false)
	return len(p), nil
}

// Sync flushes any remaining from the internal buffer, and returns any io error.
func (lw *Writer) Sync() error {
	lw.mu.Lock()
	defer lw.mu.Unlock()
	lw.flushLines(true)
	return nil
}

// Close calls Sync.
func (lw *Writer) Close() error {
	return lw.Sync()
}

func (lw *Writer) flushLines(all bool) {
	for lw.buf.Len() > 0 {
		i := bytes.IndexByte(lw.buf.Bytes(), '\n')
		if i >= 0 {
			lw.Logf("%s", lw.buf.Next(i))
			lw.buf.Next(1)
		} else if all {
			lw.Logf("%s", lw.buf.Next(lw.buf.Len()))
		} else {
			break
		}
	}
}
