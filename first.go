package main

import (
	"io"
	"strconv"
)

//// Section 1:  FIRST

//// Environment

// FIRST implements a virtual machine.  The machine has three chunks of memory:
// "main memory", "the stack", and "string storage".  When the virtual machine
// wishes to do random memory accesses, they come out of main memory--it cannot
// access the stack or string storage.
type VM struct {
	ioCore

	compiling bool // program mode
	prog      uint // program counter

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
	mem []int

	memLimit int
}

// The return stack is a LIFO data structure, independent of the
// above-mentioned "the stack", which is used by FIRST to keep track of
// function call return addresses.
func (vm *VM) call(addr uint) {
	if r := uint(vm.load(1)); r == 0 {
		vm.prog = addr // early calls are just jumps
	} else if retBase := uint(vm.load(10)); r < retBase {
		vm.halt(errRetUnderflow)
	} else if memBase := uint(vm.load(11)); r >= memBase {
		vm.halt(errRetOverflow)
	} else {
		vm.stor(1, int(vm.pushProg(r, addr)))
	}
}

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
func (vm *VM) sub() bool { b, a := vm.pop(), vm.pop(); vm.push(a - b); return false }

// Symbol   Name           Function
//    *     multiply       pop top 2 elements of stack, multiply, push
func (vm *VM) mul() bool { b, a := vm.pop(), vm.pop(); vm.push(a * b); return false }

// Symbol   Name           Function
//    /     divide         pop top 2 elements of stack, divide, push
func (vm *VM) div() bool { b, a := vm.pop(), vm.pop(); vm.push(a / b); return false }

// Symbol   Name           Function
//   <0     less than 0    pop top element of stack, push 1 if < 0 else 0
func (vm *VM) less() bool { a := vm.pop(); vm.push(boolInt(a < 0)); return false }

// Note that we can synthesize addition and negation from binary minus, but we
// cannot synthesize a time efficient divide or multiply from it. <0 is
// synthesizable, but only nonportably.

//// Memory Operations

// Symbol   Name    Function
//   @      fetch   pop top of stack, treat as address to push contents of
func (vm *VM) get() bool { addr := uint(vm.pop()); vm.push(vm.load(addr)); return false }

// Symbol   Name    Function
//   !      store   top of stack is address, 2nd is value; store to memory and
//                  pop both off the stack
func (vm *VM) set() bool { addr := uint(vm.pop()); vm.stor(addr, vm.pop()); return false }

//// Input/Output Operations

// Name    Function
// echo    write top of stack to output as a rune
func (vm *VM) echo() bool { vm.haltif(writeRune(vm.out, rune(vm.pop()))); return false }

// Name    Function
// key     read a rune from input onto top of stack
func (vm *VM) key() bool {
	r, _, err := vm.in.ReadRune()
	if err == io.EOF {
		return true
	} else if err != nil {
		vm.halt(err)
	}
	vm.push(int(r))
	return false
}

// Name    Function
// _read   read a space-delimited word, find it in the dictionary, and compile
//         a pointer to that word's code pointer onto the current end of the
//         dictionary
func (vm *VM) read() bool {
	// we're running compilation words if compiling, or we have no return stack
	compiling := vm.compiling
	var r uint
	if !compiling {
		r = uint(vm.load(1))
		compiling = r == 0
	}

	token := vm.scan()
	if word := vm.lookupToken(token); word != 0 {
		addr := word + 2
		vm.logf("read %v @%v", token, addr)

		// call it now, using Go's stack
		if compiling {
			defer func(prog uint) { vm.prog = prog }(vm.prog)
			vm.prog = addr
			vm.exec()
			return false
		}

		// call in next exec() round
		vm.stor(1, int(vm.pushProg(r, addr)))
		return false
	}

	val, err := vm.parseToken(token)
	vm.haltif(err)
	vm.logf("read pushint %v", val)
	vm.compile(vmCodePushint)
	vm.compile(int(val))

	return false
}

func (vm *VM) parseToken(token string) (int, error) {
	n, err := strconv.ParseInt(token, 10, strconv.IntSize)
	if err == nil {
		return int(n), nil
	}
	return 0, err
}

func (vm *VM) lookupToken(token string) uint {
	if name := vm.symbol(token); name != 0 {
		if word := vm.lookup(name); word != 0 {
			return word
		}
	}
	return 0
}

// Although _read could be synthesized from key, we need _read to be able to
// compile words to be able to start any syntheses.

//// Execution Operations

// Name   Function
// exit   leave the current function: pop the return stack
//        into the program counter
func (vm *VM) exit() bool {
	r := uint(vm.load(1))
	retBase := uint(vm.load(10))
	memBase := uint(vm.load(11))
	if r == 0 || r == retBase {
		return true
	} else if r < retBase {
		vm.halt(errRetUnderflow)
	} else if r > memBase {
		vm.halt(errRetOverflow)
	}
	r--
	vm.prog = uint(vm.load(r))
	vm.logf("exit prog <- %v <- ret[%v]", vm.prog, r-retBase)
	vm.stor(1, int(r))
	return false
}

