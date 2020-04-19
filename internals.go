package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"runtime/debug"
	"strconv"
	"strings"
	"unicode"
)

type vmHaltError struct{ error }

func (err vmHaltError) Error() string {
	if err.error != nil {
		return fmt.Sprintf("VM halted: %v", err.error)
	}
	return "VM halted"
}
func (err vmHaltError) Unwrap() error { return err.error }

func (vm *VM) halt(err error) {
	if ferr := vm.out.Flush(); err == nil {
		err = ferr
	}
	err = vmHaltError{err}
	vm.logf("#", "halt error: %v", err)
	panic(err)
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
		vm.halt(nil)
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
	if value, ok := runeLiteral(token); ok {
		return int(value), nil
	}
	return 0, err
}

func runeLiteral(token string) (rune, bool) {
	runes := []rune(token)
	if len(runes) < 1 || runes[0] != '\'' {
		return 0, false
	}

	switch len(runes) {
	case 3:
		if runes[2] != '\'' {
			return 0, false
		}
	case 4:
		if runes[3] != '\'' {
			return 0, false
		}
	default:
		return 0, false
	}

	value, _, _, err := strconv.UnquoteChar(token[1:], '\'')
	return value, err == nil
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
			if name := vm.string(uint(vm.mem[prev+1])); name != "" {
				return name, addr - prev
			}
			break
		}
		prev = uint(vm.load(prev))
	}
	return "", 0
}

func (vm *VM) codeName() string {
	code := vm.loadProg()
	defer func() { vm.prog-- }()
	if code >= len(vmCodeTable) {
		if name, _ := vm.wordOf(uint(code)); name != "" {
			return name
		}
		return fmt.Sprintf("call(%v)", code)
	}
	if code == vmCodePushint {
		return fmt.Sprintf("pushint(%v)", vm.mem[vm.prog])
	}
	return vmCodeNames[code]
}

func (vm *VM) step() {
	if vm.logfn != nil {
		at := fmt.Sprintf(" @%v", vm.prog)

		funcName, _ := vm.wordOf(vm.prog)
		if vm.funcWidth < len(funcName) {
			vm.funcWidth = len(funcName)
		}

		codeName := vm.codeName()
		if vm.codeWidth < len(codeName) {
			vm.codeWidth = len(codeName)
		}

		vm.logf(at, "% *v.% -*v r:%v s:%v",
			vm.funcWidth, funcName,
			vm.codeWidth, codeName,
			vm.mem[vm.load(10):vm.load(1)],
			vm.stack)
	}

	if code := vm.loadProg(); code < len(vmCodeTable) {
		vmCodeTable[code](vm)
	} else {
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
		line := vm.scanLine
		if line.Len() == 0 {
			line = vm.lastLine
		}
		vm.logf(">", "scan %v %q <- %q", line.inLoc, token, line.Buffer.String())
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

func isolate(name string, f func() error) error {
	errch := make(chan error, 1)
	go func() {
		defer close(errch)
		defer recoverExitError(name, errch)
		defer recoverPanicError(name, errch)
		errch <- f()
	}()
	return <-errch
}

func recoverExitError(name string, errch chan<- error) {
	select {
	case errch <- fmt.Errorf("%v called runtime.Goexit", name):
	default:
		// assumes that that the happy path does a (maybe nil) send
	}
}

func recoverPanicError(name string, errch chan<- error) {
	if e := recover(); e != nil {
		select {
		case errch <- panicError{name, e, string(debug.Stack())}:
		default:
		}
	}
}

func panicErrorStack(err error) string {
	var pe panicError
	if errors.As(err, &pe) {
		return pe.stack
	}
	return ""
}

type panicError struct {
	name  string
	e     interface{}
	stack string
}

func (pe panicError) Error() string {
	return fmt.Sprintf("%v paniced: %v", pe.name, pe.e)
}

func (pe panicError) Format(f fmt.State, c rune) {
	if c == 'v' && f.Flag('+') {
		fmt.Fprintf(f, "%v paniced: %v\nPanic stack: %s", pe.name, pe.e, pe.stack)
	} else {
		fmt.Fprintf(f, "%v paniced: %v", pe.name, pe.e)
	}
}

func (pe panicError) Unwrap() error {
	err, _ := pe.e.(error)
	return err
}
