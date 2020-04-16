package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
)

type inLine struct {
	fileName string
	number   int
	bytes.Buffer
}

func (il inLine) String() string {
	return fmt.Sprintf("%v:%v %q", il.fileName, il.number, il.Buffer.String())
}

type ioCore struct {
	in      io.RuneScanner
	inQueue []io.Reader

	lastLine inLine
	scanLine inLine

	out writeFlusher

	logfn func(mess string, args ...interface{})

	closers []io.Closer
}

func (ioc *ioCore) readRune() (rune, error) {
	if ioc.in == nil && !ioc.nextIn() {
		return 0, io.EOF
	}

	r, _, err := ioc.in.ReadRune()
	if r == '\n' {
		ioc.nextLine()
	} else {
		ioc.scanLine.WriteRune(r)
	}

	if r != 0 {
		return r, nil
	}
	if err == io.EOF && ioc.nextIn() {
		err = nil
	}
	return 0, err
}

func (ioc *ioCore) nextLine() {
	ioc.lastLine.Reset()
	ioc.lastLine.fileName = ioc.scanLine.fileName
	ioc.lastLine.number = ioc.scanLine.number
	ioc.lastLine.Write(ioc.scanLine.Bytes())
	ioc.scanLine.Reset()
	ioc.scanLine.number++
}

func (ioc *ioCore) nextIn() bool {
	ioc.nextLine()
	if ioc.in != nil {
		if cl, ok := ioc.in.(io.Closer); ok {
			cl.Close()
		}
		ioc.in = nil
	}
	if len(ioc.inQueue) > 0 {
		r := ioc.inQueue[0]
		ioc.inQueue = ioc.inQueue[1:]
		ioc.in = newRuneScanner(r)
		ioc.scanLine.fileName = nameOf(r)
		ioc.scanLine.number = 1
	}
	return ioc.in != nil
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

func nameOf(obj interface{}) string {
	if nom, ok := obj.(interface{ Name() string }); ok {
		return nom.Name()
	}
	return fmt.Sprintf("<unnamed %T>", obj)
}
