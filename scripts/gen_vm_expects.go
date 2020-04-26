package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"regexp"
	"time"

	"golang.org/x/net/context"
	"golang.org/x/sync/errgroup"
)

type namedReader interface {
	io.ReadCloser
	Name() string
}

var (
	in  namedReader    = os.Stdin
	out io.WriteCloser = os.Stdout
)

func parseFlags() {
	flag.Parse()

	args := flag.Args()

	if len(args) > 0 {
		name := args[0]
		f, err := os.Open(name)
		if err != nil {
			log.Fatalf("failed to open %v: %v", name, err)
		}
		args = args[1:]
		in = f
	}

	if len(args) > 0 {
		name := args[0]
		f, err := os.Create(name)
		if err != nil {
			log.Fatalf("failed to create %v: %v", name, err)
		}
		args = args[1:]
		out = f
	}
}

func main() {
	ctx := context.Background()
	parseFlags()

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	eg, ctx := errgroup.WithContext(ctx)

	ready := make(chan struct{})

	eg.Go(func() error {
		gofmt := exec.CommandContext(ctx, "goimports")
		fmtPipe, err := gofmt.StdinPipe()
		if err != nil {
			return err
		}

		defer out.Close()
		gofmt.Stdout = out
		gofmt.Stderr = os.Stderr

		out = fmtPipe

		close(ready)
		if err := gofmt.Run(); err != nil {
			return fmt.Errorf("gofmt run failed: %w", err)
		}
		return nil
	})

	eg.Go(func() (rerr error) {
		select {
		case <-ctx.Done():
		case <-ready:
		}

		defer func() {
			if cerr := in.Close(); rerr == nil {
				rerr = cerr
			}
			if cerr := out.Close(); rerr == nil {
				rerr = cerr
			}
		}()

		return run(ctx)
	})

	if err := eg.Wait(); err != nil {
		log.Fatalln(err)
	}
}

var expectMethod = regexp.MustCompile(`func \(vmt vmTestCase\) (expect|with)(.+?)\((.+?)\) vmTestCase`)

func run(ctx context.Context) error {
	var buf bytes.Buffer
	buf.Grow(1024)
	buf.WriteString("package main\n\n")

	buf.WriteString("// @generated from ")
	buf.WriteString(in.Name())
	buf.WriteString("\n\n")

	if args := flag.Args(); len(args) >= 2 {
		buf.WriteString("//go:generate go run scripts/gen_vm_expects.go --")
		for _, arg := range args {
			buf.WriteByte(' ')
			buf.WriteString(arg)
		}
		buf.WriteString("\n\n")
	}

	sc := bufio.NewScanner(in)
	for sc.Scan() {
		if match := expectMethod.FindSubmatch(sc.Bytes()); len(match) > 0 {
			var (
				baseName = match[1]
				whatName = match[2]
				args     = match[3]
			)
			buf.WriteString("func ")
			buf.Write(baseName)
			buf.WriteString("VM")
			buf.Write(whatName)
			buf.WriteString("(")
			buf.Write(args)
			buf.WriteString(") func(vmTestCase) vmTestCase {\n")
			buf.WriteString("  return func(vmt vmTestCase) vmTestCase {\n")
			buf.WriteString("    return vmt.")
			buf.Write(baseName)
			buf.Write(whatName)
			buf.WriteString("(")

			for i, part := range bytes.Split(args, []byte(",")) {
				if i > 0 {
					buf.WriteString(", ")
				}
				fields := bytes.Fields(bytes.Trim(part, " "))
				buf.Write(fields[0])
				if bytes.HasPrefix(fields[1], []byte("...")) {
					buf.WriteString("...")
				}
			}

			buf.WriteString(")\n")
			buf.WriteString("  }\n")
			buf.WriteString("}\n\n")
		}

		if buf.Len() > 0 {
			if _, err := buf.WriteTo(out); err != nil {
				return err
			}
		}
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	return sc.Err()
}
