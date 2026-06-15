// Package model defines the data types that represent Go source code
// structures (packages, structs, interfaces, methods, and functions)
// used throughout the analysis pipeline.
package model

// PackageInfo aggregates all analyzed elements from a Go package.
type PackageInfo struct {
	Name string
	// PkgPath is the package import path (e.g. "github.com/foo/bar/baz").
	// Unlike Dir, it is independent of the filesystem layout on the analyzing
	// machine, which makes it a stable identifier for diffing scores across
	// commits or environments.
	PkgPath    string
	Dir        string
	Structs    []*StructInfo
	Interfaces []*InterfaceInfo
	Functions  []*FuncInfo
}
