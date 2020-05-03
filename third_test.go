package main

import (
	"strings"
	"testing"
	"time"
)

// Test_kernel tests a layered recreation of the THIRD kernel, with a test case
// validating each each layer.
func Test_kernel(t *testing.T) {
	var k kernel

	// Test that builtin word initialization works.
	k.addSource("builtins", `
		exit : immediate _read @ ! - * / <0 echo key pick
	`, "",
		expectVMWord(1024, "", vmCodeRun, vmCodeRead, 1027, vmCodeExit),
		expectVMWord(1030, "exit", vmCodeCompIt, vmCodeExit),
		expectVMWord(1034, ":", vmCodeDefine, vmCodeExit),
		expectVMWord(1038, "immediate", vmCodeImmediate, vmCodeExit),
		expectVMWord(1042, "_read", vmCodeCompIt, vmCodeRead, vmCodeExit),
		expectVMDump(lines(
			`# VM Dump`,
			`  prog: 1028`,
			`  dict: [1087 1082 1077 1072 1067 1062 1057 1052 1047 1042 1038 1034 1030 1024]`,
			`  stack: []`,
			`  @    0 1092 dict`,
			`  @    1 255 ret`,
			`  @    2 0`,
			`  @    3 0`,
			`  @    4 0`,
			`  @    5 0`,
			`  @    6 0`,
			`  @    7 0`,
			`  @    8 0`,
			`  @    9 0`,
			`  @   10 256 retBase`,
			`  @   11 1024 memBase`,

			`# Return Stack @256`,

			`# Main Memory @1024`,
			`  @ 1024 : ø immediate runme read ø+3 exit`,
			`  @ 1030 : exit exit`,
			`  @ 1034 : : immediate define exit`,
			`  @ 1038 : immediate immediate immediate exit`,
			`  @ 1042 : _read read exit`,
			`  @ 1047 : @ get exit`,
			`  @ 1052 : ! set exit`,
			`  @ 1057 : - sub exit`,
			`  @ 1062 : * mul exit`,
			`  @ 1067 : / div exit`,
			`  @ 1072 : <0 under0 exit`,
			`  @ 1077 : echo echo exit`,
			`  @ 1082 : key key exit`,
			`  @ 1087 : pick pick exit`,
		)))

	// Build a main loop that won't exhaust the return stack.
	k.addSource("main", `
		: r 1 exit

		: ]
			r @
			r -
			r !
			_read ]

		: main immediate ] main

		: rb 10 exit
	`, `
		: test immediate
			rb @
	`,
		expectVMWord(1092, "r", vmCodeCompile, vmCodeRun, vmCodePushint, 1, vmCodeExit),
		expectVMWord(1099, "]",
			/* @1101 */ vmCodeCompile,
			/* @1102 */ vmCodeRun,
			/* @1103 */ 1096, vmCodeGet,
			/* @1105 */ 1096, vmCodeSub,
			/* @1107 */ 1096, vmCodeSet,
			/* @1109 */ vmCodeRead,
			/* @1110 */ 1103,
		),
		expectVMWord(1111, "main", vmCodeRun, 1103),
		expectVMWord(1115, "rb", vmCodeCompile, vmCodeRun, vmCodePushint, 10, vmCodeExit),
		expectVMStack(256))

	// Synthesize addition and test it.
	k.addSource("add", `
		: _x  3 @ exit
		: _x! 3 ! exit

		: + _x! 0 _x - - exit
	`, `
		: test immediate
			3 5 7 + +
	`,
		expectVMWord(1122, "_x",
			/* 1124 */ vmCodeCompile, vmCodeRun,
			/* 1126 */ vmCodePushint, 3, vmCodeGet,
			/* 1129 */ vmCodeExit),
		expectVMWord(1130, "_x!",
			/* 1132 */ vmCodeCompile, vmCodeRun,
			/* 1134 */ vmCodePushint, 3, vmCodeSet,
			/* 1137 */ vmCodeExit),
		expectVMWord(1138, "+",
			/* 1140 */ vmCodeCompile, vmCodeRun,
			/* 1142 */ 1134, // _x!
			/* 1145 */ vmCodePushint, 0,
			/* 1146 */ 1126, // _x
			/* 1147 */ vmCodeSub, vmCodeSub,
			/* 1149 */ vmCodeExit),
		expectVMStack(3+5+7))

	// ' is a standard FORTH word.
	// It should push the address of the word that follows it onto the stack.
	k.addSource("quote", `
		: dup _x! _x _x exit

		: '
			r @ @
			dup
			1 +
			r @ !
			@
			exit
	`, `
		: test immediate
			' ]
			' @
			' exit
	`, expectVMStack(
		1103,       // ' ]
		vmCodeGet,  // ' @
		vmCodeExit, // ' exit
	))

	// Reset the return stack, by way of an "exec" word.
	k.addSource("reboot", `
		: exec
			rb @ !
			rb @ r !
			exit

		: _read] _read ]
		: reboot immediate
			' _read]
			exec
		reboot
	`, `
		: test immediate
			42
			1024 1024 * @
	`,
		expectVMRStack(1110),
		expectVMStack(42),
		expectVMError(memLimitError{1024 * 1024, "get"}))

	// swap two values on the top of the stack.
	k.addSource("swap", `
		: _y  4 @ exit
		: _y! 4 ! exit

		: swap
			_x! _y! _x _y
			exit
	`, `
		: test immediate
			swap
			100
	`,
		withVMStack(44, 99),
		expectVMStack(99, 44, 100))

	// inc is a pointer incrementing word.
	k.addSource("inc", `
		: inc
		  dup @
		  1 +
		  swap !
		  exit
	`, `
		: test immediate
			5 inc
	`,
		withVMMemAt(5, 99),
		expectVMMemAt(5, 100))

	// , is a standard FORTH word.
	// It should write the top of stack into the dictionary, and advance the pointer.
	k.addSource("compile", `
		: h 0 exit

		: ,
			h @
			!
			h inc
			exit
	`, `
		: test immediate
			42 ,
			exit
	`,
		expectVMH(1284),
		expectVMMemAt(1283, 42))

	// ; should be an immediate word that pushes the address of exit onto the
	// stack, then writes it out.
	k.addSource(";", `
		: ; immediate ' exit , exit

		: drop 0 * + ;
	`, `
		: test immediate
			drop
			108
	`,
		// NOTE can't drop the last element on the stack, due to the
		// coalescing-add causing an underflow
		withVMStack(9, 12),
		expectVMStack(9, 108))

	// dec is a pointer decrementing word.
	k.addSource("dec", `
		: dec
		  dup @
		  1 -
		  swap !
		  ;
	`, `
		: test immediate
			5 dec
	`,
		withVMMemAt(5, 99),
		expectVMMemAt(5, 98))

	// tor transfers a value from the data stack to the return stack.
	// It's life is complicated, due to needing to work around its own return.
	k.addSource("tor", `
		: tor
		  r @ @
		  swap
		  r @ !
		  r @ 1 + r !
		  r @ !
		  ;
	`, /* primarily useful as a way to manipulate control flow */ `
		: hello 
			'h' echo
			'i' echo
			;

		: test immediate
			' hello tor ;
			'n' echo
			'o' echo
			;
	`,
		expectVMOutput(`hi`))

	// fromr transfers a values from the return tack to the data stack.
	// It's life is complicated, due to needing to work around its own return.
	k.addSource("fromr", `
		: fromr
		  r @ @
		  r @ 1 - r !
		  r @ @
		  swap
		  r @ !
		  ;
	`, /* primarily useful as a way to access the compilation stream */ `
		: echoit
			fromr 1 +
			dup 1 + tor
			@ echo
			;

		: test immediate
			echoit 'a'
			'h' echo
			;
	`,
		expectVMOutput(`ah`))

	k.addSource("bool", `
		: minus 0 swap - ;
		: bnot 1 swap - ;
		: < - <0 ;
		: > minus < ;
		: logical
			dup
			0 <
			swap minus
			0 <
			+
			;
		: not logical bnot ;
		: = - not ;
	`, `
		: test immediate
			2 3 <
			3 2 <
			4 5 1 - =
			;
	`, expectVMStack(1, 0, 1))

	k.addSource("branch", `
		: branch
			r @ @
			@
			r @ @
			+
			r @ !
			;
		: computebranch 1 - * 1 + ;
		: notbranch
			not
			r @ @ @
			computebranch
			r @ @ +
			r @ !
			;

		: here h @ ;

		: if immediate
			' notbranch ,
			here
			0 ,
			;

		: then immediate
			dup
			here swap -
			swap !
			;
	`, `
		: test immediate
			'h' echo
			0 if
			'm' echo
			then
			'a' echo
	`, expectVMOutput(`ha`))

	k.addSource("tail", `
		: tail fromr fromr drop tor ;
	`, `
		: printn
			dup 0 < not if
				swap
				dup @ echo
				1 +
				swap 1 -
				tail printn
			then
			;

		: print
			dup @
			swap
			1 +
			swap
			1 -
			printn
			;

		: test immediate
			'"' echo
			here
				5 ,
				'h' ,
				'e' ,
				'l' ,
				'l' ,
				'o' ,
			print
			'"' echo
			;
	`, expectVMOutput(`"hello"`))

	k.addSource("comments", `
		: find-)
			key
			')' =
			not if
			tail find-)
			then ;
		: ( immediate find-) ;
	`, `
		: test immediate
			'/' echo
			( we should be able to do FORTH-style comments now )
			'/' echo
			;
	`, expectVMOutput(`//`))

	k.addSource("else", `
		: else immediate
		  ' branch ,            ( compile a definite branch )
		  here                  ( push the backpatching address )
		  0 ,                   ( compile a dummy offset for branch )
		  swap                  ( bring old backpatch address to top )
		  dup here swap -       ( calculate the offset from old address )
		  swap !                ( put the address on top and store it )
		;
	`, `
		: bit logical
			if   'y' echo
			else 'n' echo
			then ;

		: test immediate
			0 bit
			1 bit
			3 bit
			8 bit
			;
	`, expectVMOutput(`nyyy`))

	k.addSource("printnum", `
		: mod _x! _y!           ( get x then y off of stack )
		  _y _y _x / _x *       ( y - y / x * x )
		  -
		;

		: printnum
		  dup
		  10 mod '0' +
		  swap 10 / dup
		  if
			printnum
		  else
			drop
		  then
		  echo
		;

		: .
		  dup 0 <
		  if
			'-' echo minus
		  then
		  printnum
		  <sp> echo
		;

		: cr
			<nl> echo
			;
	`, `
		: test immediate
			99 .
			44 .
			100 .
			cr
			;
	`, expectVMOutput("99 44 100 \n"))

	k.addSource("more bools", `
		: > swap < ;
		: <= 1 + < ;
		: >= swap <= ;
	`, `
		: test immediate
			5 4 >
			5 5 >

			5 5 <=
			5 4 <=
			4 5 <=

			5 5 >=
			5 4 >=
			4 5 >=

			;
	`, expectVMStack(
		1, 0,
		1, 0, 1,
		1, 1, 0,
	))

	k.addSource("loop", `
		: i r @ 1 - @ ;
		: j r @ 3 - @ ;

		: do immediate
		  ' swap ,              ( compile 'swap' to swap the limit and start )
		  ' tor ,               ( compile to push the limit onto the return stack )
		  ' tor ,               ( compile to push the start on the return stack )
		  here                  ( save this address so we can branch back to it )
		;

		: inci
		  r @ 1 -               ( get the pointer to i )
		  inc                   ( add one to it )
		  r @ 1 - @             ( find the value again )
		  r @ 2 - @             ( find the limit value )
		  <
		  if
			r @ @ @
			r @ @ +
			r @ !
			exit          ( branch )
		  then
		  fromr 1 +
		  fromr drop
		  fromr drop
		  tor
		;

		: loop immediate ' inci , here - , ;

		: loopexit
		  fromr drop            ( pop off our return address )
		  fromr drop            ( pop off i )
		  fromr drop            ( pop off the limit of i )
		;                       ( and return to the caller's caller routine )
	`, `
		: test immediate
		  11 1 do i . loop cr
		  ;
	`, expectVMOutput("1 2 3 4 5 6 7 8 9 10 \n"))

	k.addSource("runtime define", `
		( :: is going to be a word that does ':' at runtime )
		: :: ; ;
		: fix-:: immediate 1 ' :: ! ; ( vmCodeDefine = 1 )
		fix-::

		( Override old definition of ':' with a new one that invokes ] )
		: : immediate :: r dec ] ;
	`, `
		: foo 'a' echo ;
		: bar 'b' echo ;

		: test immediate
		  foo bar foo bar
		  ;
	`, expectVMRStack(), expectVMOutput(`abab`))

	k.addSource("command mode", `
		: _z  5 @ exit
		: _z! 5 ! exit

		: execute
		  dup not if            ( execute an exit on behalf of caller )
		    fromr drop          ( pop up to the caller's return )
			8 !                 ( hacked unary drop of the exit code )
		    ;                   ( back to the caller's caller )
		  then
		  8 !                   ( write code into temporary region )
		  ' exit 9 !            ( with a following exit )
		  8 tor ;               ( jump into temporary region )

		: command
		  here _z!              ( store dict pointer in temp variable )
		  _read                 ( compile a word )
		                        ( if we get control back: )
		  here _z
		  = if
			tail command        ( we didn't compile anything )
		  then
		  here 1 - h !          ( decrement the dictionary pointer )
		  here _z               ( get the original value )
		  = if
			here @              ( get the word that was compiled )
			execute             ( and run it )
		  else
			here @              ( else it was an integer constant, so push it )
			here 1 - h !        ( and decrement the dictionary pointer again )
		  then
		  tail command
		;

		: [ immediate command ;
	`, `tron

		[ 42 . cr ;

		108 . cr
	`, expectVMStack(), expectVMOutput("42 \n"))

	k.tests.run(t)
}

