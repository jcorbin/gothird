package mem_test

import (
	"log"
	"os"
	"testing"

	// . "github.com/jcorbin/gothird/internal/mem"
	"github.com/jcorbin/gothird/internal/logio"
	"github.com/jcorbin/gothird/internal/mem"
	"github.com/jcorbin/gothird/internal/panicerr"
	"github.com/stretchr/testify/require"
)

func Test_Ints(t *testing.T) {
	for _, tc := range []intsTestCase{
		intsTest("basic",
			"init", func(t *testing.T, m *mem.Ints) {
				m.PageSize = 4
				val, err := m.Load(0)
				require.NoError(t, err, "unexpected load error")
				require.Equal(t, 0, val, "expected 0 @0")
				require.Equal(t, uint(0), m.Size(), "expected 0 initial size")
			},

			"9 -> 0", func(t *testing.T, m *mem.Ints) {
				require.NoError(t, m.Stor(0, 9), "must stor @0")
				val, err := m.Load(0)
				require.NoError(t, err, "unexpected load error")
				require.Equal(t, 9, val, "expected 9 @0")
				//  0  1  2  3  :  9  0  0  0
				//  4  5  6  7  :  -  -  -  -
				//  8  9  a  b  :  -  -  -  -
				//  c  d  e  f  :  -  -  -  -
				// 10 11 12 13  :  -  -  -  -
				expectMemValuesAt(t, m, 6,
					0, 0,
					0, 0, 0, 0,
					0, 0, 0, 0,
					0, 0)
			},

			"{1, 2, 3, 4, 5, 6} -> 0x9", func(t *testing.T, m *mem.Ints) {
				require.NoError(t, m.Stor(0x9, 1, 2, 3, 4, 5, 6), "must stor @0x9")
				require.Equal(t, mem.IntsDump{
					Bases: []uint{0x0, 0x8, 0xc},
					Sizes: []uint{4, 4, 4},
					Pages: [][]int{
						{9, 0, 0, 0},
						{0, 1, 2, 3},
						{4, 5, 6, 0},
					},
				}, m.Dump(), "expected a page hole")
				//  0  1  2  3  :  9  0  0  0
				//  4  5  6  7  :  -  -  -  -
				//  8  9  a  b  :  0  1  2  3
				//  c  d  e  f  :  4  5  6  0
				// 10 11 12 13  :  -  -  -  -
				expectMemValuesAt(t, m, 6,
					0, 0,
					0, 1, 2, 3,
					4, 5, 6, 0,
					0, 0)
			},

			"7 -> 0xf", func(t *testing.T, m *mem.Ints) {
				require.NoError(t, m.Stor(0xf, 7), "must stor @0xf")
				{
					val, err := m.Load(0xf)
					require.NoError(t, err, "unexpected load error")
					require.Equal(t, 7, val, "expected 7 @0xf")
				}
				{
					val, err := m.Load(0xe)
					require.NoError(t, err, "unexpected load error")
					require.Equal(t, 6, val, "expected 6 @0xe")
				}
				//  0  1  2  3  :  9  0  0  0
				//  4  5  6  7  :  -  -  -  -
				//  8  9  a  b  :  0  1  2  3
				//  c  d  e  f  :  4  5  6  7
				// 10 11 12 13  :  -  -  -  -
				expectMemValuesAt(t, m, 6,
					0, 0,
					0, 1, 2, 3,
					4, 5, 6, 7,
					0, 0)
			},

			"8 -> 0x15", func(t *testing.T, m *mem.Ints) {
				require.NoError(t, m.Stor(0x15, 8), "must stor @0x15")
				val, err := m.Load(0x15)
				require.NoError(t, err, "unexpected load error")
				require.Equal(t, 8, val, "expected 7 @0x15")
				//  0  1  2  3  :  9  0  0  0
				//  4  5  6  7  :  -  -  -  -
				//  8  9  a  b  :  0  1  2  3
				//  c  d  e  f  :  4  5  6  7
				// 10 11 12 13  :  -  -  -  -
				// 14 15 16 17  :  0  8  0  0
				// 18 19 20 21  :  -  -  -  -
				expectMemValuesAt(t, m, 6,
					0, 0,
					0, 1, 2, 3,
					4, 5, 6, 7,
					0, 0, 0, 0,
					0, 8, 0, 0,
					0, 0)
			},

			"stor across the 0x10 page gap", func(t *testing.T, m *mem.Ints) {
				require.NoError(t, m.Stor(0xe, 96, 97, 98, 99, 91, 92, 93, 94), "must stor @0x15")
				//  0  1  2  3  :  9  0  0  0
				//  4  5  6  7  :  -- -- -- --
				//  8  9  a  b  :  0  1  2  3
				//  c  d  e  f  :  4  5  96 97
				// 10 11 12 13  :  98 99 91 92
				// 14 15 16 17  :  93 94 0  0
				// 18 19 20 21  :  -- -- -- --
				expectMemValuesAt(t, m, 0xc,
					4, 5, 96, 97,
					98, 99, 91, 92,
					93, 94, 0, 0,
					0, 0,
				)
			},
		),

		intsTest("missing lower section",
			"initial value in 2nd page", func(t *testing.T, m *mem.Ints) {
				m.PageSize = 0x10
				expectMemValueAt(t, m, 0x18, 0)
				require.NoError(t, m.Stor(0x18, 42), "unexpected stor error")
				expectMemValueAt(t, m, 0x18, 42)
			},

			"load low", func(t *testing.T, m *mem.Ints) { expectMemValueAt(t, m, 0x8, 0) },

			"create 3rd page", func(t *testing.T, m *mem.Ints) {
				require.NoError(t, m.Stor(0x28, 99), "unexpected stor error")
				expectMemValueAt(t, m, 0x28, 99)
			},

			"load low again", func(t *testing.T, m *mem.Ints) { expectMemValueAt(t, m, 0x8, 0) },

			"finally create the 1st page", func(t *testing.T, m *mem.Ints) {
				require.NoError(t, m.Stor(0x8, 3), "unexpected stor error")
				expectMemValueAt(t, m, 0x8, 3)
			},
		),

		intsTest("vm set regression",
			"init", func(t *testing.T, m *mem.Ints) {
				m.PageSize = 32

				require.NoError(t, m.Stor(10, 16), "unexpected store error @10")
				require.NoError(t, m.Stor(11, 32), "unexpected store error @11")
				require.NoError(t, m.Stor(0, 32), "unexpected store error @0")
				require.NoError(t, m.Stor(1, 16), "unexpected store error @1")

				expectMemValuesAt(t, m, 0, 32, 16, 0, 0)
				expectMemValuesAt(t, m, 32, 0, 0, 0)
			},

			"set then load @memBase", func(t *testing.T, m *mem.Ints) {
				require.NoError(t, m.Stor(33, 108), "unexpected store error @33")
				expectMemValuesAt(t, m, 32, 0, 108, 0)
			},
		),
	} {
		t.Run(tc.name, func(t *testing.T) {
			tcLogOut := &logio.Writer{Logf: t.Logf}
			log.SetOutput(tcLogOut)
			defer log.SetOutput(os.Stderr)

			var m mem.Ints
			defer func() {
				if t.Failed() {
					d := m.Dump()
					t.Logf("bases: %v", d.Bases)
					t.Logf("sizes: %v", d.Sizes)
					t.Logf("pages: %v", d.Pages)
				}
			}()

			for _, step := range tc.steps {
				if !t.Run(step.name, func(t *testing.T) {
					stepLogOut := &logio.Writer{Logf: t.Logf}
					log.SetOutput(stepLogOut)
					defer log.SetOutput(tcLogOut)

					isolateTest(t, step.bind(&m))
				}) {
					break
				}
			}
		})
	}
}

