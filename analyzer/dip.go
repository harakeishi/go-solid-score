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

// depWeight represents a weighted dependency count.
// Field dependencies (structural) are weighted more heavily than
// method parameter dependencies (call-time).
type depWeight struct {
	total float64
	iface float64
}

const (
	fieldDepWeight       = 1.0
	constructorDepWeight = 1.0
	paramDepWeight       = 0.3 // method params are less significant for DIP
)

func (a *DIPAnalyzer) analyzeStruct(s *model.StructInfo, pkg *model.PackageInfo) Result {
	r := Result{
		Principle:  DIP,
		TargetPkg:  pkg.PkgPath,
		TargetName: s.Name,
		TargetFile: s.File,
		TargetLine: s.Line,
		Score:      100,
		Confidence: ConfidenceHigh,
	}

	var dw depWeight
	var concreteDeps []string

	// Analyze struct fields (highest weight - structural dependencies)
	for _, f := range s.Fields {
		if f.Name == "" {
			continue // skip embedded fields
		}
		if a.skipDep(f.TypeName, f.IsIface, f.IsFunc, f.IsValue, s.Name) {
			continue
		}
		dw.total += fieldDepWeight
		if f.IsIface {
			dw.iface += fieldDepWeight
		} else {
			concreteDeps = append(concreteDeps, fmt.Sprintf("field %s: %s", f.Name, f.TypeName))
		}
	}

	// Analyze constructor parameters (high weight - injected at creation)
	constructor := a.findConstructor(s.Name, pkg)
	if constructor != nil {
		for _, p := range constructor.Params {
			if a.skipDep(p.TypeName, p.IsIface, p.IsFunc, p.IsValue, s.Name) {
				continue
			}
			dw.total += constructorDepWeight
			if p.IsIface {
				dw.iface += constructorDepWeight
			} else {
				concreteDeps = append(concreteDeps, fmt.Sprintf("constructor param %s: %s", p.Name, p.TypeName))
			}
		}
	}

	// Structural dependencies (fields + constructor injection) are what a type
	// *owns* and is the proper subject of DIP. If a type owns no collaborators,
	// DIP is not structurally meaningful: method parameters are call-time data
	// inputs supplied by the caller, and on their own should not drive the
	// score to zero (e.g. a Formatter whose only "dependency" is the *Entry it
	// formats). Method parameters therefore only refine the score when at least
	// one structural dependency exists.
	//
	// "Not applicable" is reported as the default top score with low
	// confidence: a type that owns no concrete dependency vacuously satisfies
	// DIP, and the low confidence flags that the value is not a meaningful
	// signal. This mirrors how a dependency-free struct is scored. (Note: the
	// aggregate total does not currently weigh by confidence, so such a type
	// contributes a high DIP to summaries — see docs/scoring-analysis.md.)
	if dw.total == 0 {
		r.Confidence = ConfidenceLow
		r.Details = append(r.Details, "no owned dependencies (fields/constructor); DIP not applicable (not penalized)")
		return r
	}

	// Analyze exported method parameters (low weight - call-time dependencies)
	for _, m := range s.Methods {
		if !m.IsExported {
			continue
		}
		for _, p := range m.Params {
			if a.skipDep(p.TypeName, p.IsIface, p.IsFunc, p.IsValue, s.Name) {
				continue
			}
			dw.total += paramDepWeight
			if p.IsIface {
				dw.iface += paramDepWeight
			} else {
				concreteDeps = append(concreteDeps, fmt.Sprintf("method %s param %s: %s (low weight)", m.Name, p.Name, p.TypeName))
			}
		}
	}

	// Score based on weighted interface dependency ratio
	ratio := dw.iface / dw.total
	r.Score = ratio * 100

	// Bonus: if constructor accepts interfaces (dependency injection pattern)
	if constructor != nil && hasIfaceParams(constructor.Params) {
		r.Score += 15
		r.Details = append(r.Details, "constructor uses dependency injection")
	}

	if len(concreteDeps) > 0 {
		r.Details = append(r.Details, fmt.Sprintf("%.1f/%.1f weighted dependencies are concrete:", dw.total-dw.iface, dw.total))
		for _, d := range concreteDeps {
			r.Details = append(r.Details, "  - "+d)
		}
	}

	if dw.total >= 3 {
		r.Confidence = ConfidenceMediumHigh
	} else {
		r.Confidence = ConfidenceMedium
	}

	r.Score = Clamp(r.Score)
	return r
}

// skipDep reports whether a field or parameter of the given type should be
// excluded from the DIP dependency ratio. A type is skipped when it is:
//
//   - a whitelisted builtin/stdlib value type (incl. collections of them);
//   - a function type (callback/strategy) — behavioral injection, neither a
//     concrete coupling to invert nor an interface collaborator;
//   - a pure-data value type — one whose core element is a builtin basic type
//     (int, string, map[string]string, named aliases like `type FieldMap
//     map[string]string`, …). These hold data, not collaborators. A collection
//     of structs (e.g. `[]*PaymentService`) is deliberately NOT skipped here:
//     its element is a concrete collaborator and remains a concrete dependency;
//   - a self-reference — recursive/tree structures are structural composition,
//     not injected collaborators.
//
// Excluding these removes the dominant source of false-positive DIP penalties
// observed on idiomatic Go types (config/aggregate structs full of value,
// callback, and self-referential fields), without masking genuine concrete
// dependencies such as `db *sql.DB` or `workers []*Worker`. A container *of
// interfaces* (e.g. `handlers []Handler`) is kept as an abstraction dependency.
func (a *DIPAnalyzer) skipDep(typeName string, isIface, isFunc, isValue bool, structName string) bool {
	if isWhitelisted(typeName, a.userWhitelist) {
		return true
	}
	// isFunc carries the precise (type-checked) answer; the string-prefix check
	// is only a fallback for when type info is unavailable (info == nil) and the
	// type name was rendered as "func(...)".
	if isFunc || strings.HasPrefix(coreTypeName(typeName), "func(") {
		return true
	}
	if isValue && !isIface {
		return true
	}
	return isSelfReference(typeName, structName)
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
