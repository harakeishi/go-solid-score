// Package model defines the data types that represent Go source code
// structures (packages, structs, interfaces, methods, and functions)
// used throughout the analysis pipeline.
package model

// PackageInfo aggregates all analyzed elements from a Go package.
type PackageInfo struct {
	Name       string
	Dir        string
	Structs    []*StructInfo
	Interfaces []*InterfaceInfo
	Functions  []*FuncInfo
}
