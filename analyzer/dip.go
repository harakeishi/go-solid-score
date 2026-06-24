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

	// dipNeutralScore is the floor applied when a type has only call-time
	// (method-parameter) dependencies and no owned (field/constructor) ones.
	// Such a concrete parameter is ambiguous (a real collaborator vs. a data
	// object), so the score is neither slammed to zero nor lifted to 100.
	dipNeutralScore = 50.0
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

	// Analyze struct fields (highest weight - structural dependencies).
	// Embedded fields (Name == "") are NOT skipped: embedding is the tightest
	// form of structural coupling, so an embedded concrete type is a concrete
	// dependency and an embedded interface is an abstraction dependency — both
	// belong in the ratio. skipDep still filters whitelisted value types,
	// callbacks, and self-references (recursive embeds) for embedded fields too.
	for _, f := range s.Fields {
		if a.skipDep(f.TypeName, f.IsIface, f.IsFunc, f.IsValue, s.Name) {
			continue
		}
		dw.total += fieldDepWeight
		if f.IsIface {
			dw.iface += fieldDepWeight
		} else {
			name := f.Name
			if name == "" {
				name = "(embedded)"
			}
			concreteDeps = append(concreteDeps, fmt.Sprintf("field %s: %s", name, f.TypeName))
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
	// *owns* and is the proper subject of DIP.
	structuralTotal := dw.total

	// Analyze exported method parameters (low weight - call-time dependencies).
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

	// No dependencies at all: the type owns nothing concrete and takes nothing
	// concrete, so it vacuously satisfies DIP. Report the top score but with low
	// confidence to flag that the value is not a meaningful signal.
	if dw.total == 0 {
		r.Confidence = ConfidenceLow
		r.Details = append(r.Details, "no dependencies; DIP not applicable")
		return r
	}

	ratio := dw.iface / dw.total

	// Only call-time (method-parameter) dependencies, no owned ones. A concrete
	// method parameter is ambiguous: it may be a genuine collaborator
	// (Run(db *sql.DB) — a real concrete coupling worth flagging) or merely a
	// data object the method operates on (Format(*Entry) — not a dependency to
	// invert). Because the two are structurally indistinguishable, the ratio is
	// floored at a neutral value with low confidence: this neither confidently
	// penalizes a DTO-taking method to zero (a false positive) nor confidently
	// absolves a concrete service coupling at 100 (a false negative). When the
	// parameters are interfaces the ratio already lifts the score above neutral.
	if structuralTotal == 0 {
		r.Score = Clamp(ratio * 100)
		if r.Score < dipNeutralScore {
			r.Score = dipNeutralScore
		}
		r.Confidence = ConfidenceLow
		r.Details = append(r.Details, "only call-time (method-parameter) dependencies; DIP weakly applicable")
		for _, d := range concreteDeps {
			r.Details = append(r.Details, "  - "+d)
		}
		return r
	}

	// Score based on weighted interface dependency ratio
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
	// Invariant: an interface dependency is always an abstraction and is never
	// skipped — counting it is the whole point of DIP. Establishing this once up
	// front means every rule below (whitelist, value-type, self-reference) is
	// understood to apply to non-interface types only, so none of them needs to
	// re-check !isIface. Erasing a whitelisted interface (io.Reader, error, …)
	// previously let a co-occurring concrete dependency drag the ratio to 0.
	//
	// A struct and an interface cannot share a name in one package, so an
	// interface field is never a self-reference; returning here changes no
	// existing behavior, it only consolidates the rule.
	if isIface {
		return false
	}
	if isWhitelisted(typeName, a.userWhitelist) {
		return true
	}
	// isFunc carries the precise (type-checked) answer; the string-prefix check
	// is only a fallback for when type info is unavailable (info == nil) and the
	// type name was rendered as "func(...)".
	if isFunc || strings.HasPrefix(coreTypeName(typeName), "func(") {
		return true
	}
	if isValue {
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
