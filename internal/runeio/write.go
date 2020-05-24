package runeio

import (
	"io"
)

// WriteANSIRune writes a rune to the given writer:
// - ASCII runes are written directly as bytes
// - NEL is written as the more conventional \r\n
// - all other C1 controls are written in their classic 7-bit form
//   e.g. "\x9b" "\x1b\x5b" for CSI
// - all other runes are written in utf8 form
func WriteANSIRune(w io.Writer, r rune) (n int, err error) {
	type runeWriter interface {
		WriteRune(r rune) (n int, err error)
	}
	if r < 0x80 {
		if bw, ok := w.(io.ByteWriter); ok {
			return 1, bw.WriteByte(byte(r))
		}
		return w.Write([]byte{byte(r)})
	}
	if r == 0x85 {
		return w.Write([]byte{'\r', '\n'})
	}
	if r <= 0x9f {
		return w.Write([]byte{0x1b, byte(r ^ 0xc0)})
	}
	if rw, ok := w.(runeWriter); ok {
		return rw.WriteRune(r)
	}
	if sw, ok := w.(io.StringWriter); ok {
		return sw.WriteString(string(r))
	}
	return w.Write([]byte(string(r)))
}

// WriteANSIString writes a string using WriteANSIRune for each rune.
func WriteANSIString(w io.Writer, s string) (n int, err error) {
	for _, r := range []rune(s) {
		m, err := WriteANSIRune(w, r)
		n += m
		if err != nil {
			return n, err
		}
	}
	return n, nil
}
