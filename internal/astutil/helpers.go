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

// IsInterfaceType reports whether t is an interface, or a container
// (pointer, slice, array, map value, or channel) whose element type is an
// interface. This means a field such as `handlers []Handler`, where Handler
// is an interface, is recognized as an abstraction dependency — matching how
// DIP and OCP reason about depending on abstractions rather than concrete
// types. Without unwrapping, collections of interfaces were misclassified as
// concrete dependencies, producing false-positive DIP penalties.
func IsInterfaceType(t types.Type) bool {
	return unwrapElem(t, func(u types.Type) bool {
		_, ok := u.(*types.Interface)
		return ok
	})
}

// IsFuncType reports whether t denotes a function type, including named
// function types (e.g. `type HandlerFunc func(...)`) and containers of
// functions. Function-typed fields are a form of behavioral injection
// (callbacks/strategies), not a concrete service collaborator, so DIP treats
// them as neutral rather than as concrete dependencies to be inverted.
func IsFuncType(t types.Type) bool {
	return unwrapElem(t, func(u types.Type) bool {
		_, ok := u.(*types.Signature)
		return ok
	})
}

// IsValueType reports whether the *core element* of t — reached by unwrapping
// pointers, slices, arrays, maps, and channels — is a basic (builtin) type.
// This captures pure data fields such as int, string, []byte,
// map[string]string, and named aliases like `type FieldMap map[string]string`,
// which model data a struct holds rather than a collaborator it calls into, so
// DIP excludes them.
//
// Crucially, a collection of a *struct* or *interface* (e.g. []*PaymentService
// or []Handler) is NOT a value type: its element is a collaborator, so DIP
// still weighs it — as a concrete dependency for structs, and (via
// IsInterfaceType) as an abstraction for interfaces. This preserves the true
// positive that an earlier, blanket "any slice/map is data" rule discarded.
func IsValueType(t types.Type) bool {
	return unwrapElem(t, func(u types.Type) bool {
		_, ok := u.(*types.Basic)
		return ok
	})
}

// unwrapElem peels pointer, slice, array, map (value), and channel wrappers
// off t (up to a bounded depth) and reports whether the resulting underlying
// type satisfies match. The depth bound guards against pathological or cyclic
// type graphs.
func unwrapElem(t types.Type, match func(types.Type) bool) bool {
	for i := 0; i < 8; i++ {
		u := t.Underlying()
		if match(u) {
			return true
		}
		switch c := u.(type) {
		case *types.Pointer:
			t = c.Elem()
		case *types.Slice:
			t = c.Elem()
		case *types.Array:
			t = c.Elem()
		case *types.Chan:
			t = c.Elem()
		case *types.Map:
			t = c.Elem()
		default:
			return false
		}
	}
	return false
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
