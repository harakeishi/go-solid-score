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

	// Structs are scored on their public interface size (an SRP-leaning signal,
	// kept for continuity and as the FatImpl recall guard).
	for _, s := range pkg.Structs {
		results = append(results, a.analyzeStruct(s, pkg))
	}

	// Interface definitions are the principle's true subject: ISP violations
	// live in fat interfaces that force clients to depend on methods they do
	// not use. Only in-package interfaces are scored, so external contracts
	// (afero.File etc.) are structurally excluded.
	for _, iface := range pkg.Interfaces {
		results = append(results, a.analyzeInterface(iface, pkg))
	}

	return results
}

func (a *ISPAnalyzer) analyzeStruct(s *model.StructInfo, pkg *model.PackageInfo) Result {
	r := Result{
		Principle:  ISP,
		TargetPkg:  pkg.PkgPath,
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

	// Detect decorator/adapter pattern: single non-whitelisted field where
	// all public methods delegate to it. In this case, the large public
	// surface is dictated by the wrapped type, not by poor design.
	if isDecoratorPattern(s, pubMethods) {
		r.Score = 85
		r.Confidence = ConfidenceMediumHigh
		r.Details = append(r.Details, fmt.Sprintf("decorator/adapter pattern detected (%d methods delegating to single field)", len(pubMethods)))
		r.Score = Clamp(r.Score)
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

	// Check interfaces defined in the same package.
	// NOTE: the "large interface" threshold here (mc > 5) intentionally mirrors
	// the interface-definition scoring in analyzeInterface (mc <= 5 → top score).
	// If you retune one side's method-count thresholds, revisit the other so the
	// struct-implements penalty and the interface-definition score stay aligned.
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

// analyzeInterface scores a single interface definition by its total method
// count (including methods promoted from embedded interfaces). The thresholds
// map the interfacebloat linter's de-facto standard — a fat interface is one
// with more than ~10 methods — onto the 0-100 scale, so an 11-method interface
// lands below the ISP pass threshold (50) and is flagged as a violation.
// Interfaces composed of smaller role interfaces via embedding are ISP-faithful
// and receive a bonus to avoid penalizing the idiomatic io.ReadWriteCloser
// composition pattern.
func (a *ISPAnalyzer) analyzeInterface(iface *model.InterfaceInfo, pkg *model.PackageInfo) Result {
	r := Result{
		Principle:         ISP,
		TargetPkg:         pkg.PkgPath,
		TargetName:        iface.Name,
		TargetFile:        iface.File,
		TargetLine:        iface.Line,
		TargetIsInterface: true,
		Score:             100,
		Confidence:        ConfidenceMedium,
	}

	// NOTE: these method-count thresholds are mirrored by the struct-side
	// "large interface" penalty in analyzeStruct (mc > 5). Keep the two in sync
	// when retuning — see the cross-reference note there.
	mc := iface.TotalMethods
	switch {
	case mc <= 3:
		// Good - small, focused interface (Go idiom).
	case mc <= 5:
		r.Score = 90
	case mc <= 7:
		r.Score = 75
		r.Details = append(r.Details, fmt.Sprintf("%d methods (consider splitting)", mc))
	case mc <= 10:
		r.Score = 60
		r.Details = append(r.Details, fmt.Sprintf("%d methods (large interface)", mc))
	case mc <= 15:
		r.Score = 40
		r.Details = append(r.Details, fmt.Sprintf("%d methods (fat interface — clients depend on methods they don't use)", mc))
	default:
		r.Score = 20
		r.Details = append(r.Details, fmt.Sprintf("%d methods (severely bloated interface)", mc))
	}

	// Interfaces composed by embedding small role interfaces are ISP-faithful
	// (the io.ReadWriteCloser pattern). The bonus is gated on the count of
	// methods declared *directly* (iface.Methods excludes embedded methods,
	// unlike iface.TotalMethods): an interface that embeds a role interface yet
	// still declares many methods of its own is structurally a fat interface,
	// and a single embed must not rescue it above the violation threshold.
	// Without this gate, e.g. 10 direct methods + 1 embed scores 40+15=55 and
	// escapes detection — the embed-bonus false negative. The interfacebloat
	// linter cannot make this distinction (it counts AST entries); we can,
	// because the loader expands embedded methods via go/types.
	if len(iface.Embeds) > 0 && len(iface.Methods) <= 5 {
		r.Score += 15
		r.Details = append(r.Details, fmt.Sprintf("composes %d embedded interface(s)", len(iface.Embeds)))
	}

	if mc >= 8 || len(iface.Embeds) > 0 {
		r.Confidence = ConfidenceMediumHigh
	}

	r.Score = Clamp(r.Score)
	return r
}

// isDecoratorPattern detects if a struct wraps a single dependency and
// all public methods access only that one field (delegation pattern).
func isDecoratorPattern(s *model.StructInfo, pubMethods []*model.MethodInfo) bool {
	if len(pubMethods) < 3 {
		return false // too few methods to be meaningful
	}

	// Find non-embedded, non-primitive fields
	var significantFields []*model.FieldInfo
	for _, f := range s.Fields {
		if f.Name == "" {
			continue // embedded
		}
		significantFields = append(significantFields, f)
	}

	if len(significantFields) != 1 {
		return false // decorator wraps exactly one dependency
	}

	targetField := significantFields[0].Name

	// Check that most public methods access only the single field
	delegating := 0
	for _, m := range pubMethods {
		accessesTarget := false
		for _, af := range m.AccessedFields {
			if af == targetField {
				accessesTarget = true
				break
			}
		}
		if accessesTarget {
			delegating++
		}
	}

	// At least 70% of public methods must delegate to the single field
	return float64(delegating)/float64(len(pubMethods)) >= 0.7
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
