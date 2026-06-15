// Package parser loads Go packages using golang.org/x/tools and extracts
// structural information into the model types used by the analyzers.
package parser

import (
	"fmt"
	"path/filepath"

	"golang.org/x/tools/go/packages"

	"github.com/harakeishi/go-solid-score/internal/astutil"
	"github.com/harakeishi/go-solid-score/model"
)

// Parse loads Go packages and extracts model information.
func Parse(patterns []string) ([]*model.PackageInfo, error) {
	cfg := &packages.Config{
		Mode: packages.NeedName |
			packages.NeedFiles |
			packages.NeedSyntax |
			packages.NeedTypes |
			packages.NeedTypesInfo |
			packages.NeedDeps,
	}

	pkgs, err := packages.Load(cfg, patterns...)
	if err != nil {
		return nil, fmt.Errorf("loading packages: %w", err)
	}

	var result []*model.PackageInfo
	for _, pkg := range pkgs {
		if len(pkg.Errors) > 0 {
			continue
		}
		pi := extractPackageInfo(pkg)
		if pi != nil {
			result = append(result, pi)
		}
	}
	return result, nil
}

func extractPackageInfo(pkg *packages.Package) *model.PackageInfo {
	pi := &model.PackageInfo{
		Name:    pkg.Name,
		PkgPath: pkg.PkgPath,
	}
	if len(pkg.GoFiles) > 0 {
		pi.Dir = filepath.Dir(pkg.GoFiles[0])
	}

	structMap := make(map[string]*model.StructInfo)
	ifaceMap := make(map[string]*model.InterfaceInfo)

	for _, file := range pkg.Syntax {
		fpath := pkg.Fset.Position(file.Pos()).Filename
		astutil.ExtractDecls(file, fpath, pkg.Fset, pkg.TypesInfo, structMap, ifaceMap, pi)
	}

	for _, file := range pkg.Syntax {
		fpath := pkg.Fset.Position(file.Pos()).Filename
		astutil.ExtractMethods(file, fpath, pkg.Fset, pkg.TypesInfo, structMap)
	}

	for _, s := range structMap {
		pi.Structs = append(pi.Structs, s)
	}
	for _, iface := range ifaceMap {
		pi.Interfaces = append(pi.Interfaces, iface)
	}

	astutil.CountImplementors(pi, pkg.Types)

	return pi
}
