package main

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_VM(t *testing.T) {
	var testCases vmTestCases

	// primitive tests that work by driving individual VM methods
	var (
		compileme = (*VM).compileme
		define    = (*VM).define
		immediate = (*VM).immediate
		read      = (*VM).read
		get       = (*VM).get
		set       = (*VM).set
		sub       = (*VM).sub
		mul       = (*VM).mul
		div       = (*VM).div
		under0    = (*VM).under0
		exit      = (*VM).exit
		echo      = (*VM).echo
		key       = (*VM).key
		pick      = (*VM).pick
		pushint   = (*VM).pushint
		step      = (*VM).step
	)
	testCases = append(testCases,
		// binary integer operation on the stack
		vmTest("sub").withStack(5, 3, 1).do(sub).expectStack(5, 2),
		vmTest("div").withStack(7, 13, 3).do(div).expectStack(7, 4),
		vmTest("mul").withStack(11, 5, 6).do(mul).expectStack(11, 30),

		// is top of stack less than 0?
		vmTest("less true").withStack(2, -3).do(under0).expectStack(2, 1),
		vmTest("less false").withStack(2, 3).do(under0).expectStack(2, 0),

		// pop top of stack, use as index into stack and copy up that element
		vmTest("pick 0").withStack(1, 2, 3, 4, 5, 0).do(pick).expectStack(1, 2, 3, 4, 5, 5),
		vmTest("pick 1").withStack(1, 2, 3, 4, 5, 1).do(pick).expectStack(1, 2, 3, 4, 5, 4),
		vmTest("pick 2").withStack(1, 2, 3, 4, 5, 2).do(pick).expectStack(1, 2, 3, 4, 5, 3),
		vmTest("pick 3").withStack(1, 2, 3, 4, 5, 3).do(pick).expectStack(1, 2, 3, 4, 5, 2),
		vmTest("pick 4").withStack(1, 2, 3, 4, 5, 4).do(pick).expectStack(1, 2, 3, 4, 5, 1),
		vmTest("pick 5").withStack(1, 2, 3, 4, 5, 5).do(pick).expectStack(1, 2, 3, 4, 5, 0),

		// read from memory
		vmTest("get").withMemAt(1024, 99, 42, 108).withStack(1025).do(get).expectStack(42),

		// write to memory
		vmTest("set").withMemAt(1024, 0, 0, 0).withStack(108, 1025).do(set).expectMemAt(1024, 0, 108, 0),

		// push an immediate value onto the stack
		vmTest("pushint").withMemAt(1024, 99, 42, 108).withProg(1025).do(pushint).expectStack(42).expectProg(1026),

		// compile the program counter
		vmTest("compileme").withMemAt(256, 2048).withR(256).withMemAt(1024,
			0,             // 1024: word prev
			vmCodeSub,     // 1025: ... name
			vmCodeCompile, // 1026: ...
			vmCodeRun,     // 1027: ...       <-- prog
			vmCodeSub,     // 1028: ...
			vmCodeExit,    // 1029: ...
			0,             // 1030:
			0,             // 1031:           <-- h
			0,             // 1032:
			0,             // 1033:
		).withProg(1027).withH(1031).do(compileme).expectMemAt(1024,
			0,             // 1024: word prev
			vmCodeSub,     // 1025: ... name
			vmCodeCompile, // 1026: ...
			vmCodeRun,     // 1027: ...
			vmCodeSub,     // 1028: ...
			vmCodeExit,    // 1029: ...
			0,             // 1030:
			1028,          // 1031:
			0,             // 1032:           <-- h
			0,             // 1033:
			//                ....:
			//                2048:           <-- prog
		).expectProg(2048).expectH(1032),

		// compile the header of a definition
		vmTest("define").withMemAt(1024,
			0, // 1024: <-- h
			0, // 1025:
			0, // 1026:
			0, // 1027:
			0, // 1028:
			0, // 1029:
			0, // 1030:
		).withH(1024).withInput(` : `).do(define).expectMemAt(1024,
			0,             // 1024: word prev
			1,             // 1025: word name
			vmCodeCompile, // 1026: ...
			vmCodeRun,     // 1027: ...
			0,             // 1028:           <-- h
			0,             // 1029:
			0,             // 1030:
		).expectH(1028).expectLast(1024).expectString(1, ":"),

		// modify the header to create an immediate word
		vmTest("immediate").withMemAt(1024,
			0, // 1024: <-- h
			0, // 1025:
			0, // 1026:
			0, // 1027:
			0, // 1028:
			0, // 1029:
			0, // 1030:
		).withH(1024).withInput(`:`).do(define, immediate).expectMemAt(1024,
			0,         // 1024: word prev
			1,         // 1025: ... name
			vmCodeRun, // 1026: ...
			vmCodeRun, // 1027: ...           <-- h
			0,         // 1028:
			0,         // 1029:
		).expectH(1027).expectString(1, ":"),

		// stop running the current function
		vmTest("exit").withRetBase(256,
			1024, // 256: ret[0]
			1025, // 257: ret[1]
			1026, // 258: ret[2]
			1027, // 259: ret[3]
		).withMemBase(1024, // NOTE this simply explicates the default 0 == exit value
			vmCodeExit, // 1024:
			vmCodeExit, // 1025:
			vmCodeExit, // 1026:
			vmCodeExit, // 1027:
		).do(exit, step, nil).expectMemAt(256, // NOTE return stack not zerod
			1024, // 256: ret[0]
			1025, // 257: ret[1]
			1026, // 258: ret[2]
			1027, // 259: ret[3]
		).expectProg(1025).expectR(255),

		// read a word from input and compile a pointer to it
		vmTest("read").withStrings(
			1, "foo",
			2, "bar",
		).withMemAt(1024,
			0,             // 1024:
			1,             // 1025:
			vmCodeCompile, // 1026:
			vmCodeRun,     // 1027:
			vmCodeSub,     // 1028:
			vmCodeExit,    // 1029:           <-- prog
			1024,          // 1030:           <-- last
			2,             // 1031:
			vmCodeRun,     // 1032:
			vmCodeRun,     // 1033:           <-- h
			0,             // 1034:
			0,             // 1035:
			0,             // 1036:
		).withH(1033).withProg(1029).withLast(1030).withInput("foo").do(
			read, step, nil,
		).expectMemAt(1024,
			0,             // 1024:
			1,             // 1025:
			vmCodeCompile, // 1026:
			vmCodeRun,     // 1027:
			vmCodeSub,     // 1028:
			vmCodeExit,    // 1029:
		).expectMemAt(1030,
			1024,      // 1030:
			2,         // 1031:
			vmCodeRun, // 1032:
			1028,      // 1033:
			0,         // 1034:           <-- h
			0,         // 1035:
			0,         // 1036:
		).expectH(1034),

		// output one character
		// input one character
		vmTest("key^2 => echo^2").withInput("ab").do(key, key, echo, echo).expectOutput("ba"),
	)

	testCases = append(testCases, vmTest("builtin setup").withInput(`
		exit : immediate _read @ ! - * / <0 echo key pick
	`).
		expectWord(1024, "", vmCodeRun, vmCodeRead, 1027, vmCodeExit).
		expectWord(1030, "exit", vmCodeCompIt, vmCodeExit).
		expectWord(1034, ":", vmCodeDefine, vmCodeExit).
		expectWord(1038, "immediate", vmCodeImmediate, vmCodeExit).
		expectWord(1042, "_read", vmCodeCompIt, vmCodeRead, vmCodeExit).
		expectDump(lines(
			`# VM Dump`,
			`  prog: 1028`,
			`  dict: [1087 1082 1077 1072 1067 1062 1057 1052 1047 1042 1038 1034 1030 1024]`,
			`  stack: []`,
			`  @    0 1092 dict`,
			`  @    1 255 ret`,
			`  @    2 0`,
			`  @    3 0`,
			`  @    4 0`,
			`  @    5 0`,
			`  @    6 0`,
			`  @    7 0`,
			`  @    8 0`,
			`  @    9 0`,
			`  @   10 256 retBase`,
			`  @   11 1024 memBase`,

			`# Return Stack @256`,

			`# Main Memory @1024`,
			`  @ 1024 : ø immediate runme read ø+3 exit`,
			`  @ 1030 : exit exit`,
			`  @ 1034 : : immediate define exit`,
			`  @ 1038 : immediate immediate immediate exit`,
			`  @ 1042 : _read read exit`,
			`  @ 1047 : @ get exit`,
			`  @ 1052 : ! set exit`,
			`  @ 1057 : - sub exit`,
			`  @ 1062 : * mul exit`,
			`  @ 1067 : / div exit`,
			`  @ 1072 : <0 under0 exit`,
			`  @ 1077 : echo echo exit`,
			`  @ 1082 : key key exit`,
			`  @ 1087 : pick pick exit`,
		)))

	// better main loop
	testCases = append(testCases, vmTest("new main").withInput(`
		exit : immediate _read @ ! - * / <0 echo key pick

		: ]
			1 @
			1 -
			1 !
			_read
			]

		: main immediate ]
		main
	`).expectMemAt(1092,
		/* @1092 */ 1087,
		/* @1093 */ 14,
		/* @1094 */ vmCodeCompile,
		/* @1095 */ vmCodeRun,
		/* @1096 */ vmCodePushint, 1, vmCodeGet,
		/* @1099 */ vmCodePushint, 1, vmCodeSub,
		/* @1102 */ vmCodePushint, 1, vmCodeSet,
		/* @1105 */ vmCodeRead,
		/* @1106 */ 1096,
	).expectMemAt(1107,
		/* @1107 */ 1092,
		/* @1108 */ 15,
		/* @1109 */ vmCodeRun,
		/* @1110 */ 1096,
	).expectDump(lines(
		`# VM Dump`,
		`  prog: 1106`,
		`  dict: [1107 1092 1087 1082 1077 1072 1067 1062 1057 1052 1047 1042 1038 1034 1030 1024]`,
		`  stack: []`,
		`  @    0 1111 dict`,
		`  @    1 268 ret`,
		`  @    2 0`,
		`  @    3 0`,
		`  @    4 0`,
		`  @    5 0`,
		`  @    6 0`,
		`  @    7 0`,
		`  @    8 0`,
		`  @    9 0`,
		`  @   10 256 retBase`,
		`  @   11 1024 memBase`,

		`# Return Stack @256`,
		`  @  256 1029 ret_0`,
		`  @  257 1029 ret_1`,
		`  @  258 1029 ret_2`,
		`  @  259 1029 ret_3`,
		`  @  260 1029 ret_4`,
		`  @  261 1029 ret_5`,
		`  @  262 1029 ret_6`,
		`  @  263 1029 ret_7`,
		`  @  264 1029 ret_8`,
		`  @  265 1029 ret_9`,
		`  @  266 1029 ret_10`,
		`  @  267 1029 ret_11`,
		`  @  268 1028 ret_12`,

		`# Main Memory @1024`,
		`  @ 1024 : ø immediate runme read ø+3 exit`,
		`  @ 1030 : exit exit`,
		`  @ 1034 : : immediate define exit`,
		`  @ 1038 : immediate immediate immediate exit`,
		`  @ 1042 : _read read exit`,
		`  @ 1047 : @ get exit`,
		`  @ 1052 : ! set exit`,
		`  @ 1057 : - sub exit`,
		`  @ 1062 : * mul exit`,
		`  @ 1067 : / div exit`,
		`  @ 1072 : <0 under0 exit`,
		`  @ 1077 : echo echo exit`,
		`  @ 1082 : key key exit`,
		`  @ 1087 : pick pick exit`,
		`  @ 1092 : ] runme pushint(1) get pushint(1) sub pushint(1) set read ]+4`,
		`  @ 1107 : main immediate runme ]+4`,
	)))

	testCases = append(testCases, vmTest("add").withInput(`
		exit : immediate _read @ ! - * / < echo key pick
		: ] 1 @ 1 - 1 ! _read ]
		: main immediate ]
		main

		: _x  3 @ exit
		: _x! 3 ! exit
		: + _x! 0 _x - - exit

		: test immediate
			3 5 7 + +
		test
	`).expectWord(1111, "_x", vmCodeCompile, vmCodeRun,
		vmCodePushint, 3, vmCodeGet, vmCodeExit,
	).expectWord(1119, "_x!", vmCodeCompile, vmCodeRun,
		vmCodePushint, 3, vmCodeSet, vmCodeExit,
	).expectWord(1127, "+", vmCodeCompile, vmCodeRun,
		1123, // _x!
		vmCodePushint, 0,
		1115, // _x
		vmCodeSub, vmCodeSub, vmCodeExit,
	).expectStack(3+5+7))

	testCases = append(testCases, vmTest("hello").withInput(`
		exit : immediate _read @ ! - * / < echo key pick
		: ] 1 @ 1 - 1 ! _read ]
		: main immediate ] main
		: _x  3 @ exit
		: _x! 3 ! exit
		: + _x! 0 _x - - exit

		: digit '0' + echo exit

		: test immediate
			0         digit
			10 3 -    digit
			21 3 /    digit
			9 2 3 * - digit
			2 2 *     digit
			'\n'      echo
			exit
		test
	`).expectOutput("07734\n"))

	testCases = append(testCases, vmTest("ansi literals").withInput(`
		exit : immediate _read @ ! - * / < echo key pick
		: ] 1 @ 1 - 1 ! _read ]
		: main immediate ] main
		: _x  3 @ exit
		: _x! 3 ! exit
		: + _x! 0 _x - - exit

		: digit '0' + echo exit

		: sgr_reset
			<CSI> echo
			'0'   echo
			'm'   echo
			exit

		: sgr_fg
			<CSI> echo
			'3'   echo
			'0' + echo
			'm'   echo
			exit

		: test immediate
			sgr_reset 2 sgr_fg
				'S' echo
				'u' echo
				'p' echo
				'e' echo
				'r' echo
			sgr_reset
			<cr> echo
			<nl> echo
			exit
		test
	`).expectOutput("\x1b[0m\x1b[32mSuper\x1b[0m\r\n"))

	testCases = append(testCases, vmTest("reboot").withInput(`
		exit : immediate _read @ ! - * / < echo key pick

		: ] 1 @ 1 - 1 ! _read ]
		: main immediate ] main

		: r  1  exit
		: rb 10 exit

		: _x  3 @ exit
		: _x! 3 ! exit
		: _y  4 @ exit
		: _y! 4 ! exit
		: _z  5 @ exit
		: _z! 5 ! exit

		: + _x! 0 _x - - exit
		: dup _x! _x _x exit

		: '
			r @ @
			dup
			-1 -
			r @ !
			@
			exit

		: exec
			rb @ !
			rb @ r !
			exit

		: _read] _read ]
		: reboot immediate
			' _read]
			exec
		reboot

		: _flags! rb @ 1 - ! exit
		: _tron  1 _flags! exit
		: _troff 0 _flags! exit

		: test immediate
			_tron
			42
			1024 1024 * @
		test
	`).expectRStack(1106).expectStack(42).expectError(memLimitError{1024 * 1024, "get"}))

	testCases.run(t)
}

