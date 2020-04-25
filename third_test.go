package main

import (
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
		test
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

	// Build the standard FORTH quote word, and test it.
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
		test
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

	// k.addSource("hello", "", `
	// 	: digit '0' + echo exit

	// 	: test immediate
	// 		0         digit
	// 		10 3 -    digit
	// 		21 3 /    digit
	// 		9 2 3 * - digit
	// 		2 2 *     digit
	// 		'\n'      echo
	// 		exit
	// 	test
	// `, expectVMOutput("07734\n"))

	// k.addSource("ansi literals", "", `
	// 	: digit '0' + echo exit

	// 	: sgr_reset
	// 		<CSI> echo
	// 		'0'   echo
	// 		'm'   echo
	// 		exit

	// 	: sgr_fg
	// 		<CSI> echo
	// 		'3'   echo
	// 		'0' + echo
	// 		'm'   echo
	// 		exit

	// 	: test immediate
	// 		sgr_reset 2 sgr_fg
	// 			'S' echo
	// 			'u' echo
	// 			'p' echo
	// 			'e' echo
	// 			'r' echo
	// 		sgr_reset
	// 		<cr> echo
	// 		<nl> echo
	// 		exit
	// 	test
	// `, expectVMOutput("\x1b[0m\x1b[32mSuper\x1b[0m\r\n"))

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
