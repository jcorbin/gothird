package main

import (
	"context"
	"flag"
	"io"
	"os"
	"regexp"
	"time"
)

func main() {
	log := logger{Out: os.Stderr}
	defer log.Exit()

	var (
		retBase              = 16
		memBase              = 80
		kernel   io.WriterTo = thirdKernel
		memLimit int
		timeout  time.Duration
		trace    bool
		dump     bool
	)

	flag.IntVar(&memLimit, "mem-limit", 0, "enable memory limit")
	flag.DurationVar(&timeout, "timeout", 0, "specify a time limit")
	flag.BoolVar(&trace, "trace", false, "enable trace logging")
	flag.BoolVar(&dump, "dump", false, "print a dump after execution")
	flag.Parse()

	if err := func(ctx context.Context, opts ...VMOption) error {
		if memLimit != 0 {
			opts = append(opts, WithMemLimit(memLimit))
		}

		if trace {
			log.Wrap(scanPipe("trace scanner",
				patternScanner(scanPattern, &locScanner{}),
				// patternScanner(stepPattern, &retScanner{}),
			))
			opts = append(opts, WithLogf(log.Leveledf("TRACE")))
		}

		vm := New(opts...)

		if dump {
			lw := &logWriter{logf: log.Leveledf("DUMP")}
			defer lw.Close()
			defer vmDumper{vm: vm, out: lw}.dump()
		}

		defer log.Unwrap()

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
		log.Errorf("%+v", err)
	}
}

var scanPattern = regexp.MustCompile(`> scan (.+:\d+) .* <- .*`)

type locScanner struct{ lastLoc string }

func (sc *locScanner) scan(ms *markScanner, match [][]byte) bool {
	ms.Last.closeMark()
	if loc := string(match[1]); sc.lastLoc != loc {
		ms.Last.closeMark()
		if !ms.Next() {
			return true
		}
		ms.Last.openMark()
		sc.lastLoc = loc
	} else if !ms.Next() {
		return true
	}
	ms.Last.openMark()
	return true
}

var stepPattern = regexp.MustCompile(`@(\d+)\s+(.+?)\.(.+?)\s+r:\[(.*)\] s:\[(.*)\]`)

type retScanner struct{ lastRs string }

func (sc *retScanner) scan(ms *markScanner, match [][]byte) bool {
	if rs := string(match[4]); rs != sc.lastRs {
		prefix := commonPrefix(sc.lastRs, rs)
		if len(prefix) < len(sc.lastRs) {
			ms.Last.closeMark()
		}
		ms.Next()
		if len(prefix) < len(rs) {
			ms.Last.openMark()
		}
		sc.lastRs = rs
		return true
	}
	return false
}

func commonPrefix(a, b string) string {
	if b < a {
		a, b = b, a
	}
	for i := 0; i < len(a) && i < len(b); i++ {
		if a[i] != b[i] {
			return a[:i]
		}
	}
	return a
}