type vmTestCases []vmTestCase

func (vmts vmTestCases) run(t *testing.T) {
	{
		var exclusive []vmTestCase
		for _, vmt := range vmts {
			if vmt.exclusive {
				exclusive = append(exclusive, vmt)
			}
		}
		if len(exclusive) > 0 {
			vmts = exclusive
		}
	}
	for _, vmt := range vmts {
		if !t.Run(vmt.name, vmt.run) {
			return
		}
	}
}

func vmTest(name string) (vmt vmTestCase) {
	vmt.name = name
	return vmt
}

type optFunc func(vm *VM)

func (f optFunc) apply(vm *VM) { f(vm) }

type vmTestCase struct {
	name    string
	opts    []interface{}
	setup   []func(t *testing.T, vm *VM)
	ops     []func(vm *VM)
	expect  []func(t *testing.T, vm *VM)
	timeout time.Duration
	wantErr error

	exclusive   bool
	nextInputID int
}

func (vmt vmTestCase) exclusiveTest() vmTestCase {
	vmt.exclusive = true
	return vmt
}

func (vmt vmTestCase) withOptions(opts ...VMOption) vmTestCase {
	for _, opt := range opts {
		vmt.opts = append(vmt.opts, opt)
	}
	return vmt
}

func (vmt vmTestCase) withProg(prog uint) vmTestCase {
	vmt.opts = append(vmt.opts, optFunc(func(vm *VM) {
		vm.prog = prog
	}))
	return vmt
}

