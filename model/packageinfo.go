package model

// PackageInfo aggregates all analyzed elements from a Go package.
type PackageInfo struct {
	Name       string
	Dir        string
	Structs    []*StructInfo
	Interfaces []*InterfaceInfo
	Functions  []*FuncInfo
}
