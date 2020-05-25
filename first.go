package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode"

	"github.com/jcorbin/gothird/internal/mem"
	"github.com/jcorbin/gothird/internal/runeio"
)

//// Section 1:  FIRST

//// Environment

// FIRST implements a virtual machine.  The machine has three chunks of memory:
// "main memory", "the stack", and "string storage".  When the virtual machine
// wishes to do random memory accesses, they come out of main memory--it cannot
// access the stack or string storage.
type VM struct {
	ioCore
	logging

	prog uint // program counter
	last uint // last word

	// The stack is simply a standard LIFO data structure that is used
	// implicitly by most of the FIRST primitives.  The stack is made up of
	// ints, whatever size they are on the host machine.
	stack []int

	// String storage is used to store the names of built-in and defined
	// primitives.  Separate storage is used for these because it allows the Go
	// code to use Go string operations, reducing Go source code size.
	symbols

	// Main memory is a large array of ints.  When we speak of addresses, we
	// actually mean indices into main memory.  Main memory is used for two
	// things, primarily: the return stack and the dictionary.
	mem mem.Ints
}

// The return stack is a LIFO data structure, independent of the
// above-mentioned "the stack", which is used by FIRST to keep track of
// function call return addresses.
func (vm *VM) call(addr uint) { vm.pushr(vm.prog); vm.prog = addr }

// The dictionary is a list of words.  Each word contains a header and a data
// field.  In the header is the address of the previous word, an index into the
// string storage indicating where the name of this word is stored, and a "code
// pointer".  The code pointer is simply an integer which names which
// "machine-language-primitive" implements this instruction.  For example, for
// defined words the code pointer names the "run some code" primitive, which
// pushes the current program counter onto the return stack and sets the
// counter to the address of the data field for this word.

// There are several important pointers into main memory.  There is a pointer
// to the most recently defined word, which is used to start searches back
// through memory when compiling words.  There is a pointer to the top of the
// return stack.  There is a pointer to the current end of the dictionary, used
// while compiling.

// For the last two pointers, namely the return stack pointer and the
// dictionary pointer, there is an important distinction: the pointers
// themselves are stored in main memory (in FIRST's main memory).  This is
// critical, because it means FIRST programs can get at them without any
// further primitives needing to be defined.

//// Instructions

// There are two kinds of FIRST instructions, normal instructions and immediate
// instructions.  Immediate instructions do something significant when they are
// used.  Normal instructions compile a pointer to their executable part onto
// the end of the dictionary.  As we will see, this means that by default FIRST
// simply compiles things.

//// Integer Operations

// Symbol   Name           Function
//    -     binary minus   pop top 2 elements of stack, subtract, push
func (vm *VM) sub() { b, a := vm.pop(), vm.pop(); vm.push(a - b) }

// Symbol   Name           Function
//    *     multiply       pop top 2 elements of stack, multiply, push
func (vm *VM) mul() { b, a := vm.pop(), vm.pop(); vm.push(a * b) }

// Symbol   Name           Function
//    /     divide         pop top 2 elements of stack, divide, push
func (vm *VM) div() { b, a := vm.pop(), vm.pop(); vm.push(a / b) }

// Symbol   Name           Function
//   <0     less than 0    pop top element of stack, push 1 if < 0 else 0
func (vm *VM) under0() { a := vm.pop(); vm.push(boolInt(a < 0)) }

// Note that we can synthesize addition and negation from binary minus, but we
// cannot synthesize a time efficient divide or multiply from it. <0 is
// synthesizable, but only nonportably.

//// Memory Operations

// Symbol   Name    Function
//   @      fetch   pop top of stack, treat as address to push contents of
func (vm *VM) get() { addr := uint(vm.pop()); vm.push(vm.load(addr)) }

// Symbol   Name    Function
//   !      store   top of stack is address, 2nd is value; store to memory and
//                  pop both off the stack
func (vm *VM) set() { addr := uint(vm.pop()); vm.stor(addr, vm.pop()) }

//// Input/Output Operations

// Name    Function
// echo    write top of stack to output as a rune
func (vm *VM) echo() { vm.writeRune(rune(vm.pop())) }

// Name    Function
// key     read a rune from input onto top of stack
func (vm *VM) key() { vm.push(int(vm.readRune())) }

