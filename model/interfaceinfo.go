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
}
