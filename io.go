package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strconv"

	"github.com/jcorbin/gothird/internal/flushio"
	"github.com/jcorbin/gothird/internal/panicerr"
	"github.com/jcorbin/gothird/internal/runeio"
)

type inLoc struct {
	fileName string
	number   int
}

type inLine struct {
	inLoc
	bytes.Buffer
}

func (loc inLoc) String() string { return fmt.Sprintf("%v:%v", loc.fileName, loc.number) }
func (il inLine) String() string { return fmt.Sprintf("%v %q", il.inLoc, il.Buffer.String()) }

type ioCore struct {
	in      io.RuneReader
	inQueue []io.Reader

	lastLine inLine
	scanLine inLine

	out flushio.WriteFlusher

	closers []io.Closer
}

func (ioc *ioCore) readRune() (rune, error) {
	if ioc.in == nil && !ioc.nextIn() {
		return 0, io.EOF
	}

	r, _, err := ioc.in.ReadRune()
	if r == '\n' {
		ioc.nextLine()
	} else {
		ioc.scanLine.WriteRune(r)
	}

	if r != 0 {
		return r, nil
	}
	if err == io.EOF && ioc.nextIn() {
		err = nil
	}
	return 0, err
}

func (ioc *ioCore) nextLine() {
	ioc.lastLine.Reset()
	ioc.lastLine.fileName = ioc.scanLine.fileName
	ioc.lastLine.number = ioc.scanLine.number
	ioc.lastLine.Write(ioc.scanLine.Bytes())
	ioc.scanLine.Reset()
	ioc.scanLine.number++
}

func (ioc *ioCore) nextIn() bool {
	ioc.nextLine()
	if ioc.in != nil {
		if cl, ok := ioc.in.(io.Closer); ok {
			cl.Close()
		}
		ioc.in = nil
	}
	if len(ioc.inQueue) > 0 {
		r := ioc.inQueue[0]
		ioc.inQueue = ioc.inQueue[1:]
		ioc.in = runeio.NewReader(r)
		ioc.scanLine.fileName = nameOf(r)
		ioc.scanLine.number = 1
	}
	return ioc.in != nil
}

func (ioc *ioCore) Close() (err error) {
	for i := len(ioc.closers) - 1; i >= 0; i-- {
		if cerr := ioc.closers[i].Close(); err == nil {
			err = cerr
		}
	}
	return err
}

type named interface {
	Name() string
}

func nameOf(obj interface{}) string {
	if nom, ok := obj.(named); ok {
		return nom.Name()
	}
	return fmt.Sprintf("<unnamed %T>", obj)
}

func runMarkScanner(name string, out io.WriteCloser, sc scanner) io.WriteCloser {
	return runPipeWorker(name, func(r io.Reader) (rerr error) {
		ms := markScanner{
			Scanner: bufio.NewScanner(r),
			out:     out,
		}
		defer func() {
			if err := ms.Close(); rerr == nil {
				rerr = err
			}
		}()
		for ms.Scan() {
			sc.scan(&ms)
		}
		return ms.Err()
	})
}

func runPipeWorker(name string, fun func(r io.Reader) error) io.WriteCloser {
	pr, pw := io.Pipe()
	work := pipeWorker{
		name:        name,
		WriteCloser: pw,
		done:        make(chan error),
	}
	go work.run(pr, fun)
	return work
}

type pipeWorker struct {
	io.WriteCloser
	name string
	done chan error
}

func (work pipeWorker) Name() string { return "<" + work.name + ">" }

func (work pipeWorker) Close() error {
	err := work.WriteCloser.Close()
	if derr := <-work.done; err == nil {
		err = derr
	}
	return err
}

func (work pipeWorker) run(rc io.ReadCloser, fun func(r io.Reader) error) {
	defer close(work.done)
	work.done <- panicerr.Recover(work.name, func() error {
		defer rc.Close()
		err := fun(rc)
		return err
	})
}

func scanPipe(name string, scs ...scanner) func(out io.WriteCloser) io.WriteCloser {
	sc := scanners(scs...)
	return func(out io.WriteCloser) io.WriteCloser {
		return runMarkScanner(name, out, sc)
	}
}

func patternScanner(pattern *regexp.Regexp, ss ...subscanner) scanner {
	return regexpScanner{pattern, subscanners(ss...)}
}

type scanner interface {
	scan(ms *markScanner) bool
}

type subscanner interface {
	scan(ms *markScanner, submatch [][]byte) bool
}

func scanners(ss ...scanner) scanner {
	switch len(ss) {
	case 0:
		return nil
	case 1:
		return ss[0]
	default:
		return firstScanner(ss)
	}
}

