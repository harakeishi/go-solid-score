package analyzer

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
	"context.Context":         true,
	"time.Time":               true,
	"time.Duration":           true,
	"time.Location":           true,
	"sync.Mutex":              true,
	"sync.RWMutex":            true,
	"sync.WaitGroup":          true,
	"sync.Once":               true,
	"sync.Map":                true,
	"sync.Pool":               true,
	"bytes.Buffer":            true,
	"strings.Builder":         true,
	"strings.Reader":          true,
	"regexp.Regexp":           true,
	"url.URL":                 true,
	"net.IP":                  true,
	"net.Conn":                true,
	"http.Request":            true,
	"http.Response":           true,
	"http.ResponseWriter":     true,
	"http.Client":             true,
	"http.Server":             true,
	"http.Handler":            true,
	"http.HandlerFunc":        true,
	"json.Decoder":            true,
	"json.Encoder":            true,
	// log.Logger is concrete - not whitelisted for DIP
	"slog.Logger":             true,
	"slog.Handler":            true,
	"testing.T":               true,
	"testing.B":               true,
	"testing.M":               true,
	"os.File":                 true,
	"os.Signal":               true,
	"big.Int":                 true,
	"big.Float":               true,
	"template.Template":       true,
	// sql.DB, sql.Tx etc. are concrete - not whitelisted for DIP
	"tls.Config":              true,
	"x509.Certificate":        true,
	"exec.Cmd":                true,
	"filepath.WalkFunc":       true,
	"context.CancelFunc":      true,
	"atomic.Value":            true,
	"atomic.Int32":            true,
	"atomic.Int64":            true,
	"atomic.Bool":             true,
	"flag.FlagSet":            true,
	"io.ReadCloser":           true,
	"io.WriteCloser":          true,
	"io.ReadWriteCloser":      true,
	"io.ReadWriter":           true,
	"io.Reader":               true,
	"io.Writer":               true,
	"io.Closer":               true,
	"io.Seeker":               true,
	"io.ReaderAt":             true,
	"io.WriterAt":             true,
	"io.ReadSeeker":           true,
	"io.WriteSeeker":          true,
	"io.ReadWriteSeeker":      true,
	"io.LimitedReader":        true,
	"io.SectionReader":        true,
	"io.PipeReader":           true,
	"io.PipeWriter":           true,
	"fs.FS":                   true,
	"fs.File":                 true,
	"fs.FileInfo":             true,
	"embed.FS":                true,
	"fmt.Stringer":            true,
	"sort.Interface":          true,
	"encoding.BinaryMarshaler": true,
	"encoding.TextMarshaler":  true,
}

// isWhitelisted checks if a type name is in the whitelist.
func isWhitelisted(typeName string, userWhitelist []string) bool {
	if stdlibWhitelist[typeName] {
		return true
	}
	// Strip pointer prefix
	clean := typeName
	for len(clean) > 0 && clean[0] == '*' {
		clean = clean[1:]
	}
	if stdlibWhitelist[clean] {
		return true
	}
	// Strip slice/map prefixes
	if len(clean) > 2 && clean[:2] == "[]" {
		if stdlibWhitelist[clean[2:]] {
			return true
		}
	}
	// Check user whitelist
	for _, w := range userWhitelist {
		if clean == w {
			return true
		}
	}
	return false
}
