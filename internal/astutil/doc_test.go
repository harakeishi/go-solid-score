package astutil_test

import (
	"go/parser"
	"go/token"
	"testing"

	"github.com/harakeishi/go-solid-score/internal/astutil"
	"github.com/harakeishi/go-solid-score/model"
)

// TestExtractDecls_Doc verifies that type doc comments are captured for both
// single and grouped type declarations, which the evaluation harness relies on
// to read inline `// solid:want` ground-truth labels.
func TestExtractDecls_Doc(t *testing.T) {
	src := `package test

// Single is documented on the GenDecl.
// solid:want SRP=ok
type Single struct{}

type (
	// Grouped is documented on the TypeSpec.
	Grouped struct{}

	Undocumented struct{}
)

// Iface is a documented interface.
type Iface interface{ M() }
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}

	structs := map[string]*model.StructInfo{}
	ifaces := map[string]*model.InterfaceInfo{}
	pi := &model.PackageInfo{}
	astutil.ExtractDecls(file, "test.go", fset, nil, structs, ifaces, pi)

	cases := []struct {
		name     string
		wantSub  string
		wantSolI bool // expects the solid:want line present
	}{
		{"Single", "documented on the GenDecl", true},
		{"Grouped", "documented on the TypeSpec", false},
	}
	for _, c := range cases {
		s, ok := structs[c.name]
		if !ok {
			t.Fatalf("struct %q not extracted", c.name)
		}
		if !contains(s.Doc, c.wantSub) {
			t.Errorf("struct %q Doc = %q, want substring %q", c.name, s.Doc, c.wantSub)
		}
		if c.wantSolI && !contains(s.Doc, "solid:want SRP=ok") {
			t.Errorf("struct %q Doc = %q, want solid:want line", c.name, s.Doc)
		}
	}

	if s := structs["Undocumented"]; s == nil {
		t.Fatal("struct Undocumented not extracted")
	} else if s.Doc != "" {
		t.Errorf("undocumented struct Doc = %q, want empty", s.Doc)
	}

	if ii := ifaces["Iface"]; ii == nil {
		t.Fatal("interface Iface not extracted")
	} else if !contains(ii.Doc, "documented interface") {
		t.Errorf("interface Iface Doc = %q, want substring", ii.Doc)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