func subscanners(ss ...subscanner) subscanner {
	switch len(ss) {
	case 0:
		return nil
	case 1:
		return ss[0]
	default:
		return firstSubscanner(ss)
	}
}

type firstScanner []scanner
type firstSubscanner []subscanner

type regexpScanner struct {
	*regexp.Regexp
	subscanner
}

func (sc regexpScanner) scan(ms *markScanner) bool {
	if submatch := sc.FindSubmatch(ms.Bytes()); len(submatch) > 0 {
		return sc.subscanner.scan(ms, submatch)
	}
	return false
}

func (ss firstScanner) scan(ms *markScanner) bool {
	for _, s := range ss {
		if s.scan(ms) {
			return true
		}
	}
	return false
}

func (ss firstSubscanner) scan(ms *markScanner, submatch [][]byte) bool {
	for _, s := range ss {
		if s.scan(ms, submatch) {
			return true
		}
	}
	return false
}

type markScanner struct {
	Last markBuffer
	*bufio.Scanner

	pend  bool
	prior bool
	out   io.WriteCloser
	err   error
}

func (sc *markScanner) Scan() bool {
	if sc.pend {
		sc.Next()
	}
	if sc.err != nil {
		return false
	}
	sc.pend = sc.Scanner.Scan()
	return sc.pend
}

func (sc *markScanner) Next() bool {
	if sc.pend && sc.err == nil {
		if err := sc.Flush(); err != nil {
			return false
		}
		sc.Last.Write(sc.Bytes())
		sc.prior = true
		sc.pend = false
	}
	return sc.err == nil
}

func (sc *markScanner) Flush() error {
	_, werr := sc.Last.WriteTo(sc.out)
	if sc.err == nil {
		sc.err = werr
	}
	return werr
}

func (sc *markScanner) Close() error {
	for sc.Last.level > 0 {
		sc.Last.closeMark()
	}
	err := sc.Flush()
	if cerr := sc.out.Close(); err == nil {
		err = cerr
	}
	return err
}

func (sc *markScanner) Err() error {
	if sc.err != nil {
		return sc.err
	}
	return sc.Scanner.Err()
}

const (
	openMark  = " {{{"
	closeMark = " }}}"
)

type markBuffer struct {
	lineBuffer
	level  int
	opened bool
}

func (buf *markBuffer) openMark() {
	buf.level++
	buf.WriteString(openMark)
	buf.opened = true
}

func (buf *markBuffer) closeMark() {
	if buf.opened {
		b := buf.Next(buf.Len())
		if i := bytes.Index(b, []byte(openMark)); i >= 0 {
			b = b[:i]
		}
		buf.Write(b)
		buf.opened = false
	} else if buf.level > 0 {
		buf.level--
		buf.WriteString(closeMark)
	}
}

func (buf *markBuffer) WriteTo(w io.Writer) (n int64, err error) {
	buf.opened = false
	return buf.lineBuffer.WriteTo(w)
}

type lineBuffer struct{ bytes.Buffer }

func (buf *lineBuffer) WriteTo(w io.Writer) (n int64, err error) {
	if b := buf.Bytes(); len(b) == 0 || b[len(b)-1] != '\n' {
		buf.WriteByte('\n')
	}
	return buf.Buffer.WriteTo(w)
}

type fmtBuf interface {
	Len() int
	Write(p []byte) (n int, err error)
	WriteByte(c byte) error
	WriteRune(r rune) (n int, err error)
	WriteString(s string) (n int, err error)
}

type vmDumper struct {
	vm  *VM
	out io.Writer

	addrWidth int
	words     []uint
	wordID    int

	rawWords bool
}

func (dump vmDumper) dump() {
	fmt.Fprintf(dump.out, "# VM Dump\n")
	fmt.Fprintf(dump.out, "  prog: %v\n", dump.vm.prog)

	dump.scanWords()
	fmt.Fprintf(dump.out, "  dict: %v\n", dump.words)

	dump.dumpStack()
	dump.dumpMem()
}

func (dump *vmDumper) dumpStack() {
	fmt.Fprintf(dump.out, "  stack: %v\n", dump.vm.stack)
}

