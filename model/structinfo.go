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