func (vmt vmTestCase) withLast(last uint) vmTestCase {
	vmt.opts = append(vmt.opts, optFunc(func(vm *VM) {
		vm.last = last
	}))
	return vmt
}

func (vmt vmTestCase) withStack(values ...int) vmTestCase {
	vmt.opts = append(vmt.opts, optFunc(func(vm *VM) {
		vm.stack = append(vm.stack, values...)
	}))
	return vmt
}

func (vmt vmTestCase) withStrings(idStringPairs ...interface{}) vmTestCase {
	if len(idStringPairs)%2 == 1 {
		panic("must be given variadic pairs")
	}
	for i := 0; i < len(idStringPairs); i++ {
		id := idStringPairs[i].(int)
		i++
		s := idStringPairs[i].(string)
		vmt = vmt.withString(uint(id), s)
	}
	return vmt
}

func (vmt vmTestCase) withString(id uint, s string) vmTestCase {
	vmt.opts = append(vmt.opts, optFunc(func(vm *VM) {
		if need := int(id) - len(vm.symbols.strings); need > 0 {
			vm.symbols.strings = append(vm.symbols.strings, make([]string, need)...)
		}
		if vm.symbols.symbols == nil {
			vm.symbols.symbols = make(map[string]uint)
		}
		vm.symbols.strings[id-1] = s
		vm.symbols.symbols[s] = id
	}))
	return vmt
}

