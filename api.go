package main

import (
	"context"
	"errors"
	"io"
)

func New(opts ...VMOption) *VM {
	var vm VM
	vm.apply(defaultOptions...)
	vm.apply(opts...)
	return &vm
}

func (vm *VM) Run(ctx context.Context) error {
	err := isolate("VM", func() error {
		return vm.run(ctx)
	})
	if err == nil || errors.Is(err, io.EOF) {
		return nil
	}
	var vmErr vmHaltError
	if errors.As(err, &vmErr) {
		err = vmErr.error
	}
	return err
}

func WithInput(r io.Reader) VMOption              { return withInput(r) }
func WithInputWriter(w io.WriterTo) VMOption      { return withInputWriter(w) }
func WithOutput(w io.Writer) VMOption             { return withOutput(w) }
func WithTee(w io.Writer) VMOption                { return withTee(w) }
func WithMemLimit(limit int) VMOption             { return withMemLimit(limit) }
func WithMemLayout(retBase, memBase int) VMOption { return withMemLayout(retBase, memBase) }

func WithLogf(logfn func(mess string, args ...interface{})) VMOption { return withLogfn(logfn) }

func NamedReader(name string, r io.Reader) io.Reader {
	if rr, is := r.(io.RuneReader); is {
		return runeReaderName{r, rr, name}
	}
	return readerName{r, name}
}
