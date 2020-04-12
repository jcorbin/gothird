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

func (vm *VM) load(addr uint) int {
	if addr >= uint(len(vm.mem)) {
		vm.halt(loadError(addr))
	}
	vm.logf("load %v <- @%v", vm.mem[addr], addr)
	return vm.mem[addr]
}

func (vm *VM) stor(addr uint, val int) {
	if addr >= uint(len(vm.mem)) {
		vm.halt(storError(addr))
	}
	vm.mem[addr] = val
	vm.logf("stor %v -> @%v", val, addr)
}

func (vm *VM) push(val int) {
	vm.stack = append(vm.stack, val)
}

func (vm *VM) pop() (val int) {
	i := len(vm.stack) - 1
	val, vm.stack = vm.stack[i], vm.stack[:i]
	return val
}

func (vm *VM) loadProg() int {
	if vm.prog < vm.memBase {
		vm.halt(progError(vm.prog))
	}
	val := vm.load(vm.prog)
	vm.prog++
	return val
}

func (vm *VM) compile(val int) {
	addr := uint(vm.load(0))
	end := addr + 1
	if end >= uint(len(vm.mem)) {
		vm.halt(errOOM)
	}
	vm.stor(0, int(end))
	vm.stor(addr, val)
}

func (vm *VM) compileHeader(name uint) {
	addr := uint(vm.load(0))
	prev := vm.last
	vm.compile(int(prev))
	vm.compile(int(name))
	vm.compile(vmCodeCompile) // compile time code
	vm.compile(vmCodeRun)     // run time code
	vm.last = addr
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

func (vm *VM) step() {
	// TODO unsafe direct threading rather than token threading
	for {
		if code := vm.loadProg(); code < len(vmCodeTable) {
			vm.logf("step @%v %v -- %v", vm.prog-1, vmCodeNames[code], vm.stack)
			vmCodeTable[code](vm)
		} else {
			addr := uint(code)
			vm.logf("step @%v call %v -- %v", vm.prog-1, addr, vm.stack)
			vm.call(addr)
		}
	}
}

func (vm *VM) run() {
	vm.stor(0, int(vm.memBase))
	vm.stor(1, int(vm.retBase))
	vm.compileBuiltins()
	vm.compileMain()
	vm.logf("run main")
	vm.step()
}

var (
	errOOM          = errors.New("out of memory")
	errRetOverflow  = errors.New("return stack overflow")
	errRetUnderflow = errors.New("return stack underflow")
)

type progError uint
type loadError uint
type storError uint
type codeError uint

func (addr progError) Error() string { return fmt.Sprintf("program smashed %v", uint(addr)) }
func (addr loadError) Error() string { return fmt.Sprintf("invalid load at %v", uint(addr)) }
func (addr storError) Error() string { return fmt.Sprintf("invalid store at %v", uint(addr)) }
func (code codeError) Error() string { return fmt.Sprintf("invalid code %v", uint(code)) }

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
