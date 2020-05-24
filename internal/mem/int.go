package mem

// DefaultIntsPageSize provides a default for Ints.PageSize.
const DefaultIntsPageSize = 255

// Ints implements an integer-oriented paged memory.
// Pages may not necessarily be the same size, but usually are in practice.
type Ints struct {
	PagedCore
	pages [][]int
}

// Size returns an address one position higher than the last position in the
// last page allocated so far.
func (m *Ints) Size() uint {
	if i := len(m.bases) - 1; i >= 0 {
		return m.bases[i] + uint(len(m.pages[i]))
	}
	return 0
}

// Load returns a single value from the given address.
// Unallocated pages are left unallocated, resulting in implicit 0 values.
// Returns an error if addr exceeds any MemLimit.
func (m *Ints) Load(addr uint) (int, error) {
	if err := m.checkLimit(addr, "load"); err != nil {
		return 0, err
	}

	if m.PageSize == 0 || len(m.pages) == 0 {
		return 0, nil
	}

	pageID := m.findPage(addr)
	base := m.bases[pageID]
	page := m.pages[pageID]
	if i := int(addr) - int(base); 0 <= i && i < len(page) {
		return page[i], nil
	}

	return 0, nil
}

// LoadInto reads len(buf) integers from memory starting at addr.
// Skips any unallocated pages, zeroing the result buffer where encountered.
// Returns an error if MemLimit would be exceeded; no partial load is done.
func (m *Ints) LoadInto(addr uint, buf []int) error {
	if len(buf) == 0 {
		return nil
	}

	end := addr + uint(len(buf))
	if err := m.checkLimit(end, "load"); err != nil {
		return err
	}

	for pageID := m.findPage(addr); addr < end && pageID < len(m.bases); pageID++ {
		base := m.bases[pageID]
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

		page := m.pages[pageID]
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

	for i := range buf {
		buf[i] = 0
	}

	return nil
}

// Stor stores any values at addr, allocating pages if necessary.
// Returns an error if MemLimit would be exceeded; no partial store is done.
func (m *Ints) Stor(addr uint, values ...int) error {
	if len(values) == 0 {
		return nil
	}

	end := addr + uint(len(values))
	if err := m.checkLimit(end, "stor"); err != nil {
		return err
	}

	if m.PageSize == 0 {
		m.PageSize = DefaultIntsPageSize
	}

	for pageID := m.findPage(addr); addr < end; pageID++ {
		base, size, page := m.allocPage(pageID, addr)
		if skip := addr - base; skip > 0 {
			if skip >= size {
				continue
			}
			base += skip
			page = page[skip:]
		}
		n := copy(page, values)
		values = values[n:]
		addr += uint(n)
	}

	return nil
}

func (m *Ints) allocPage(pageID int, addr uint) (base, size uint, page []int) {
	base, size, isNew := m.PagedCore.allocPage(pageID, addr)
	if isNew {
		page = make([]int, size)
		if pageID == len(m.bases) {
			m.pages = append(m.pages, page)
		} else {
			m.pages = append(m.pages, nil)
			copy(m.pages[pageID+1:], m.pages[pageID:])
			m.pages[pageID] = page
		}
	} else {
		page = m.pages[pageID]
	}
	return base, size, page
}
