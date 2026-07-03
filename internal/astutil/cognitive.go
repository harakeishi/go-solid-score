package astutil

import (
	"go/ast"
	"go/token"
)

// CognitiveComplexity computes the cognitive complexity of a function body
// following the SonarSource specification (G. Ann Campbell, "Cognitive
// Complexity — a new way of measuring understandability", 2018), as
// implemented for Go by gocognit. funcName is the enclosing function's name
// and recvName the receiver variable's name; together they detect direct
// recursion. For a method (recvName != "") only recvName.funcName(...) calls
// count — a bare funcName(...) call there reaches a same-named package
// function, not the method. For a package-level function (recvName == "")
// bare funcName(...) calls count. Indirect recursion (through another
// variable holding the same value, or mutual recursion) is out of scope.
//
// Unlike cyclomatic complexity (which counts branches flat), cognitive
// complexity charges a growing nesting penalty: each control-flow structure
// costs 1 plus the number of enclosing control-flow structures, so deeply
// nested code scores much higher than the same branches written flat. The
// increments are:
//
//   - +1 (+nesting) for if, for, range, switch, type switch, select
//   - +1 flat for else / else-if (the nested if adds no extra nesting penalty)
//   - +1 for each new sequence of logical operators and each alternation
//     between && and || within an expression
//   - +1 flat for goto and for break/continue with a label
//   - +1 flat for a direct recursive call
//   - function literals add a nesting level but no increment
func CognitiveComplexity(body *ast.BlockStmt, funcName, recvName string) int {
	if body == nil {
		return 0
	}
	v := &cognitiveVisitor{funcName: funcName, recvName: recvName}
	v.walkStmt(body, 0)
	return v.score
}

type cognitiveVisitor struct {
	funcName string
	recvName string
	score    int
}

// walkStmt dispatches a statement at the given nesting level. Structures that
// open a new scope of control flow recurse with nesting+1 for their bodies.
func (v *cognitiveVisitor) walkStmt(s ast.Stmt, nesting int) {
	switch st := s.(type) {
	case nil:
	case *ast.BlockStmt:
		for _, inner := range st.List {
			v.walkStmt(inner, nesting)
		}
	case *ast.IfStmt:
		v.walkIf(st, nesting, false)
	case *ast.ForStmt:
		v.score += 1 + nesting
		v.walkStmt(st.Init, nesting)
		v.walkExpr(st.Cond, nesting)
		v.walkStmt(st.Post, nesting)
		v.walkStmt(st.Body, nesting+1)
	case *ast.RangeStmt:
		v.score += 1 + nesting
		v.walkExpr(st.X, nesting)
		v.walkStmt(st.Body, nesting+1)
	case *ast.SwitchStmt:
		v.score += 1 + nesting
		v.walkStmt(st.Init, nesting)
		v.walkExpr(st.Tag, nesting)
		v.walkStmt(st.Body, nesting+1)
	case *ast.TypeSwitchStmt:
		v.score += 1 + nesting
		v.walkStmt(st.Init, nesting)
		v.walkStmt(st.Assign, nesting)
		v.walkStmt(st.Body, nesting+1)
	case *ast.SelectStmt:
		v.score += 1 + nesting
		v.walkStmt(st.Body, nesting+1)
	case *ast.CaseClause:
		// The switch itself was charged; individual cases are free.
		for _, e := range st.List {
			v.walkExpr(e, nesting)
		}
		for _, inner := range st.Body {
			v.walkStmt(inner, nesting)
		}
	case *ast.CommClause:
		v.walkStmt(st.Comm, nesting)
		for _, inner := range st.Body {
			v.walkStmt(inner, nesting)
		}
	case *ast.BranchStmt:
		// break/continue to a label and goto interrupt linear flow.
		if st.Label != nil || st.Tok == token.GOTO {
			v.score++
		}
	case *ast.LabeledStmt:
		v.walkStmt(st.Stmt, nesting)
	case *ast.ExprStmt:
		v.walkExpr(st.X, nesting)
	case *ast.AssignStmt:
		for _, e := range st.Rhs {
			v.walkExpr(e, nesting)
		}
		for _, e := range st.Lhs {
			v.walkExpr(e, nesting)
		}
	case *ast.ReturnStmt:
		for _, e := range st.Results {
			v.walkExpr(e, nesting)
		}
	case *ast.DeclStmt:
		if gd, ok := st.Decl.(*ast.GenDecl); ok {
			for _, spec := range gd.Specs {
				if vs, ok := spec.(*ast.ValueSpec); ok {
					for _, e := range vs.Values {
						v.walkExpr(e, nesting)
					}
				}
			}
		}
	case *ast.GoStmt:
		v.walkExpr(st.Call, nesting)
	case *ast.DeferStmt:
		v.walkExpr(st.Call, nesting)
	case *ast.SendStmt:
		v.walkExpr(st.Chan, nesting)
		v.walkExpr(st.Value, nesting)
	case *ast.IncDecStmt:
		v.walkExpr(st.X, nesting)
	}
}

