package analyzer

import (
	"fmt"

	"github.com/harakeishi/go-solid-score/model"
)

// LSPAnalyzer checks the Liskov Substitution Principle by detecting
// methods that panic, are no-ops, or may violate interface contracts.
type LSPAnalyzer struct{}

func NewLSPAnalyzer() *LSPAnalyzer { return &LSPAnalyzer{} }

func (a *LSPAnalyzer) Principle() Principle { return LSP }

func (a *LSPAnalyzer) Analyze(pkg *model.PackageInfo) []Result {
	var results []Result
	for _, s := range pkg.Structs {
		results = append(results, a.analyzeStruct(s, pkg))
	}
	return results
}

func (a *LSPAnalyzer) analyzeStruct(s *model.StructInfo, pkg *model.PackageInfo) Result {
	r := Result{
		Principle:  LSP,
		TargetPkg:  pkg.PkgPath,
		TargetName: s.Name,
		TargetFile: s.File,
		TargetLine: s.Line,
		Score:      100,
		Confidence: ConfidenceMedium,
	}

	// Find which interfaces this struct's methods match
	ifaceMethods := a.findImplementedInterfaceMethods(s, pkg)
	if len(ifaceMethods) == 0 {
		// No interfaces implemented - default score with low confidence
		r.Confidence = ConfidenceLow
		r.Details = append(r.Details, "no interface implementations detected")
		return r
	}

	r.Confidence = ConfidenceMediumHigh

	for _, m := range s.Methods {
		if !ifaceMethods[m.Name] {
			continue
		}

		// Penalty: method panics unconditionally (e.g. a "not implemented"
		// stub). Guard panics that only fire on invalid arguments/state are
		// idiomatic fail-fast in Go and are not treated as LSP violations.
		if m.HasUnconditionalPanic {
			r.Score -= 20
			r.Details = append(r.Details, fmt.Sprintf("method %s panics unconditionally (possible LSP violation)", m.Name))
		}

		// Penalty: no-op method
		if m.IsNoop {
			r.Score -= 15
			r.Details = append(r.Details, fmt.Sprintf("method %s is a no-op implementation", m.Name))
		}
	}

	// Penalty: embedded interface without full method override
	for _, embed := range s.Embeddings {
		for _, iface := range pkg.Interfaces {
			if iface.Name == embed {
				overridden := make(map[string]bool)
				for _, m := range s.Methods {
					overridden[m.Name] = true
				}
				for _, ifaceMethod := range iface.Methods {
					if !overridden[ifaceMethod] {
						r.Score -= 10
						r.Details = append(r.Details, fmt.Sprintf("embeds interface %s but does not override method %s", embed, ifaceMethod))
					}
				}
			}
		}
	}

	r.Score = Clamp(r.Score)
	return r
}

// findImplementedInterfaceMethods returns a set of method names that are part of
// any interface defined in the package that the struct implements.
func (a *LSPAnalyzer) findImplementedInterfaceMethods(s *model.StructInfo, pkg *model.PackageInfo) map[string]bool {
	methodNames := make(map[string]bool)
	for _, m := range s.Methods {
		methodNames[m.Name] = true
	}

	ifaceMethods := make(map[string]bool)
	for _, iface := range pkg.Interfaces {
		// Check if struct implements this interface
		allMatch := len(iface.Methods) > 0
		for _, im := range iface.Methods {
			if !methodNames[im] {
				allMatch = false
				break
			}
		}
		if allMatch {
			for _, im := range iface.Methods {
				ifaceMethods[im] = true
			}
		}
	}
	return ifaceMethods
}
