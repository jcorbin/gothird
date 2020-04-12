package main

import (
	"bytes"
	"io"
	"io/ioutil"
)

type VMOption interface{ apply(vm *VM) }

var defaults = []VMOption{
	withRetBase(32),
	withMemBase(256),
	withMemorySize(256),
	withInput(bytes.NewReader(nil)),
	withOutput(ioutil.Discard),
}

func (vm *VM) apply(opts ...VMOption) {
	for _, opt := range defaults {
		if opt != nil {
			opt.apply(vm)
		}
	}
	for _, opt := range opts {
		if opt != nil {
			opt.apply(vm)
		}
	}
}

type withRetBase uint
type withMemBase uint
type withMemorySize int
type withLogfn func(mess string, args ...interface{})

func (base withRetBase) apply(vm *VM) {
	vm.retBase = uint(base)
}
func (base withMemBase) apply(vm *VM) {
	size := uint(len(vm.mem))
	if size > 0 {
		size -= vm.memBase
	}
	vm.memBase = uint(base)
	if size > 0 {
		withMemorySize(size).apply(vm)
	}
}
func (size withMemorySize) apply(vm *VM) {
	if need := vm.memBase + uint(size) - uint(len(vm.mem)); need > 0 {
		vm.mem = append(vm.mem, make([]int, need)...)
	}
}
func (logfn withLogfn) apply(vm *VM) {
	vm.logfn = logfn
}

type inputOption struct{ io.Reader }
type outputOption struct{ io.Writer }
type teeOption struct{ io.Writer }

func withInput(r io.Reader) inputOption   { return inputOption{r} }
func withOutput(w io.Writer) outputOption { return outputOption{w} }
func withTee(w io.Writer) teeOption       { return teeOption{w} }

func (i inputOption) apply(vm *VM) {
	vm.in = newRuneScanner(i.Reader)
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
