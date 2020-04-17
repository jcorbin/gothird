package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"reflect"
	"runtime"
	"runtime/debug"
	"sort"
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
		vmTest("get").withMemBase(32, 99, 42, 108).withStack(33).do(get).expectStack(42),

		// write to memory
		vmTest("set").withMemBase(32, 0, 0, 0).withStack(108, 33).do(set).expectMemAt(32, 0, 108, 0),

		// push an immediate value onto the stack
		vmTest("pushint").withMemAt(32, 99, 42, 108).withProg(33).do(pushint).expectStack(42).expectProg(34),

		// compile the program counter
		vmTest("compileme").withH(39).withRetBase(16,
			108,
		).withMemBase(32,
			0,             // 32: word prev
			vmCodeSub,     // 33: ... name
			vmCodeCompile, // 34: ...
			vmCodeRun,     // 35: ...       <-- prog
			vmCodeSub,     // 36: ...
			vmCodeExit,    // 37: ...
			0,             // 38:
			0,             // 39:           <-- h
			0,             // 40:
			0,             // 41:
		).withProg(35).do(compileme).expectH(40).expectMemAt(32,
			0,             // 32: word prev
			vmCodeSub,     // 33: ... name
			vmCodeCompile, // 34: ...
			vmCodeRun,     // 35: ...
			vmCodeSub,     // 36: ...
			vmCodeExit,    // 37: ...
			0,             // 38:
			36,            // 39:
			0,             // 40:           <-- h
			0,             // 41:
		).expectProg(108),

		// compile the header of a definition
		vmTest("define").withH(32).withMemAt(32,
			0, // 32: <-- h
			0, // 33:
			0, // 34:
			0, // 35:
			0, // 36:
			0, // 37:
			0, // 38:
		).withInput(` : `).do(define).expectH(36).expectMemAt(32,
			0,             // 32: word prev
			1,             // 33: word name
			vmCodeCompile, // 34: ...
			vmCodeRun,     // 35: ...
			0,             // 36:           <-- h
			0,             // 37:
			0,             // 38:
		).expectString(1, ":").expectLast(32),

		// modify the header to create an immediate word
		vmTest("immediate").withH(32).withMemAt(32,
			0, // 32: <-- h
			0, // 33:
			0, // 34:
			0, // 35:
			0, // 36:
			0, // 37:
			0, // 38:
		).withInput(`:`).do(define, immediate).expectH(35).expectMemAt(32,
			0,         // 32: word prev
			1,         // 33: ... name
			vmCodeRun, // 34: ...
			vmCodeRun, // 35: ...           <-- h
			0,         // 36:
			0,         // 37:
		).expectString(1, ":"),

		// stop running the current function
		vmTest("exit").withH(0).withRetBase(12,
			16, // 12: ret[0]
			17, // 13: ret[1]
			18, // 14: ret[2]
			19, // 15: ret[3]
		).withMemBase(16,
			vmCodeExit, // 16:
			vmCodeExit, // 17:
			vmCodeExit, // 18:
			vmCodeExit, // 19:
		).do(exit, step, nil).expectMemAt(12,
			16, // 12: ret[0]
			17, // 13: ret[1]
			18, // 14: ret[2]
			19, // 15: ret[3]
		).expectProg(17).expectH(16).expectR(12),

		// read a word from input and compile a pointer to it
		vmTest("read").withStrings(
			1, "foo",
			2, "bar",
		).withH(41).withMemBase(32,
			0,             // 32:
			1,             // 33:
			vmCodeCompile, // 34:
			vmCodeRun,     // 35:
			vmCodeSub,     // 36:
			vmCodeExit,    // 37:           <-- prog
			32,            // 38:           <-- last
			2,             // 39:
			vmCodeRun,     // 40:
			vmCodeRun,     // 41:           <-- h
			0,             // 42:
			0,             // 43:
			0,             // 44:
		).withProg(37).withLast(38).withInput("foo").do(
			read, step, nil,
		).expectH(42).expectMemAt(32,
			0,             // 32:
			1,             // 33:
			vmCodeCompile, // 34:
			vmCodeRun,     // 35:
			vmCodeSub,     // 36:
			vmCodeExit,    // 37:
		).expectMemAt(38,
			32,        // 38:
			2,         // 39:
			vmCodeRun, // 40:
			36,        // 41:
			0,         // 42:           <-- h
			0,         // 43:
			0,         // 44:
		),

		// output one character
		// input one character
		vmTest("key^2 => echo^2").withInput("ab").do(key, key, echo, echo).expectOutput("ba"),
	)

	testCases = append(testCases, vmTest("builtin setup").withInput(`
		: immediate _read @ ! - * / <0 exit echo key pick
	`).
		expectMemAt(32, 0, 0, vmCodeRun, vmCodeRead, 35, vmCodeExit).
		expectMemAt(38, 32, 1, vmCodeDefine, vmCodeExit).
		expectMemAt(42, 38, 2, vmCodeImmediate, vmCodeExit).
		expectMemAt(46, 42, 3, vmCodeCompIt, vmCodeRead, vmCodeExit).
		expectDump(lines(
			`prog: 36`,
			`dict: [95 90 85 81 76 71 66 61 56 51 46 42 38 32]`,
			`stack: []`,
			`@   0 100 dict`,
			`@   1 16 ret`,
			`@   2 0`,
			`@   3 0`,
			`@   4 0`,
			`@   5 0`,
			`@   6 0`,
			`@   7 0`,
			`@   8 0`,
			`@   9 0`,
			`@  10 16 retBase`,
			`@  11 32 memBase`,
			`@  12 0`,
			`@  13 0`,
			`@  14 0`,
			`@  15 0`,
			`@  32 : ø immediate runme read ø+3 exit`,
			`@  38 : : immediate define exit`,
			`@  42 : immediate immediate immediate exit`,
			`@  46 : _read read exit`,
			`@  51 : @ get exit`,
			`@  56 : ! set exit`,
			`@  61 : - sub exit`,
			`@  66 : * mul exit`,
			`@  71 : / div exit`,
			`@  76 : <0 under0 exit`,
			`@  81 : exit exit`,
			`@  85 : echo echo exit`,
			`@  90 : key key exit`,
			`@  95 : pick pick exit`,
		)))

	// better main loop
	testCases = append(testCases, vmTest("new main").withInput(`
		: immediate _read @ ! - * / <0 exit echo key pick

		: ]
			1 @
			1 -
			1 !
			_read
			]

		: main immediate ]
		main
	`).expectMemAt(100,
		95,
		14,
		vmCodeCompile,
		vmCodeRun,
		vmCodePushint, 1, vmCodeGet,
		vmCodePushint, 1, vmCodeSub,
		vmCodePushint, 1, vmCodeSet,
		vmCodeRead,
		104,
	).expectMemAt(115, 100, 15, vmCodeRun, 104).withTestLog().expectDump(lines(
		`prog: 114`,
		`dict: [115 100 95 90 85 81 76 71 66 61 56 51 46 42 38 32]`,
		`stack: []`,
		`@   0 119 dict`,
		`@   1 29 ret`,
		`@   2 0`,
		`@   3 0`,
		`@   4 0`,
		`@   5 0`,
		`@   6 0`,
		`@   7 0`,
		`@   8 0`,
		`@   9 0`,
		`@  10 16 retBase`,
		`@  11 32 memBase`,
		`@  12 0`,
		`@  13 0`,
		`@  14 0`,
		`@  15 0`,
		`@  16 37 ret_0`,
		`@  17 37 ret_1`,
		`@  18 37 ret_2`,
		`@  19 37 ret_3`,
		`@  20 37 ret_4`,
		`@  21 37 ret_5`,
		`@  22 37 ret_6`,
		`@  23 37 ret_7`,
		`@  24 37 ret_8`,
		`@  25 37 ret_9`,
		`@  26 37 ret_10`,
		`@  27 37 ret_11`,
		`@  28 36 ret_12`,
		`@  32 : ø immediate runme read ø+3 exit`,
		`@  38 : : immediate define exit`,
		`@  42 : immediate immediate immediate exit`,
		`@  46 : _read read exit`,
		`@  51 : @ get exit`,
		`@  56 : ! set exit`,
		`@  61 : - sub exit`,
		`@  66 : * mul exit`,
		`@  71 : / div exit`,
		`@  76 : <0 under0 exit`,
		`@  81 : exit exit`,
		`@  85 : echo echo exit`,
		`@  90 : key key exit`,
		`@  95 : pick pick exit`,
		`@ 100 : ] runme pushint(1) get pushint(1) sub pushint(1) set read ]+4`,
		`@ 115 : main immediate runme ]+4`,
	)))

	/*
		{"hello", vmTestScript{
			retBase: 16,
			memBase: 32,
			memSize: 256,
			program: `
				: immediate _read @ ! - * / < exit echo key pick
				: ] 1 @ 1 - 1 ! _read ]
				: main immediate ]
				main

				: '0' 48 exit
				: nl  10 exit
				: itoa '0' +

				: test immediate
					0         itoa echo
					10 3 -    itoa echo
					21 3 /    itoa echo
					9 2 3 * - itoa echo
					2 2 *     itoa echo
					nl echo
					exit
				test
				`,
		}.run},
	*/

	testCases.run(t)
}

