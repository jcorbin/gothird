package main

import "fmt"

type memCore struct {
	pages [][]int
	bases []uint

	memLimit uint
	pageSize uint
}

func (mem *memCore) memSize() uint {
	if i := len(mem.bases) - 1; i >= 0 {
		return mem.bases[i] + mem.pageSize
	}
	return 0
}

func (mem *memCore) load(addr uint) (int, error) {
	if maxSize := mem.memLimit; maxSize != 0 && addr > maxSize {
		return 0, memLimitError{addr, "get"}
	}

	if mem.pageSize == 0 || len(mem.pages) == 0 {
		return 0, nil
	}

	pageID := mem.findPage(addr)
	if pageID < 0 {
		return 0, nil
	}

	base := mem.bases[pageID]
	page := mem.pages[pageID]
	if i := addr - base; int(i) < len(page) {
		return page[i], nil
	}

	return 0, nil
}

func (mem *memCore) loadInto(addr uint, buf []int) error {
	end := addr + uint(len(buf))
	if maxSize := mem.memLimit; maxSize != 0 && end > maxSize {
		return memLimitError{end, "get"}
	}

	defer func() {
		for i := range buf {
			buf[i] = 0
		}
	}()

	if len(buf) == 0 || mem.pageSize == 0 {
		return nil
	}

	pageID := mem.findPage(addr)

	if pageID < 0 {
		return nil
	}

	for ; addr < end && pageID < len(mem.bases); pageID++ {
		base := mem.bases[pageID]
		if base > end {
			break
		}

		if skip := int(base) - int(addr); skip > 0 {
			if skip >= len(buf) {
				break
			}
			addr += uint(skip)
			for i := range buf[:skip] {
				buf[i] = 0
			}
			buf = buf[skip:]
		}

		page := mem.pages[pageID]
		if skip := int(addr) - int(base); skip > 0 {
			if skip >= len(page) {
				continue
			}
			base += uint(skip)
			page = page[skip:]
		}

		n := copy(buf, page)
		buf = buf[n:]
		addr += uint(n)
	}
	return nil
}

func (mem *memCore) stor(addr uint, values ...int) error {
	end := addr + uint(len(values))
	if maxSize := mem.memLimit; maxSize != 0 && end > maxSize {
		return memLimitError{end, "stor"}
	}

	if len(values) == 0 {
		return nil
	}

	if mem.pageSize == 0 {
		mem.pageSize = defaultPageSize
	}

	for pageID := mem.findPage(addr); addr < end; pageID++ {
		if pageID == len(mem.bases) {
			base := addr / mem.pageSize * mem.pageSize
			size := mem.pageSize
			if i := len(mem.bases) - 1; i >= 0 {
				lastEnd := mem.bases[i] + uint(len(mem.pages[i]))
				if base < lastEnd {
					size -= lastEnd - base
					base = lastEnd
				}
			}
			mem.bases = append(mem.bases, base)
			mem.pages = append(mem.pages, make([]int, size))
		}

		base := mem.bases[pageID]
		if addr < base {
			nextBase := base
			base = addr / mem.pageSize * mem.pageSize
			size := mem.pageSize
			if gapSize := nextBase - base; size > gapSize {
				size = gapSize
			}
			mem.bases = append(mem.bases, 0)
			mem.pages = append(mem.pages, nil)
			copy(mem.bases[pageID+1:], mem.bases[pageID:])
			copy(mem.pages[pageID+1:], mem.pages[pageID:])
			mem.bases[pageID] = base
			mem.pages[pageID] = make([]int, size)
		}

		page := mem.pages[pageID]
		if skip := int(addr) - int(base); skip > 0 {
			if skip >= len(page) {
				continue
			}
			base += uint(skip)
			page = page[skip:]
		}

		n := copy(page, values)
		values = values[n:]
		addr += uint(n)
	}
	return nil
}

func (mem *memCore) findPage(addr uint) int {
	i, j := 0, len(mem.bases)
	for i < j {
		h := int(uint(i+j)>>1) + 1
		if h < len(mem.bases) && mem.bases[h] <= addr {
			i = h
		} else {
			j = h - 1
		}
	}
	return i
}

type memLimitError struct {
	addr uint
	op   string
}

func (lim memLimitError) Error() string {
	return fmt.Sprintf("memory limit exceeded by %v @%v", lim.op, lim.addr)
}
