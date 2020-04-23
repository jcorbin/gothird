package main

import (
	"testing"
)

func Test_VM(t *testing.T) {
	var testCases vmTestCases

	// primitive tests that work by driving individual VM methods
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
		under0    = (*VM).under0
		exit      = (*VM).exit
		echo      = (*VM).echo
		key       = (*VM).key
		pick      = (*VM).pick
		pushint   = (*VM).pushint
		step      = (*VM).step
	)
	testCases = append(testCases,
		// binary integer operation on the stack
		vmTest("sub").withStack(5, 3, 1).do(sub).expectStack(5, 2),
		vmTest("div").withStack(7, 13, 3).do(div).expectStack(7, 4),
		vmTest("mul").withStack(11, 5, 6).do(mul).expectStack(11, 30),

		// is top of stack less than 0?
		vmTest("less true").withStack(2, -3).do(under0).expectStack(2, 1),
		vmTest("less false").withStack(2, 3).do(under0).expectStack(2, 0),

		// pop top of stack, use as index into stack and copy up that element
		vmTest("pick 0").withStack(1, 2, 3, 4, 5, 0).do(pick).expectStack(1, 2, 3, 4, 5, 5),
		vmTest("pick 1").withStack(1, 2, 3, 4, 5, 1).do(pick).expectStack(1, 2, 3, 4, 5, 4),
		vmTest("pick 2").withStack(1, 2, 3, 4, 5, 2).do(pick).expectStack(1, 2, 3, 4, 5, 3),
		vmTest("pick 3").withStack(1, 2, 3, 4, 5, 3).do(pick).expectStack(1, 2, 3, 4, 5, 2),
		vmTest("pick 4").withStack(1, 2, 3, 4, 5, 4).do(pick).expectStack(1, 2, 3, 4, 5, 1),
		vmTest("pick 5").withStack(1, 2, 3, 4, 5, 5).do(pick).expectStack(1, 2, 3, 4, 5, 0),

		// read from memory
		vmTest("get").withMemAt(1024, 99, 42, 108).withStack(1025).do(get).expectStack(42),

		// write to memory
		vmTest("set").withMemAt(1024, 0, 0, 0).withStack(108, 1025).do(set).expectMemAt(1024, 0, 108, 0),

		// push an immediate value onto the stack
		vmTest("pushint").withMemAt(1024, 99, 42, 108).withProg(1025).do(pushint).expectStack(42).expectProg(1026),

		// compile the program counter
		vmTest("compileme").withMemAt(256, 2048).withR(256).withMemAt(1024,
			0,             // 1024: word prev
			vmCodeSub,     // 1025: ... name
			vmCodeCompile, // 1026: ...
			vmCodeRun,     // 1027: ...       <-- prog
			vmCodeSub,     // 1028: ...
			vmCodeExit,    // 1029: ...
			0,             // 1030:
			0,             // 1031:           <-- h
			0,             // 1032:
			0,             // 1033:
		).withProg(1027).withH(1031).do(compileme).expectMemAt(1024,
			0,             // 1024: word prev
			vmCodeSub,     // 1025: ... name
			vmCodeCompile, // 1026: ...
			vmCodeRun,     // 1027: ...
			vmCodeSub,     // 1028: ...
			vmCodeExit,    // 1029: ...
			0,             // 1030:
			1028,          // 1031:
			0,             // 1032:           <-- h
			0,             // 1033:
			//                ....:
			//                2048:           <-- prog
		).expectProg(2048).expectH(1032),

		// compile the header of a definition
		vmTest("define").withMemAt(1024,
			0, // 1024: <-- h
			0, // 1025:
			0, // 1026:
			0, // 1027:
			0, // 1028:
			0, // 1029:
			0, // 1030:
		).withH(1024).withInput(` : `).do(define).expectMemAt(1024,
			0,             // 1024: word prev
			1,             // 1025: word name
			vmCodeCompile, // 1026: ...
			vmCodeRun,     // 1027: ...
			0,             // 1028:           <-- h
			0,             // 1029:
			0,             // 1030:
		).expectH(1028).expectLast(1024).expectString(1, ":"),

		// modify the header to create an immediate word
		vmTest("immediate").withMemAt(1024,
			0, // 1024: <-- h
			0, // 1025:
			0, // 1026:
			0, // 1027:
			0, // 1028:
			0, // 1029:
			0, // 1030:
		).withH(1024).withInput(`:`).do(define, immediate).expectMemAt(1024,
			0,         // 1024: word prev
			1,         // 1025: ... name
			vmCodeRun, // 1026: ...
			vmCodeRun, // 1027: ...           <-- h
			0,         // 1028:
			0,         // 1029:
		).expectH(1027).expectString(1, ":"),

		// stop running the current function
		vmTest("exit").withRetBase(256,
			1024, // 256: ret[0]
			1025, // 257: ret[1]
			1026, // 258: ret[2]
			1027, // 259: ret[3]
		).withMemBase(1024, // NOTE this simply explicates the default 0 == exit value
			vmCodeExit, // 1024:
			vmCodeExit, // 1025:
			vmCodeExit, // 1026:
			vmCodeExit, // 1027:
		).do(exit, step, nil).expectMemAt(256, // NOTE return stack not zerod
			1024, // 256: ret[0]
			1025, // 257: ret[1]
			1026, // 258: ret[2]
			1027, // 259: ret[3]
		).expectProg(1025).expectR(255),

		// read a word from input and compile a pointer to it
		vmTest("read").withStrings(
			1, "foo",
			2, "bar",
		).withMemAt(1024,
			0,             // 1024:
			1,             // 1025:
			vmCodeCompile, // 1026:
			vmCodeRun,     // 1027:
			vmCodeSub,     // 1028:
			vmCodeExit,    // 1029:           <-- prog
			1024,          // 1030:           <-- last
			2,             // 1031:
			vmCodeRun,     // 1032:
			vmCodeRun,     // 1033:           <-- h
			0,             // 1034:
			0,             // 1035:
			0,             // 1036:
		).withH(1033).withProg(1029).withLast(1030).withInput("foo").do(
			read, step, nil,
		).expectMemAt(1024,
			0,             // 1024:
			1,             // 1025:
			vmCodeCompile, // 1026:
			vmCodeRun,     // 1027:
			vmCodeSub,     // 1028:
			vmCodeExit,    // 1029:
		).expectMemAt(1030,
			1024,      // 1030:
			2,         // 1031:
			vmCodeRun, // 1032:
			1028,      // 1033:
			0,         // 1034:           <-- h
			0,         // 1035:
			0,         // 1036:
		).expectH(1034),

		// output one character
		// input one character
		vmTest("key^2 => echo^2").withInput("ab").do(key, key, echo, echo).expectOutput("ba"),
	)
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
	const tronCode = `
		: _flags! rb @ 1 - ! exit
		: _tron  immediate 1 _flags! exit
		: _troff immediate 0 _flags! exit
		_tron`

	vmt := vmTest(name)
	for i, name := range k.names {
		vmt = vmt.withNamedInput("kernel_"+name, k.inputs[i])
	}
	vmt = vmt.withNamedInput(name, input)
	if len(test) > 0 {
		vmt = vmt.
			withNamedInput("tron", tronCode).
			withNamedInput("kernel_"+name+"_test", test)
	}
	vmt = vmt.apply(wraps...)

	k.names = append(k.names, name)
	k.inputs = append(k.inputs, input)
	k.tests = append(k.tests, vmt)
}

