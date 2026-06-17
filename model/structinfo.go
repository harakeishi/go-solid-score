package model

// StructInfo represents a Go struct type and its associated methods.
type StructInfo struct {
	Name       string
	File       string
	Line       int
	Fields     []*FieldInfo
	Methods    []*MethodInfo
	Embeddings []string // embedded type names
	// Doc is the doc comment attached to the type declaration (without the
	// leading "// " markers). Empty when the type has no doc comment. Used by
	// the evaluation harness to read inline `// solid:want` ground-truth labels.
	Doc string
}

// FieldInfo represents a single struct field.
type FieldInfo struct {
	Name     string
	TypeName string
	IsIface  bool
	// IsFunc reports whether the field's (unwrapped) type is a function type,
	// i.e. a callback/strategy field rather than a concrete collaborator.
	IsFunc bool
	// IsValue reports whether the field's core element type (after unwrapping
	// pointers/slices/maps/etc.) is a builtin basic type — i.e. pure data such
	// as int, string, map[string]string, or a named alias of one. Collections
	// of structs or interfaces are NOT value types: their element is a
	// collaborator that DIP still weighs.
	IsValue bool
}

// PublicMethods returns methods with exported names.
func (s *StructInfo) PublicMethods() []*MethodInfo {
	var result []*MethodInfo
	for _, m := range s.Methods {
		if m.IsExported {
			result = append(result, m)
		}
	}
	return result
}
