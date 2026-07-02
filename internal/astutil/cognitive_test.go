package astutil_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"

	"github.com/harakeishi/go-solid-score/internal/astutil"
)

// parseFunc parses src (a complete file) and returns the body and name of the
// first function declaration.
func parseFunc(t *testing.T, src string) (*ast.BlockStmt, string) {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range file.Decls {
		if fd, ok := d.(*ast.FuncDecl); ok {
			return fd.Body, fd.Name.Name
		}
	}
	t.Fatal("no function declaration found")
	return nil, ""
}

func TestCognitiveComplexity(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want int
	}{
		{
			name: "straight-line code scores zero",
			src: `package p
func f(a, b int) int {
	x := a + b
	return x * 2
}`,
			want: 0,
		},
		{
			name: "flat ifs count one each",
			src: `package p
func f(a, b int) int {
	if a > 0 { // +1
		return 1
	}
	if b > 0 { // +1
		return 2
	}
	return 0
}`,
			want: 2,
		},
		{
			name: "nesting charges a growing penalty",
			src: `package p
func f(a, b, c int) int {
	if a > 0 { // +1
		if b > 0 { // +2 (nesting=1)
			if c > 0 { // +3 (nesting=2)
				return 1
			}
		}
	}
	return 0
}`,
			want: 6,
		},
		{
			name: "else and else-if cost one flat",
			src: `package p
func f(a int) int {
	if a > 0 { // +1
		return 1
	} else if a < -10 { // +1
		return 2
	} else { // +1
		return 3
	}
}`,
			want: 3,
		},
		{
			name: "loop inside loop",
			src: `package p
func f(n int) int {
	total := 0
	for i := 0; i < n; i++ { // +1
		for j := 0; j < n; j++ { // +2 (nesting=1)
			total++
		}
	}
	return total
}`,
			want: 3,
		},
		{
			name: "range switch and select each charge with nesting",
			src: `package p
func f(xs []int, ch chan int) int {
	for _, x := range xs { // +1
		switch x { // +2 (nesting=1)
		case 0:
			return 0
		case 1:
			return 1
		}
	}
	select { // +1
	case v := <-ch:
		return v
	default:
		return -1
	}
}`,
			want: 4,
		},
		{
			name: "logical sequences: one per run plus alternations",
			src: `package p
func f(a, b, c, d bool) bool {
	if a && b && c { // +1 if, +1 for the && run
		return true
	}
	if a && b || c && d { // +1 if, +3 for && -> || -> &&
		return true
	}
	return false
}`,
			want: 6,
		},
		{
			name: "parenthesized logical subtree starts a new sequence",
			src: `package p
func f(a, b, c bool) bool {
	return a || (b && c) // +1 for ||, +1 for the parenthesized && run
}`,
			want: 2,
		},
		{
			name: "goto and labeled break cost one flat",
			src: `package p
func f(n int) int {
outer:
	for i := 0; i < n; i++ { // +1
		for j := 0; j < n; j++ { // +2
			if j > i { // +3
				break outer // +1
			}
		}
	}
	if n < 0 { // +1
		goto done // +1
	}
done:
	return n
}`,
			want: 9,
		},
		{
			name: "direct recursion costs one",
			src: `package p
func fib(n int) int {
	if n < 2 { // +1
		return n
	}
	return fib(n-1) + fib(n-2) // +1 +1 recursion
}`,
			want: 3,
		},
		{
			name: "function literal adds nesting but no increment",
			src: `package p
func f(xs []int) func() int {
	return func() int { // nesting+1, no charge
		if len(xs) > 0 { // +2 (nesting=1)
			return xs[0]
		}
		return 0
	}
}`,
			want: 2,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body, name := parseFunc(t, tc.src)
			got := astutil.CognitiveComplexity(body, name)
			if got != tc.want {
				t.Errorf("CognitiveComplexity = %d, want %d", got, tc.want)
			}
		})
	}
}

// TestCognitiveComplexity_DivergesFromCyclomatic pins the motivating property:
// deeply nested branches and flat branches have identical cyclomatic
// complexity but very different cognitive complexity.
func TestCognitiveComplexity_DivergesFromCyclomatic(t *testing.T) {
	flat := `package p
func f(a, b, c, d int) int {
	if a > 0 {
		return 1
	}
	if b > 0 {
		return 2
	}
	if c > 0 {
		return 3
	}
	if d > 0 {
		return 4
	}
	return 0
}`
	nested := `package p
func f(a, b, c, d int) int {
	if a > 0 {
		if b > 0 {
			if c > 0 {
				if d > 0 {
					return 4
				}
			}
		}
	}
	return 0
}`

	flatBody, flatName := parseFunc(t, flat)
	nestedBody, nestedName := parseFunc(t, nested)

	flatScore := astutil.CognitiveComplexity(flatBody, flatName)
	nestedScore := astutil.CognitiveComplexity(nestedBody, nestedName)

	// Both have cyclomatic complexity 5 (4 branches + 1), but cognitively the
	// nested version is 1+2+3+4=10 vs 4 flat.
	if flatScore != 4 {
		t.Errorf("flat cognitive complexity = %d, want 4", flatScore)
	}
	if nestedScore != 10 {
		t.Errorf("nested cognitive complexity = %d, want 10", nestedScore)
	}

	for name, src := range map[string]string{"flat": flat, "nested": nested} {
		body, fn := parseFunc(t, src)
		m := astutil.WalkBody(body, nil, nil)
		if m.Complexity+1 != 5 {
			t.Errorf("%s cyclomatic complexity = %d, want 5", name, m.Complexity+1)
		}
		_ = fn
	}
}

func TestCognitiveComplexity_NilBody(t *testing.T) {
	if got := astutil.CognitiveComplexity(nil, "f"); got != 0 {
		t.Errorf("nil body = %d, want 0", got)
	}
}
