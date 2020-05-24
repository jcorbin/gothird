package runeio

import (
	"errors"
	"strconv"
	"strings"
)

// ControlRune represents a named control unicode codepoint.
type ControlRune struct {
	N string
	R rune
}

// C0Ctls contains the classic ASCII control characters.
var C0Ctls = [32]ControlRune{
	{"<NUL>", 0x00},
	{"<SOH>", 0x01},
	{"<STX>", 0x02},
	{"<ETX>", 0x03},
	{"<EOT>", 0x04},
	{"<ENQ>", 0x05},
	{"<ACK>", 0x06},
	{"<BEL>", 0x07},
	{"<BS>", 0x08},
	{"<HT>", 0x09},
	{"<NL>", 0x0A},
	{"<VT>", 0x0B},
	{"<NP>", 0x0C},
	{"<CR>", 0x0D},
	{"<SO>", 0x0E},
	{"<SI>", 0x0F},
	{"<DLE>", 0x10},
	{"<DC1>", 0x11},
	{"<DC2>", 0x12},
	{"<DC3>", 0x13},
	{"<DC4>", 0x14},
	{"<NAK>", 0x15},
	{"<SYN>", 0x16},
	{"<ETB>", 0x17},
	{"<CAN>", 0x18},
	{"<EM>", 0x19},
	{"<SUB>", 0x1A},
	{"<ESC>", 0x1B},
	{"<FS>", 0x1C},
	{"<GS>", 0x1D},
	{"<RS>", 0x1E},
	{"<US>", 0x1F},
}

// PseudoCtls provides the typical mneumonis for space and delete.
var PseudoCtls = [2]ControlRune{
	{"<SP>", 0x20},
	{"<DEL>", 0x7F},
}

// C1Ctls contains the extended ISO-8859 control characters.
var C1Ctls = [32]ControlRune{
	{"<PAD>", 0x80},
	{"<HOP>", 0x81},
	{"<BPH>", 0x82},
	{"<NBH>", 0x83},
	{"<IND>", 0x84},
	{"<NEL>", 0x85},
	{"<SSA>", 0x86},
	{"<ESA>", 0x87},
	{"<HTS>", 0x88},
	{"<HTJ>", 0x89},
	{"<VTS>", 0x8A},
	{"<PLD>", 0x8B},
	{"<PLU>", 0x8C},
	{"<RI>", 0x8D},
	{"<SS2>", 0x8E},
	{"<SS3>", 0x8F},
	{"<DCS>", 0x90},
	{"<PU1>", 0x91},
	{"<PU2>", 0x92},
	{"<STS>", 0x93},
	{"<CCH>", 0x94},
	{"<MW>", 0x95},
	{"<SPA>", 0x96},
	{"<EPA>", 0x97},
	{"<SOS>", 0x98},
	{"<SGCI>", 0x99},
	{"<SCI>", 0x9A},
	{"<CSI>", 0x9B},
	{"<ST>", 0x9C},
	{"<OSC>", 0x9D},
	{"<PM>", 0x9E},
	{"<APC>", 0x9F},
}

func buildControlWords(table map[string]rune, ctls []ControlRune) {
	for _, ctl := range ctls {
		table[strings.ToUpper(ctl.N)] = ctl.R
		table[strings.ToLower(ctl.N)] = ctl.R
		if caret := CaretForm(ctl.R); caret != "" {
			table[caret] = ctl.R
		}
	}
}

// ControlWords maps control mnemonic strings to runes.
// Includes alias for caret forms like ^@ for <NUL>, ^C for <ETX>, and ^[ for <ESC> .
var ControlWords map[string]rune

func init() {
	ControlWords = make(map[string]rune, 3*(len(C0Ctls)+len(PseudoCtls)+len(C1Ctls)))
	buildControlWords(ControlWords, C0Ctls[:])
	buildControlWords(ControlWords, PseudoCtls[:])
	buildControlWords(ControlWords, C1Ctls[:])
}

// CaretForm computes the ^-escaped printable form of a C0 control rune.
func CaretForm(r rune) string {
	if r < 0x20 || r == 0x7f {
		return "^" + string(r^0x40)
	} else if 0x80 <= r && r <= 0x9f {
		return "^[" + string(r^0xc0)
	}
	return ""
}

var errInvalidRune = errors.New(`rune literal must be "^X" "<NAME>" or 'X'`)

// UnquoteRune extends the standard strconv.UnquoteChar parsing with additional
// mnemonics like <ESC> and caret-forms like ^[.
// Returns a parsed rune and true if successful, or 0 and false otherwise.
func UnquoteRune(token string) (rune, error) {
	if r, defined := ControlWords[token]; defined {
		return r, nil
	}

	runes := []rune(token)
	if len(runes) < 1 || runes[0] != '\'' {
		return 0, errInvalidRune
	}

	switch len(runes) {
	case 3:
		if runes[2] != '\'' {
			return 0, errInvalidRune
		}
	case 4:
		if runes[3] != '\'' {
			return 0, errInvalidRune
		}
	default:
		return 0, errInvalidRune
	}

	value, _, _, err := strconv.UnquoteChar(token[1:], '\'')
	return value, err
}
