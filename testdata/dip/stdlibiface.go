package dip

import "io"

// Concrete is a plain concrete collaborator (not whitelisted).
type Concrete struct{ x int }

// MixedStdlibIface depends on whitelisted stdlib interfaces (io.Reader,
// io.Writer) AND one concrete type. Those interface dependencies are
// abstractions and must count toward the DIP ratio just like a user-defined
// interface would — being in the stdlib whitelist must not erase their
// abstraction credit. With 2 abstract + 1 concrete owned dependencies the ratio
// is 2/3 ≈ 67 (above the DIP threshold), not 0 (which is what erasing the
// whitelisted interfaces would produce).
// solid:want DIP=ok reason="depends on io.Reader/io.Writer abstractions plus one concrete; whitelisted stdlib interfaces still count as abstraction dependencies"
type MixedStdlibIface struct {
	r io.Reader
	w io.Writer
	c *Concrete
}

func (m *MixedStdlibIface) Do() {}