func Test_kernel(t *testing.T) {
	var k kernel

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
		test
	`,
		expectVMWord(1092, "r",
			/* 1094 */ vmCodeCompile, vmCodeRun,
			/* 1096 */ vmCodePushint, 1,
			/* 1098 */ vmCodeExit,
		),
		expectVMWord(1099, "]",
			/* @1101 */ vmCodeCompile,
			/* @1102 */ vmCodeRun,
			/* @1103 */ 1096, vmCodeGet,
			/* @1105 */ 1096, vmCodeSub,
			/* @1107 */ 1096, vmCodeSet,
			/* @1109 */ vmCodeRead,
			/* @1110 */ 1103,
		),
		expectVMWord(1111, "main",
			/* @1113 */ vmCodeRun,
			/* @1114 */ 1103,
		),
		expectVMWord(1115, "rb",
			/* 1117 */ vmCodeCompile, vmCodeRun,
			/* 1119 */ vmCodePushint, 10,
			/* 1121 */ vmCodeExit,
		),
		expectVMStack(256))

	k.addSource("add", `
		: _x  3 @ exit
		: _x! 3 ! exit
		: + _x! 0 _x - - exit
	`, `
		: test immediate
			3 5 7 + +
		test
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

	k.addSource("hello", "", `
		: digit '0' + echo exit

		: test immediate
			0         digit
			10 3 -    digit
			21 3 /    digit
			9 2 3 * - digit
			2 2 *     digit
			'\n'      echo
			exit
		test
	`, expectVMOutput("07734\n"))

	k.addSource("ansi literals", "", `
		: digit '0' + echo exit

		: sgr_reset
			<CSI> echo
			'0'   echo
			'm'   echo
			exit

		: sgr_fg
			<CSI> echo
			'3'   echo
			'0' + echo
			'm'   echo
			exit

		: test immediate
			sgr_reset 2 sgr_fg
				'S' echo
				'u' echo
				'p' echo
				'e' echo
				'r' echo
			sgr_reset
			<cr> echo
			<nl> echo
			exit
		test
	`, expectVMOutput("\x1b[0m\x1b[32mSuper\x1b[0m\r\n"))

	// TODO breakout "quote"

	k.addSource("reboot", `
		: dup _x! _x _x exit

		: '
			r @ @
			dup
			1 +
			r @ !
			@
			exit

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
		test
	`,
		expectVMRStack(1110),
		expectVMStack(42),
		expectVMError(memLimitError{1024 * 1024, "get"}))

	// : + _x! 0 _x - - exit

	// : _y  4 @ exit
	// : _y! 4 ! exit
	// : _z  5 @ exit
	// : _z! 5 ! exit

	k.tests.run(t)
}
