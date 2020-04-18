package main

import (
	"bufio"
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"regexp"
	"sync"
	"time"
)

func main() {
	var (
		timeout  time.Duration
		trace    bool
		memLimit int
		retBase              = 16
		memBase              = 80
		kernel   io.WriterTo = thirdKernel
	)

	flag.DurationVar(&timeout, "timeout", 0, "specify a time limit")
	flag.BoolVar(&trace, "trace", false, "enable trace logging")
	flag.IntVar(&memLimit, "mem-limit", 0, "enable memory limit")
	flag.Parse()

	if err := func(ctx context.Context, opts ...VMOption) error {
		if memLimit != 0 {
			opts = append(opts, WithMemLimit(memLimit))
		}

		if trace {
			tg := newScanGrouper(os.Stderr)
			defer tg.Close()
			opts = append(opts, WithLogf((&logger{
				prefix: "TRACE: ",
				w:      tg,
			}).printf))
		}

		vm := New(opts...)

		if timeout != 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, timeout)
			defer cancel()
		}

		return vm.Run(ctx)
	}(context.Background(),
		WithMemLayout(retBase, memBase),
		WithInputWriter(kernel),
		WithInput(os.Stdin),
		WithOutput(os.Stdout),
	); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %+v\n", err)
		os.Exit(1)
	}
}

var scanPattern = regexp.MustCompile(`scan .* from (.+:\d+)`)

func newScanGrouper(out io.Writer) io.WriteCloser {
	var lastLoc string
	return runMarkScanner("scan grouper", out, func(sc *markScanner) error {
		if match := scanPattern.FindSubmatch(sc.Bytes()); len(match) > 0 {
			if loc := string(match[1]); lastLoc != loc {
				if sc.Last.closeMark(); sc.Next() {
					sc.Last.openMark()
				}
				lastLoc = loc
			}
		}
		return nil
	})
}

func runMarkScanner(name string, out io.Writer, fn func(sc *markScanner) error) io.WriteCloser {
	return runPipeWorker(name, func(r io.Reader) (rerr error) {
		sc := markScanner{
			Scanner: bufio.NewScanner(r),
			out:     out,
		}
		defer sc.Close()
		for sc.Scan() {
			if err := fn(&sc); err != nil {
				return err
			}
		}
		return sc.Err()
	})
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

func runPipeWorker(name string, fun func(r io.Reader) error) io.WriteCloser {
	var work pipeWorker
	pr, pw := io.Pipe()
	work.name = name
	work.WriteCloser = pw
	work.done = make(chan error)
	go func(rc io.ReadCloser) {
		work.done <- isolate(name, func() error {
			defer rc.Close()
			return fun(rc)
		})
	}(pr)
	return work
}

type markScanner struct {
	Last markBuffer
	*bufio.Scanner

	pend  bool
	prior bool
	out   io.Writer
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
	return sc.Flush()
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

type logger struct {
	sync.Mutex
	w      io.Writer
	prefix string
	buf    lineBuffer
	err    error
}

func (log *logger) printf(mess string, args ...interface{}) {
	log.Lock()
	defer log.Unlock()

	if log.err != nil {
		return
	}

	log.buf.WriteString(log.prefix)
	if len(args) > 0 {
		fmt.Fprintf(&log.buf, mess, args...)
	} else {
		log.buf.WriteString(mess)
	}
	_, log.err = log.buf.WriteTo(log.w)
}

type lineBuffer struct{ bytes.Buffer }

func (buf *lineBuffer) WriteTo(w io.Writer) (n int64, err error) {
	if buf.Len() == 0 {
		buf.WriteByte('\n')
	} else if b := buf.Bytes(); b[len(b)-1] != '\n' {
		buf.WriteByte('\n')
	}
	return buf.Buffer.WriteTo(w)
}
