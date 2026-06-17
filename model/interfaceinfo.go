package model

// InterfaceInfo represents a Go interface type definition.
type InterfaceInfo struct {
	Name         string
	File         string
	Line         int
	Methods      []string // method signature strings
	Embeds       []string // embedded interface names
	TotalMethods int      // total including from embedded interfaces
	Implementors int      // number of types that implement this interface in the analyzed scope
	// Doc is the doc comment attached to the type declaration (without the
	// leading "// " markers). Empty when the type has no doc comment. Used by
	// the evaluation harness to read inline `// solid:want` ground-truth labels.
	Doc string
}