// Name    Function
// _read   read a space-delimited word, find it in the dictionary, and compile
//         a pointer to that word's code pointer onto the current end of the
//         dictionary
func (vm *VM) read() {
	token := vm.scan()
	if word := vm.lookup(token); word != 0 {
		vm.logf(".", "read %v @%v", token, word)
		vm.pushr(vm.prog)
		vm.prog = word + 2
		return
	}

	val := vm.literal(token)
	vm.logf(".", "read pushint(%v)", val)
	vm.compile(vmCodePushint)
	vm.compile(int(val))
}

// Although _read could be synthesized from key, we need _read to be able to
// compile words to be able to start any syntheses.

//// Execution Operations

// Name   Function
// exit   leave the current function: pop the return stack
//        into the program counter
func (vm *VM) exit() { vm.prog = vm.popr() }

//// Immediate (compilation) Operations

// Symbol      Name        Function
//    :        define      read in the next space-delimited word, add it to the
//                         end of our string storage, and generate a header for
//                         the new word so that when it is typed it compiles a
//                         pointer to itself so that it can be executed.
func (vm *VM) define() {
	token := vm.scan()
	vm.logf(".", "define %v -> @%v", token, uint(vm.load(0)))
	vm.compileHeader(vm.symbolicate(token))
}

// Symbol      Name        Function
// immediate   immediate   when used immediately after a name following a ':',
//                         makes the word being defined run whenever it is
//                         typed.
func (vm *VM) immediate() {
	h := uint(vm.load(0))
	h--                  // back
	code := vm.load(h)   // read run time code
	h--                  // back
	vm.stor(h, code)     // overwrite compile time code
	vm.stor(0, int(h+1)) // continue
	vm.logf(".", "immediate @%v <- %v <- @%v", h-1, code, h)
}

// : cannot be synthesized, because we could not synthesize anything.
// immediate has to be an immediate operation, so it could not be synthesized
// unless by default operations were immediate; but that would preclude our
// being able to do any useful compilation.

//// Stack Operations

// Name   Function
// pick   pop top of stack, use as index into stack and copy up that element
func (vm *VM) pick() {
	i := vm.pop()
	i = len(vm.stack) - 1 - i
	if i < 0 || i >= len(vm.stack) {
		vm.push(0)
	} else {
		vm.push(vm.stack[i])
	}
}

// If the data stack were stored in main memory, we could synthesize pick; but
// putting the stack and stack pointer in main memory would significantly
// increase the C source code size.

// There are three more primitives, but they are "internal only"-- they have no
// names and no dictionary entries.  The first is "pushint".  It takes the next
// integer out of the instruction stream and pushes it on the stack.  This
// could be synthesized, but probably not without using integer constants.  It
// is generated by _read when the input is not a known word.  The second is
// "compile me".  When this instruction is executed, a pointer to the word's
// data field is appended to the dictionary.  The third is "run me"--the word's
// data field is taken to be a stream of pointers to words, and is executed.

//// Internal primitives have no names and no dictionary entries.

// The first is "pushint".
// It takes the next integer out of the instruction stream and pushes it on the
// stack.  It is generated by _read when the input is not a known word.
func (vm *VM) pushint() { vm.push(vm.loadProg()) }

// The second is "compile me".
// When this instruction is executed, a pointer to the word's data field is
// appended to the dictionary.
func (vm *VM) compileme() {
	addr := vm.prog
	if vm.load(addr) == vmCodeRun {
		addr++
	}
	vm.compile(int(addr))
	vm.exit()
}

func (vm *VM) compileit() {
	code := vm.loadProg()
	for {
		vm.compile(code)
		next := vm.loadProg()
		if code == vmCodeExit || next == vmCodeExit {
			vm.exit()
			return
		}
		code = next
	}
}

// The third is "run me"--the word's data field is taken to be a stream of
// pointers to words, and is executed.
func (vm *VM) runme() {}

// One last note about the environment: FIRST builds a very small word
// internally that it executes as its main loop.  This word calls _read and
// then calls itself.  Each time it calls itself, it uses up a word on the
// return stack, so it will eventually trash things. This is discussed some
// more in section 2.
func (vm *VM) compileEntry() uint {
	vm.compileHeader(0)
	vm.immediate()
	h := uint(vm.load(0))
	vm.compile(vmCodeRead)
	// vm.immediate() FIXME
	vm.compile(int(h))
	vm.compile(vmCodeExit)
	return h
}

