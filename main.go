package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"
)

func main() {
	ctx := context.Background()

	var memSize int
	var timeout time.Duration
	var trace bool
	flag.IntVar(&memSize, "mem-size", 1024, "specify VM memory size")
	flag.DurationVar(&timeout, "timeout", 0, "specify a time limit")
	flag.BoolVar(&trace, "trace", false, "enable trace logging")
	flag.Parse()

	if timeout != 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	var logOption VMOption
	if trace {
		logOption = WithLogf(log.Printf)
	}

	if err := New(
		WithMemorySize(memSize),
		WithInput(os.Stdin),
		WithOutput(os.Stdout),
		logOption,
	).Run(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %+v\n", err)
		os.Exit(1)
	}
}
