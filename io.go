package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"sync"
)

type inLoc struct {
	fileName string
	number   int
}

type inLine struct {
	inLoc
	bytes.Buffer
}

func (loc inLoc) String() string { return fmt.Sprintf("%v:%v", loc.fileName, loc.number) }
func (il inLine) String() string { return fmt.Sprintf("%v %q", il.inLoc, il.Buffer.String()) }

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

func runMarkScanner(name string, out io.WriteCloser, fn func(sc *markScanner) error) io.WriteCloser {
	return runPipeWorker(name, func(r io.Reader) (rerr error) {
		sc := markScanner{
			Scanner: bufio.NewScanner(r),
			out:     out,
		}
		defer func() {
			if err := sc.Close(); rerr == nil {
				rerr = err
			}
		}()
		for sc.Scan() {
			if err := fn(&sc); err != nil {
				return err
			}
		}
		return sc.Err()
	})
}

func runPipeWorker(name string, fun func(r io.Reader) error) io.WriteCloser {
	pr, pw := io.Pipe()
	work := pipeWorker{
		name:        name,
		WriteCloser: pw,
		done:        make(chan error),
	}
	go work.run(pr, fun)
	return work
}

type pipeWorker struct {
	io.WriteCloser
	name string
	done chan error
}

func (work pipeWorker) Name() string { return "<" + work.name + ">" }

func (work pipeWorker) Close() error {
	err := work.WriteCloser.Close()
	if derr := <-work.done; err == nil {
		err = derr
	}
	return err
}

func (work pipeWorker) run(rc io.ReadCloser, fun func(r io.Reader) error) {
	defer close(work.done)
	work.done <- isolate(work.name, func() error {
		defer rc.Close()
		err := fun(rc)
		return err
	})
}

type markScanner struct {
	Last markBuffer
	*bufio.Scanner

	pend  bool
	prior bool
	out   io.WriteCloser
	err   error
}

func (sc *markScanner) Scan() bool {
	if sc.pend {
		sc.Next()
	}
	if sc.err != nil {
		return false
	}
	sc.pend = sc.Scanner.Scan()
	return sc.pend
}

func (sc *markScanner) Next() bool {
	if sc.pend && sc.err == nil {
		if err := sc.Flush(); err != nil {
			return false
		}
		sc.Last.Write(sc.Bytes())
		sc.prior = true
		sc.pend = false
	}
	return sc.err == nil
}

func (sc *markScanner) Flush() error {
	_, werr := sc.Last.WriteTo(sc.out)
	if sc.err == nil {
		sc.err = werr
	}
	return werr
}

func (sc *markScanner) Close() error {
	for sc.Last.level > 0 {
		sc.Last.closeMark()
		sc.Last.WriteByte('\n')
	}
	err := sc.Flush()
	if cerr := sc.out.Close(); err == nil {
		err = cerr
	}
	return err
}

func (sc *markScanner) Err() error {
	if sc.err != nil {
		return sc.err
	}
	return sc.Scanner.Err()
}

const (
	openMark  = " {{{"
	closeMark = " }}}"
)

type markBuffer struct {
	lineBuffer
	level int
}

func (buf *markBuffer) openMark() {
	buf.level++
	buf.WriteString(openMark)
}

func (buf *markBuffer) closeMark() {
	if buf.level > 0 {
		buf.level--
		buf.WriteString(closeMark)
	}
}

type lineBuffer struct{ bytes.Buffer }

func (buf *lineBuffer) WriteTo(w io.Writer) (n int64, err error) {
	if b := buf.Bytes(); len(b) == 0 || b[len(b)-1] != '\n' {
		buf.WriteByte('\n')
	}
	return buf.Buffer.WriteTo(w)
}

type logger struct {
	sync.Mutex
	Out      io.WriteCloser
	fallback io.WriteCloser
	buf      bytes.Buffer
	err      []error
	errored  bool
}

func (log *logger) Through(pipe func(wc io.WriteCloser) io.WriteCloser) {
	log.Lock()
	defer log.Unlock()
	wc := log.Out
	if log.fallback == nil {
		log.fallback = wc
		wc = writeNoCloser{wc}
	}
	log.Out = pipe(wc)
}

type writeNoCloser struct{ io.Writer }

func (writeNoCloser) Close() error { return nil }

func (log *logger) Exit() {
	log.Lock()
	defer log.Unlock()
	log.reportError(log.Out.Close())
	if log.errored {
		os.Exit(1)
	}
}

func (log *logger) Close() {
	log.Lock()
	defer log.Unlock()
	log.reportError(log.Out.Close())
}

func (log *logger) Leveledf(level string) func(mess string, args ...interface{}) {
	return func(mess string, args ...interface{}) { log.Printf(level, mess, args...) }
}

func (log *logger) Errorf(mess string, args ...interface{}) {
	log.Lock()
	defer log.Unlock()
	log.reportError(log.Out.Close())
	log.printf("ERROR", mess, args...)
	log.errored = true
}

func (log *logger) Printf(level, mess string, args ...interface{}) {
	log.Lock()
	defer log.Unlock()
	if len(log.err) == 0 {
		log.reportError(log.printf(level, mess, args...))
	}
}

func (log *logger) printf(level, mess string, args ...interface{}) error {
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
	_, err := log.buf.WriteTo(log.Out)
	return err
}

func (log *logger) reportError(err error) {
	if err == nil {
		return
	}
	log.err = append(log.err, err)
	if log.fallback != nil {
		log.Out = log.fallback
		log.fallback = nil
		for _, err := range log.err {
			if log.err != nil {
				log.Errorf("%+v", err)
			}
		}
		log.err = nil
	}
}
