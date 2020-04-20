package main

import (
	"bytes"
	"io"
	"io/ioutil"
)

type VMOption interface{ apply(vm *VM) }

var defaultOptions = []VMOption{
	withInput(bytes.NewReader(nil)),
	withOutput(ioutil.Discard),
}

func (vm *VM) apply(opts ...VMOption) {
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

func (i inputOption) apply(vm *VM) {
	vm.inQueue = append(vm.inQueue, i.Reader)
}

func (o outputOption) apply(vm *VM) {
	if vm.out != nil {
		vm.out.Flush()
	}
	vm.out = newWriteFlusher(o.Writer)
}

func (o teeOption) apply(vm *VM) {
	vm.out = multiWriteFlusher(vm.out, newWriteFlusher(o.Writer))
}

func (lim memLimitOption) apply(vm *VM) {
	vm.memLimit = uint(lim)
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
	vm.inQueue = append(vm.inQueue, pi)
	vm.closers = append(vm.closers, pi)
}
