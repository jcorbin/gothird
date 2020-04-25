package main

import (
	"log"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_memCore(t *testing.T) {
	for _, tc := range []memCoreTestCase{
		memCoreTest("basic",
			"init", func(t *testing.T, mem *memCore) {
				mem.pageSize = 4
				val, err := mem.load(0)
				require.NoError(t, err, "unexpected load error")
				require.Equal(t, 0, val, "expected 0 @0")
				require.Equal(t, uint(0), mem.memSize(), "expected 0 initial size")
			},

			"9 -> 0", func(t *testing.T, mem *memCore) {
				require.NoError(t, mem.stor(0, 9), "must stor @0")
				val, err := mem.load(0)
				require.NoError(t, err, "unexpected load error")
				require.Equal(t, 9, val, "expected 9 @0")
				//  0  1  2  3  :  9  0  0  0
				//  4  5  6  7  :  -  -  -  -
				//  8  9  a  b  :  -  -  -  -
				//  c  d  e  f  :  -  -  -  -
				// 10 11 12 13  :  -  -  -  -
				expectMemValuesAt(t, mem, 6,
					0, 0,
					0, 0, 0, 0,
					0, 0, 0, 0,
					0, 0)
			},

			"{1, 2, 3, 4, 5, 6} -> 0x9", func(t *testing.T, mem *memCore) {
				require.NoError(t, mem.stor(0x9, 1, 2, 3, 4, 5, 6), "must stor @0x9")
				require.Equal(t, mem.bases, []uint{0x0, 0x8, 0xc}, "expected a page hole")
				//  0  1  2  3  :  9  0  0  0
				//  4  5  6  7  :  -  -  -  -
				//  8  9  a  b  :  0  1  2  3
				//  c  d  e  f  :  4  5  6  0
				// 10 11 12 13  :  -  -  -  -
				expectMemValuesAt(t, mem, 6,
					0, 0,
					0, 1, 2, 3,
					4, 5, 6, 0,
					0, 0)
			},

			"7 -> 0xf", func(t *testing.T, mem *memCore) {
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
				expectMemValuesAt(t, mem, 6,
					0, 0,
					0, 1, 2, 3,
					4, 5, 6, 7,
					0, 0)
			},

			"8 -> 0x15", func(t *testing.T, mem *memCore) {
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
				expectMemValuesAt(t, mem, 6,
					0, 0,
					0, 1, 2, 3,
					4, 5, 6, 7,
					0, 0, 0, 0,
					0, 8, 0, 0,
					0, 0)
			},

			"stor across the 0x10 page gap", func(t *testing.T, mem *memCore) {
				require.NoError(t, mem.stor(0xe, 96, 97, 98, 99, 91, 92, 93, 94), "must stor @0x15")
				//  0  1  2  3  :  9  0  0  0
				//  4  5  6  7  :  -- -- -- --
				//  8  9  a  b  :  0  1  2  3
				//  c  d  e  f  :  4  5  96 97
				// 10 11 12 13  :  98 99 91 92
				// 14 15 16 17  :  93 94 0  0
				// 18 19 20 21  :  -- -- -- --
				expectMemValuesAt(t, mem, 0xc,
					4, 5, 96, 97,
					98, 99, 91, 92,
					93, 94, 0, 0,
					0, 0,
				)
			},
		),

		memCoreTest("missing lower section",
			"initial value in 2nd page", func(t *testing.T, mem *memCore) {
				mem.pageSize = 0x10
				expectMemValueAt(t, mem, 0x18, 0)
				require.NoError(t, mem.stor(0x18, 42), "unexpected stor error")
				expectMemValueAt(t, mem, 0x18, 42)
			},

			"load low", func(t *testing.T, mem *memCore) { expectMemValueAt(t, mem, 0x8, 0) },

			"create 3rd page", func(t *testing.T, mem *memCore) {
				require.NoError(t, mem.stor(0x28, 99), "unexpected stor error")
				expectMemValueAt(t, mem, 0x28, 99)
			},

			"load low again", func(t *testing.T, mem *memCore) { expectMemValueAt(t, mem, 0x8, 0) },

			"finally create the 1st page", func(t *testing.T, mem *memCore) {
				require.NoError(t, mem.stor(0x8, 3), "unexpected stor error")
				expectMemValueAt(t, mem, 0x8, 3)
			},
		),

		memCoreTest("vm set regression",
			"init", func(t *testing.T, mem *memCore) {
				mem.pageSize = 32

				require.NoError(t, mem.stor(10, 16), "unexpected store error @10")
				require.NoError(t, mem.stor(11, 32), "unexpected store error @11")
				require.NoError(t, mem.stor(0, 32), "unexpected store error @0")
				require.NoError(t, mem.stor(1, 16), "unexpected store error @1")

				expectMemValuesAt(t, mem, 0, 32, 16, 0, 0)
				expectMemValuesAt(t, mem, 32, 0, 0, 0)
			},

			"set then load @memBase", func(t *testing.T, mem *memCore) {
				require.NoError(t, mem.stor(33, 108), "unexpected store error @33")
				expectMemValuesAt(t, mem, 32, 0, 108, 0)
			},
		),
	} {
		t.Run(tc.name, func(t *testing.T) {
			tcLogOut := &logWriter{logf: t.Logf}
			log.SetOutput(tcLogOut)
			defer log.SetOutput(os.Stderr)

			var mem memCore
			defer func() {
				if t.Failed() {
					t.Logf("bases: %v", mem.bases)
					t.Logf("pages: %v", mem.pages)
				}
			}()

			for _, step := range tc.steps {
				if !t.Run(step.name, func(t *testing.T) {
					stepLogOut := &logWriter{logf: t.Logf}
					log.SetOutput(stepLogOut)
					defer log.SetOutput(tcLogOut)

					isolateTest(t, step.bind(&mem))
				}) {
					break
				}
			}
		})
	}
}

func isolateTest(t *testing.T, f func(t *testing.T)) {
	if err := isolate(t.Name(), func() error {
		f(t)
		return nil
	}); err != nil {
		t.Logf("%+v", err)
		t.Fail()
	}
}

func expectMemValueAt(t *testing.T, mem *memCore, addr uint, value int) {
	val, err := mem.load(addr)
	require.NoError(t, err, "unexpected load @0x%x error", addr)
	require.Equal(t, value, val, "expected value @0x%x", addr)
}

func expectMemValuesAt(t *testing.T, mem *memCore, addr uint, values ...int) {
	buf := make([]int, len(values))
	require.NoError(t, mem.loadInto(addr, buf),
		"must load %v values from @0x%x", len(values), addr)
	require.Equal(t, values, buf, "expected values @0x%x", addr)
}

func memCoreTest(name string, args ...interface{}) (tc memCoreTestCase) {
	tc.name = name
	for i := 0; i < len(args); i++ {
		var step memCoreTestStep

		step.name = args[i].(string)

		if i++; i >= len(args) {
			panic("memCoreTest: not missing function argument after name")
		}
		step.f = args[i].(func(t *testing.T, mem *memCore))

		tc.steps = append(tc.steps, step)
	}
	return tc
}

type memCoreTestCase struct {
	name  string
	steps []memCoreTestStep
}

type memCoreTestStep struct {
	name string
	f    func(t *testing.T, mem *memCore)

	mem *memCore
}

func (step memCoreTestStep) bind(mem *memCore) func(t *testing.T) {
	step.mem = mem
	return step.boundTest
}

func (step memCoreTestStep) boundTest(t *testing.T) {
	step.f(t, step.mem)
}
