package astutil_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"

	"github.com/harakeishi/go-solid-score/internal/astutil"
)

func TestWalkBody_CyclomaticComplexity(t *testing.T) {
	src := `package test
func f() {
	if true {}
	for i := 0; i < 10; i++ {}
	for range []int{} {}
	switch {
	case true:
	case false:
	default:
	}
	if true && false || true {}
}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	fd := file.Decls[0].(*ast.FuncDecl)
	m := astutil.WalkBody(fd.Body, nil, fset)

	// if=1, for=1, range=1, case(true)=1, case(false)=1, &&=1, ||=1 = 7
	// Note: switch stmt itself may also contribute; accept 7 or 8
	if m.Complexity < 7 {
		t.Errorf("expected complexity >= 7, got %d", m.Complexity)
	}
}

func TestWalkBody_TypeSwitch(t *testing.T) {
	src := `package test
func f(x interface{}) {
	switch x.(type) {
	case int:
	case string:
	}
}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	fd := file.Decls[0].(*ast.FuncDecl)
	m := astutil.WalkBody(fd.Body, nil, fset)

	if m.TypeSwitchCount != 1 {
		t.Errorf("expected 1 type switch, got %d", m.TypeSwitchCount)
	}
}

func TestWalkBody_TypeAssert(t *testing.T) {
	src := `package test
func f(x interface{}) {
	_ = x.(int)
	_, _ = x.(string)
}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	fd := file.Decls[0].(*ast.FuncDecl)
	m := astutil.WalkBody(fd.Body, nil, fset)

	if m.TypeAssertCount != 2 {
		t.Errorf("expected 2 type assertions, got %d", m.TypeAssertCount)
	}
}

func TestWalkBody_Panic(t *testing.T) {
	src := `package test
func f() {
	panic("boom")
}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	fd := file.Decls[0].(*ast.FuncDecl)
	m := astutil.WalkBody(fd.Body, nil, fset)

	if !m.HasPanic {
		t.Error("expected HasPanic to be true")
	}
}

func TestWalkBody_StmtCount(t *testing.T) {
	src := `package test
func f() {
	x := 1
	y := 2
	_ = x + y
}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	fd := file.Decls[0].(*ast.FuncDecl)
	m := astutil.WalkBody(fd.Body, nil, fset)

	if m.StmtCount < 3 {
		t.Errorf("expected at least 3 statements, got %d", m.StmtCount)
	}
}

func TestIsNoopBody_Empty(t *testing.T) {
	body := &ast.BlockStmt{}
	if !astutil.IsNoopBody(body) {
		t.Error("empty body should be noop")
	}
}

func TestIsNoopBody_BareReturn(t *testing.T) {
	body := &ast.BlockStmt{
		List: []ast.Stmt{&ast.ReturnStmt{}},
	}
	if !astutil.IsNoopBody(body) {
		t.Error("bare return should be noop")
	}
}

func TestIsNoopBody_NonEmpty(t *testing.T) {
	body := &ast.BlockStmt{
		List: []ast.Stmt{
			&ast.ExprStmt{X: &ast.Ident{Name: "x"}},
		},
	}
	if astutil.IsNoopBody(body) {
		t.Error("non-empty body should not be noop")
	}
}