func (vmt vmTestCase) withMemAt(addr uint, values ...int) vmTestCase {
	if len(values) != 0 {
		vmt.opts = append(vmt.opts, optFunc(func(vm *VM) {
			vm.stor(uint(addr), values...)
		}))
	}
	return vmt
}

func (vmt vmTestCase) withH(val int) vmTestCase {
	vmt.opts = append(vmt.opts, optFunc(func(vm *VM) {
		vm.stor(0, val)
	}))
	return vmt
}

func (vmt vmTestCase) withR(val int) vmTestCase {
	vmt.opts = append(vmt.opts, optFunc(func(vm *VM) {
		vm.stor(1, val)
	}))
	return vmt
}

func (vmt vmTestCase) withPageSize(pageSize uint) vmTestCase {
	vmt.opts = append(vmt.opts, optFunc(func(vm *VM) {
		vm.pageSize = pageSize
	}))
	return vmt
}

func (vmt vmTestCase) withRetBase(addr uint, values ...int) vmTestCase {
	vmt.opts = append(vmt.opts, optFunc(func(vm *VM) {
		vm.stor(10, int(addr))
	}))
	return vmt.withMemAt(addr, values...).withR(int(addr) + len(values) - 1)
}

func (vmt vmTestCase) withMemBase(addr uint, values ...int) vmTestCase {
	vmt.opts = append(vmt.opts, optFunc(func(vm *VM) {
		vm.stor(11, int(addr))
	}))
	return vmt.withMemAt(addr, values...)
}

