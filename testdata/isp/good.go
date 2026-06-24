package isp

// Reader is a small, focused interface (Go idiomatic).
// solid:want ISP=ok reason="single-method role interface (io.Reader idiom); minimal client coupling"
type Reader interface {
	Read(p []byte) (n int, err error)
}

// Writer is a small, focused interface.
// solid:want ISP=ok reason="single-method role interface; minimal client coupling"
type Writer interface {
	Write(p []byte) (n int, err error)
}

// ReadWriter composes two small interfaces.
// solid:want ISP=ok reason="composed from small role interfaces via embedding (io.ReadWriter pattern); ISP-faithful"
type ReadWriter interface {
	Reader
	Writer
}

// SimpleReader implements only the Reader interface.
// solid:want ISP=ok reason="single-method public surface (Read); minimal client coupling"
type SimpleReader struct {
	data []byte
	pos  int
}

func (r *SimpleReader) Read(p []byte) (int, error) {
	n := copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}