//// Immediate (compilation) Operations

// Symbol      Name        Function
//    :        define      read in the next space-delimited word, add it to the
//                         end of our string storage, and generate a header for
//                         the new word so that when it is typed it compiles a
//                         pointer to itself so that it can be executed.
func (vm *VM) define() bool {
	token := vm.scan()
	vm.logf("define %v -> @%v", token, vm.here())
	vm.compileHeader(vm.symbolicate(token))
	return false
}

// Symbol      Name        Function
// immediate   immediate   when used immediately after a name following a ':',
//                         makes the word being defined run whenever it is
//                         typed.
func (vm *VM) immediate() bool {
	h := vm.here()
	h--                  // back
	code := vm.load(h)   // read run time code
	h--                  // back
	vm.stor(h, code)     // overwrite compile time code
	vm.stor(0, int(h+1)) // continue
	vm.logf("immediate @%v <- %v <- @%v", h-1, code, h)
	return false
}

// : cannot be synthesized, because we could not synthesize anything.
// immediate has to be an immediate operation, so it could not be synthesized
// unless by default operations were immediate; but that would preclude our
// being able to do any useful compilation.

//// Stack Operations

// Name   Function
// pick   pop top of stack, use as index into stack and copy up that element
func (vm *VM) pick() bool {
	i := vm.pop()
	i = len(vm.stack) - 1 - i
	if i < 0 || i >= len(vm.stack) {
		vm.push(0)
	} else {
		vm.push(vm.stack[i])
	}
	return false
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
func (vm *VM) pushint() bool { vm.push(vm.loadProg()); return false }

// The second is "compile me".
// When this instruction is executed, a pointer to the word's data field is
// appended to the dictionary.
func (vm *VM) compileme() bool {
	vm.compile(int(vm.prog))
	return vm.exit()
}

// The third is "run me"--the word's data field is taken to be a stream of
// pointers to words, and is executed.
func (vm *VM) runme() bool { return false }

// One last note about the environment: FIRST builds a very small word
// internally that it executes as its main loop.  This word calls _read and
// then calls itself.  Each time it calls itself, it uses up a word on the
// return stack, so it will eventually trash things. This is discussed some
// more in section 2.
func (vm *VM) compileEntry() uint {
	vm.compileHeader(0)
	vm.immediate()
	h := vm.here()
	vm.compile(vmCodeRead)
	// vm.immediate() FIXME
	vm.compile(int(h))
	vm.compile(vmCodeExit)
	return h
}

const (
	vmCodeCompile = iota // <INTERNAL>  compile the program counter

	// Here's a handy summary of all the FIRST words:
	vmCodeDefine    // :           compile the header of a definition
	vmCodeImmediate // immediate   modify the header to create an immediate word
	vmCodeRead      // _read       read a word from input and compile a pointer to it
	vmCodeGet       // @           read from memory
	vmCodeSet       // !           write to memory
	vmCodeSub       // -           binary integer operation on the stack
	vmCodeMul       // *           binary integer operation on the stack
	vmCodeDiv       // /           binary integer operation on the stack
	vmCodeLess      // <0          is top of stack less than 0?
	vmCodeExit      // exit        stop running the current function
	vmCodeEcho      // echo        output one character
	vmCodeKey       // key         input one character
	vmCodePick      // pick        pop top of stack, use as index into stack and copy up that element

	vmCodeRun     // <INTERNAL>  run at the program counter
	vmCodePushint // <INTERNAL>  push from memory at program counter

	vmCodeMax
	vmCodeLastBuiltin = vmCodePick
)

func (vm *VM) compileBuiltins() {
	for code := vmCodeDefine; code <= vmCodeLastBuiltin; code++ {
		vm.define()
		if code <= vmCodeImmediate {
			vm.immediate()
		}
		vm.compile(code)
		vm.immediate() // write the builtin code over the "runme" token with
		if code != vmCodeExit {
			vm.compile(vmCodeExit)
		}
	}
}

var vmCodeTable [vmCodeMax]func(vm *VM) bool
var vmCodeNames [vmCodeMax]string

func init() {
	vmCodeTable = [...]func(vm *VM) bool{
		(*VM).compileme,

		(*VM).define,
		(*VM).immediate,
		(*VM).read,
		(*VM).get,
		(*VM).set,
		(*VM).sub,
		(*VM).mul,
		(*VM).div,
		(*VM).less,
		(*VM).exit,
		(*VM).echo,
		(*VM).key,
		(*VM).pick,

		(*VM).runme,
		(*VM).pushint,
	}

	vmCodeNames = [...]string{
		"compileme",

		"define",
		"immediate",
		"read",
		"get",
		"set",
		"sub",
		"mul",
		"div",
		"less",
		"exit",
		"echo",
		"key",
		"pick",

		"runme",
		"pushint",
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
