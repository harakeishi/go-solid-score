package analyzer

import (
	"fmt"

	"github.com/harakeishi/go-solid-score/model"
)

// ISPAnalyzer checks the Interface Segregation Principle by evaluating
// interface sizes. Go idiom strongly favors small interfaces (1-3 methods).
type ISPAnalyzer struct{}

func NewISPAnalyzer() *ISPAnalyzer { return &ISPAnalyzer{} }

func (a *ISPAnalyzer) Principle() Principle { return ISP }

func (a *ISPAnalyzer) Analyze(pkg *model.PackageInfo) []Result {
	var results []Result

	// Analyze structs based on their public interface size and
	// the interfaces defined in the package
	for _, s := range pkg.Structs {
		results = append(results, a.analyzeStruct(s, pkg))
	}

	return results
}

func (a *ISPAnalyzer) analyzeStruct(s *model.StructInfo, pkg *model.PackageInfo) Result {
	r := Result{
		Principle:  ISP,
		TargetName: s.Name,
		TargetFile: s.File,
		TargetLine: s.Line,
		Score:      100,
		Confidence: ConfidenceMedium,
	}

	pubMethods := s.PublicMethods()
	if len(pubMethods) == 0 {
		r.Confidence = ConfidenceLow
		r.Details = append(r.Details, "no public methods")
		return r
	}

	// Score based on public method count (public interface size)
	methodCount := len(pubMethods)
	switch {
	case methodCount <= 5:
		// Good - small interface
	case methodCount <= 10:
		r.Score = 80
		r.Details = append(r.Details, fmt.Sprintf("%d public methods (consider splitting)", methodCount))
	case methodCount <= 15:
		r.Score = 60
		r.Details = append(r.Details, fmt.Sprintf("%d public methods (too many for one type)", methodCount))
	case methodCount <= 20:
		r.Score = 40
		r.Details = append(r.Details, fmt.Sprintf("%d public methods (fat interface)", methodCount))
	default:
		r.Score = 20
		r.Details = append(r.Details, fmt.Sprintf("%d public methods (severely bloated interface)", methodCount))
	}

	// Check interfaces defined in the same package
	for _, iface := range pkg.Interfaces {
		mc := iface.TotalMethods
		if mc > 5 {
			// Check if this struct implements the large interface
			if a.structImplements(s, iface) {
				penalty := 0.0
				switch {
				case mc <= 8:
					penalty = 10
				case mc <= 12:
					penalty = 20
				default:
					penalty = 30
				}
				r.Score -= penalty
				r.Details = append(r.Details, fmt.Sprintf("implements large interface %s (%d methods)", iface.Name, mc))
			}
		}

		// Bonus: interface uses embedding (composition of small interfaces)
		if len(iface.Embeds) > 0 && a.structImplements(s, iface) {
			r.Score += 10
			r.Details = append(r.Details, fmt.Sprintf("interface %s uses composition", iface.Name))
		}
	}

	// LCOM4 on public methods only - cohesion check
	if len(pubMethods) >= 4 {
		lcom4 := calculateLCOM4(pubMethods)
		if lcom4 > 2 {
			r.Score -= 15
			r.Details = append(r.Details, fmt.Sprintf("low public interface cohesion (LCOM4=%d)", lcom4))
		}
	}

	if len(pubMethods) >= 5 {
		r.Confidence = ConfidenceMediumHigh
	}

	r.Score = Clamp(r.Score)
	return r
}

func (a *ISPAnalyzer) structImplements(s *model.StructInfo, iface *model.InterfaceInfo) bool {
	methodNames := make(map[string]bool)
	for _, m := range s.Methods {
		methodNames[m.Name] = true
	}
	for _, im := range iface.Methods {
		if !methodNames[im] {
			return false
		}
	}
	return len(iface.Methods) > 0
}
