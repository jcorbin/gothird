package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode"
)

func (vm *VM) halt(err error) {
	if ferr := vm.out.Flush(); err == nil {
		err = ferr
	}
	switch err {
	case nil:
		vm.logf("halt")
		panic(errHalt)
	case io.EOF:
		vm.logf("halt EOF")
		panic(errHalt)
	default:
		vm.logf("halt error: %v", err)
		panic(err)
	}
}

func (vm *VM) haltif(err error) {
	if err != nil {
		vm.halt(err)
	}
}

func (vm *VM) grow(size int) {
	const chunkSize = 256
	size = (size + chunkSize - 1) / chunkSize * chunkSize
	if need := size - len(vm.mem); need > 0 {
		if maxSize := vm.memLimit; maxSize != 0 && size > maxSize {
			vm.halt(errOOM)
		}
		vm.mem = append(vm.mem, make([]int, need)...)
	}
}

func (vm *VM) load(addr uint) int {
	if maxSize := vm.memLimit; maxSize != 0 && int(addr) > maxSize {
		vm.halt(errOOM)
	} else if addr >= uint(len(vm.mem)) {
		return 0
	}
	// vm.logf("load %v <- @%v", vm.mem[addr], addr)
	return vm.mem[addr]
}

func (vm *VM) stor(addr uint, val int) {
	if addr >= uint(len(vm.mem)) {
		vm.grow(int(addr) + 1)
	}
	vm.mem[addr] = val
	// vm.logf("stor %v -> @%v", val, addr)
}

func (vm *VM) loadProg() int {
	if memBase := uint(vm.load(11)); vm.prog < memBase {
		vm.halt(progError(vm.prog))
	}
	val := vm.load(vm.prog)
	vm.prog++
	return val
}

func (vm *VM) push(val int) {
	vm.stack = append(vm.stack, val)
}

func (vm *VM) pop() (val int) {
	i := len(vm.stack) - 1
	val, vm.stack = vm.stack[i], vm.stack[:i]
	return val
}

func (vm *VM) pushr(addr uint) error {
	r := uint(vm.load(1))
	if retBase := uint(vm.load(10)); r < retBase {
		return errRetUnderflow
	}
	if memBase := uint(vm.load(11)); r >= memBase {
		return errRetOverflow
	}
	vm.stor(r, int(addr))
	vm.stor(1, int(r+1))
	return nil
}

func (vm *VM) popr() (uint, error) {
	r := uint(vm.load(1))
	if retBase := uint(vm.load(10)); r == retBase {
		return 0, errHalt
	} else if r < retBase {
		return 0, errRetUnderflow
	}
	if memBase := uint(vm.load(11)); r > memBase {
		return 0, errRetOverflow
	}
	r--
	vm.stor(1, int(r))
	return uint(vm.load(r)), nil
}

func (vm *VM) compile(val int) {
	h := uint(vm.load(0))
	end := h + 1
	vm.stor(0, int(end))
	vm.stor(h, val)
}

func (vm *VM) compileHeader(name uint) {
	h := uint(vm.load(0))
	prev := vm.last
	vm.compile(int(prev))
	vm.compile(int(name))
	vm.compile(vmCodeCompile) // compile time code
	vm.compile(vmCodeRun)     // run time code
	vm.last = h
}

func (vm *VM) lookup(token string) uint {
	if name := vm.symbol(token); name != 0 {
		for prev := vm.last; prev != 0; {
			if sym := uint(vm.load(prev + 1)); sym == name {
				return prev
			}
			prev = uint(vm.load(prev))
		}
	}
	return 0
}

func (vm *VM) literal(token string) (int, error) {
	n, err := strconv.ParseInt(token, 10, strconv.IntSize)
	if err == nil {
		return int(n), nil
	}
	return 0, err
}

func (vm *VM) exec(ctx context.Context) {
	if vm.logfn != nil {
		defer vm.withLogPrefix("	")()
	}

	for {
		vm.step()
		vm.haltif(ctx.Err())
	}
}

func (vm *VM) step() {
	at := vm.prog
	if code := vm.loadProg(); code < len(vmCodeTable) {
		if vm.logfn != nil {
			rstack := vm.mem[vm.load(10):vm.load(1)]
			vm.logf("exec @%v %v -- r:%v s:%v", at, vmCodeNames[code], rstack, vm.stack)
		}
		vmCodeTable[code](vm)
	} else {
		vm.logf("exec @%v call %v", at, code)
		vm.call(uint(code))
	}
}

func (vm *VM) init() {
	const (
		defaultRetBase = 16
		defaultMemBase = 32
	)

	retBase := uint(vm.load(10))
	if retBase == 0 {
		retBase = defaultRetBase
		vm.stor(10, int(retBase))
	}

	memBase := uint(vm.load(11))
	if memBase == 0 {
		memBase = defaultMemBase
		vm.stor(11, int(memBase))
	}

	if h := uint(vm.load(0)); h == 0 {
		vm.stor(0, int(memBase))
	}

	if r := uint(vm.load(1)); r == 0 {
		vm.stor(1, int(retBase))
	} else if r < retBase {
		vm.halt(errRetUnderflow)
	} else if r > memBase {
		vm.halt(errRetOverflow)
	}
}

func (vm *VM) run(ctx context.Context) {
	vm.init()

	// clear program counter and compile builtins
	vm.prog = 0
	entry := vm.compileEntry()
	vm.compileBuiltins()

	// run the entry point
	vm.prog = entry
	vm.exec(ctx)
}

func (vm *VM) scan() (token string) {
	defer func() {
		line := vm.scanLine
		if line.Len() == 0 {
			line = vm.lastLine
		}
		vm.logf("scan %q from %v", token, line)
	}()

	var sb strings.Builder
	for {
		r, err := vm.readRune()
		vm.haltif(err)
		if !unicode.IsControl(r) && !unicode.IsSpace(r) {
			sb.WriteRune(r)
			break
		}
	}
	for {
		r, err := vm.readRune()
		if err == io.EOF {
			break
		} else if err != nil {
			vm.halt(err)
		} else if unicode.IsControl(r) || unicode.IsSpace(r) {
			break
		} else {
			sb.WriteRune(r)
		}
	}
	return sb.String()
}

var (
	errHalt         = errors.New("normal halt")
	errOOM          = errors.New("out of memory")
	errRetOverflow  = errors.New("return stack overflow")
	errRetUnderflow = errors.New("return stack underflow")
)

type progError uint
type storError uint
type codeError uint

func (addr progError) Error() string { return fmt.Sprintf("program smashed %v", uint(addr)) }
func (addr storError) Error() string { return fmt.Sprintf("invalid store at %v", uint(addr)) }
func (code codeError) Error() string { return fmt.Sprintf("invalid code %v", uint(code)) }

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
