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
)

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
	ops     []func(vm *VM)
	expect  []func(t *testing.T, vm *VM)
	timeout time.Duration
	wantErr error

	exclusive   bool
	nextInputID int
}

func (vmt vmTestCase) apply(wraps ...func(vmTestCase) vmTestCase) vmTestCase {
	for _, wrap := range wraps {
		vmt = wrap(vmt)
	}
	return vmt
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

func (vmt vmTestCase) withInput(input string) vmTestCase {
	vmt.opts = append(vmt.opts, func(vmt *vmTestCase, t *testing.T) VMOption {
		name := t.Name() + "/input"
		if id := vmt.nextInputID; id > 0 {
			name += "_" + strconv.Itoa(id+1)
		}
		vmt.nextInputID++
		return WithInput(NamedReader(name, strings.NewReader(input)))
	})
	return vmt
}

func (vmt vmTestCase) withNamedInput(name string, input string) vmTestCase {
	vmt.opts = append(vmt.opts, func(vmt *vmTestCase, t *testing.T) VMOption {
		return WithInput(NamedReader(name, strings.NewReader(input)))
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
	vmt.opts = append(vmt.opts, func(vmt *vmTestCase, t *testing.T) VMOption {
		lw := &logWriter{logf: func(mess string, args ...interface{}) {
			t.Logf("out: "+mess, args...)
		}}
		return WithTee(lw)
	})
	return vmt
}

func (vmt vmTestCase) withTestHexOutput() vmTestCase {
	vmt.opts = append(vmt.opts, func(vmt *vmTestCase, t *testing.T) VMOption {
		lw := &logWriter{logf: func(mess string, args ...interface{}) {
			t.Logf("out: "+mess, args...)
		}}
		enc := hex.Dumper(lw)
		w := writeCloser{enc, closerChain{enc, lw}}
		return WithTee(w)
	})
	return vmt
}

func (vmt vmTestCase) run(t *testing.T) {
	defer func(then time.Time) {
		label := "PASS"
		if t.Failed() {
			label = "FAIL"
		}
		t.Logf("%v\t%v\t%v", label, t.Name(), time.Now().Sub(then))
	}(time.Now())

	if testFails(func(t *testing.T) {
		vmt.runVMTest(context.Background(), t, vmt.buildVM(t))
	}) {
		vm := vmt.buildVM(t)
		WithLogf(t.Logf).apply(vm)
		vmt.runVMTest(context.Background(), t, vm)
	}
}

func (vmt vmTestCase) runVMTest(ctx context.Context, t *testing.T, vm *VM) {
	const defaultTimeout = time.Second
	timeout := vmt.timeout
	if timeout == 0 {
		timeout = defaultTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	defer func() {
		if t.Failed() {
			vmt.dumpToTest(t, vm)
		}
	}()

	var halted vmHaltError
	if err := vmt.runVM(ctx, vm); vmt.wantErr != nil {
		assert.True(t, errors.Is(err, vmt.wantErr), "expected error: %v\ngot: %+v", vmt.wantErr, err)
	} else if errors.As(err, &halted) {
		assert.NoError(t, halted.error, "unexpected abnormal VM halt")
	} else {
		assert.NoError(t, err, "unexpected VM run error")
	}

	if !t.Failed() {
		for _, expect := range vmt.expect {
			expect(t, vm)
		}
	}
}

func (vmt vmTestCase) runVM(ctx context.Context, vm *VM) (rerr error) {
	defer func() {
		if err := vm.Close(); err != nil && rerr == nil {
			rerr = fmt.Errorf("vm.Close failed: %w", err)
		}
	}()

	if len(vmt.ops) == 0 {
		return vm.Run(ctx)
	}

	names := make([]string, len(vmt.ops))
	for i, op := range vmt.ops {
		names[i] = runtime.FuncForPC(reflect.ValueOf(op).Pointer()).Name()
	}
	return isolate("vmTestCase.ops", func() error {
		vm.init()
		for i := 0; i < len(vmt.ops); i++ {
			if vmt.ops[i] == nil {
				i--
			}
			vm.logf(">", "do[%v] %v", i, names[i])
			vmt.ops[i](vm)
			if err := ctx.Err(); err != nil {
				return err
			}
		}
		return nil
	})
}

func (vmt vmTestCase) buildVM(t *testing.T) *VM {
	const defaultMemLimit = 4 * 1024

	var vm VM
	vm.memLimit = defaultMemLimit

	var opt VMOption
	for _, o := range vmt.opts {
		switch impl := o.(type) {
		case func(vmt *vmTestCase, t *testing.T) VMOption:
			opt = VMOptions(opt, impl(&vmt, t))
		case VMOption:
			opt = VMOptions(opt, impl)
		default:
			t.Logf("unsupported vmTestCase opt type %T", o)
			t.FailNow()
		}
	}
	opt.apply(&vm)

	if vm.in == nil {
		vm.in = strings.NewReader("")
	}
	if vm.out == nil {
		vm.out = newWriteFlusher(ioutil.Discard)
	}

	return &vm
}

func (vmt vmTestCase) dumpToTest(t *testing.T, vm *VM) {
	lw := logWriter{logf: t.Logf}
	defer lw.Close()
	vmDumper{vm: vm, out: &lw}.dump()
}

//// utilities

func testFails(fn func(t *testing.T)) bool {
	var fakeT testing.T
	done := make(chan struct{})
	go func() {
		defer close(done)
		fn(&fakeT)
	}()
	<-done
	return fakeT.Failed()
}

type writeCloser struct {
	io.Writer
	io.Closer
}

type closerChain []io.Closer

func (cc closerChain) Close() (rerr error) {
	for _, cl := range cc {
		if cerr := cl.Close(); rerr == nil {
			rerr = cerr
		}
	}
	return rerr
}

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
