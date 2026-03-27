package analyzer

import (
	"fmt"
	"strings"

	"github.com/harakeishi/go-solid-score/model"
)

// DIPAnalyzer checks the Dependency Inversion Principle by measuring
// the ratio of interface vs concrete type dependencies.
type DIPAnalyzer struct {
	userWhitelist []string
}

func NewDIPAnalyzer(userWhitelist []string) *DIPAnalyzer {
	return &DIPAnalyzer{userWhitelist: userWhitelist}
}

func (a *DIPAnalyzer) Principle() Principle { return DIP }

func (a *DIPAnalyzer) Analyze(pkg *model.PackageInfo) []Result {
	var results []Result
	for _, s := range pkg.Structs {
		results = append(results, a.analyzeStruct(s, pkg))
	}
	return results
}

func (a *DIPAnalyzer) analyzeStruct(s *model.StructInfo, pkg *model.PackageInfo) Result {
	r := Result{
		Principle:  DIP,
		TargetName: s.Name,
		TargetFile: s.File,
		TargetLine: s.Line,
		Score:      100,
		Confidence: ConfidenceHigh,
	}

	var totalDeps, ifaceDeps int
	var concreteDeps []string

	// Analyze struct fields
	for _, f := range s.Fields {
		if f.Name == "" {
			continue // skip embedded fields
		}
		if isWhitelisted(f.TypeName, a.userWhitelist) {
			continue
		}
		totalDeps++
		if f.IsIface {
			ifaceDeps++
		} else {
			concreteDeps = append(concreteDeps, fmt.Sprintf("field %s: %s", f.Name, f.TypeName))
		}
	}

	// Analyze constructor parameters (New* functions)
	constructor := a.findConstructor(s.Name, pkg)
	if constructor != nil {
		for _, p := range constructor.Params {
			if isWhitelisted(p.TypeName, a.userWhitelist) {
				continue
			}
			totalDeps++
			if p.IsIface {
				ifaceDeps++
			} else {
				concreteDeps = append(concreteDeps, fmt.Sprintf("constructor param %s: %s", p.Name, p.TypeName))
			}
		}
	}

	// Analyze exported method parameters only
	for _, m := range s.Methods {
		if !m.IsExported {
			continue
		}
		for _, p := range m.Params {
			if isWhitelisted(p.TypeName, a.userWhitelist) {
				continue
			}
			totalDeps++
			if p.IsIface {
				ifaceDeps++
			} else {
				concreteDeps = append(concreteDeps, fmt.Sprintf("method %s param %s: %s", m.Name, p.Name, p.TypeName))
			}
		}
	}

	if totalDeps == 0 {
		r.Confidence = ConfidenceLowMedium
		r.Details = append(r.Details, "no non-whitelisted dependencies")
		return r
	}

	// Score based on interface dependency ratio
	ratio := float64(ifaceDeps) / float64(totalDeps)
	r.Score = ratio * 100

	// Bonus: if constructor accepts interfaces (dependency injection pattern)
	if constructor != nil && hasIfaceParams(constructor.Params) {
		r.Score += 15
		r.Details = append(r.Details, "constructor uses dependency injection")
	}

	if len(concreteDeps) > 0 {
		r.Details = append(r.Details, fmt.Sprintf("%d/%d dependencies are concrete:", len(concreteDeps), totalDeps))
		for _, d := range concreteDeps {
			r.Details = append(r.Details, "  - "+d)
		}
	}

	if totalDeps >= 3 {
		r.Confidence = ConfidenceMediumHigh
	} else {
		r.Confidence = ConfidenceMedium
	}

	r.Score = Clamp(r.Score)
	return r
}

func (a *DIPAnalyzer) findConstructor(structName string, pkg *model.PackageInfo) *model.FuncInfo {
	target := "New" + structName
	for _, f := range pkg.Functions {
		if f.Name == target || (strings.HasPrefix(f.Name, "New") && strings.Contains(f.Name, structName)) {
			return f
		}
	}
	return nil
}

func hasIfaceParams(params []*model.ParamInfo) bool {
	for _, p := range params {
		if p.IsIface {
			return true
		}
	}
	return false
}