const (
	// Here's a handy summary of all the FIRST words:
	vmCodeExit      = iota // exit        stop running the current function
	vmCodeDefine           // :           compile the header of a definition
	vmCodeImmediate        // immediate   modify the header to create an immediate word
	vmCodeRead             // _read       read a word from input and compile a pointer to it
	vmCodeGet              // @           read from memory
	vmCodeSet              // !           write to memory
	vmCodeSub              // -           binary integer operation on the stack
	vmCodeMul              // *           binary integer operation on the stack
	vmCodeDiv              // /           binary integer operation on the stack
	vmCodeLess             // <0          is top of stack less than 0?
	vmCodeEcho             // echo        output one character
	vmCodeKey              // key         input one character
	vmCodePick             // pick        pop top of stack, use as index into stack and copy up that element

	vmCodeCompile // <INTERNAL>  compile the program counter
	vmCodeRun     // <INTERNAL>  run at the program counter
	vmCodePushint // <INTERNAL>  push from memory at program counter
	vmCodeCompIt  // <INTERNAL>  compile from memory at program counter

	vmCodeMax
	vmCodeLastBuiltin = vmCodePick
)

func (vm *VM) compileBuiltins() {
	vm.define()
	vm.stor(vm.last+2, vmCodeCompIt) // compile inline
	vm.compile(vmCodeExit)
	vm.immediate() // write the vmCodeExit token over the prior vmCodeRun

	for code := vmCodeDefine; code <= vmCodeLastBuiltin; code++ {
		vm.define()
		vm.stor(vm.last+2, vmCodeCompIt) // compile inline
		if code <= vmCodeImmediate {
			vm.immediate()
		}
		vm.compile(code)
		vm.immediate() // write the builtin token over the prior vmCodeRun
		vm.compile(vmCodeExit)
	}
}

var vmCodeTable [vmCodeMax]func(vm *VM)
var vmCodeNames [vmCodeMax]string

func init() {
	vmCodeTable = [...]func(vm *VM){
		(*VM).exit,
		(*VM).define,
		(*VM).immediate,
		(*VM).read,
		(*VM).get,
		(*VM).set,
		(*VM).sub,
		(*VM).mul,
		(*VM).div,
		(*VM).under0,
		(*VM).echo,
		(*VM).key,
		(*VM).pick,

		(*VM).compileme,
		(*VM).runme,
		(*VM).pushint,
		(*VM).compileit,
	}

	vmCodeNames = [...]string{
		"exit",
		"define",
		"immediate",
		"read",
		"get",
		"set",
		"sub",
		"mul",
		"div",
		"under0",
		"echo",
		"key",
		"pick",

		"compileme",
		"runme",
		"pushint",
		"compileit",
	}
}

// Here is a sample FIRST program.  I'm assuming you're using the ASCII
// character set.  FIRST does not depend upon ASCII, but since FIRST has no
// syntax for character constants, one normally has to use decimal values. This
// can be gotten around using getchar, though. Oh.  One other odd thing. FIRST
// initially builds its symbol table by calling : several times, so it needs to
// get the names of the base symbols as its first 13 words of input. You could
// even name them differently if you wanted.

// These FIRST programs have FORTH comments in them: they are contained inside
// parentheses.  FIRST programs cannot have FORTH comments; but I need some
// device to indicate what's going on.  (THIRD programs are an entirely
// different subject.)

//    ( Our first line gives the symbols for the built-ins )
//    : immediate _read @ ! - * / <0 exit echo key pick
//
//    ( now we define a simple word that will print out a couple characters )
//
//    : L                ( define a word named 'L' )
//      108 echo         ( output an ascii 'l' )
//      exit
//
//    : hello            ( define a word named 'hello')
//      72 echo          ( output an ascii 'H' )
//      101 echo         ( output an ascii 'e' )
//      111              ( push ascii 'o' onto the stack )
//      L L              ( output two ascii 'l's )
//      echo             ( output the 'o' we pushed on the stack before )
//      10 echo          ( print a newline )
//      exit             ( stop running this routine )
//
//    : test immediate   ( define a word named 'test' that runs whenever typed )
//      hello            ( call hello )
//      exit
//
//    test
//
//    ( The result of running this program should be:
//    Hello
//    )

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
	for {
		vm.step()
		if err := ctx.Err(); err != nil {
			return err
		}
	}
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

var (
	errStackUnderflow = errors.New("stack underflow")
)

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

	// ignore any panics while logging
	func() {
		defer func() { recover() }()
		vm.logf("#", "halt error: %v", err)
	}()

	panic(vmHaltError{err})
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

func (err vmHaltError) Error() string {
	if err.error != nil {
		return fmt.Sprintf("VM halted: %v", err.error)
	}
	return "VM halted"
}
func (err vmHaltError) Unwrap() error { return err.error }
