package main

import (
	"context"
	"fmt"
	"io"
)

func New(opts ...VMOption) *VM {
	var vm VM
	vm.apply(opts...)
	return &vm
}

func (vm *VM) Run(ctx context.Context) error {
	done := make(chan error)
	go func(done chan<- error) {
		defer close(done)
		defer func() {
			e := recover()
			if err, ok := e.(error); ok {
				done <- err
			} else if e != nil {
				done <- fmt.Errorf("paniced: %v", e)
			}
		}()
		vm.run()
	}(done)
	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-done:
		return err
	}
}

func WithInput(r io.Reader) VMOption  { return withInput(r) }
func WithOutput(w io.Writer) VMOption { return withOutput(w) }
func WithTee(w io.Writer) VMOption    { return withTee(w) }
func WithMemLimit(limit int) VMOption { return withMemLimit(limit) }

func WithLogf(logfn func(mess string, args ...interface{})) VMOption { return withLogfn(logfn) }
