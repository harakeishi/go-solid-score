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

// TestIsEmptyInterfaceType exercises the empty-interface check on type-checked
// types. The critical case is the generic type parameter: its underlying type
// is its constraint interface — method-less for `any` — but `v T` keeps full
// type identity, so it must NOT be classified as an empty interface (that
// would strip generic methods of the OCP interface-parameter bonus precisely
// when the constraint is loosest).
func TestIsEmptyInterfaceType(t *testing.T) {
	src := `package p
type Iface interface{ M() }
type NamedEmpty interface{}
type S[T any] struct {
	tp       T             // type parameter -> keeps type identity, not empty
	tpSlice  []T           // collection of a type parameter -> not empty
	empty    any           // the empty interface itself
	emptyOld interface{}   // spelled the old way
	emptyPtr *any          // behind a pointer
	emptyCol []interface{} // behind a collection
	named    NamedEmpty    // named empty interface
	iface    Iface         // non-empty interface
	basic    int           // not an interface at all
}`
	ft := fieldTypes(t, src)

	cases := []struct {
		field string
		want  bool
	}{
		{"tp", false},
		{"tpSlice", false},
		{"empty", true},
		{"emptyOld", true},
		{"emptyPtr", true},
		{"emptyCol", true},
		{"named", true},
		{"iface", false},
		{"basic", false},
	}
	for _, tc := range cases {
		tv := ft[tc.field]
		if tv == nil {
			t.Fatalf("field %q not found", tc.field)
		}
		if got := astutil.IsEmptyInterfaceType(tv); got != tc.want {
			t.Errorf("IsEmptyInterfaceType(%s) = %v, want %v", tc.field, got, tc.want)
		}
	}
}
