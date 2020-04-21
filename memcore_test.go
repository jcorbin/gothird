package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_memCore(t *testing.T) {
	type step struct {
		name string
		f    func(t *testing.T, mem *memCore)
	}

	expectAt := func(t *testing.T, mem *memCore, addr uint, values ...int) {
		buf := make([]int, len(values))
		require.NoError(t, mem.loadInto(addr, buf),
			"must load %v values from @%v", len(values), addr)
		require.Equal(t, values, buf, "expected values @%v", addr)
	}

	for _, tc := range []struct {
		name  string
		steps []step
	}{
		{"basic", []step{
			{"init", func(t *testing.T, mem *memCore) {
				mem.pageSize = 4
				val, err := mem.load(0)
				require.NoError(t, err, "unexpected load error")
				require.Equal(t, 0, val, "expected 0 @0")
				require.Equal(t, uint(0), mem.memSize(), "expected 0 initial size")
			}},

			{"9 -> 0", func(t *testing.T, mem *memCore) {
				require.NoError(t, mem.stor(0, 9), "must stor @0")
				val, err := mem.load(0)
				require.NoError(t, err, "unexpected load error")
				require.Equal(t, 9, val, "expected 9 @0")
				//  0  1  2  3  :  9  0  0  0
				//  4  5  6  7  :  -  -  -  -
				//  8  9  a  b  :  -  -  -  -
				//  c  d  e  f  :  -  -  -  -
				// 10 11 12 13  :  -  -  -  -
				expectAt(t, mem, 6,
					0, 0,
					0, 0, 0, 0,
					0, 0, 0, 0,
					0, 0)
			}},

			{"{1, 2, 3, 4, 5, 6} -> 0x9", func(t *testing.T, mem *memCore) {
				require.NoError(t, mem.stor(0x9, 1, 2, 3, 4, 5, 6), "must stor @0x9")
				require.Equal(t, mem.bases, []uint{0x0, 0x8, 0xc}, "expected a page hole")
				//  0  1  2  3  :  9  0  0  0
				//  4  5  6  7  :  -  -  -  -
				//  8  9  a  b  :  0  1  2  3
				//  c  d  e  f  :  4  5  6  0
				// 10 11 12 13  :  -  -  -  -
				expectAt(t, mem, 6,
					0, 0,
					0, 1, 2, 3,
					4, 5, 6, 0,
					0, 0)
			}},

			{"7 -> 0xf", func(t *testing.T, mem *memCore) {
				require.NoError(t, mem.stor(0xf, 7), "must stor @0xf")
				{
					val, err := mem.load(0xf)
					require.NoError(t, err, "unexpected load error")
					require.Equal(t, 7, val, "expected 7 @0xf")
				}
				{
					val, err := mem.load(0xe)
					require.NoError(t, err, "unexpected load error")
					require.Equal(t, 6, val, "expected 6 @0xe")
				}
				//  0  1  2  3  :  9  0  0  0
				//  4  5  6  7  :  -  -  -  -
				//  8  9  a  b  :  0  1  2  3
				//  c  d  e  f  :  4  5  6  7
				// 10 11 12 13  :  -  -  -  -
				expectAt(t, mem, 6,
					0, 0,
					0, 1, 2, 3,
					4, 5, 6, 7,
					0, 0)
			}},

			{"8 -> 0x15", func(t *testing.T, mem *memCore) {
				require.NoError(t, mem.stor(0x15, 8), "must stor @0x15")
				val, err := mem.load(0x15)
				require.NoError(t, err, "unexpected load error")
				require.Equal(t, 8, val, "expected 7 @0x15")
				//  0  1  2  3  :  9  0  0  0
				//  4  5  6  7  :  -  -  -  -
				//  8  9  a  b  :  0  1  2  3
				//  c  d  e  f  :  4  5  6  7
				// 10 11 12 13  :  -  -  -  -
				// 14 15 16 17  :  0  8  0  0
				// 18 19 20 21  :  -  -  -  -
				expectAt(t, mem, 6,
					0, 0,
					0, 1, 2, 3,
					4, 5, 6, 7,
					0, 0, 0, 0,
					0, 8, 0, 0,
					0, 0)
			}},
		}},

		{"vm set regression", []step{
			{"init", func(t *testing.T, mem *memCore) {
				mem.pageSize = 32

				require.NoError(t, mem.stor(10, 16), "unexpected store error @10")
				require.NoError(t, mem.stor(11, 32), "unexpected store error @11")
				require.NoError(t, mem.stor(0, 32), "unexpected store error @0")
				require.NoError(t, mem.stor(1, 16), "unexpected store error @1")

				expectAt(t, mem, 0, 32, 16, 0, 0)
				expectAt(t, mem, 32, 0, 0, 0)
			}},

			{"set then load @memBase", func(t *testing.T, mem *memCore) {
				require.NoError(t, mem.stor(33, 108), "unexpected store error @33")
				expectAt(t, mem, 32, 0, 108, 0)
			}},
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var mem memCore
			defer func() {
				if t.Failed() {
					t.Logf("bases: %v", mem.bases)
					t.Logf("pages: %v", mem.pages)
				}
			}()
			for _, step := range tc.steps {
				if !isolateTestRun(t, step.name, func(t *testing.T) {
					step.f(t, &mem)
				}) {
					break
				}
			}
		})
	}
}
