package astutil_test

import (
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"testing"

	"github.com/harakeishi/go-solid-score/internal/astutil"
)

// fieldTypes type-checks src and returns a lookup from the field name of the
// struct `S` to its resolved types.Type, so tests can exercise IsValueType /
// IsInterfaceType on real type-checked types rather than ad-hoc constructions.
func fieldTypes(t *testing.T, src string) map[string]types.Type {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "src.go", src, 0)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	conf := types.Config{Importer: importer.Default()}
	info := &types.Info{Types: make(map[ast.Expr]types.TypeAndValue)}
	if _, err := conf.Check("p", fset, []*ast.File{file}, info); err != nil {
		t.Fatalf("typecheck: %v", err)
	}
	out := map[string]types.Type{}
	ast.Inspect(file, func(n ast.Node) bool {
		ts, ok := n.(*ast.TypeSpec)
		if !ok || ts.Name.Name != "S" {
			return true
		}
		st, ok := ts.Type.(*ast.StructType)
		if !ok {
			return true
		}
		for _, f := range st.Fields.List {
			if len(f.Names) == 0 {
				continue
			}
			out[f.Names[0].Name] = info.TypeOf(f.Type)
		}
		return false
	})
	return out
}

func TestIsValueType(t *testing.T) {
	src := `package p
type Coll struct{ Name string }
type Iface interface{ M() }
type S struct {
	basic       int
	str         string
	bytes       []byte
	strMap      map[string]string
	valSlice    []Coll            // value-element collection of a struct -> data
	valMap      map[string]Coll   // value-element map of a struct -> data
	valArray    [3]Coll           // value-element array of a struct -> data
	ptrSlice    []*Coll           // pointer collection of a struct -> collaborator
	ptrMap      map[string]*Coll  // pointer map of a struct -> collaborator
	barePtr     *Coll             // bare pointer -> collaborator
	bareStruct  Coll              // bare value struct -> collaborator (not a collection)
	ifaceSlice  []Iface           // interface element -> abstraction, not data
	deepVal     [][][][][][][][]Coll // deeply nested value-element collection -> still data
	deepPtr     [][][][][][][][]*Coll // deeply nested pointer collection -> collaborator
}`
	ft := fieldTypes(t, src)

	cases := []struct {
		field string
		want  bool
	}{
		{"basic", true},
		{"str", true},
		{"bytes", true},
		{"strMap", true},
		{"valSlice", true},
		{"valMap", true},
		{"valArray", true},
		{"ptrSlice", false},
		{"ptrMap", false},
		{"barePtr", false},
		{"bareStruct", false},
		{"ifaceSlice", false},
		{"deepVal", true},
		{"deepPtr", false},
	}
	for _, tc := range cases {
		tv := ft[tc.field]
		if tv == nil {
			t.Fatalf("field %q not found", tc.field)
		}
		if got := astutil.IsValueType(tv); got != tc.want {
			t.Errorf("IsValueType(%s) = %v, want %v", tc.field, got, tc.want)
		}
	}
}
