package panicerr

import (
	"errors"
	"fmt"
)

func recoverExitError(name string, errch chan<- error) {
	select {
	case errch <- exitError(name):
	default:
		// assumes that that the happy path does a (maybe nil) send
	}
}

type exitError string

func (name exitError) Error() string {
	if name == "" {
		return "runtime.Goexit called"
	}
	return fmt.Sprintf("%v called runtime.Goexit", string(name))
}

// IsExit returns true if err indicates a recovered goroutine exit.
func IsExit(err error) bool {
	var xe exitError
	return errors.As(err, &xe)
}
