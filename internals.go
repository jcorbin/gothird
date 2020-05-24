package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode"

	"github.com/jcorbin/gothird/internal/runeio"
)

func (vm *VM) halt(err error) {
	// ignore any panics while trying to flush output
	func() {
		defer func() { recover() }()
		if vm.out != nil {
			if ferr := vm.out.Flush(); err == nil {
				err = ferr
			}
		}
	}()

	// ignore any panics while loggging
	func() {
		defer func() { recover() }()
		vm.logf("#", "halt error: %v", err)
	}()

	panic(vmHaltError{err})
}

func (vm *VM) load(addr uint) int {
	val, err := vm.mem.Load(addr)
	if err != nil {
		vm.halt(err)
	}
	return val
}

func (vm *VM) loadInto(addr uint, buf []int) {
	if err := vm.mem.LoadInto(addr, buf); err != nil {
		vm.halt(err)
	}
}

func (vm *VM) stor(addr uint, values ...int) {
	if err := vm.mem.Stor(addr, values...); err != nil {
		vm.halt(err)
	}
}

func (vm *VM) loadProg() int {
	// FIXME conflicts with low tmp space needed by third's execute
	// if memBase := uint(vm.load(11)); vm.prog < memBase {
	// 	vm.halt(progError(vm.prog))
	// }
	val := vm.load(vm.prog)
	vm.prog++
	return val
}

func (vm *VM) push(val int) {
	vm.stack = append(vm.stack, val)
}

func (vm *VM) pop() (val int) {
	i := len(vm.stack) - 1
	if i < 0 {
		vm.halt(errStackUnderflow)
	}
	val, vm.stack = vm.stack[i], vm.stack[:i]
	return val
}

func (vm *VM) pushr(addr uint) {
	r := uint(vm.load(1))
	if retBase := uint(vm.load(10)); r < retBase-1 {
		vm.halt(retUnderError(r))
	}
	if memBase := uint(vm.load(11)); r >= memBase-1 {
		vm.halt(retOverError(r))
	}
	r++
	vm.stor(r, int(addr))
	vm.stor(1, int(r))
}

