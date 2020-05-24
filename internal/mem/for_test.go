package mem

// IntsDump provides data for testing.
type IntsDump struct {
	Bases []uint
	Sizes []uint
	Pages [][]int
}

// Dump memory data for testing.
func (m *Ints) Dump() (d IntsDump) {
	d.Bases = m.bases
	d.Sizes = m.sizes
	d.Pages = m.pages
	return d
}
