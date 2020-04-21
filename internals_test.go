package main

import (
	"errors"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_isolate(t *testing.T) {
	for _, tc := range []struct {
		name      string
		errStr    string
		wrapStr   string
		fun       func() error
		haveStack bool
	}{
		{
			name:   "normal",
			errStr: "",
			fun: func() error {
				return nil
			},
		},
		{
			name:   "normal err",
			errStr: "bang",
			fun: func() error {
				return errors.New("bang")
			},
		},
		{
			name:      "panic err",
			errStr:    "panic err paniced: bang",
			wrapStr:   "bang",
			haveStack: true,
			fun: func() error {
				panic(errors.New("bang"))
				return nil
			},
		},
		{
			name:      "hello panic",
			errStr:    "hello panic paniced: hello",
			haveStack: true,
			fun: func() error {
				panic("hello")
				return nil
			},
		},
		{
			name:   "exit",
			errStr: "exit called runtime.Goexit",
			fun:    func() error { runtime.Goexit(); return nil },
		},
		{
			name:      "index panic",
			errStr:    "index panic paniced: runtime error: index out of range [1] with length 0",
			haveStack: true,
			fun: func() error {
				var some []int
				some[1]++
				return nil
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := isolate(tc.name, tc.fun)
			if tc.errStr == "" {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, tc.errStr)
				if tc.wrapStr != "" {
					assert.EqualError(t, errors.Unwrap(err), tc.wrapStr, "expected panic(error) value")
				}
			}
			stack := panicErrorStack(err)
			if tc.haveStack {
				assert.NotEqual(t, "", stack, "expected a stack trace")
			} else {
				assert.Equal(t, "", stack, "expected no stack trace")
			}
			if t.Failed() && stack != "" {
				t.Logf("panic stack: %v", stack)
			}
		})
	}
}
