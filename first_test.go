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

	testCases.run(t)
}
