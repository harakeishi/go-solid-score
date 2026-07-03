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
	// IsValue reports whether the field models stored data rather than a
	// collaborator: a type whose core element is a builtin basic type (int,
	// string, map[string]string, a named alias of one), a value-element
	// collection of a struct ([]Message), or a bare method-less value struct.
	// Pointer collections of structs ([]*Worker), bare pointers, and interface
	// elements are NOT value types: those are collaborators or abstractions
	// that DIP still weighs. See astutil.IsValueType.
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
