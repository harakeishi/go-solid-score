package astutil

import (
	"go/ast"
	"go/types"
)

// ReceiverTypeName extracts the type name from a method receiver.
func ReceiverTypeName(recv *ast.FieldList) string {
	if recv == nil || len(recv.List) == 0 {
		return ""
	}
	return exprTypeName(recv.List[0].Type)
}

func exprTypeName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return exprTypeName(t.X)
	case *ast.IndexExpr: // generic type
		return exprTypeName(t.X)
	case *ast.IndexListExpr:
		return exprTypeName(t.X)
	default:
		return ""
	}
}

// ExprToString converts an AST expression to a human-readable type string.
func ExprToString(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return "*" + ExprToString(t.X)
	case *ast.SelectorExpr:
		return ExprToString(t.X) + "." + t.Sel.Name
	case *ast.ArrayType:
		return "[]" + ExprToString(t.Elt)
	case *ast.MapType:
		return "map[" + ExprToString(t.Key) + "]" + ExprToString(t.Value)
	case *ast.InterfaceType:
		return "interface{}"
	case *ast.FuncType:
		return "func(...)"
	case *ast.ChanType:
		return "chan " + ExprToString(t.Value)
	case *ast.Ellipsis:
		return "..." + ExprToString(t.Elt)
	case *ast.IndexExpr:
		return ExprToString(t.X) + "[" + ExprToString(t.Index) + "]"
	case *ast.IndexListExpr:
		return ExprToString(t.X) + "[...]"
	default:
		return "unknown"
	}
}

// IsInterfaceType checks whether a type's underlying type is an interface.
func IsInterfaceType(t types.Type) bool {
	t = t.Underlying()
	_, ok := t.(*types.Interface)
	return ok
}

// IsNoopBody checks if a function body is a no-op (empty or bare return).
func IsNoopBody(body *ast.BlockStmt) bool {
	if body == nil || len(body.List) == 0 {
		return true
	}
	if len(body.List) == 1 {
		if ret, ok := body.List[0].(*ast.ReturnStmt); ok {
			return len(ret.Results) == 0
		}
	}
	return false
}
