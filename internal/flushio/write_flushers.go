package flushio

import "io"

// WriteFlushers combines any number of WriteFlusher-s into a single one that
// will write into and flush all of them.
func WriteFlushers(wfs ...WriteFlusher) WriteFlusher {
	switch wfs := appendWriteFlusher(nil, wfs...); len(wfs) {
	case 0:
		return nil
	case 1:
		return wfs[0]
	default:
		return wfs
	}
}

type writeFlushers []WriteFlusher

func (wfs writeFlushers) Write(p []byte) (n int, err error) {
	for _, wf := range wfs {
		n, err = wf.Write(p)
		if err != nil {
			return n, err
		}
		if n != len(p) {
			return n, io.ErrShortWrite
		}
	}
	return len(p), nil
}

func (wfs writeFlushers) Flush() (err error) {
	for _, wf := range wfs {
		if ferr := wf.Flush(); err == nil {
			err = ferr
		}
	}
	return err
}

func appendWriteFlusher(all writeFlushers, some ...WriteFlusher) writeFlushers {
	for _, one := range some {
		if many, ok := one.(writeFlushers); ok {
			all = append(all, many...)
		} else if one != nil {
			all = append(all, one)
		}
	}
	return all
}
