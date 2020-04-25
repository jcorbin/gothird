package main

import (
	"errors"
	"fmt"
	"runtime/debug"
)

func isolate(name string, f func() error) error {
	errch := make(chan error, 1)
	go func() {
		defer close(errch)
		defer recoverExitError(name, errch)
		defer recoverPanicError(name, errch)
		errch <- f()
	}()
	return <-errch
}

func recoverExitError(name string, errch chan<- error) {
	select {
	case errch <- exitError(name):
	default:
		// assumes that that the happy path does a (maybe nil) send
	}
}

func recoverPanicError(name string, errch chan<- error) {
	var pe panicError
	if pe.e = recover(); pe.e != nil {
		pe.name = name
		pe.stack = debug.Stack()
		select {
		case errch <- pe:
		default:
		}
	}
}

type exitError string

func (name exitError) Error() string {
	if name == "" {
		return "runtime.Goexit called"
	}
	return fmt.Sprintf("%v called runtime.Goexit", string(name))
}

type panicError struct {
	name  string
	e     interface{}
	stack []byte
}

func (pe panicError) Error() string {
	return fmt.Sprint(pe)
}

func (pe panicError) Format(f fmt.State, c rune) {
	if pe.name == "" {
		fmt.Fprintf(f, "paniced: %v", pe.e)
	} else {
		fmt.Fprintf(f, "%v paniced: %v", pe.name, pe.e)
	}
	if c == 'v' && f.Flag('+') {
		fmt.Fprintf(f, "\nPanic stack: %s", pe.stack)
	}
}

func (pe panicError) Unwrap() error {
	err, _ := pe.e.(error)
	return err
}

func panicErrorStack(err error) string {
	var pe panicError
	if errors.As(err, &pe) {
		return string(pe.stack)
	}
	return ""
}
