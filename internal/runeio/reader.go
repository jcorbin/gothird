package runeio

import (
	"bufio"
	"io"
)

// Reader is an io.Reader that also supports reading runes.
type Reader interface {
	io.Reader
	io.RuneReader
}

// NewReader returns a Reader from r; if r already implements, it is simply returned.
// Otherwise bufio.Reader is used to provide rune reading around the given reader.
// If the r implements Name() string, so will the returned Reader.
func NewReader(r io.Reader) Reader {
	if impl, ok := r.(Reader); ok {
		return impl
	}
	rr := runeReader{r, bufio.NewReader(r)}
	if impl, ok := r.(interface{ Name() string }); ok {
		return namedRuneReader{rr, impl.Name()}
	}
	return rr
}

type runeReader struct {
	io.Reader
	io.RuneReader
}

type namedRuneReader struct {
	Reader
	name string
}

func (nr namedRuneReader) Name() string { return nr.name }
