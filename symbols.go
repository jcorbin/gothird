package main

type symbols struct {
	strings []string
	symbols map[string]uint
}

func (sym symbols) string(id uint) string {
	if i := int(id) - 1; i >= 0 && i < len(sym.strings) {
		return sym.strings[i]
	}
	return ""
}

func (sym symbols) symbol(s string) uint {
	return sym.symbols[s]
}

func (sym *symbols) symbolicate(s string) (id uint) {
	id, defined := sym.symbols[s]
	if !defined {
		if sym.symbols == nil {
			sym.symbols = make(map[string]uint)
		}
		id = uint(len(sym.strings)) + 1
		sym.strings = append(sym.strings, s)
		sym.symbols[s] = id
	}
	return id
}
