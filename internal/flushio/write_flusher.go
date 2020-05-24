package flushio

import (
	"bufio"
	"io"
	"io/ioutil"
)

// WriteFlusher is a flush-able io.Writer.
type WriteFlusher interface {
	io.Writer
	Flush() error
}

var discardWriteFlusher WriteFlusher = nopFlusher{ioutil.Discard}

// NewWriteFlusher creates a new flushable writer: if the given writer is a
// buffer, a wrapping with a noop Flush is returned; otherwise, unless the
// original writer WriteFlusher, a new bufio.Writer is returned.
func NewWriteFlusher(w io.Writer) WriteFlusher {
	// discard writer does not need flushing
	if w == ioutil.Discard {
		return discardWriteFlusher
	}

	if wf, is := w.(WriteFlusher); is {
		return wf
	}

	// in memory buffers, as implemented by types like bytes.Buffer and
	// strings.Builder, do not need to be flushed
	type buffer interface {
		io.Writer
		Cap() int
		Len() int
		Grow(n int)
		Reset()
	}
	if _, isBuffer := w.(buffer); isBuffer {
		return nopFlusher{w}
	}

	return bufio.NewWriter(w)
}

type nopFlusher struct{ io.Writer }

func (nf nopFlusher) Flush() error { return nil }
