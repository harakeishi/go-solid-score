package cgopkg

/*
struct point { int x; int y; };
static struct point origin = {0, 0};
*/
import "C"

// Wrapper is a real user-defined struct in a cgo package. It must be detected.
type Wrapper struct {
	px C.int
}

func (w *Wrapper) Get() int { return int(w.px) }

// Origin references a C struct value, which makes cgo synthesize a Go-side
// _Ctype_struct_point type in the generated files.
func Origin() C.struct_point { return C.origin }
