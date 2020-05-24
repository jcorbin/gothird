package fileinput

import (
	"bytes"
	"fmt"
	"io"

	"github.com/jcorbin/gothird/internal/runeio"
)

// Location names an a line in an Input file.
type Location struct {
	Name string
	Line int
}

// Line combines a Location along with a bytes.Buffer for handling it.
type Line struct {
	Location
	bytes.Buffer
}

func (loc Location) String() string { return fmt.Sprintf("%v:%v", loc.Name, loc.Line) }
func (il Line) String() string      { return fmt.Sprintf("%v %q", il.Location, il.Buffer.String()) }

// Input implements sequential rune reading through a Queue of one or more
// input streams. Both the current and last scanned lines are tracked to
// facilitate user feedback.
type Input struct {
	rr    io.RuneReader
	Queue []io.Reader
	Last  Line
	Scan  Line
}

// ReadRune reads one rune from the current input stream, appending it into the
// current Scan line, and rolling Scan over to Last after line feed.
func (in *Input) ReadRune() (rune, int, error) {
	if in.rr == nil && !in.nextIn() {
		return 0, 0, io.EOF
	}

	r, n, err := in.rr.ReadRune()
	if r == '\n' {
		in.nextLine()
	} else {
		in.Scan.WriteRune(r)
	}

	if r != 0 {
		return r, n, nil
	}
	if err == io.EOF && in.nextIn() {
		err = nil
	}
	return 0, n, err
}

func (in *Input) nextLine() {
	in.Last.Reset()
	in.Last.Name = in.Scan.Name
	in.Last.Line = in.Scan.Line
	in.Last.Write(in.Scan.Bytes())
	in.Scan.Reset()
	in.Scan.Line++
}

func (in *Input) nextIn() bool {
	in.nextLine()
	if in.rr != nil {
		if cl, ok := in.rr.(io.Closer); ok {
			cl.Close()
		}
		in.rr = nil
	}
	if len(in.Queue) > 0 {
		r := in.Queue[0]
		in.Queue = in.Queue[1:]
		in.rr = runeio.NewReader(r)
		in.Scan.Name = nameOf(r)
		in.Scan.Line = 1
	}
	return in.rr != nil
}

func nameOf(obj interface{}) string {
	if nom, ok := obj.(interface{ Name() string }); ok {
		return nom.Name()
	}
	return fmt.Sprintf("<unnamed %T>", obj)
}
