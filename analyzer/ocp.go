package analyzer

import (
	"fmt"

	"github.com/harakeishi/go-solid-score/model"
)

// OCPAnalyzer checks the Open/Closed Principle by detecting type switches,
// type assertions, and reflect usage.
type OCPAnalyzer struct{}

func NewOCPAnalyzer() *OCPAnalyzer { return &OCPAnalyzer{} }

func (a *OCPAnalyzer) Principle() Principle { return OCP }

func (a *OCPAnalyzer) Analyze(pkg *model.PackageInfo) []Result {
	var results []Result
	for _, s := range pkg.Structs {
		results = append(results, a.analyzeStruct(s, pkg.PkgPath))
	}
	return results
}

func (a *OCPAnalyzer) analyzeStruct(s *model.StructInfo, pkgPath string) Result {
	r := Result{
		Principle:  OCP,
		TargetPkg:  pkgPath,
		TargetName: s.Name,
		TargetFile: s.File,
		TargetLine: s.Line,
		Score:      100,
		Confidence: ConfidenceMedium,
	}

	if len(s.Methods) == 0 {
		r.Confidence = ConfidenceLow
		return r
	}

	var totalTypeSwitches, totalTypeAsserts, totalReflect, totalStmts int
	var ifaceParamCount int

	for _, m := range s.Methods {
		totalTypeSwitches += m.TypeSwitchCount
		totalTypeAsserts += m.TypeAssertCount
		totalReflect += m.ReflectUsageCount
		totalStmts += m.StmtCount

		for _, p := range m.Params {
			if p.IsIface {
				ifaceParamCount++
			}
		}
	}

	// Type switch penalty
	if totalTypeSwitches > 0 {
		penalty := float64(totalTypeSwitches) * 15
		if penalty > 40 {
			penalty = 40
		}
		r.Score -= penalty
		r.Details = append(r.Details, fmt.Sprintf("%d type switches detected", totalTypeSwitches))
	}

	// Type assertion penalty
	if totalTypeAsserts > 0 {
		penalty := float64(totalTypeAsserts) * 10
		if penalty > 40 {
			penalty = 40
		}
		r.Score -= penalty
		r.Details = append(r.Details, fmt.Sprintf("%d type assertions detected", totalTypeAsserts))
	}

	// Reflect usage penalty
	if totalReflect > 0 {
		penalty := float64(totalReflect) * 5
		if penalty > 20 {
			penalty = 20
		}
		r.Score -= penalty
		r.Details = append(r.Details, fmt.Sprintf("%d reflect usages detected", totalReflect))
	}

	// Conditional density penalty
	if totalStmts > 0 {
		density := float64(totalTypeSwitches+totalTypeAsserts+totalReflect) / float64(totalStmts)
		if density > 0.3 {
			r.Score -= 20
			r.Details = append(r.Details, fmt.Sprintf("high type-check density: %.2f", density))
		} else if density > 0.15 {
			r.Score -= 10
		}
	}

	// Interface parameter bonus (encourages accepting interfaces)
	if ifaceParamCount > 0 {
		bonus := float64(ifaceParamCount) * 5
		if bonus > 20 {
			bonus = 20
		}
		r.Score += bonus
		r.Details = append(r.Details, fmt.Sprintf("+%.0f bonus for %d interface parameters", bonus, ifaceParamCount))
	}

	if len(s.Methods) >= 5 {
		r.Confidence = ConfidenceHigh
	}

	r.Score = Clamp(r.Score)
	return r
}
