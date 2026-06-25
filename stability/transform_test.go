package stability

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

const transformSrc = `package p

import "fmt"

type Counter struct {
	n int
}

func (c *Counter) Inc() { c.n++ }

func (c *Counter) Value() int { return c.n }

type Greeter struct{}

func (g Greeter) Hello(name string) string { return fmt.Sprintf("hi %s", name) }
`

// mustParse fails the test if src is not a valid Go file.
func mustParse(t *testing.T, src []byte) {
	t.Helper()
	if _, err := parser.ParseFile(token.NewFileSet(), "x.go", src, parser.SkipObjectResolution); err != nil {
		t.Fatalf("transformed source does not parse: %v\n---\n%s", err, src)
	}
}

// TestTransformsChangeBytesAndStillParse is the basic contract every transform
// owes: it must visibly change the source (otherwise it tests nothing) and must
// leave a compilable file behind.
func TestTransformsChangeBytesAndStillParse(t *testing.T) {
	for _, tr := range Transforms() {
		tr := tr
		t.Run(tr.Name, func(t *testing.T) {
			got, err := tr.Apply([]byte(transformSrc))
			if err != nil {
				t.Fatalf("apply: %v", err)
			}
			if string(got) == transformSrc {
				t.Fatal("transform left source byte-identical")
			}
			mustParse(t, got)
		})
	}
}

// TestRenameReceivers_RewritesFieldAccess checks the property that matters for
// cohesion scoring: the receiver name changes everywhere it is used, including
// in field selectors, so field-access detection sees the same fields.
func TestRenameReceivers_RewritesFieldAccess(t *testing.T) {
	got, err := renameReceivers([]byte(transformSrc))
	if err != nil {
		t.Fatal(err)
	}
	out := string(got)
	if strings.Contains(out, "c.n") || strings.Contains(out, "(c *Counter)") {
		t.Errorf("old receiver name still present:\n%s", out)
	}
	if !strings.Contains(out, "rcv.n") {
		t.Errorf("field access not rewritten to new receiver:\n%s", out)
	}
}

// TestReorderDecls_KeepsImportsFirst checks the transform produced a still-valid
// file with the non-import declarations reversed (Greeter now precedes Counter).
func TestReorderDecls_KeepsImportsFirst(t *testing.T) {
	got, err := reorderDecls([]byte(transformSrc))
	if err != nil {
		t.Fatal(err)
	}
	out := string(got)
	mustParse(t, got)
	gi := strings.Index(out, "type Greeter")
	ci := strings.Index(out, "type Counter")
	if gi < 0 || ci < 0 || gi > ci {
		t.Errorf("declarations not reordered (Greeter should precede Counter):\n%s", out)
	}
	if imp := strings.Index(out, `import "fmt"`); imp < 0 || imp > gi {
		t.Errorf("import declaration not kept first:\n%s", out)
	}
}

// TestInjectComments_AddsComments confirms comment lines were introduced before
// declarations without breaking the file.
func TestInjectComments_AddsComments(t *testing.T) {
	got, err := injectComments([]byte(transformSrc))
	if err != nil {
		t.Fatal(err)
	}
	out := string(got)
	mustParse(t, got)
	if strings.Count(out, "stability: injected noise comment") == 0 {
		t.Errorf("no comments injected:\n%s", out)
	}
	if !strings.Contains(out, "stability: trailing noise comment") {
		t.Errorf("trailing comment missing:\n%s", out)
	}
}
