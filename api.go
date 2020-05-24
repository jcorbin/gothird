package main

import (
	"context"
	"errors"
	"io"

	"github.com/jcorbin/gothird/internal/panicerr"
)

func New(opts ...VMOption) *VM {
	var vm VM
	defaultOptions.apply(&vm)
	VMOptions(opts...).apply(&vm)
	return &vm
}

func (vm *VM) Run(ctx context.Context) error {
	err := panicerr.Recover("VM", func() error {
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
func WithMemLimit(limit uint) VMOption            { return withMemLimit(limit) }
func WithMemLayout(retBase, memBase int) VMOption { return withMemLayout(retBase, memBase) }

func WithLogf(logfn func(mess string, args ...interface{})) VMOption { return withLogfn(logfn) }