func (vmt vmTestCase) withMemLimit(limit uint) vmTestCase {
	vmt.opts = append(vmt.opts, withMemLimit(limit))
	return vmt
}

func (vmt vmTestCase) withInput(source string) vmTestCase {
	vmt.opts = append(vmt.opts, func(vmt *vmTestCase, t *testing.T) VMOption {
		name := t.Name() + "/input"
		if id := vmt.nextInputID; id > 0 {
			name += "_" + strconv.Itoa(id+1)
		}
		vmt.nextInputID++
		return WithInput(NamedReader(name, strings.NewReader(source)))
	})
	return vmt
}

func (vmt vmTestCase) withInputWriter(w io.WriterTo) vmTestCase {
	vmt.opts = append(vmt.opts, WithInputWriter(w))
	return vmt
}

func (vmt vmTestCase) do(ops ...func(vm *VM)) vmTestCase {
	vmt.ops = append(vmt.ops, ops...)
	return vmt
}

func (vmt vmTestCase) withTimeout(timeout time.Duration) vmTestCase {
	vmt.timeout = timeout
	return vmt
}

func (vmt vmTestCase) expectError(err error) vmTestCase {
	vmt.wantErr = err
	return vmt
}

func (vmt vmTestCase) expectProg(prog uint) vmTestCase {
	vmt.expect = append(vmt.expect, func(t *testing.T, vm *VM) {
		assert.Equal(t, prog, vm.prog, "expected program counter")
	})
	return vmt
}

