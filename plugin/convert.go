package plugin

import (
	"golang.org/x/tools/go/analysis"

	"github.com/harakeishi/go-solid-score/internal/astutil"
	"github.com/harakeishi/go-solid-score/model"
)

// PackageInfoFromPass constructs a model.PackageInfo from an analysis.Pass.
func PackageInfoFromPass(pass *analysis.Pass) *model.PackageInfo {
	pi := &model.PackageInfo{
		Name:    pass.Pkg.Name(),
		PkgPath: pass.Pkg.Path(),
	}

	structMap := make(map[string]*model.StructInfo)
	ifaceMap := make(map[string]*model.InterfaceInfo)

	for _, file := range pass.Files {
		fpath := pass.Fset.Position(file.Pos()).Filename
		astutil.ExtractDecls(file, fpath, pass.Fset, pass.TypesInfo, structMap, ifaceMap, pi)
	}

	for _, file := range pass.Files {
		fpath := pass.Fset.Position(file.Pos()).Filename
		astutil.ExtractMethods(file, fpath, pass.Fset, pass.TypesInfo, structMap)
	}

	for _, s := range structMap {
		pi.Structs = append(pi.Structs, s)
	}
	for _, iface := range ifaceMap {
		pi.Interfaces = append(pi.Interfaces, iface)
	}

	astutil.CountImplementors(pi, pass.Pkg)

	return pi
}
