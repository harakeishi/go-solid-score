package model

// StructInfo represents a Go struct type and its associated methods.
type StructInfo struct {
	Name       string
	File       string
	Line       int
	Fields     []*FieldInfo
	Methods    []*MethodInfo
	Embeddings []string // embedded type names
}

// FieldInfo represents a single struct field.
type FieldInfo struct {
	Name     string
	TypeName string
	IsIface  bool
	// IsFunc reports whether the field's (unwrapped) type is a function type,
	// i.e. a callback/strategy field rather than a concrete collaborator.
	IsFunc bool
	// IsValue reports whether the field's type is fundamentally a value/data
	// type (basic, slice, array, map, or channel — including named aliases),
	// as opposed to a struct or interface collaborator.
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