type vmTestCases []vmTestCase

func (vmts vmTestCases) run(t *testing.T) {
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
	name   string
	opts   []interface{}
	setup  []func(t *testing.T, vm *VM)
	ops    []func(vm *VM)
	opErr  error
	expect []func(t *testing.T, vm *VM)

	nextInputID int
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

func (vmt vmTestCase) withMemAt(addr int, values ...int) vmTestCase {
	if len(values) != 0 {
		vmt.opts = append(vmt.opts, optFunc(func(vm *VM) {
			end := addr + len(values)
			if need := end - len(vm.mem); need > 0 {
				vm.mem = append(vm.mem, make([]int, need)...)
			}
			copy(vm.mem[addr:], values)
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

func (vmt vmTestCase) withRetBase(addr int, values ...int) vmTestCase {
	vmt.opts = append(vmt.opts, optFunc(func(vm *VM) {
		vm.stor(10, addr)
	}))
	return vmt.withMemAt(addr, values...).withR(addr + len(values))
}

func (vmt vmTestCase) withMemBase(addr int, values ...int) vmTestCase {
	vmt.opts = append(vmt.opts, optFunc(func(vm *VM) {
		vm.stor(11, addr)
	}))
	return vmt.withMemAt(addr, values...)
}

func (vmt vmTestCase) withMemLimit(limit int) vmTestCase {
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
		assert.Equal(t, values, vm.stack, "expected stack values")
	})
	return vmt
}

func (vmt vmTestCase) expectString(id uint, s string) vmTestCase {
	vmt.expect = append(vmt.expect, func(t *testing.T, vm *VM) {
		assert.Equal(t, s, vm.string(id), "expected string #%v", id)
	})
	return vmt
}

func (vmt vmTestCase) expectMemAt(addr int, values ...int) vmTestCase {
	vmt.expect = append(vmt.expect, func(t *testing.T, vm *VM) {
		end := addr + len(values)
		assert.Equal(t, values, vm.mem[addr:end], "expected memory values @%v", addr)
	})
	return vmt
}

func (vmt vmTestCase) expectH(value int) vmTestCase {
	vmt.expect = append(vmt.expect, func(t *testing.T, vm *VM) {
		assert.Equal(t, value, vm.mem[0], "expected H value")
	})
	return vmt
}

func (vmt vmTestCase) expectR(value int) vmTestCase {
	vmt.expect = append(vmt.expect, func(t *testing.T, vm *VM) {
		assert.Equal(t, value, vm.mem[1], "expected R value")
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
	var out strings.Builder
	vmt.expect = append(vmt.expect, func(t *testing.T, vm *VM) {
		dumpVM(vm, (&lineLogger{Writer: &out}).printf)
		assert.Equal(t, dump, out.String(), "expected dump")
	})
	return vmt
}

func (vmt vmTestCase) withTestLog() vmTestCase {
	vmt.setup = append(vmt.setup, func(t *testing.T, vm *VM) {
		WithLogf(t.Logf).apply(vm)
	})
	return vmt
}

func (vmt vmTestCase) withTestOutput() vmTestCase {
	vmt.setup = append(vmt.setup, func(t *testing.T, vm *VM) {
		WithTee(logWriter{"out: ", t.Logf}).apply(vm)
	})
	return vmt
}

func (vmt vmTestCase) withTestHexOutput() vmTestCase {
	vmt.setup = append(vmt.setup, func(t *testing.T, vm *VM) {
		enc := hex.Dumper(logWriter{"out: ", t.Logf})
		WithTee(enc).apply(vm)
		vm.closers = append(vm.closers, enc)
	})
	return vmt
}

func (vmt vmTestCase) buildOptions(t *testing.T) []VMOption {
	opts := make([]VMOption, 0, len(vmt.opts))
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
	return opts
}

func (vmt vmTestCase) run(t *testing.T) {
	const defaultMemLimit = 4 * 1024

	var vm VM
	vm.apply(vmt.buildOptions(t)...)

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

	defer vm.Close()

	defer func() {
		if t.Failed() {
			t.Logf("VM dump:")
			dumpVM(&vm, t.Logf)
		}
	}()

	withTimeout(context.Background(), time.Second, func(ctx context.Context) {
		if len(vmt.ops) > 0 {
			vmt.runOps(ctx, t, &vm)
		} else {
			require.NoError(t, vm.Run(ctx), "expected no VM error")
		}
	})

	for _, expect := range vmt.expect {
		expect(t, &vm)
	}
}

func (vmt vmTestCase) runOps(ctx context.Context, t *testing.T, vm *VM) {
	names := make([]string, len(vmt.ops))
	for i, op := range vmt.ops {
		names[i] = runtime.FuncForPC(reflect.ValueOf(op).Pointer()).Name()
	}

	if _, paniced, stack := isolate(func() {
		vm.init()
		for i := 0; i < len(vmt.ops); i++ {
			if vmt.ops[i] == nil {
				i--
			}
			t.Logf("do[%v] %v", i, names[i])
			vmt.ops[i](vm)
			if err := ctx.Err(); err != nil {
				panic(err)
			}
		}
	}); paniced != nil {
		opErr := vmt.opErr
		if opErr == nil {
			opErr = errHalt
		}
		panicErr, ok := paniced.(error)
		panicFail := !assert.True(t, ok, "expected a panic error, got %#v instead", paniced)
		panicFail = panicFail || !assert.EqualError(t, panicErr, opErr.Error(), "expected panic error")
		if panicFail {
			t.Logf("Panic stack:\t%s", stack)
		}
	}
}

//// utilities

type vmDumper struct {
	*VM
	printf    func(string, ...interface{})
	addrWidth int
	words     []uint
	wordID    int
}

func dumpVM(VM *VM, printf func(string, ...interface{})) {
	var dump vmDumper
	dump.VM = VM
	dump.printf = printf
	dump.printf("prog: %v", dump.prog)

	dump.scanWords()
	dump.printf("dict: %v", dump.words)

	dump.dumpStack()
	dump.dumpMem()
}

func (vm *vmDumper) dumpStack() {
	vm.printf("stack: %v", vm.stack)
}

func (vm *vmDumper) dumpMem() {
	if vm.addrWidth == 0 {
		vm.addrWidth = 1
		for n := len(vm.mem); n > 0; n /= 10 {
			vm.addrWidth++
		}
	}
	if vm.words == nil {
		vm.scanWords()
	}
	vm.wordID = len(vm.words) - 1
	var sb strings.Builder
	for addr := uint(0); addr < uint(len(vm.mem)); {
		sb.Reset()
		fmt.Fprintf(&sb, "@% *v ", vm.addrWidth, addr)
		n := sb.Len()

		addr = vm.formatMem(&sb, addr)
		if sb.Len() != n {
			vm.printf(sb.String())
		}
	}
}

type fmtBuf interface {
	Len() int
	Write(p []byte) (n int, err error)
	WriteByte(c byte) error
	WriteRune(r rune) (n int, err error)
	WriteString(s string) (n int, err error)
}

func (vm *vmDumper) formatMem(buf fmtBuf, addr uint) uint {
	val := vm.mem[addr]

	// low memory addresses
	if addr <= 11 {
		buf.WriteString(strconv.Itoa(val))
		switch addr {
		case 0:
			buf.WriteString(" dict")
		case 1:
			buf.WriteString(" ret")
		case 10:
			buf.WriteString(" retBase")
		case 11:
			buf.WriteString(" memBase")
		}
		return addr + 1
	}

	// other pre-return-stack addresses
	retBase := uint(vm.mem[10])
	if addr < retBase {
		buf.WriteString(strconv.Itoa(val))
		return addr + 1
	}

	// return stack addresses
	memBase := uint(vm.mem[11])
	if addr < memBase {
		if r := uint(vm.mem[1]); addr < r {
			buf.WriteString(strconv.Itoa(vm.mem[addr]))
			buf.WriteString(" ret_")
			buf.WriteString(strconv.Itoa(int(addr - retBase)))
		}
		return addr + 1
	}

	// dictionary words
	if word := vm.word(); word != 0 && addr == word {
		buf.WriteString(": ")
		addr++

		if name := uint(vm.mem[addr]); name != 0 {
			buf.WriteString(vm.string(name))
		} else {
			buf.WriteRune('ø')
		}
		addr++

		switch code := uint(vm.mem[addr]); code {
		case vmCodeCompile, vmCodeCompIt:
			addr++
		default:
			buf.WriteByte(' ')
			buf.WriteString("immediate")
		}

		nextWord := vm.nextWord()
		if nextWord == 0 {
			nextWord = uint(vm.mem[0])
		}
		for addr < nextWord {
			buf.WriteByte(' ')
			if nextAddr := vm.formatCode(buf, addr); nextAddr > addr {
				addr = nextAddr
				continue
			}
			break
		}
		return addr
	}

	// other memory ranges
	if val != 0 {
		buf.WriteString(strconv.Itoa(val))
	}

	return addr + 1
}

func (vm *vmDumper) formatCode(buf fmtBuf, addr uint) uint {
	code := vm.mem[addr]
	addr++

	// builtin code
	if code < vmCodeMax {
		buf.WriteString(vmCodeNames[code])
		if code == vmCodePushint {
			buf.WriteByte('(')
			buf.WriteString(strconv.Itoa(vm.mem[addr]))
			buf.WriteByte(')')
			addr++
		}
		return addr
	}

	// call to word+offset
	if i := sort.Search(len(vm.words), func(i int) bool {
		return vm.words[i] < uint(code)
	}); i < len(vm.words) {
		word := vm.words[i]
		if name := uint(vm.mem[word]); name != 0 {
			buf.WriteString(vm.string(name))
		} else {
			buf.WriteRune('ø')
		}
		if offset := uint(code) - word; offset > 0 {
			buf.WriteByte('+')
			buf.WriteString(strconv.Itoa(int(offset)))
		}
		return addr
	}

	// call to unknown address
	buf.WriteString(strconv.Itoa(code))
	return addr
}

func (vm *vmDumper) scanWords() {
	for word := vm.last; word != 0; {
		if word >= uint(len(vm.mem)) {
			return
		}
		vm.words = append(vm.words, word)
		word = uint(vm.mem[word])
	}
}

func (vm *vmDumper) word() uint {
	if vm.wordID >= 0 {
		return vm.words[vm.wordID]
	}
	return 0
}

func (vm *vmDumper) nextWord() uint {
	if vm.wordID >= 0 {
		vm.wordID--
	}
	return vm.word()
}

func lines(parts ...string) string {
	return strings.Join(parts, "\n")
}

func withTimeout(ctx context.Context, timeout time.Duration, f func(ctx context.Context)) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	f(ctx)
}

type logWriter struct {
	prefix string
	printf func(string, ...interface{})
}

func (lw logWriter) Write(p []byte) (n int, err error) {
	lw.printf("%s%s", lw.prefix, p)
	return len(p), nil
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

func Test_isolate(t *testing.T) {
	t.Run("normal", func(t *testing.T) {
		exited, paniced, _ := isolate(func() {})
		assert.False(t, exited, "expected to not exit")
		assert.Nil(t, paniced, "expected to not panic")
	})
	t.Run("hello panic", func(t *testing.T) {
		exited, paniced, stack := isolate(func() { panic("hello") })
		assert.False(t, exited, "expected to not exit")
		assert.Equal(t, "hello", paniced, "expected to panic")
		assert.NotEqual(t, "", stack, "expected panic stack")
	})
	t.Run("exit", func(t *testing.T) {
		exited, paniced, stack := isolate(func() { runtime.Goexit() })
		assert.True(t, exited, "expected to exit")
		assert.Nil(t, paniced, "expected to not panic")
		assert.Equal(t, "", stack, "expected no stack")
	})
	t.Run("index panic", func(t *testing.T) {
		exited, paniced, stack := isolate(func() {
			var some []int
			some[1]++
		})
		assert.False(t, exited, "expected to not exit")
		assert.NotNil(t, paniced, "expected to panic")
		assert.NotEqual(t, "", stack, "expected panic stack")
	})
}

func isolate(f func()) (exited bool, panicValue interface{}, panicStack string) {
	type recovered struct {
		value interface{}
		stack string
	}
	wait := make(chan recovered)

	go func() {
		defer close(wait)
		defer func() {
			if message := recover(); message != nil {
				wait <- recovered{message, string(debug.Stack())}
			}
		}()
		f()
		wait <- recovered{}
	}()

	paniced, ok := <-wait
	return !ok, paniced.value, paniced.stack
}
