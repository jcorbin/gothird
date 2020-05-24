/* Package main: FIRST & THIRD -- almost FORTH

FORTH is a language mostly familiar to users of "small" machines. FORTH
programs are small because they are interpreted--a function call in FORTH takes
two bytes.  FORTH is an extendable language-- built-in primitives are
indistinguishable from user-defined _words_.  FORTH interpreters are small
because much of the system can be coded in FORTH--only a small number of
primitives need to be implemented.  Some FORTH interpreters can also compile
defined words into machine code, resulting in a fast system.

FIRST is an incredibly small language which is sufficient for defining the
language THIRD, which is mostly like FORTH.  There are some differences, and
THIRD is probably just enough like FORTH for those differences to be disturbing
to regular FORTH users.

The only existing FIRST interpreter is written in obfuscated C, and rings in at
under 800 bytes of source code, although through deletion of whitespace and
unobfuscation it can be brought to about 650 bytes.

This document FIRST defines the FIRST environment and primitives, with relevent
design decision explanations.  It secondly documents the general strategies we
will use to implement THIRD.  The THIRD section demonstrates how the complete
THIRD system is built up using FIRST.

Section 1: see first.go

Section 2: Motivating THIRD

What is missing from FIRST?  There are a large number of important primitives
that aren't implemented, but which are easy to implement.  drop , which throws
away the top of the stack, can be implemented as { 0 * + } -- that is, multiply
the top of the stack by 0 (which turns the top of the stack into a 0), and then
add the top two elements of the stack.

dup , which copies the top of the stack, can be easily implemented using
temporary storage locations.  Conveniently, FIRST leaves memory locations 3, 4,
and 5 unused.  So we can implement dup by writing the top of stack into 3, and
then reading it out twice: { 3 ! 3 @ 3 @ }.

We will never use the FIRST primitive 'pick' in building THIRD, just to show
that it can be done; 'pick' is only provided because pick itself cannot be
built out of the rest of FIRST's building blocks.

So, instead of worrying about stack primitives and the like, what else is
missing from FIRST?  We get recursion, but no control flow--no conditional
operations.  We cannot at the moment write a looping routine which terminates.

Another glaring dissimilarity between FIRST and FORTH is that there is no
"command mode"--you cannot be outside of a : definition and issue some straight
commands to be executed. Also, as we noted above, we cannot do comments.

FORTH also provides a system for defining new data types, using the words [in
one version of FORTH] <builds and does> . We would like to implement these
words as well.

As the highest priority thing, we will build control flow structures first.
Once we have control structures, we can write recursive routines that
terminate, and we are ready to tackle tasks like parsing, and the building of a
command mode.

By the way, location 0 holds the dictionary pointer, location 1 holds the
return stack pointer, and location 2 should always be 0--it's a fake dictionary
entry that means "pushint".

Section 3: see third.go

*/
package main

import (
	"bytes"
	"context"
	"flag"
	"os"
	"regexp"
	"time"

	"github.com/jcorbin/gothird/internal/logio"
)

func main() {
	var (
		memLimit uint
		timeout  time.Duration
		trace    bool
		dump     bool
	)
	flag.UintVar(&memLimit, "mem-limit", 0, "enable memory limit")
	flag.DurationVar(&timeout, "timeout", 0, "specify a time limit")
	flag.BoolVar(&trace, "trace", false, "enable trace logging")
	flag.BoolVar(&dump, "dump", false, "print a dump after execution")
	flag.Parse()

	log := logio.Logger{}
	log.SetOutput(os.Stderr)
	defer os.Exit(log.ExitCode())

	var in namedBuffer
	in.name = "<pre-stdin>"
	if trace {
		in.WriteString("\ntron\n")
	}
	in.WriteString("\n[\n")

	vm := New(
		WithLogf(log.Leveledf("TRACE")),
		WithMemLimit(memLimit),
		WithInputWriter(thirdKernel),
		WithInput(&in),
		WithInput(os.Stdin),
		WithOutput(os.Stdout),
	)

	if dump {
		lw := &logio.Writer{Logf: log.Leveledf("DUMP")}
		defer lw.Close()
		defer vmDumper{vm: vm, out: lw}.dump()
	}

	if trace {
		log.Wrap(scanPipe("trace scanner",
			patternScanner(scanPattern, &locScanner{}),
			// patternScanner(stepPattern, &retScanner{}),
		))
	}

	defer log.Unwrap()

	ctx := context.Background()
	if timeout != 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	log.ErrorIf(vm.Run(ctx))
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

type namedBuffer struct {
	bytes.Buffer
	name string
}

func (nb namedBuffer) Name() string { return nb.name }