func (vmt vmTestCase) expectLast(last uint) vmTestCase {
	vmt.expect = append(vmt.expect, func(t *testing.T, vm *VM) {
		assert.Equal(t, last, vm.last, "expected last address")
	})
	return vmt
}

func (vmt vmTestCase) expectStack(values ...int) vmTestCase {
	vmt.expect = append(vmt.expect, func(t *testing.T, vm *VM) {
		if values == nil {
			values = []int{}
		}
		assert.Equal(t, values, vm.stack, "expected stack values")
	})
	return vmt
}

func (vmt vmTestCase) expectRStack(values ...int) vmTestCase {
	vmt.expect = append(vmt.expect, func(t *testing.T, vm *VM) {
		if values == nil {
			values = []int{}
		}
		assert.Equal(t, values, vm.rstack(), "expected return stack values")
	})
	return vmt
}

func (vmt vmTestCase) expectString(id uint, s string) vmTestCase {
	vmt.expect = append(vmt.expect, func(t *testing.T, vm *VM) {
		assert.Equal(t, s, vm.string(id), "expected string #%v", id)
	})
	return vmt
}

func (vmt vmTestCase) expectMemAt(addr uint, values ...int) vmTestCase {
	vmt.expect = append(vmt.expect, func(t *testing.T, vm *VM) {
		buf := make([]int, len(values))
		vm.loadInto(addr, buf)
		if !assert.Equal(t, values, buf, "expected memory values @%v", addr) {
			for i, value := range values {
				a := addr + uint(i)
				assert.Equal(t, value, vm.load(a), "expected memory value @%v", a)
			}
			t.Logf("bases: %v", vm.bases)
			t.Logf("pages: %v", vm.pages)
		}
	})
	return vmt
}

func (vmt vmTestCase) expectWord(addr uint, name string, code ...int) vmTestCase {
	vmt.expect = append(vmt.expect, func(t *testing.T, vm *VM) {
		buf := make([]int, len(code))
		assert.Equal(t, name, vm.string(uint(vm.load(addr+1))), "expected word @%v name", addr)
		vm.loadInto(addr+2, buf)
		assert.Equal(t, code, buf, "expected %q @%v+2 code", name, addr)
	})
	return vmt
}

func (vmt vmTestCase) expectH(value int) vmTestCase {
	vmt.expect = append(vmt.expect, func(t *testing.T, vm *VM) {
		assert.Equal(t, value, vm.load(0), "expected H value")
	})
	return vmt
}

func (vmt vmTestCase) expectR(value int) vmTestCase {
	vmt.expect = append(vmt.expect, func(t *testing.T, vm *VM) {
		assert.Equal(t, value, vm.load(1), "expected R value")
	})
	return vmt
}

func (vmt vmTestCase) expectOutput(output string) vmTestCase {
	var out strings.Builder
	vmt.opts = append(vmt.opts, WithOutput(&out))
	vmt.expect = append(vmt.expect, func(t *testing.T, vm *VM) {
		assert.Equal(t, output, out.String(), "expected output")
	})
	return vmt
}

func (vmt vmTestCase) expectDump(dump string) vmTestCase {
	vmt.expect = append(vmt.expect, func(t *testing.T, vm *VM) {
		var out strings.Builder
		vmDumper{
			vm:  vm,
			out: &out,
		}.dump()
		assert.Equal(t, dump, out.String(), "expected dump")
	})
	return vmt
}

func (vmt vmTestCase) withTestDump() vmTestCase {
	vmt.expect = append(vmt.expect, vmt.dumpToTest)
	return vmt
}

