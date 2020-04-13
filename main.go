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

	var timeout time.Duration
	var trace bool
	var memLimit int
	flag.DurationVar(&timeout, "timeout", 0, "specify a time limit")
	flag.BoolVar(&trace, "trace", false, "enable trace logging")
	flag.IntVar(&memLimit, "mem-limit", 0, "enable memory limit")
	flag.Parse()

	var opts = []VMOption{
		WithInput(os.Stdin),
		WithOutput(os.Stdout),
	}
	if trace {
		opts = append(opts, WithLogf(log.Printf))
	}
	if memLimit != 0 {
		opts = append(opts, WithMemLimit(memLimit))
	}
	vm := New(opts...)

	if timeout != 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	if err := vm.Run(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %+v\n", err)
		os.Exit(1)
	}
}
