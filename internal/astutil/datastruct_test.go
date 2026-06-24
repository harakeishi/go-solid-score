package astutil

import (
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"testing"
)

// typeOfField type-checks src and returns the types.Type of the single field
// named fieldName inside the struct named structName.
func typeOfField(t *testing.T, src, structName, fieldName string) types.Type {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "src.go", src, 0)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	conf := types.Config{Importer: importer.Default()}
	info := &types.Info{Defs: map[*ast.Ident]types.Object{}}
	pkg, err := conf.Check("p", fset, []*ast.File{f}, info)
	if err != nil {
		t.Fatalf("typecheck: %v", err)
	}
	obj := pkg.Scope().Lookup(structName)
	if obj == nil {
		t.Fatalf("struct %q not found", structName)
	}
	st := obj.Type().Underlying().(*types.Struct)
	for i := 0; i < st.NumFields(); i++ {
		if st.Field(i).Name() == fieldName {
			return st.Field(i).Type()
		}
	}
	t.Fatalf("field %q not found in %q", fieldName, structName)
	return nil
}

func TestIsMethodlessDataStruct(t *testing.T) {
	const src = `package p

type Option struct{ Name string }            // methodless data struct (DTO)

type Service struct{ x int }
func (s *Service) Call() error { return nil } // pointer-receiver method -> collaborator

type ValueMethoded struct{ y int }
func (v ValueMethoded) Get() int { return v.y } // value-receiver method -> collaborator

type Holder struct {
	opt      Option          // -> data struct
	opts     []Option        // slice of data struct -> data struct
	optPtr   *Option         // pointer to data struct -> data struct
	svc      *Service        // pointer-receiver collaborator -> NOT data
	vm       ValueMethoded   // value-receiver collaborator -> NOT data
	name     string          // builtin -> NOT a named struct, NOT data
	tags     map[string]int  // builtin element -> NOT data
}
`
	cases := []struct {
		field string
		want  bool
	}{
		{"opt", true},
		{"opts", true},
		{"optPtr", true},
		{"svc", false},
		{"vm", false},
		{"name", false},
		{"tags", false},
	}
	for _, c := range cases {
		t.Run(c.field, func(t *testing.T) {
			ft := typeOfField(t, src, "Holder", c.field)
			if got := IsMethodlessDataStruct(ft); got != c.want {
				t.Errorf("IsMethodlessDataStruct(%s) = %v, want %v", c.field, got, c.want)
			}
		})
	}
}
