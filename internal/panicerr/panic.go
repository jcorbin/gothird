package panicerr

import (
	"errors"
	"fmt"
	"runtime/debug"
)

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

// IsPanic returns true if err indicates a recovered goroutine panic.
func IsPanic(err error) bool {
	var pe panicError
	return errors.As(err, &pe)
}

// PanicStack returns a non-empty stacktrace string if err is a recovered
// goroutine panic.
func PanicStack(err error) string {
	var pe panicError
	if errors.As(err, &pe) {
		return string(pe.stack)
	}
	return ""
}
