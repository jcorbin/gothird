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
	in      io.RuneReader
	inQueue []io.Reader

	lastLine inLine
	scanLine inLine

	out writeFlusher

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
		ioc.in = newRuneReader(r)
		ioc.scanLine.fileName = nameOf(r)
		ioc.scanLine.number = 1
	}
	return ioc.in != nil
}

func (ioc *ioCore) Close() (err error) {
	for i := len(ioc.closers) - 1; i >= 0; i-- {
		if cerr := ioc.closers[i].Close(); err == nil {
			err = cerr
		}
	}
	return err
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

func newRuneReader(r io.Reader) io.RuneReader {
	switch impl := r.(type) {
	case io.RuneReader:
		return impl
	case readerName:
		br := bufio.NewReader(impl.Reader)
		return runeReaderName{br, br, impl.name}
	case named:
		return runeReaderName{r, bufio.NewReader(r), impl.Name()}
	default:
		return bufio.NewReader(r)
	}
}

type readerName struct {
	io.Reader
	name string
}

type runeReaderName struct {
	io.Reader
	io.RuneReader
	name string
}

func (nr readerName) Name() string     { return nr.name }
func (nr runeReaderName) Name() string { return nr.name }

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

type named interface {
	Name() string
}

func nameOf(obj interface{}) string {
	if nom, ok := obj.(named); ok {
		return nom.Name()
	}
	return fmt.Sprintf("<unnamed %T>", obj)
}
