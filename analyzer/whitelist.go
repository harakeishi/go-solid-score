package analyzer

import "strings"

// stdlibWhitelist contains standard library and builtin types that are acceptable
// as concrete dependencies (not penalized by DIP analyzer).
var stdlibWhitelist = map[string]bool{
	// Builtins
	"string": true, "int": true, "int8": true, "int16": true, "int32": true, "int64": true,
	"uint": true, "uint8": true, "uint16": true, "uint32": true, "uint64": true,
	"float32": true, "float64": true, "complex64": true, "complex128": true,
	"bool": true, "byte": true, "rune": true, "uintptr": true,
	"error": true, "any": true, "comparable": true,

	// Common stdlib types
	"context.Context":     true,
	"time.Time":           true,
	"time.Duration":       true,
	"time.Location":       true,
	"sync.Mutex":          true,
	"sync.RWMutex":        true,
	"sync.WaitGroup":      true,
	"sync.Once":           true,
	"sync.Map":            true,
	"sync.Pool":           true,
	"bytes.Buffer":        true,
	"strings.Builder":     true,
	"strings.Reader":      true,
	"regexp.Regexp":       true,
	"url.URL":             true,
	"net.IP":              true,
	"net.Conn":            true,
	"http.Request":        true,
	"http.Response":       true,
	"http.ResponseWriter": true,
	"http.Client":         true,
	"http.Server":         true,
	"http.Handler":        true,
	"http.HandlerFunc":    true,
	"json.Decoder":        true,
	"json.Encoder":        true,
	// log.Logger is concrete - not whitelisted for DIP
	"slog.Logger":       true,
	"slog.Handler":      true,
	"testing.T":         true,
	"testing.B":         true,
	"testing.M":         true,
	"os.File":           true,
	"os.Signal":         true,
	"big.Int":           true,
	"big.Float":         true,
	"template.Template": true,
	// sql.DB, sql.Tx etc. are concrete - not whitelisted for DIP
	"tls.Config":               true,
	"x509.Certificate":         true,
	"exec.Cmd":                 true,
	"filepath.WalkFunc":        true,
	"context.CancelFunc":       true,
	"atomic.Value":             true,
	"atomic.Int32":             true,
	"atomic.Int64":             true,
	"atomic.Bool":              true,
	"flag.FlagSet":             true,
	"io.ReadCloser":            true,
	"io.WriteCloser":           true,
	"io.ReadWriteCloser":       true,
	"io.ReadWriter":            true,
	"io.Reader":                true,
	"io.Writer":                true,
	"io.Closer":                true,
	"io.Seeker":                true,
	"io.ReaderAt":              true,
	"io.WriterAt":              true,
	"io.ReadSeeker":            true,
	"io.WriteSeeker":           true,
	"io.ReadWriteSeeker":       true,
	"io.LimitedReader":         true,
	"io.SectionReader":         true,
	"io.PipeReader":            true,
	"io.PipeWriter":            true,
	"fs.FS":                    true,
	"fs.File":                  true,
	"fs.FileInfo":              true,
	"embed.FS":                 true,
	"fmt.Stringer":             true,
	"sort.Interface":           true,
	"encoding.BinaryMarshaler": true,
	"encoding.TextMarshaler":   true,
}

// isWhitelisted checks if a type name is in the whitelist.
func isWhitelisted(typeName string, userWhitelist []string) bool {
	if stdlibWhitelist[typeName] {
		return true
	}
	// Reduce the type to the element it ultimately holds, so that container
	// types such as []string, map[string]int, or chan error are recognized as
	// the whitelisted value type they wrap. Previously only a single pointer
	// or slice prefix was stripped, so maps and channels of primitive/value
	// types leaked through as concrete "dependencies".
	clean := coreTypeName(typeName)
	if stdlibWhitelist[clean] {
		return true
	}
	// Check user whitelist (against both the raw and the reduced type name).
	for _, w := range userWhitelist {
		if clean == w || typeName == w {
			return true
		}
	}
	return false
}

// coreTypeName strips pointer, slice, array, map, channel, and variadic
// wrappers from a type-name string to expose the element type it ultimately
// holds. For example "[]*Foo" -> "Foo", "map[string]Bar" -> "Bar", and
// "chan error" -> "error". Map keys are discarded; the value type is what a
// dependency analysis cares about.
func coreTypeName(typeName string) string {
	s := typeName
	for {
		switch {
		case strings.HasPrefix(s, "*"):
			s = s[1:]
		case strings.HasPrefix(s, "..."):
			s = s[3:]
		case strings.HasPrefix(s, "[]"):
			s = s[2:]
		case strings.HasPrefix(s, "<-chan "):
			s = s[7:]
		case strings.HasPrefix(s, "chan "):
			s = s[5:]
		case strings.HasPrefix(s, "map["):
			// Skip past the key type to the matching ']', then take the value.
			depth, idx := 0, -1
			for i := 3; i < len(s); i++ {
				switch s[i] {
				case '[':
					depth++
				case ']':
					depth--
					if depth == 0 {
						idx = i
					}
				}
				if idx >= 0 {
					break
				}
			}
			if idx < 0 || idx+1 >= len(s) {
				return s
			}
			s = s[idx+1:]
		default:
			return s
		}
	}
}

// isSelfReference reports whether typeName refers (possibly through pointers
// or collections) to the struct being analyzed. Self-references model
// recursive/tree structures (e.g. a node pointing to its parent or children)
// and linked aggregates, which are structural composition rather than
// injected collaborators, so DIP should not treat them as dependencies to
// invert.
func isSelfReference(typeName, structName string) bool {
	return structName != "" && coreTypeName(typeName) == structName
}
