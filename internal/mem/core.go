package mem

import "fmt"

// PagedCore provides functionality common to any paged memory model.
type PagedCore struct {
	// PageSize specifies the length for newly allocated pages.
	PageSize uint

	// Limit specifies a limit, past which any store or load should result in an error.
	Limit uint

	bases []uint
	sizes []uint
}

// LimitError indicates that a memory operation, like load or store, exceeded a limit.
type LimitError struct {
	Addr uint
	Op   string
}

func (lim LimitError) Error() string {
	return fmt.Sprintf("memory limit exceeded by %v @%v", lim.Op, lim.Addr)
}

func (m *PagedCore) findPage(addr uint) int {
	i, j := 0, len(m.bases)
	for i < j {
		h := int(uint(i+j)>>1) + 1
		if h < len(m.bases) && m.bases[h] <= addr {
			i = h
		} else {
			j = h - 1
		}
	}
	return i
}

func (m *PagedCore) allocPage(pageID int, addr uint) (base, size uint, isNew bool) {
	if pageID == len(m.bases) {
		base = addr / m.PageSize * m.PageSize
		size = m.PageSize
		if i := len(m.bases) - 1; i >= 0 {
			lastEnd := m.bases[i] + m.sizes[i]
			if base < lastEnd {
				size -= lastEnd - base
				base = lastEnd
			}
		}
		m.bases = append(m.bases, base)
		m.sizes = append(m.sizes, size)
		return base, size, true
	}

	base = m.bases[pageID]
	if addr < base {
		size = m.PageSize
		nextBase := base
		base = addr / m.PageSize * m.PageSize
		if gapSize := nextBase - base; size > gapSize {
			size = gapSize
		}
		m.bases = append(m.bases, 0)
		m.sizes = append(m.sizes, 0)
		copy(m.bases[pageID+1:], m.bases[pageID:])
		copy(m.sizes[pageID+1:], m.sizes[pageID:])
		m.bases[pageID] = base
		m.sizes[pageID] = size
		return base, size, true
	}

	return base, m.sizes[pageID], false
}

func (m *PagedCore) checkLimit(addr uint, op string) error {
	if maxSize := m.Limit; maxSize != 0 && addr > maxSize {
		return LimitError{addr, op}
	}
	return nil
}
