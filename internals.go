package main

import (
	"errors"
	"fmt"
	"io"
	"runtime"
)

func (vm *VM) halt(err error) {
	if ferr := vm.out.Flush(); err == nil {
		err = ferr
	}
	switch err {
	case nil:
		vm.logf("halt")
		runtime.Goexit()
	case io.EOF:
		vm.logf("halt eof")
		runtime.Goexit()
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

func (vm *VM) pushProg(r, addr uint) uint {
	vm.stor(r, int(vm.prog))
	vm.logf("prog <- %v (call from %v)", addr, vm.prog)
	vm.prog = addr
	return r + 1
}

func (vm *VM) push(val int) {
	vm.stack = append(vm.stack, val)
}

func (vm *VM) pop() (val int) {
	i := len(vm.stack) - 1
	val, vm.stack = vm.stack[i], vm.stack[:i]
	return val
}

func (vm *VM) here() uint {
	h := uint(vm.load(0))
	if h == 0 {
		const defaultMemBase = 32
		if len(vm.mem) < 12 {
			vm.grow(defaultMemBase)
		}
		memBase := uint(vm.load(11))
		if memBase == 0 {
			memBase = defaultMemBase
			vm.stor(11, int(memBase))
		}
		h = memBase
		vm.stor(0, int(h))
	}
	return h
}

func (vm *VM) compile(val int) {
	h := vm.here()
	end := h + 1
	vm.stor(0, int(end))
	vm.stor(h, val)
}

func (vm *VM) compileHeader(name uint) {
	h := vm.here()
	prev := vm.last
	vm.compile(int(prev))
	vm.compile(int(name))
	vm.compile(vmCodeCompile) // compile time code
	vm.compile(vmCodeRun)     // run time code
	vm.last = h
}

func (vm *VM) lookup(name uint) (addr uint) {
	for addr := vm.last; addr != 0; {
		if sym := uint(vm.load(addr + 1)); sym == name {
			return addr + 2
		}
		addr = uint(vm.load(addr))
	}
	return 0
}

func (vm *VM) exec() {
	if vm.compiling {
		goto compileit
	}

runit:
	for {
		if code := vm.loadProg(); code == vmCodeCompile {
			vm.logf("exec -> compileit")
			vm.compiling = true
			goto compileit
		} else if code < len(vmCodeTable) {
			vm.logf("step @%v %v -- %v", vm.prog-1, vmCodeNames[code], vm.stack)
			if done := vmCodeTable[code](vm); done {
				return
			}
		} else {
			vm.call(uint(code))
		}
	}

compileit:
	for {
		vm.logf("compileit @%v", vm.prog)
		switch code := vm.loadProg(); code {
		case vmCodeRun:
			vm.logf("exec done (compiled call)")
			vm.compile(int(vm.prog))
			return
		case vmCodeExit:
			vm.logf("exec -> runit (exit)")
			vm.compiling = false
			goto runit
		case vmCodePushint:
			vm.compile(code)
			vm.compile(vm.loadProg())
		default:
			vm.compile(code)
		}
	}
}

func (vm *VM) run() {
	entry := vm.compileEntry()
	vm.compileBuiltins()

	vm.compiling = false
	vm.prog = entry
	vm.exec()
}

var (
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
