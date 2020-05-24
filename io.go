package main

import (
	"bufio"
	"bytes"
	"io"
	"regexp"

	"github.com/jcorbin/gothird/internal/fileinput"
	"github.com/jcorbin/gothird/internal/flushio"
	"github.com/jcorbin/gothird/internal/panicerr"
)

type ioCore struct {
	fileinput.Input
	out     flushio.WriteFlusher
	closers []io.Closer
}

func (ioc *ioCore) Close() (err error) {
	for i := len(ioc.closers) - 1; i >= 0; i-- {
		if cerr := ioc.closers[i].Close(); err == nil {
			err = cerr
		}
	}
	return err
}

func runMarkScanner(name string, out io.WriteCloser, sc scanner) io.WriteCloser {
	return runPipeWorker(name, func(r io.Reader) (rerr error) {
		ms := markScanner{
			Scanner: bufio.NewScanner(r),
			out:     out,
		}
		defer func() {
			if err := ms.Close(); rerr == nil {
				rerr = err
			}
		}()
		for ms.Scan() {
			sc.scan(&ms)
		}
		return ms.Err()
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
	work.done <- panicerr.Recover(work.name, func() error {
		defer rc.Close()
		err := fun(rc)
		return err
	})
}

func scanPipe(name string, scs ...scanner) func(out io.WriteCloser) io.WriteCloser {
	sc := scanners(scs...)
	return func(out io.WriteCloser) io.WriteCloser {
		return runMarkScanner(name, out, sc)
	}
}

func patternScanner(pattern *regexp.Regexp, ss ...subscanner) scanner {
	return regexpScanner{pattern, subscanners(ss...)}
}

type scanner interface {
	scan(ms *markScanner) bool
}

type subscanner interface {
	scan(ms *markScanner, submatch [][]byte) bool
}

func scanners(ss ...scanner) scanner {
	switch len(ss) {
	case 0:
		return nil
	case 1:
		return ss[0]
	default:
		return firstScanner(ss)
	}
}

func subscanners(ss ...subscanner) subscanner {
	switch len(ss) {
	case 0:
		return nil
	case 1:
		return ss[0]
	default:
		return firstSubscanner(ss)
	}
}

type firstScanner []scanner
type firstSubscanner []subscanner

type regexpScanner struct {
	*regexp.Regexp
	subscanner
}

func (sc regexpScanner) scan(ms *markScanner) bool {
	if submatch := sc.FindSubmatch(ms.Bytes()); len(submatch) > 0 {
		return sc.subscanner.scan(ms, submatch)
	}
	return false
}

func (ss firstScanner) scan(ms *markScanner) bool {
	for _, s := range ss {
		if s.scan(ms) {
			return true
		}
	}
	return false
}

func (ss firstSubscanner) scan(ms *markScanner, submatch [][]byte) bool {
	for _, s := range ss {
		if s.scan(ms, submatch) {
			return true
		}
	}
	return false
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
	level  int
	opened bool
}

func (buf *markBuffer) openMark() {
	buf.level++
	buf.WriteString(openMark)
	buf.opened = true
}

func (buf *markBuffer) closeMark() {
	if buf.opened {
		b := buf.Next(buf.Len())
		if i := bytes.Index(b, []byte(openMark)); i >= 0 {
			b = b[:i]
		}
		buf.Write(b)
		buf.opened = false
	} else if buf.level > 0 {
		buf.level--
		buf.WriteString(closeMark)
	}
}

func (buf *markBuffer) WriteTo(w io.Writer) (n int64, err error) {
	buf.opened = false
	return buf.lineBuffer.WriteTo(w)
}

type lineBuffer struct{ bytes.Buffer }

func (buf *lineBuffer) WriteTo(w io.Writer) (n int64, err error) {
	if b := buf.Bytes(); len(b) == 0 || b[len(b)-1] != '\n' {
		buf.WriteByte('\n')
	}
	return buf.Buffer.WriteTo(w)
}