func (vm *VM) popr() uint {
	r := uint(vm.load(1))
	if retBase := uint(vm.load(10)); r == retBase-1 {
		vm.halt(nil)
	} else if r < retBase-1 {
		vm.halt(retUnderError(r))
	} else if memBase := uint(vm.load(11)); r > memBase-1 {
		vm.halt(retOverError(r))
	}
	val := uint(vm.load(r))
	vm.stor(1, int(r-1))
	return val
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

func (vm *VM) literal(token string) int {
	if n, err := strconv.ParseInt(token, 0, strconv.IntSize); err == nil {
		return int(n)
	}
	if value, err := runeio.UnquoteRune(token); err == nil {
		return int(value)
	}
	vm.halt(literalError(token))
	return 0
}

func (vm *VM) exec(ctx context.Context) error {
	for {
		vm.step()
		if err := ctx.Err(); err != nil {
			return err
		}
	}
}

func (vm *VM) wordOf(addr uint) (string, uint) {
	for prev := vm.last; prev != 0; {
		if prev < addr {
			if sym := uint(vm.load(prev + 1)); sym == 0 {
				return "ø", addr - prev
			} else if name := vm.string(sym); name != "" {
				return name, addr - prev
			}
			break
		}
		prev = uint(vm.load(prev))
	}
	return "", 0
}

func (vm *VM) codeName() string {
	code := uint(vm.loadProg())
	defer func() { vm.prog-- }()
	if code >= uint(len(vmCodeTable)) {
		if name, _ := vm.wordOf(code); name != "" {
			return name
		}
		return fmt.Sprintf("call(%v)", code)
	}
	if code == vmCodePushint {
		return fmt.Sprintf("pushint(%v)", vm.load(vm.prog))
	}
	return vmCodeNames[code]
}

const (
	debugTRON = 1 << iota
)

// TODO use a portal instead

func (vm *VM) checkFlag(flag int) bool {
	retBase, err := vm.mem.Load(10)
	if err != nil {
		return false
	}
	val, err := vm.mem.Load(uint(retBase) - 1)
	if err != nil {
		return false
	}
	return val&flag != 0
}

func (vm *VM) logf(mark, message string, args ...interface{}) {
	if vm.checkFlag(debugTRON) {
		vm.logging.logf(mark, message, args...)
	}
}

func (vm *VM) step() {
	if vm.logfn != nil && vm.checkFlag(debugTRON) {
		at := fmt.Sprintf(" @%v", vm.prog)

		funcName, _ := vm.wordOf(vm.prog)
		if vm.funcWidth < len(funcName) {
			vm.funcWidth = len(funcName)
		}

		codeName := vm.codeName()
		if vm.codeWidth < len(codeName) {
			vm.codeWidth = len(codeName)
		}

		vm.logging.logf(at, "% *v.% -*v s:%v r:%v",
			vm.funcWidth, funcName,
			vm.codeWidth, codeName,
			vm.stack,
			vm.rstack(),
		)
	}

	if code := uint(vm.loadProg()); code < uint(len(vmCodeTable)) {
		vmCodeTable[code](vm)
	} else {
		vm.call(uint(code))
	}
}

func (vm *VM) rstack() []int {
	rb := uint(vm.load(10))
	r := uint(vm.load(1))
	if r < rb-1 {
		vm.halt(retUnderError(r))
	} else if r < rb {
		return []int{}
	}
	rstack := make([]int, r-rb+1)
	vm.loadInto(rb, rstack)
	return rstack
}

const (
	defaultPageSize = 256
	defaultRetBase  = 1 // in pages
	defaultMemBase  = 4 // in pages
)

func (vm *VM) init() {
	pageSize := vm.mem.PageSize
	if pageSize == 0 {
		pageSize = defaultPageSize
		vm.mem.PageSize = pageSize
	}

	retBase := uint(vm.load(10))
	if retBase == 0 {
		retBase = defaultRetBase * pageSize
		vm.stor(10, int(retBase))
	}

	memBase := uint(vm.load(11))
	if memBase == 0 {
		memBase = defaultMemBase * pageSize
		vm.stor(11, int(memBase))
	}

	if h := uint(vm.load(0)); h == 0 {
		vm.stor(0, int(memBase))
	}

	if r := uint(vm.load(1)); r == 0 {
		vm.stor(1, int(retBase-1))
	} else if r < retBase {
		vm.halt(retUnderError(r))
	} else if r > memBase {
		vm.halt(retOverError(r))
	}
}

func (vm *VM) run(ctx context.Context) error {
	vm.init()

	// clear program counter and compile builtins
	vm.prog = 0
	entry := vm.compileEntry()
	vm.compileBuiltins()

	// run the entry point
	vm.prog = entry
	return vm.exec(ctx)
}

func (vm *VM) scan() (token string) {
	defer func() {
		line := vm.Scan
		if line.Len() == 0 {
			line = vm.Last
		}
		vm.logf(">", "scan %v %q <- %q", line.Location, token, line.Buffer.String())
	}()

	if err := vm.out.Flush(); err != nil {
		vm.halt(err)
	}

	var sb strings.Builder
	for {
		if r, _, err := vm.ioCore.ReadRune(); err != nil {
			vm.halt(err)
		} else if !unicode.IsControl(r) && !unicode.IsSpace(r) {
			sb.WriteRune(r)
			break
		}
	}
	for {
		r, _, err := vm.ioCore.ReadRune()
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

func (vm *VM) writeRune(r rune) {
	if _, err := runeio.WriteANSIRune(vm.out, r); err != nil {
		vm.halt(err)
	}
}

func (vm *VM) readRune() rune {
	if err := vm.out.Flush(); err != nil {
		vm.halt(err)
	}

	r, _, err := vm.ioCore.ReadRune()
	for r == 0 {
		if err != nil {
			vm.halt(err)
		}
		r, _, err = vm.ioCore.ReadRune()
	}
	return r
}

type vmHaltError struct{ error }

var (
	errStackUnderflow = errors.New("stack underflow")
)

func (err vmHaltError) Error() string {
	if err.error != nil {
		return fmt.Sprintf("VM halted: %v", err.error)
	}
	return "VM halted"
}
func (err vmHaltError) Unwrap() error { return err.error }

type progError uint
type retOverError uint
type retUnderError uint
type literalError string

func (r retOverError) Error() string   { return fmt.Sprintf("return stack overflow @%v", uint(r)) }
func (r retUnderError) Error() string  { return fmt.Sprintf("return stack underflow @%v", uint(r)) }
func (addr progError) Error() string   { return fmt.Sprintf("program smashed %v", uint(addr)) }
func (lit literalError) Error() string { return fmt.Sprintf("invalid literal %q", string(lit)) }

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

type logging struct {
	logfn func(mess string, args ...interface{})

	markWidth int
	funcWidth int
	codeWidth int
}

func (log *logging) withLogPrefix(prefix string) func() {
	logfn := log.logfn
	log.logfn = func(mess string, args ...interface{}) {
		logfn(prefix+mess, args...)
	}
	return func() {
		log.logfn = logfn
	}
}

func (log logging) logf(mark, mess string, args ...interface{}) {
	if log.logfn == nil {
		return
	}
	if n := log.markWidth - len(mark); n > 0 {
		for _, r := range mark {
			mark = strings.Repeat(string(r), n) + mark
			break
		}
	} else if n < 0 {
		log.markWidth = len(mark)
	}
	if len(args) > 0 {
		mess = fmt.Sprintf(mess, args...)
	}
	log.logfn("%v %v", mark, mess)
}
