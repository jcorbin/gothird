package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"

	"github.com/jcorbin/gothird/internal/flushio"
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
	var he haltError
	if errors.As(err, &he) {
		err = he.error
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

type VMOption interface{ apply(vm *VM) }

var defaultOptions = VMOptions(
	withInput(bytes.NewReader(nil)),
	withOutput(ioutil.Discard),
)

func VMOptions(opts ...VMOption) VMOption {
	var res options
	for _, opt := range opts {
		switch impl := opt.(type) {
		case nil, noption:
		case options:
			res = append(res, impl...)
		default:
			res = append(res, opt)
		}
	}
	switch len(res) {
	case 0:
		return noption{}
	case 1:
		return res[0]
	default:
		return res
	}
}

type noption struct{}

func (noption) apply(vm *VM) {}

type options []VMOption

func (opts options) apply(vm *VM) {
	for _, opt := range opts {
		if opt != nil {
			opt.apply(vm)
		}
	}
}

type withLogfn func(mess string, args ...interface{})

func (logfn withLogfn) apply(vm *VM) {
	vm.logfn = logfn
}

type inputOption struct{ io.Reader }
type outputOption struct{ io.Writer }
type teeOption struct{ io.Writer }
type memLimitOption uint

func withInput(r io.Reader) inputOption      { return inputOption{r} }
func withOutput(w io.Writer) outputOption    { return outputOption{w} }
func withTee(w io.Writer) teeOption          { return teeOption{w} }
func withMemLimit(limit uint) memLimitOption { return memLimitOption(limit) }

func withInputWriter(wto io.WriterTo) pipeInput {
	r, w := io.Pipe()
	go func() {
		defer w.Close()
		wto.WriteTo(w)
	}()
	return pipeInput{r, nameOf(wto)}
}

func nameOf(obj interface{}) string {
	if nom, ok := obj.(interface{ Name() string }); ok {
		return nom.Name()
	}
	return fmt.Sprintf("<unnamed %T>", obj)
}

func (i inputOption) apply(vm *VM) {
	vm.Queue = append(vm.Queue, i.Reader)
}

func (o outputOption) apply(vm *VM) {
	if vm.out != nil {
		vm.out.Flush()
	}
	vm.out = flushio.NewWriteFlusher(o.Writer)
	if cl, ok := o.Writer.(io.Closer); ok {
		vm.closers = append(vm.closers, cl)
	}
}

func (o teeOption) apply(vm *VM) {
	vm.out = flushio.WriteFlushers(vm.out, flushio.NewWriteFlusher(o.Writer))
	if cl, ok := o.Writer.(io.Closer); ok {
		vm.closers = append(vm.closers, cl)
	}
}

func (lim memLimitOption) apply(vm *VM) {
	vm.mem.Limit = uint(lim)
}

type memLayoutOption struct {
	retBase int
	memBase int
}

func withMemLayout(retBase, memBase int) memLayoutOption { return memLayoutOption{retBase, memBase} }

func (lay memLayoutOption) apply(vm *VM) {
	if lay.retBase != 0 {
		vm.stor(10, lay.retBase)
	}
	if lay.memBase != 0 {
		vm.stor(11, lay.memBase)
	}
}

type pipeInput struct {
	*io.PipeReader
	name string
}

func (pi pipeInput) Name() string { return pi.name }

func (pi pipeInput) apply(vm *VM) {
	vm.Queue = append(vm.Queue, pi)
	vm.closers = append(vm.closers, pi)
}
