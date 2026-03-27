package astutil

import (
	"go/ast"
	"go/token"
	"go/types"
	"strings"
)

// BodyMetrics holds metrics extracted from a function/method body.
type BodyMetrics struct {
	Complexity        int
	AccessedFields    []string
	CalledMethods     []string
	HasPanic          bool
	TypeSwitchCount   int
	TypeAssertCount   int
	ReflectUsageCount int
	StmtCount         int
}

// WalkBody walks a function body and extracts all metrics.
func WalkBody(body *ast.BlockStmt, info *types.Info, fset *token.FileSet) *BodyMetrics {
	w := &bodyWalker{
		info:        info,
		fset:        fset,
		fieldsSeen:  make(map[string]bool),
		methodsSeen: make(map[string]bool),
	}
	ast.Inspect(body, w.visit)
	return &BodyMetrics{
		Complexity:        w.complexity,
		AccessedFields:    w.accessedFields,
		CalledMethods:     w.calledMethods,
		HasPanic:          w.hasPanic,
		TypeSwitchCount:   w.typeSwitchCount,
		TypeAssertCount:   w.typeAssertCount,
		ReflectUsageCount: w.reflectUsageCount,
		StmtCount:         w.stmtCount,
	}
}

type bodyWalker struct {
	info              *types.Info
	fset              *token.FileSet
	complexity        int
	accessedFields    []string
	calledMethods     []string
	hasPanic          bool
	typeSwitchCount   int
	typeAssertCount   int
	reflectUsageCount int
	stmtCount         int
	fieldsSeen        map[string]bool
	methodsSeen       map[string]bool
}

func (w *bodyWalker) visit(n ast.Node) bool {
	if n == nil {
		return false
	}

	switch node := n.(type) {
	// Cyclomatic complexity: concrete Stmt types MUST come before any
	// interface match. We count stmtCount via a post-switch check below.
	case *ast.IfStmt:
		w.complexity++
	case *ast.ForStmt:
		w.complexity++
	case *ast.RangeStmt:
		w.complexity++
	case *ast.CaseClause:
		if node.List != nil { // not default
			w.complexity++
		}
	case *ast.CommClause:
		if node.Comm != nil { // not default
			w.complexity++
		}
	case *ast.TypeSwitchStmt:
		w.typeSwitchCount++

	// Expr types (these don't implement ast.Stmt, so order is safe)
	case *ast.BinaryExpr:
		if node.Op == token.LAND || node.Op == token.LOR {
			w.complexity++
		}
	case *ast.TypeAssertExpr:
		w.typeAssertCount++
	case *ast.SelectorExpr:
		w.checkFieldAccess(node)
		w.checkMethodCall(node)
		w.checkReflectUsage(node)
	case *ast.CallExpr:
		if ident, ok := node.Fun.(*ast.Ident); ok && ident.Name == "panic" {
			w.hasPanic = true
		}
	}

	// Count all statements (checked separately to avoid interface-match ordering issues)
	if _, ok := n.(ast.Stmt); ok {
		w.stmtCount++
	}

	return true
}

func (w *bodyWalker) checkFieldAccess(sel *ast.SelectorExpr) {
	if w.info == nil {
		return
	}
	obj := w.info.ObjectOf(sel.Sel)
	if obj == nil {
		return
	}
	if _, ok := obj.(*types.Var); ok {
		fieldName := sel.Sel.Name
		if !w.fieldsSeen[fieldName] {
			w.fieldsSeen[fieldName] = true
			w.accessedFields = append(w.accessedFields, fieldName)
		}
	}
}

func (w *bodyWalker) checkMethodCall(sel *ast.SelectorExpr) {
	if w.info == nil {
		return
	}
	obj := w.info.ObjectOf(sel.Sel)
	if obj == nil {
		return
	}
	if _, ok := obj.(*types.Func); ok {
		methodName := sel.Sel.Name
		if !w.methodsSeen[methodName] {
			w.methodsSeen[methodName] = true
			w.calledMethods = append(w.calledMethods, methodName)
		}
	}
}

func (w *bodyWalker) checkReflectUsage(sel *ast.SelectorExpr) {
	if ident, ok := sel.X.(*ast.Ident); ok {
		if strings.EqualFold(ident.Name, "reflect") {
			w.reflectUsageCount++
		}
	}
}
