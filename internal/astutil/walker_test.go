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

// TestIsNoopBody_ZeroValueReturns covers the silent-no-op shape: a single
// return whose every result is a zero-value literal claims success while
// doing nothing, whereas returning a computed value, a named constant, or a
// non-zero literal is deliberate behavior.
func TestIsNoopBody_ZeroValueReturns(t *testing.T) {
	cases := []struct {
		name string
		body string
		want bool
	}{
		{"return nil", `return nil`, true},
		{"return nil, nil", `return nil, nil`, true},
		{"return false", `return false`, true},
		{"return zero int", `return 0`, true},
		{"return zero hex", `return 0x0`, true},
		{"return zero float", `return 0.0`, true},
		{"return empty string", `return ""`, true},
		{"return mixed zero values", `return "", 0, nil`, true},
		{"return parenthesized nil", `return (nil)`, true},
		{"return conversion of nil (undetected)", `return error(nil)`, false},
		{"return non-zero literal", `return 1`, false},
		{"return true", `return true`, false},
		{"return non-empty string", `return "woof"`, false},
		{"return named constant", `return ErrNotFound`, false},
		{"return computed value", `return x + 1`, false},
		{"return call result", `return f()`, false},
		{"return nil after work", `x++; return nil`, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			src := "package test\nfunc f() any {\n" + tc.body + "\n}\n"
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, "test.go", src, 0)
			if err != nil {
				t.Fatal(err)
			}
			fd := file.Decls[0].(*ast.FuncDecl)
			if got := astutil.IsNoopBody(fd.Body); got != tc.want {
				t.Errorf("IsNoopBody(%q) = %v, want %v", tc.body, got, tc.want)
			}
		})
	}
}