func (vmt vmTestCase) withTestOutput() vmTestCase {
	vmt.setup = append(vmt.setup, func(t *testing.T, vm *VM) {
		lw := &logWriter{logf: func(mess string, args ...interface{}) {
			t.Logf("out: "+mess, args...)
		}}
		vm.closers = append(vm.closers, lw)
		WithTee(lw).apply(vm)
	})
	return vmt
}

func (vmt vmTestCase) withTestHexOutput() vmTestCase {
	vmt.setup = append(vmt.setup, func(t *testing.T, vm *VM) {
		lw := &logWriter{logf: func(mess string, args ...interface{}) {
			t.Logf("out: "+mess, args...)
		}}
		enc := hex.Dumper(lw)
		WithTee(enc).apply(vm)
		vm.closers = append(vm.closers, enc)
		vm.closers = append(vm.closers, lw)
	})
	return vmt
}

func (vmt vmTestCase) buildOptions(t *testing.T) VMOption {
	var opts []VMOption
	for _, opt := range vmt.opts {
		switch impl := opt.(type) {
		case func(vmt *vmTestCase, t *testing.T) VMOption:
			opts = append(opts, impl(&vmt, t))
		case VMOption:
			opts = append(opts, impl)
		default:
			t.Logf("unsupported vmTestCase opt type %T", opt)
			t.FailNow()
		}
	}
	return VMOptions(opts...)
}

func (vmt vmTestCase) run(t *testing.T) {
	ctx := context.TODO()

	const (
		defaultTimeout  = time.Second
		defaultMemLimit = 4 * 1024
	)

	var vm VM
	vmt.buildOptions(t).apply(&vm)

	for _, setup := range vmt.setup {
		setup(t, &vm)
	}
	if vm.in == nil {
		vm.in = strings.NewReader("")
	}
	if vm.out == nil {
		vm.out = newWriteFlusher(ioutil.Discard)
	}
	if vm.memLimit == 0 {
		vm.memLimit = defaultMemLimit
	}
	WithLogf(t.Logf).apply(&vm)

	timeout := vmt.timeout
	if timeout == 0 {
		timeout = defaultTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	defer vm.Close()

	defer func() {
		if t.Failed() {
			vmt.dumpToTest(t, &vm)
		}
	}()

	if len(vmt.ops) > 0 {
		vmt.runOps(ctx, t, &vm)
	} else if err := vm.Run(ctx); vmt.wantErr != nil {
		require.True(t, errors.Is(err, vmt.wantErr), "expected error: %v\ngot: %+v", vmt.wantErr, err)
	} else {
		require.NoError(t, err, "expected no VM error")
	}

	for _, expect := range vmt.expect {
		expect(t, &vm)
	}
}

func (vmt vmTestCase) dumpToTest(t *testing.T, vm *VM) {
	lw := logWriter{logf: t.Logf}
	defer lw.Close()
	vmDumper{vm: vm, out: &lw}.dump()
}

func (vmt vmTestCase) runOps(ctx context.Context, t *testing.T, vm *VM) {
	names := make([]string, len(vmt.ops))
	for i, op := range vmt.ops {
		names[i] = runtime.FuncForPC(reflect.ValueOf(op).Pointer()).Name()
	}

	if err := isolate("VMOps", func() error {
		vm.init()
		for i := 0; i < len(vmt.ops); i++ {
			if vmt.ops[i] == nil {
				i--
			}
			t.Logf("do[%v] %v", i, names[i])
			vmt.ops[i](vm)
			if err := ctx.Err(); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		wantErr := vmt.wantErr
		if wantErr == nil {
			wantErr = vmHaltError{nil}
		}
		if !errors.Is(err, wantErr) {
			assert.NoError(t, err, "expected vm to halt with %v", wantErr)
		}
	}
}

//// utilities

func lines(parts ...string) string {
	return strings.Join(parts, "\n") + "\n"
}

type lineLogger struct {
	io.Writer
	prior bool
}

func (ll *lineLogger) printf(mess string, args ...interface{}) {
	if ll.prior {
		io.WriteString(ll.Writer, "\n")
	} else {
		ll.prior = true
	}
	fmt.Fprintf(ll.Writer, mess, args...)
}
