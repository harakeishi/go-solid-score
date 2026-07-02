package astutil

import (
	"go/ast"
	"go/token"
	"go/types"
	"strconv"
)

// docText returns the text of a doc comment group with the comment markers
// stripped, or "" when the group is nil. It is a thin wrapper over
// (*ast.CommentGroup).Text so callers can pass a possibly-nil group.
func docText(g *ast.CommentGroup) string {
	if g == nil {
		return ""
	}
	return g.Text()
}

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

// IsEmptyInterfaceType reports whether t is the empty interface (any /
// interface{}), possibly behind pointers or collections, including named
// empty interfaces. An empty interface abandons type information rather than
// abstracting behavior, so OCP does not reward it as an interface parameter.
func IsEmptyInterfaceType(t types.Type) bool {
	return unwrapElem(t, func(u types.Type) bool {
		iface, ok := u.(*types.Interface)
		return ok && iface.NumMethods() == 0
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

// IsValueType reports whether t models data a struct *holds* rather than a
// collaborator it *calls into*, so DIP excludes it from the dependency ratio.
// Two shapes qualify as data:
//
//   - A field whose *core element* — reached by unwrapping pointers, slices,
//     arrays, maps, and channels — is a basic (builtin) type: int, string,
//     []byte, map[string]string, named aliases like `type FieldMap
//     map[string]string`, and so on.
//   - A *value-element collection* of a non-basic type — a slice/array/map/chan
//     whose element is reached without an intervening pointer, e.g. []Message,
//     map[string]Event, [3]Record. These are overwhelmingly data records the
//     struct stores (an in-memory store/cache/registry/buffer), not collaborators
//     it invokes, so penalizing them produced a systematic DIP false positive.
//
// Crucially, a *pointer collection* of a struct (e.g. []*PaymentService,
// map[string]*Conn, []*Worker) is NOT a value type: collaborators are
// idiomatically held by pointer, so the pointer below the collection marks the
// element as a concrete collaborator that DIP still weighs. A bare pointer field
// (`db *sql.DB`, *Service) likewise remains a concrete dependency. This keeps
// the true positive an earlier blanket "any slice/map is data" rule discarded
// (see testdata/dip/facade.go Pipeline's []*stage) while skipping data
// containers like []Message.
func IsValueType(t types.Type) bool {
	// Core element is a builtin basic type -> pure data (int, string, []byte, …).
	if unwrapElem(t, func(u types.Type) bool {
		_, ok := u.(*types.Basic)
		return ok
	}) {
		return true
	}
	// Otherwise: a value-element collection of a non-basic type is a data
	// container; a pointer-element collection (or a bare pointer) is a
	// collaborator and is NOT a value type.
	return isValueElementCollection(t)
}

// isValueElementCollection reports whether t is a slice, array, map, or channel
// whose element is reached *without* passing through a pointer — i.e. a
// value-element collection such as []Message or map[string]Event, the shape of a
// data container. It returns false for a bare pointer ([]*T, *T) at or below the
// collection level, which marks a held collaborator rather than stored data.
func isValueElementCollection(t types.Type) bool {
	sawCollection := false
	for i := 0; i < unwrapNestDepth; i++ {
		switch c := t.Underlying().(type) {
		case *types.Pointer:
			// A pointer anywhere on the path (the field itself, or the collection
			// element) signals a collaborator held by reference, not stored data.
			return false
		case *types.Slice:
			sawCollection = true
			t = c.Elem()
		case *types.Array:
			sawCollection = true
			t = c.Elem()
		case *types.Map:
			sawCollection = true
			t = c.Elem()
		case *types.Chan:
			sawCollection = true
			t = c.Elem()
		case *types.Interface:
			// An interface element (`[]Handler`) is an abstraction dependency, not
			// stored data — IsInterfaceType handles it. Never treat it as data.
			return false
		default:
			// Reached a non-collection, non-pointer, non-interface core element
			// (e.g. a struct): it is data only if we arrived here through at least
			// one collection.
			return sawCollection
		}
	}
	// Depth bound hit before reaching the core element: whether a pointer sits
	// below is unknown, so stay conservative and do NOT classify it as data. The
	// bound is set high enough (unwrapNestDepth) that only pathological or cyclic
	// types reach here, none of which occur in real field declarations.
	return false
}

// unwrapNestDepth bounds how many pointer/collection wrappers the type-unwrapping
// helpers peel before giving up, guarding against pathological or cyclic type
// graphs. It is set well above any nesting seen in real field declarations so the
// bound is only ever hit by degenerate types — keeping the helpers' depth limits
// in sync so isValueElementCollection and IsValueType agree on the same types.
const unwrapNestDepth = 16

// unwrapElem peels pointer, slice, array, map (value), and channel wrappers
// off t (up to a bounded depth) and reports whether the resulting underlying
// type satisfies match. The depth bound guards against pathological or cyclic
// type graphs.
func unwrapElem(t types.Type, match func(types.Type) bool) bool {
	for i := 0; i < unwrapNestDepth; i++ {
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

// IsNoopBody checks if a function body is a no-op: empty, a bare return, or a
// single return whose every result is a zero-value literal (nil, false, 0, "").
// Such a body claims the contract was fulfilled while doing nothing — the
// silent-no-op LSP smell. A return of a computed value, a named constant, or a
// non-zero literal is deliberate behavior and does not count.
func IsNoopBody(body *ast.BlockStmt) bool {
	if body == nil || len(body.List) == 0 {
		return true
	}
	if len(body.List) == 1 {
		if ret, ok := body.List[0].(*ast.ReturnStmt); ok {
			for _, r := range ret.Results {
				if !isZeroValueExpr(r) {
					return false
				}
			}
			return true
		}
	}
	return false
}

// isZeroValueExpr reports whether e is a zero-value literal: nil, false, a
// numeric literal equal to zero, or an empty string. Identifiers other than
// nil/false (named constants, variables) are excluded — a named constant
// expresses a deliberate result even when its value happens to be zero.
func isZeroValueExpr(e ast.Expr) bool {
	switch v := e.(type) {
	case *ast.Ident:
		return v.Name == "nil" || v.Name == "false"
	case *ast.BasicLit:
		switch v.Kind {
		case token.INT:
			n, err := strconv.ParseUint(v.Value, 0, 64)
			return err == nil && n == 0
		case token.FLOAT:
			f, err := strconv.ParseFloat(v.Value, 64)
			return err == nil && f == 0
		case token.STRING:
			return v.Value == `""` || v.Value == "``"
		}
	}
	return false
}