// walkIf charges an if statement. elseIf is true when this if is the `else if`
// of an enclosing if: it still costs +1 but takes no nesting penalty and does
// not deepen nesting relative to the chain head, per the specification's
// hybrid increment for else-if.
func (v *cognitiveVisitor) walkIf(st *ast.IfStmt, nesting int, elseIf bool) {
	if elseIf {
		v.score++
	} else {
		v.score += 1 + nesting
	}
	v.walkStmt(st.Init, nesting)
	v.walkExpr(st.Cond, nesting)
	v.walkStmt(st.Body, nesting+1)
	switch el := st.Else.(type) {
	case *ast.IfStmt:
		v.walkIf(el, nesting, true)
	case *ast.BlockStmt:
		v.score++ // +1 for else, no nesting penalty
		v.walkStmt(el, nesting+1)
	}
}

// walkExpr charges logical-operator sequences and recursion inside an
// expression, and descends into function literals with one more nesting level.
func (v *cognitiveVisitor) walkExpr(e ast.Expr, nesting int) {
	if e == nil {
		return
	}
	switch ex := e.(type) {
	case *ast.BinaryExpr:
		if ex.Op == token.LAND || ex.Op == token.LOR {
			v.score += countLogicalSequences(ex)
			// Operands may contain calls/func literals; descend past the
			// logical operators themselves.
			v.walkLogicalOperands(ex, nesting)
			return
		}
		v.walkExpr(ex.X, nesting)
		v.walkExpr(ex.Y, nesting)
	case *ast.ParenExpr:
		v.walkExpr(ex.X, nesting)
	case *ast.UnaryExpr:
		v.walkExpr(ex.X, nesting)
	case *ast.CallExpr:
		if v.isRecursiveCall(ex) {
			v.score++ // direct recursion
		}
		v.walkExpr(ex.Fun, nesting)
		for _, arg := range ex.Args {
			v.walkExpr(arg, nesting)
		}
	case *ast.FuncLit:
		v.walkStmt(ex.Body, nesting+1)
	case *ast.SelectorExpr:
		v.walkExpr(ex.X, nesting)
	case *ast.IndexExpr:
		v.walkExpr(ex.X, nesting)
		v.walkExpr(ex.Index, nesting)
	case *ast.SliceExpr:
		v.walkExpr(ex.X, nesting)
		v.walkExpr(ex.Low, nesting)
		v.walkExpr(ex.High, nesting)
		v.walkExpr(ex.Max, nesting)
	case *ast.StarExpr:
		v.walkExpr(ex.X, nesting)
	case *ast.TypeAssertExpr:
		v.walkExpr(ex.X, nesting)
	case *ast.CompositeLit:
		for _, el := range ex.Elts {
			v.walkExpr(el, nesting)
		}
	case *ast.KeyValueExpr:
		v.walkExpr(ex.Value, nesting)
	}
}

// isRecursiveCall reports whether a call re-enters the enclosing function: a
// bare funcName(...) for a package-level function, or recvName.funcName(...)
// for a method. Bare-ident matching is deliberately disabled inside methods —
// there such a call reaches a same-named package function, not the method.
func (v *cognitiveVisitor) isRecursiveCall(call *ast.CallExpr) bool {
	if v.funcName == "" {
		return false
	}
	switch fun := call.Fun.(type) {
	case *ast.Ident:
		return v.recvName == "" && fun.Name == v.funcName
	case *ast.SelectorExpr:
		if v.recvName == "" || fun.Sel.Name != v.funcName {
			return false
		}
		recv, ok := fun.X.(*ast.Ident)
		return ok && recv.Name == v.recvName
	}
	return false
}

// walkLogicalOperands descends into the non-logical operands of a &&/|| tree
// without re-charging the logical operators (countLogicalSequences already
// scored the whole tree).
func (v *cognitiveVisitor) walkLogicalOperands(e ast.Expr, nesting int) {
	switch ex := e.(type) {
	case *ast.BinaryExpr:
		if ex.Op == token.LAND || ex.Op == token.LOR {
			v.walkLogicalOperands(ex.X, nesting)
			v.walkLogicalOperands(ex.Y, nesting)
			return
		}
		v.walkExpr(ex, nesting)
	case *ast.ParenExpr:
		// A parenthesized logical tree starts a new sequence: route it back
		// through walkExpr so countLogicalSequences charges it independently.
		v.walkExpr(ex.X, nesting)
	default:
		v.walkExpr(ex, nesting)
	}
}

// countLogicalSequences returns the cognitive-complexity charge for the
// logical-operator tree rooted at e: +1 for the first operator and +1 for each
// alternation between && and || in the flattened left-to-right sequence
// (a && b && c → 1; a && b || c → 2).
func countLogicalSequences(e ast.Expr) int {
	ops := flattenLogicalOps(e, nil)
	if len(ops) == 0 {
		return 0
	}
	count := 1
	for i := 1; i < len(ops); i++ {
		if ops[i] != ops[i-1] {
			count++
		}
	}
	return count
}

// flattenLogicalOps appends the &&/|| operators of a binary tree in
// left-to-right source order. Parenthesized sub-expressions restart a
// sequence per the specification, so they are treated as opaque here and
// charged by their own walkExpr visit... except that walkLogicalOperands
// routes parenthesized logical trees back through walkExpr, which calls
// countLogicalSequences on them independently — matching the spec's "each new
// sequence" rule.
func flattenLogicalOps(e ast.Expr, ops []token.Token) []token.Token {
	be, ok := e.(*ast.BinaryExpr)
	if !ok || (be.Op != token.LAND && be.Op != token.LOR) {
		return ops
	}
	ops = flattenLogicalOps(be.X, ops)
	ops = append(ops, be.Op)
	ops = flattenLogicalOps(be.Y, ops)
	return ops
}
