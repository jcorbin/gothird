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
			log.Through(newScanGrouper)
			opts = append(opts, WithLogf(log.Leveledf("TRACE")))
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
		log.Errorf("%+v", err)
	}
}

var scanPattern = regexp.MustCompile(`> scan (.+:\d+) .* <- .*`)

func newScanGrouper(out io.WriteCloser) io.WriteCloser {
	var lastLoc string
	return runMarkScanner("scan grouper", out, func(sc *markScanner) error {
		if match := scanPattern.FindSubmatch(sc.Bytes()); len(match) > 0 {
			sc.Last.closeMark()
			if loc := string(match[1]); lastLoc != loc {
				sc.Last.closeMark()
				if !sc.Next() {
					return sc.Err()
				}
				sc.Last.openMark()
				lastLoc = loc
			} else if !sc.Next() {
				return sc.Err()
			}
			sc.Last.openMark()
		}
		return nil
	})
}
