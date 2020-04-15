package main

import (
	"bufio"
	"io"
	"io/ioutil"
	"strings"
	"unicode"
)

type ioCore struct {
	in  io.RuneScanner
	out writeFlusher

	logfn   func(mess string, args ...interface{})
	closers []io.Closer
}

func (ioc *ioCore) withLogPrefix(prefix string) func() {
	logfn := ioc.logfn
	ioc.logfn = func(mess string, args ...interface{}) {
		logfn(prefix+mess, args...)
	}
	return func() {
		ioc.logfn = logfn
	}
}

func (ioc *ioCore) Close() (err error) {
	for i := len(ioc.closers) - 1; i >= 0; i-- {
		if cerr := ioc.closers[i].Close(); err == nil {
			err = cerr
		}
	}
	return err
}

func (ioc ioCore) logf(mess string, args ...interface{}) {
	if ioc.logfn != nil {
		ioc.logfn(mess, args...)
	}
}

func writeRune(w io.Writer, r rune) (err error) {
	type runeWriter interface {
		WriteRune(r rune) (size int, err error)
	}
	if r < 0x80 {
		if bw, ok := w.(io.ByteWriter); ok {
			err = bw.WriteByte(byte(r))
		} else {
			_, err = w.Write([]byte{byte(r)})
		}
	} else if rw, ok := w.(runeWriter); ok {
		_, err = rw.WriteRune(r)
	} else if sw, ok := w.(io.StringWriter); ok {
		_, err = sw.WriteString(string(r))
	} else {
		_, err = w.Write([]byte(string(r)))
	}
	return err
}

func newRuneScanner(r io.Reader) io.RuneScanner {
	if rs, is := r.(io.RuneScanner); is {
		return rs
	}
	return bufio.NewReader(r)
}

type writeFlusher interface {
	io.Writer
	Flush() error
}

var discardWriteFlusher writeFlusher = nopFlusher{ioutil.Discard}

func newWriteFlusher(w io.Writer) writeFlusher {
	// discard writer does not need flushing
	if w == ioutil.Discard {
		return discardWriteFlusher
	}

	if wf, is := w.(writeFlusher); is {
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

func (vm *VM) scan() string {
	var sb strings.Builder
	for {
		if r, _, err := vm.in.ReadRune(); err == io.EOF {
			vm.halt(errHalt)
		} else if err != nil {
			vm.halt(err)
		} else if !unicode.IsSpace(r) {
			sb.WriteRune(r)
			break
		}
	}
	for {
		if r, _, err := vm.in.ReadRune(); err == io.EOF {
			break
		} else if err != nil {
			vm.halt(err)
		} else if unicode.IsSpace(r) {
			break
		} else {
			sb.WriteRune(r)
		}
	}
	return sb.String()
}

type writeFlushers []writeFlusher

func (wfs writeFlushers) Write(p []byte) (n int, err error) {
	for _, wf := range wfs {
		n, err = wf.Write(p)
		if err != nil {
			return n, err
		}
		if n != len(p) {
			return n, io.ErrShortWrite
		}
	}
	return len(p), nil
}

func (wfs writeFlushers) Flush() (err error) {
	for _, wf := range wfs {
		if ferr := wf.Flush(); err == nil {
			err = ferr
		}
	}
	return err
}

func appendWriteFlusher(all writeFlushers, some ...writeFlusher) writeFlushers {
	for _, one := range some {
		if many, ok := one.(writeFlushers); ok {
			all = append(all, many...)
		} else if one != nil {
			all = append(all, one)
		}
	}
	return all
}

func multiWriteFlusher(a, b writeFlusher) writeFlusher {
	switch wfs := appendWriteFlusher(nil, a, b); len(wfs) {
	case 0:
		return nil
	case 1:
		return wfs[0]
	default:
		return wfs
	}
}
