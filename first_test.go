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
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_VM(t *testing.T) {
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
		less      = (*VM).less
		exit      = (*VM).exit
		echo      = (*VM).echo
		key       = (*VM).key
		pick      = (*VM).pick
		pushint   = (*VM).pushint
		step      = (*VM).step
	)

	vmTestCases{
		// binary integer operation on the stack
		vmTest("sub").withStack(5, 3, 1).do(sub).expectStack(5, 2),
		vmTest("div").withStack(7, 13, 3).do(div).expectStack(7, 4),
		vmTest("mul").withStack(11, 5, 6).do(mul).expectStack(11, 30),

		// is top of stack less than 0?
		vmTest("less true").withStack(2, -3).do(less).expectStack(2, 1),
		vmTest("less false").withStack(2, 3).do(less).expectStack(2, 0),

		// pop top of stack, use as index into stack and copy up that element
		vmTest("pick 0").withStack(1, 2, 3, 4, 5, 0).do(pick).expectStack(1, 2, 3, 4, 5, 5),
		vmTest("pick 1").withStack(1, 2, 3, 4, 5, 1).do(pick).expectStack(1, 2, 3, 4, 5, 4),
		vmTest("pick 2").withStack(1, 2, 3, 4, 5, 2).do(pick).expectStack(1, 2, 3, 4, 5, 3),
		vmTest("pick 3").withStack(1, 2, 3, 4, 5, 3).do(pick).expectStack(1, 2, 3, 4, 5, 2),
		vmTest("pick 4").withStack(1, 2, 3, 4, 5, 4).do(pick).expectStack(1, 2, 3, 4, 5, 1),
		vmTest("pick 5").withStack(1, 2, 3, 4, 5, 5).do(pick).expectStack(1, 2, 3, 4, 5, 0),

		// read from memory
		vmTest("get").withMem(99, 42, 108).withStack(1).do(get).expectStack(42),

		// write to memory
		vmTest("set").withMem(0, 0, 0).withStack(108, 1).do(set).expectMem(0, 108, 0),

		// push an immediate value onto the stack
		vmTest("pushint").withMem(99, 42, 108).withProg(1).do(pushint).expectStack(42).expectProg(2),

		// compile the program counter
		vmTest("compileme: sub exit").withMemAt(0,
			39, // 0: h
			0,  // 1: r
		).withMemAt(32,
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
		).withProg(35).do(compileme).expectMemAt(0,
			40, // 0: h
			0,  // 1: r
		).expectMemAt(32,
			0,             // 32: word prev
			vmCodeSub,     // 33: ... name
			vmCodeCompile, // 34: ...
			vmCodeRun,     // 35: ...
			vmCodeSub,     // 36: ...       <-- prog
			vmCodeExit,    // 37: ...
			0,             // 38:
			35,            // 39:
			0,             // 40:           <-- h
			0,             // 41:
		).expectProg(35),

		// compile the header of a definition
		vmTest("define").withMem(
			32, // 0: h
			0,  // 1: r
		).withMemAt(32,
			0, // 32: <-- h
			0, // 33:
			0, // 34:
			0, // 35:
			0, // 36:
			0, // 37:
			0, // 38:
		).withInput(`
			:
		`).do(define).expectMemAt(0,
			36, // 0: h
			0,  // 1: r
		).expectMemAt(32,
			0,             // 32: word prev
			1,             // 33: word name
			vmCodeCompile, // 34: ...
			vmCodeRun,     // 35: ...
			0,             // 36: <-- h
			0,             // 37:
			0,             // 38:
		).expectString(1, ":").expectLast(32),

		// modify the header to create an immediate word
		vmTest("immediate").withMem(
			32, // 0: h
			0,  // 1: r
		).withMemAt(32,
			0, // 32: <-- h
			0, // 33:
			0, // 34:
			0, // 35:
			0, // 36:
			0, // 37:
			0, // 38:
		).withInput(`:`).do(define, immediate).expectMemAt(0,
			35, // 0: h
			0,  // 1: r
		).expectMemAt(32,
			0,         // 32: word prev
			1,         // 33: ... name
			vmCodeRun, // 34: ...
			vmCodeRun, // 35: ... <-- h
			0,         // 36:
			0,         // 37:
		).expectString(1, ":"),

		// stop running the current function
		vmTest("exit").withOptions(
			WithRetBase(4),
			WithMemBase(8),
		).withMem(
			0,          // 0: h
			7,          // 1: r
			0,          // 2:
			0,          // 3:
			8,          // 4: ret[0]
			9,          // 5: ret[1]
			10,         // 6: ret[2]
			11,         // 7: ret[3]
			vmCodeExit, // 8:
			vmCodeExit, // 9:
			vmCodeExit, // 10:
			vmCodeExit, // 11:
		).do(exit, step).expectMem(
			0,          // 0: h
			4,          // 1: r
			0,          // 2:
			0,          // 3:
			8,          // 4: ret[0]
			9,          // 5: ret[1]
			10,         // 6: ret[2]
			11,         // 7: ret[3]
			vmCodeExit, // 8:
			vmCodeExit, // 9:
			vmCodeExit, // 10:
			vmCodeExit, // 11:
		),

		// read a word from input and compile a pointer to it
		vmTest("read").withOptions(
			WithRetBase(16),
			WithMemBase(32),
		).withMem(
			41, // 0: h
			16, // 1: r
		).withString(1, "foo").withString(2, "bar").withMemAt(32,
			0,             // 32:
			1,             // 33:
			vmCodeCompile, // 34:
			vmCodeRun,     // 35:
			vmCodeSub,     // 36:
			vmCodeExit,    // 37:
			32,            // 38:
			2,             // 39:
			vmCodeRun,     // 40:
			vmCodeRun,     // 41:  <-- h
			0,             // 42:
			0,             // 43:
			0,             // 44:
		).withProg(37).withLast(38).withInput("foo").do(read, step).expectMemAt(0,
			42, // 0: h
			16, // 1: r
		).expectMemAt(32,
			0,             // 32:
			1,             // 33:
			vmCodeCompile, // 34:
			vmCodeRun,     // 35:
			vmCodeSub,     // 36:
			vmCodeExit,    // 37:
			32,            // 38:
			2,             // 39:
			vmCodeRun,     // 40:
			35,            // 41:
			0,             // 42:  <-- h
			0,             // 43:
			0,             // 44:
		),

		// output one character
		// input one character
		vmTest("key^2 => echo^2").withInput("ab").do(key, key, echo, echo).expectOutput("ba"),

		vmTest("builtin setup").withOptions(
			WithRetBase(16),
			WithMemBase(32),
			WithMemorySize(256),
		).withInput(`
			: immediate _read @ ! - * / < exit echo key pick
		`).expectDump(lines(
			`prog: 112`,
			`dict: [107 101 95 89 84 78 72 66 60 54 48 42 37 32]`,
			`stack: []`,
			`@   0 113 dict`,
			`@   1 16 ret`,
			`@  32 : : immediate 14 1 10`,
			`@  37 : immediate immediate 14 2 10`,
			`@  42 : _read 14 3 10`,
			`@  48 : @ 14 4 10`,
			`@  54 : ! 14 5 10`,
			`@  60 : - 14 6 10`,
			`@  66 : * 14 7 10`,
			`@  72 : / 14 8 10`,
			`@  78 : < 14 9 10`,
			`@  84 : exit 14 10`,
			`@  89 : echo 14 11 10`,
			`@  95 : key 14 12 10`,
			`@ 101 : pick 14 13 10`,
			`@ 107 : _main 14 3 110`,
		)),

		/*

			{"new main", vmTestScript{
				retBase: 16,
				memBase: 32,
				memSize: 256,
				program: lines(
					`: immediate _read @ ! - * / < exit echo key pick`,
					`: ] 1 @ 1 - 1 ! _read ]`,
					`: main immediate ]`,
					`main`,
				),
				dump: lines(
					`prog: 47`,
					`dict: [128 113 107 101 95 89 84 78 72 66 60 54 48 42 37 32]`,
					`stack: []`,
					`@   0 132 dict`,
					`@   1 31 ret`,
					`@  16 113 ret_0`,
					`@  17 113 ret_1`,
					`@  18 113 ret_2`,
					`@  19 113 ret_3`,
					`@  20 113 ret_4`,
					`@  21 113 ret_5`,
					`@  22 113 ret_6`,
					`@  23 113 ret_7`,
					`@  24 113 ret_8`,
					`@  25 113 ret_9`,
					`@  26 113 ret_10`,
					`@  27 113 ret_11`,
					`@  28 112 ret_12`,
					`@  29 132 ret_13`,
					`@  30 127 ret_14`,
					`@  32 : : immediate 14 1 10`,
					`@  37 : immediate immediate 14 2 10`,
					`@  42 : _read 14 3 10`,
					`@  48 : @ 14 4 10`,
					`@  54 : ! 14 5 10`,
					`@  60 : - 14 6 10`,
					`@  66 : * 14 7 10`,
					`@  72 : / 14 8 10`,
					`@  78 : < 14 9 10`,
					`@  84 : exit 14 10`,
					`@  89 : echo 14 11 10`,
					`@  95 : key 14 12 10`,
					`@ 101 : pick 14 13 10`,
					`@ 107 : _main 14 3 110`,
					`@ 113 : ] 14 15 1 51 15 1 63 15 1 57 45 116`,
					`@ 128 : main immediate 14 116`,
				),
			}.run},

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
	}.run(t)
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
	opts   []VMOption
	setup  []func(t *testing.T, vm *VM)
	ops    []func(vm *VM)
	expect []func(t *testing.T, vm *VM)
}

func (vmt vmTestCase) withOptions(opts ...VMOption) vmTestCase {
	vmt.opts = append(vmt.opts, opts...)
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

func (vmt vmTestCase) withMem(values ...int) vmTestCase {
	vmt.opts = append(vmt.opts, optFunc(func(vm *VM) {
		vm.mem = append(vm.mem, values...)
	}))
	return vmt
}

func (vmt vmTestCase) withMemAt(addr int, values ...int) vmTestCase {
	vmt.opts = append(vmt.opts, optFunc(func(vm *VM) {
		end := addr + len(values)
		if need := end - len(vm.mem); need > 0 {
			vm.mem = append(vm.mem, make([]int, need)...)
		}
		copy(vm.mem[addr:], values)
	}))
	return vmt
}

func (vmt vmTestCase) withInput(source string) vmTestCase {
	vmt.opts = append(vmt.opts, WithInput(strings.NewReader(source)))
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

func (vmt vmTestCase) expectMem(values ...int) vmTestCase {
	vmt.expect = append(vmt.expect, func(t *testing.T, vm *VM) {
		assert.Equal(t, values, vm.mem, "expected memory values")
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

func (vmt vmTestCase) run(t *testing.T) {
	var vm VM

	for _, opt := range vmt.opts {
		opt.apply(&vm)
	}
	for _, setup := range vmt.setup {
		setup(t, &vm)
	}
	if vm.in == nil {
		vm.in = strings.NewReader("")
	}
	if vm.out == nil {
		vm.out = newWriteFlusher(ioutil.Discard)
	}

	defer vm.Close()

	defer func() {
		if t.Failed() {
			t.Logf("VM dump:")
			dumpVM(&vm, t.Logf)
		}
	}()

	if len(vmt.ops) > 0 {
		_, paniced, stack := isolate(func() {
			for i, op := range vmt.ops {
				t.Logf("do[%v] %v", i, runtime.FuncForPC(reflect.ValueOf(op).Pointer()).Name())
				op(&vm)
			}
		})
		if paniced != nil {
			require.Fail(t, "unexpected VM panic", "Panic value: %v\n\tPanic stack:\t%s", paniced, stack)
		}
	} else {
		withTimeout(context.Background(), time.Second, func(ctx context.Context) {
			require.NoError(t, vm.Run(ctx), "expected no VM error")
		})
	}
	for _, expect := range vmt.expect {
		expect(t, &vm)
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
	for addr := uint(0); addr < uint(len(vm.mem)); {
		next, desc := vm.describe(addr)
		if desc != "" {
			vm.addrf(addr, desc)
		}
		addr = next
	}
}

func (vm *vmDumper) describe(addr uint) (next uint, desc string) {
	for _, fn := range []func(addr uint) (next uint, desc string){
		vm.describeLow,
		vm.describeRet,
		vm.describeWord,
		vm.describeMem,
	} {
		if next, desc := fn(addr); next > addr {
			return next, desc
		}
	}
	return addr, ""
}

func (vm *vmDumper) describeLow(addr uint) (uint, string) {
	var sb strings.Builder
	sb.Grow(32)
	val := vm.mem[addr]
	sb.WriteString(strconv.Itoa(val))
	switch addr {
	case 0:
		sb.WriteString(" dict")
	case 1:
		sb.WriteString(" ret")
	default:
		if rb := vm.retBase; rb != 0 && addr >= rb {
			return addr, ""
		}
	}
	return addr + 1, sb.String()
}

func (vm *vmDumper) describeRet(addr uint) (uint, string) {
	if addr >= vm.memBase {
		return addr, ""
	}
	var sb strings.Builder
	sb.Grow(32)
	sb.WriteString(strconv.Itoa(vm.mem[addr]))
	sb.WriteString(" ret_")
	sb.WriteString(strconv.Itoa(int(addr - vm.retBase)))
	if addr >= uint(vm.mem[1]) {
		return addr + 1, ""
	}
	return addr + 1, sb.String()
}

func (vm *vmDumper) describeWord(addr uint) (uint, string) {
	if word := vm.word(); word == 0 || addr != word {
		return addr, ""
	}

	var sb strings.Builder
	sb.WriteString(": ")
	addr++

	sb.WriteString(vm.string(uint(vm.mem[addr])))
	addr++

	if code := uint(vm.mem[addr]); code != vmCodeCompile {
		sb.WriteString(" immediate")
	} else {
		addr++
	}

	next := vm.nextWord()
	if next == 0 {
		next = uint(vm.mem[0])
	}
	for ; addr < next; addr++ {
		sb.WriteByte(' ')
		sb.WriteString(strconv.Itoa(vm.mem[addr]))
	}

	return addr, sb.String()
}

func (vm *vmDumper) describeMem(addr uint) (next uint, desc string) {
	if val := vm.mem[addr]; val != 0 {
		desc = fmt.Sprintf("%v", val)
	}
	return addr + 1, desc
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

func (vm *vmDumper) addrf(addr uint, desc string, args ...interface{}) {
	if len(args) > 0 {
		desc = fmt.Sprintf(desc, args...)
	}
	vm.printf("@% *v %v", vm.addrWidth, addr, desc)
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