// Test_third tests a, minimally modified copy of, the original third kernel code.
func Test_Third(t *testing.T) {
	t.Skip()
	vmTest("setup").
		withInputWriter(thirdKernel).
		withTestHexOutput().
		withTimeout(10*time.Second).
		withMemAt(255, 1 /* TRON */).
		run(t)
}

type kernel struct {
	names  []string
	inputs []string
	tests  vmTestCases
}

func (k *kernel) addSource(
	name, input, test string,
	wraps ...func(vmTestCase) vmTestCase,
) {
	vmt := vmTest(name)
	for i, name := range k.names {
		vmt = vmt.withNamedInput("kernel_"+name, k.inputs[i])
	}

	const tronCode = `
		: flags! rb @ 1 - ! exit
		: tron  immediate 1 flags! exit
		: troff immediate 0 flags! exit`
	tron := false

	if strings.HasPrefix(input, "tron") {
		input = input[4:]
		vmt = vmt.
			withNamedInput("tron", tronCode).
			withNamedInput("tron_over", "\ntron\n")
		tron = true
	}

	vmt = vmt.withNamedInput(name, input)

	if len(test) > 0 {
		if !tron {
			vmt = vmt.withNamedInput("tron", tronCode)
		}
		if strings.HasPrefix(test, "tron") {
			tron = true
		}
		vmt = vmt.withNamedInput("kernel_test_"+name, test)
		if !tron {
			vmt = vmt.withNamedInput("kernel_test_run", "\ntron test\n")
		}
		tron = true
	}

	vmt = vmt.apply(wraps...)

	k.names = append(k.names, name)
	k.inputs = append(k.inputs, input)
	k.tests = append(k.tests, vmt)
}
