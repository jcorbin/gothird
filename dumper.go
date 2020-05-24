package main

import (
	"fmt"
	"io"
	"sort"
	"strconv"
)

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
