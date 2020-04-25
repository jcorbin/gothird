package main

import (
	"errors"
	"fmt"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_isolate(t *testing.T) {
	for _, tc := range []struct {
		name      string
		err       string
		wraps     string
		fun       func() error
		haveStack bool
	}{
		{
			name:      "",
			err:       "paniced: shrug",
			wraps:     "shrug",
			haveStack: true,
			fun:       func() error { panic(errors.New("shrug")) },
		},
		{
			name: "",
			err:  "runtime.Goexit called",
			fun:  func() error { runtime.Goexit(); return nil },
		},

		{
			name: "normal",
			err:  "",
			fun:  func() error { return nil },
		},
		{
			name: "normal err",
			err:  "bang",
			fun:  func() error { return errors.New("bang") },
		},
		{
			name:      "panic err",
			err:       "panic err paniced: bang",
			wraps:     "bang",
			haveStack: true,
			fun:       func() error { panic(errors.New("bang")) },
		},
		{
			name:      "hello panic",
			err:       "hello panic paniced: hello",
			haveStack: true,
			fun:       func() error { panic("hello") },
		},
		{
			name: "exit",
			err:  "exit called runtime.Goexit",
			fun:  func() error { runtime.Goexit(); return nil },
		},
		{
			name:      "index panic",
			err:       "index panic paniced: runtime error: index out of range [1] with length 0",
			haveStack: true,
			fun:       func() error { _ = ([]int)(nil)[1]; return nil },
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := isolate(tc.name, tc.fun)
			if tc.err == "" {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, tc.err)
				if tc.wraps != "" {
					assert.EqualError(t, errors.Unwrap(err), tc.wraps, "expected panic(error) value")
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

func Test_isolate_stacktrace(t *testing.T) {
	err := isolate("", func() error {
		panic("nope")
	})
	require.Error(t, err, "must have an isolate error")

	assert.True(t,
		strings.HasSuffix(fmt.Sprintf("%+v", err), panicErrorStack(err)),
		"expected verbose format to end with a stack trace")
}
