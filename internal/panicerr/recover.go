package panicerr

// Recover runs f in a new goroutine wrappe in a defer logic to recover any
// abnormal exits or panics as non-nil error returns.
func Recover(name string, f func() error) error {
	errch := make(chan error, 1)
	go func() {
		defer close(errch)
		defer recoverExitError(name, errch)
		defer recoverPanicError(name, errch)
		errch <- f()
	}()
	return <-errch
}
