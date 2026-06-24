package analyzer

import (
	"strings"

	"github.com/harakeishi/go-solid-score/model"
)

// DIP (Dependency Inversion) scoring is now declarative: the weighted
// interface-vs-concrete dependency ratio and the dependency-injection bonus are
// computed in metrics.go and combined by the rules in rules/presets.yaml. This
// file retains the dependency-weighting helpers the metric layer depends on.

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
func skipDep(typeName string, isIface, isFunc, isValue bool, structName string, whitelist []string) bool {
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
	if isWhitelisted(typeName, whitelist) {
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

// findConstructor locates a likely constructor function for the named struct
// (a New<Name>-style function returning the type), used to weigh injected
// dependencies.
func findConstructor(structName string, pkg *model.PackageInfo) *model.FuncInfo {
	target := "New" + structName
	for _, f := range pkg.Functions {
		if f.Name == target || (strings.HasPrefix(f.Name, "New") && strings.Contains(f.Name, structName)) {
			return f
		}
	}
	return nil
}

// hasIfaceParams reports whether any parameter is an interface type.
func hasIfaceParams(params []*model.ParamInfo) bool {
	for _, p := range params {
		if p.IsIface {
			return true
		}
	}
	return false
}
