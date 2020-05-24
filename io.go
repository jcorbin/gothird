package main

import (
	"io"

	"github.com/jcorbin/gothird/internal/fileinput"
	"github.com/jcorbin/gothird/internal/flushio"
)

type ioCore struct {
	fileinput.Input
	out     flushio.WriteFlusher
	closers []io.Closer
}

func (ioc *ioCore) Close() (err error) {
	for i := len(ioc.closers) - 1; i >= 0; i-- {
		if cerr := ioc.closers[i].Close(); err == nil {
			err = cerr
		}
	}
	return err
}
