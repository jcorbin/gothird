package main

import (
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"text/template"

	"github.com/jcorbin/gothird/internal/mem"
)

var testThirdKernel = kernel{name: "third"}

// Test_kernel tests a layered recreation of the THIRD kernel, with a test case
// validating each each layer.
func Test_kernel(t *testing.T) {
	// Test that builtin word initialization works.
	testThirdKernel.addSource("builtins", `
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
	testThirdKernel.addSource("main", `
		: r 1 exit

		: ]
		  r @
		  1 -
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
			/* @1105 */ 15, 1, vmCodeSub,
			/* @1108 */ 1096, vmCodeSet,
			/* @1110 */ vmCodeRead,
			/* @1111 */ 1103,
		),
		expectVMWord(1112, "main", vmCodeRun, 1103),
		expectVMWord(1116, "rb", vmCodeCompile, vmCodeRun, vmCodePushint, 10, vmCodeExit),
		expectVMStack(256))

	// Synthesize addition and test it.
	testThirdKernel.addSource("add", `
		: _x  3 @ exit
		: _x! 3 ! exit

		: + _x! 0 _x - - exit
	`, `
		: test immediate
			3 5 7 + +
	`,
		expectVMWord(1123, "_x",
			/* 1125 */ vmCodeCompile, vmCodeRun,
			/* 1127 */ vmCodePushint, 3, vmCodeGet,
			/* 1130 */ vmCodeExit),
		expectVMWord(1131, "_x!",
			/* 1133 */ vmCodeCompile, vmCodeRun,
			/* 1135 */ vmCodePushint, 3, vmCodeSet,
			/* 1138 */ vmCodeExit),
		expectVMWord(1139, "+",
			/* 1141 */ vmCodeCompile, vmCodeRun,
			/* 1143 */ 1135, // _x!
			/* 1146 */ vmCodePushint, 0,
			/* 1147 */ 1127, // _x
			/* 1148 */ vmCodeSub, vmCodeSub,
			/* 1150 */ vmCodeExit),
		expectVMStack(3+5+7))

	// ' is a standard FORTH word.
	// It should push the address of the word that follows it onto the stack.
	testThirdKernel.addSource("quote", `
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
	testThirdKernel.addSource("reboot", `
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
		expectVMRStack(1111),
		expectVMStack(42),
		expectVMError(mem.LimitError{1024 * 1024, "load"}))

	// swap two values on the top of the stack.
	testThirdKernel.addSource("swap", `
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
	testThirdKernel.addSource("inc", `
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
	testThirdKernel.addSource("compile", `
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
		expectVMH(1285),
		expectVMMemAt(1284, 42))

	// ; should be an immediate word that pushes the address of exit onto the
	// stack, then writes it out.
	testThirdKernel.addSource(";", `
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
	testThirdKernel.addSource("dec", `
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
	testThirdKernel.addSource("tor", `
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
	testThirdKernel.addSource("fromr", `
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

	testThirdKernel.addSource("bool", `
		: minus 0 swap - ;
		: bnot 1 swap - ;
		: < - <0 ;
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

	testThirdKernel.addSource("branch", `
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

	testThirdKernel.addSource("tail", `
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

	testThirdKernel.addSource("comments", `
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

	testThirdKernel.addSource("else", `
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

	testThirdKernel.addSource("printnum", `
		: mod _x! _y!           ( get x then y off of stack )
		  _y _y _x / _x *       ( y - y / x * x )
		  -
		;

		: printnum
		  dup
		  10 mod '0' +
		  swap 10 / dup
		  if
			printnum 0
		  then
		  drop echo
		;

		: .
		  dup 0 <
		  if
			'-' echo minus
		  then
		  printnum
		  <sp> echo
		;

		: nl <nl> echo ;
	`, `
		: test immediate
			99 .
			44 .
			100 .
			nl
			;
	`, expectVMOutput("99 44 100 \n"))

	testThirdKernel.addSource("more bools", `
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

	testThirdKernel.addSource("loop", `
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
		  11 1 do i . loop nl
		  ;
	`, expectVMOutput("1 2 3 4 5 6 7 8 9 10 \n"))

	testThirdKernel.addSource("runtime define", `
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

	testThirdKernel.addSource("command mode", `
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

		[ 42 . nl ;

		108 . nl
	`, expectVMStack(), expectVMOutput("42 \n"))

	testThirdKernel.addSource("tron", tronCode, "")

	testThirdKernel.tests.run(t)
}

const tronCode = `
	: flags! rb @ 1 - ! exit
	: tron  immediate 1 flags! exit
	: troff immediate 0 flags! exit`

var genThirdFlag = flag.Bool("generate-third", false,
	"generate the third.go kernel source from the tested sources in third_test.go")

// Test_third tests a, minimally modified copy of, the original third kernel code.
func Test_Third(t *testing.T) {
	t.Skip()
	vmTest("third").
		withInputWriter(thirdKernel).
		withNamedInput("test", `tron [
		  11 1 do i . loop nl
		`).
		expectOutput("1 2 3 4 5 6 7 8 9 10 \n").
		run(t)
}

func TestMain(m *testing.M) {
	flag.Parse()

	exitCode := m.Run()

	if *genThirdFlag && exitCode == 0 {
		if err := generateFile("third.go", func(w io.Writer) error {
			return withGoimports(w, func(w io.Writer) error {
				io.WriteString(w, "package main\n\n")
				fmt.Fprintf(w, "//go:generate go test -generate-third .\n\n")
				return kernelTmpl.Execute(w, testThirdKernel)
			})
		}); err != nil {
			fmt.Fprintf(os.Stderr, "ERROR generating tested third kernel: %+v\n", err)
			exitCode = 1
		}
	}

	os.Exit(exitCode)
}

var kernelTmpl = template.Must(template.New("").Parse(`
const {{ .SourceName }} = {{ .QuotedSource }}

type {{ .TypeName }} struct{}

func ({{ .TypeName }}) Name() string { return "{{ .FileName }}" }

func ({{ .TypeName }}) WriteTo(w io.Writer) (_ int64, err error) {
	var n int
	if sw, ok := w.(io.StringWriter); ok {
		n, err = sw.WriteString({{ .SourceName }})
	} else {
		n, err = w.Write([]byte({{ .SourceName }}))
	}
	return int64(n), err
}

func ({{ .TypeName }}) Open() io.Reader {
	return struct {
		{{ .TypeName }}
		io.Reader
	}{Reader: strings.NewReader({{ .SourceName }})}
}

var {{ .VarName }} {{ .TypeName }}
`))

func (k kernel) FileName() string   { return k.name }
func (k kernel) SourceName() string { return "_" + k.name + "Source" }
func (k kernel) TypeName() string   { return "_" + k.name + "Kernel" }
func (k kernel) VarName() string    { return k.name + "Kernel" }
func (k kernel) QuotedSource() string {
	const includeNameComments = false
	var sb strings.Builder
	var quote rune
	for i, input := range k.inputs {
		input = strings.TrimRight(input, " \t\n")

		if includeNameComments {
			if quote != 0 {
				sb.WriteRune(quote)
				sb.WriteRune('+')
				sb.WriteRune('\n')
				quote = 0
			}
			fmt.Fprintf(&sb, "/* %v */", k.names[i])
		}

		first := true
		indent := ""
		for _, line := range strings.Split(input, "\n") {
			// detect indent from initial non-empty line
			if first && len(line) > 0 {
				rest := strings.TrimLeft(line, " \t")
				if n := len(line) - len(rest); n > 0 {
					indent = line[:n]
				}
				first = false
			}

			// trim any indent
			if indent != "" {
				line = strings.TrimPrefix(line, indent)
			}

			// determine needed quoting
			needQuote := '`'
			if !strconv.CanBackquote(line) {
				line = strconv.Quote(line)
				needQuote = []rune(line)[0]
				line = line[1 : len(line)-1]
			}

			// transition quote state
			if quote != needQuote {
				if quote != 0 {
					sb.WriteRune(quote)
					sb.WriteRune('+')
					sb.WriteRune('\n')
				}
				sb.WriteRune(needQuote)
				quote = needQuote
			}

			// write line content
			sb.WriteString(line)

			// write line end
			if quote == '`' {
				sb.WriteRune('\n')
			} else {
				sb.WriteString(`\n`)
			}
		}
	}
	if quote != 0 {
		sb.WriteRune(quote)
	}
	return sb.String()
}

type kernel struct {
	name   string
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

func withGoimports(out io.Writer, under func(w io.Writer) error) (rerr error) {
	goimp := exec.Command("goimports")

	in, err := goimp.StdinPipe()
	if err != nil {
		return err
	}
	defer in.Close()

	goimp.Stdout = out
	goimp.Stderr = os.Stderr
	if err := goimp.Start(); err != nil {
		return err
	}
	defer func() {
		if werr := goimp.Wait(); rerr == nil {
			rerr = werr
		}
	}()

	defer func() {
		if cerr := in.Close(); rerr == nil {
			rerr = cerr
		}
	}()

	return under(in)
}

func generateFile(name string, under func(w io.Writer) error) (rerr error) {
	f, err := ioutil.TempFile(filepath.Dir(name), name+".tmp*")
	if err != nil {
		return err
	}
	defer func() {
		if rerr == nil {
			rerr = renameTempFile(name, f)
		}
		if rerr != nil {
			os.Remove(f.Name())
		}
	}()
	return under(f)
}

func renameTempFile(dest string, f *os.File) error {
	if err := f.Sync(); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), dest)
}
