package isp

// Reader is a small, focused interface (Go idiomatic).
type Reader interface {
	Read(p []byte) (n int, err error)
}

// Writer is a small, focused interface.
type Writer interface {
	Write(p []byte) (n int, err error)
}

// ReadWriter composes two small interfaces.
type ReadWriter interface {
	Reader
	Writer
}

// SimpleReader implements only the Reader interface.
type SimpleReader struct {
	data []byte
	pos  int
}

func (r *SimpleReader) Read(p []byte) (int, error) {
	n := copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}