func (dump *vmDumper) dumpMem() {
	retBase := uint(dump.vm.load(10))
	memBase := uint(dump.vm.load(11))

	if dump.addrWidth == 0 {
		dump.addrWidth = len(strconv.Itoa(int(dump.vm.memSize()))) + 1
	}
	if dump.words == nil {
		dump.scanWords()
	}
	dump.wordID = len(dump.words) - 1
	var buf lineBuffer
	for addr := uint(0); addr < uint(dump.vm.memSize()); {
		// section headers
		switch addr {
		case retBase:
			fmt.Fprintf(&buf, "# Return Stack @%v", retBase)
		case memBase:
			fmt.Fprintf(&buf, "# Main Memory @%v", memBase)
		}
		if buf.Len() > 0 {
			buf.WriteTo(dump.out)
		}

		fmt.Fprintf(&buf, "  @% *v ", dump.addrWidth, addr)
		n := buf.Len()

		addr = dump.formatMem(&buf, addr)
		if buf.Len() == n {
			buf.Reset()
		} else {
			buf.WriteTo(dump.out)
		}
	}
}

func (dump *vmDumper) formatMem(buf fmtBuf, addr uint) uint {
	val := dump.vm.load(addr)

	// low memory addresses
	if addr <= 11 {
		buf.WriteString(strconv.Itoa(val))
		switch addr {
		case 0:
			buf.WriteString(" dict")
		case 1:
			buf.WriteString(" ret")
		case 10:
			buf.WriteString(" retBase")
		case 11:
			buf.WriteString(" memBase")
		}
		return addr + 1
	}

	// other pre-return-stack addresses
	retBase := uint(dump.vm.load(10))
	if addr < retBase {
		if val != 0 {
			buf.WriteString(strconv.Itoa(val))
		}
		return addr + 1
	}

	// return stack addresses
	memBase := uint(dump.vm.load(11))
	if addr < memBase {
		if r := uint(dump.vm.load(1)); addr <= r {
			buf.WriteString(strconv.Itoa(dump.vm.load(addr)))
			buf.WriteString(" ret_")
			buf.WriteString(strconv.Itoa(int(addr - retBase)))
		}
		return addr + 1
	}

	// dictionary words
	if word := dump.word(); word != 0 && addr == word {
		buf.WriteString(": ")
		addr++

		dump.formatName(buf, dump.vm.load(addr))
		addr++

		switch code := uint(dump.vm.load(addr)); code {
		case vmCodeCompile, vmCodeCompIt:
			addr++
		default:
			buf.WriteByte(' ')
			buf.WriteString("immediate")
		}

		nextWord := dump.nextWord()
		if nextWord == 0 {
			nextWord = uint(dump.vm.load(0))
		}
		for addr < nextWord {
			buf.WriteByte(' ')
			if nextAddr := dump.formatCode(buf, addr); nextAddr > addr {
				addr = nextAddr
				continue
			}
			break
		}

		if dump.rawWords {
			code := make([]int, addr-word)
			dump.vm.loadInto(word, code)
			fmt.Fprintf(buf, "\n % *v %v", dump.addrWidth, "", code)
		}

		return addr
	}

	// other memory ranges
	if val != 0 {
		buf.WriteString(strconv.Itoa(val))
	}

	return addr + 1
}

func (dump *vmDumper) formatCode(buf fmtBuf, addr uint) uint {
	code := uint(dump.vm.load(addr))
	addr++

	// builtin code
	if code < vmCodeMax {
		buf.WriteString(vmCodeNames[code])
		if code == vmCodePushint {
			buf.WriteByte('(')
			buf.WriteString(strconv.Itoa(dump.vm.load(addr)))
			buf.WriteByte(')')
			addr++
		}
		return addr
	}

	// call to word+offset
	if i := sort.Search(len(dump.words), func(i int) bool {
		return dump.words[i] < code
	}); i < len(dump.words) {
		word := dump.words[i]
		dump.formatName(buf, dump.vm.load(word+1))
		if offset := code - word; offset > 0 {
			buf.WriteByte('+')
			buf.WriteString(strconv.Itoa(int(offset)))
		}
		return addr
	}

	// call to unknown address
	buf.WriteString(strconv.FormatUint(uint64(code), 10))
	return addr
}

func (dump *vmDumper) formatName(buf fmtBuf, sym int) {
	if sym == 0 {
		buf.WriteRune('ø')
	} else if nameStr := dump.vm.string(uint(sym)); nameStr != "" {
		buf.WriteString(nameStr)
	} else {
		fmt.Fprintf(buf, "UNDEFINED_NAME_%v", sym)
	}
}

func (dump *vmDumper) scanWords() {
	for word := dump.vm.last; word != 0; {
		if word >= uint(dump.vm.memSize()) {
			return
		}
		dump.words = append(dump.words, word)
		word = uint(dump.vm.load(word))
	}
}

func (dump *vmDumper) word() uint {
	if dump.wordID >= 0 {
		return dump.words[dump.wordID]
	}
	return 0
}

func (dump *vmDumper) nextWord() uint {
	if dump.wordID >= 0 {
		dump.wordID--
	}
	return dump.word()
}
