package main

import (
	"fmt"
	"io"
	"strings"

	"github.com/jcorbin/gothird/internal/fileinput"
	"github.com/jcorbin/gothird/internal/flushio"
	"github.com/jcorbin/gothird/internal/runeio"
)

type Core struct {
	logging
	fileinput.Input
	out     flushio.WriteFlusher
	closers []io.Closer
}

func (core *Core) Close() (err error) {
	for i := len(core.closers) - 1; i >= 0; i-- {
		if cerr := core.closers[i].Close(); err == nil {
			err = cerr
		}
	}
	return err
}

func (core *Core) halt(err error) {
	// ignore any panics while trying to flush output
	func() {
		defer func() { recover() }()
		if core.out != nil {
			if ferr := core.out.Flush(); err == nil {
				err = ferr
			}
		}
	}()

	// ignore any panics while logging
	func() {
		defer func() { recover() }()
		core.logf("#", "halt error: %v", err)
	}()

	panic(haltError{err})
}

func (core *Core) writeRune(r rune) {
	if _, err := runeio.WriteANSIRune(core.out, r); err != nil {
		core.halt(err)
	}
}

func (core *Core) readRune() rune {
	if err := core.out.Flush(); err != nil {
		core.halt(err)
	}

	r, _, err := core.Input.ReadRune()
	for r == 0 {
		if err != nil {
			core.halt(err)
		}
		r, _, err = core.Input.ReadRune()
	}
	return r
}

type haltError struct{ error }

func (err haltError) Error() string {
	if err.error != nil {
		return fmt.Sprintf("halted: %v", err.error)
	}
	return "halted"
}
func (err haltError) Unwrap() error { return err.error }

type logging struct {
	logfn func(mess string, args ...interface{})

	markWidth int
	funcWidth int
	codeWidth int
}

func (log *logging) withLogPrefix(prefix string) func() {
	logfn := log.logfn
	log.logfn = func(mess string, args ...interface{}) {
		logfn(prefix+mess, args...)
	}
	return func() {
		log.logfn = logfn
	}
}

func (log logging) logf(mark, mess string, args ...interface{}) {
	if log.logfn == nil {
		return
	}
	if n := log.markWidth - len(mark); n > 0 {
		for _, r := range mark {
			mark = strings.Repeat(string(r), n) + mark
			break
		}
	} else if n < 0 {
		log.markWidth = len(mark)
	}
	if len(args) > 0 {
		mess = fmt.Sprintf(mess, args...)
	}
	log.logfn("%v %v", mark, mess)
}

type symbols struct {
	strings []string
	symbols map[string]uint
}

func (sym symbols) string(id uint) string {
	if i := int(id) - 1; i >= 0 && i < len(sym.strings) {
		return sym.strings[i]
	}
	return ""
}

func (sym symbols) symbol(s string) uint {
	return sym.symbols[s]
}

func (sym *symbols) symbolicate(s string) (id uint) {
	id, defined := sym.symbols[s]
	if !defined {
		if sym.symbols == nil {
			sym.symbols = make(map[string]uint)
		}
		id = uint(len(sym.strings)) + 1
		sym.strings = append(sym.strings, s)
		sym.symbols[s] = id
	}
	return id
}
