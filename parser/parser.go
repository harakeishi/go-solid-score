// Package parser loads Go packages using golang.org/x/tools and extracts
// structural information into the model types used by the analyzers.
package parser

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"

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
		// go/packages drops comments by default; keep them so the evaluation
		// harness can read inline `// solid:want` ground-truth labels from type
		// doc comments. Other Mode bits are unaffected.
		ParseFile: func(fset *token.FileSet, filename string, src []byte) (*ast.File, error) {
			return parser.ParseFile(fset, filename, src, parser.ParseComments|parser.SkipObjectResolution)
		},
	}

	pkgs, err := packages.Load(cfg, patterns...)
	if err != nil {
		return nil, fmt.Errorf("loading packages: %w", err)
	}

	// packages.Load reports per-package failures (e.g. a directory with Go
	// files but no go.mod, or a type error) in pkg.Errors rather than the
	// top-level err. We skip packages that failed to load, but track their
	// errors so a load that produced no usable package surfaces as an error
	// instead of an empty result. Otherwise the CLI would print "No structs
	// found" and exit 0, which a CI gate reads as "passed" even though nothing
	// was analyzed.
	var result []*model.PackageInfo
	var loadErrs []string
	for _, pkg := range pkgs {
		if len(pkg.Errors) > 0 {
			for _, e := range pkg.Errors {
				loadErrs = append(loadErrs, e.Error())
			}
			continue
		}
		pi := extractPackageInfo(pkg)
		if pi != nil {
			result = append(result, pi)
		}
	}

	// Only fail when nothing could be analyzed. If at least one package loaded
	// cleanly, a partial failure should not abort the whole run — those errors
	// are reported separately by the caller.
	if len(result) == 0 && len(loadErrs) > 0 {
		return nil, fmt.Errorf("loading packages: %s", strings.Join(loadErrs, "; "))
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

	// pkg.Syntax may include files the user never wrote: for cgo packages it
	// contains cgo-generated AST (in the build cache) whose synthetic _Ctype_*
	// structs would otherwise be scored as phantom targets. pkg.GoFiles lists
	// only the real source files, so restrict extraction to those.
	userFiles := make(map[string]bool, len(pkg.GoFiles))
	for _, f := range pkg.GoFiles {
		userFiles[f] = true
	}

	for _, file := range pkg.Syntax {
		fpath := pkg.Fset.Position(file.Pos()).Filename
		if !userFiles[fpath] {
			continue
		}
		astutil.ExtractDecls(file, fpath, pkg.Fset, pkg.TypesInfo, structMap, ifaceMap, pi)
	}

	for _, file := range pkg.Syntax {
		fpath := pkg.Fset.Position(file.Pos()).Filename
		if !userFiles[fpath] {
			continue
		}
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