func isolateTest(t *testing.T, f func(t *testing.T)) {
	if err := panicerr.Recover(t.Name(), func() error {
		f(t)
		return nil
	}); err != nil {
		t.Logf("%+v", err)
		t.Fail()
	}
}

func expectMemValueAt(t *testing.T, m *mem.Ints, addr uint, value int) {
	val, err := m.Load(addr)
	require.NoError(t, err, "unexpected load @0x%x error", addr)
	require.Equal(t, value, val, "expected value @0x%x", addr)
}

func expectMemValuesAt(t *testing.T, m *mem.Ints, addr uint, values ...int) {
	buf := make([]int, len(values))
	require.NoError(t, m.LoadInto(addr, buf),
		"must load %v values from @0x%x", len(values), addr)
	require.Equal(t, values, buf, "expected values @0x%x", addr)
}

func intsTest(name string, args ...interface{}) (tc intsTestCase) {
	tc.name = name
	for i := 0; i < len(args); i++ {
		var step memCoreTestStep

		step.name = args[i].(string)

		if i++; i >= len(args) {
			panic("intsTest: not missing function argument after name")
		}
		step.f = args[i].(func(t *testing.T, m *mem.Ints))

		tc.steps = append(tc.steps, step)
	}
	return tc
}

type intsTestCase struct {
	name  string
	steps []memCoreTestStep
}

type memCoreTestStep struct {
	name string
	f    func(t *testing.T, m *mem.Ints)

	m *mem.Ints
}

func (step memCoreTestStep) bind(m *mem.Ints) func(t *testing.T) {
	step.m = m
	return step.boundTest
}

func (step memCoreTestStep) boundTest(t *testing.T) {
	step.f(t, step.m)
}
