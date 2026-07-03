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

// dataConventionMethods are method names that data records idiomatically
// implement to satisfy fmt/error/encoding protocols (fmt.Stringer,
// fmt.GoStringer, error, json/text/yaml/xml/binary marshalling). They expose
// the record's own data in another shape rather than invoking behavior on
// collaborators, so implementing them does not make a type behavioral for the
// DIP applicability test in isDataType.
var dataConventionMethods = map[string]bool{
	"String": true, "GoString": true, "Error": true, "Format": true,
	"MarshalJSON": true, "UnmarshalJSON": true,
	"MarshalText": true, "UnmarshalText": true,
	"MarshalYAML": true, "UnmarshalYAML": true,
	"MarshalXML": true, "UnmarshalXML": true,
	"MarshalBinary": true, "UnmarshalBinary": true,
}

// isDataType reports whether the struct is a behavior-less data record: a type
// that carries data but exposes no behavior of its own. DIP governs the
// inversion of *collaborator* dependencies — abstractions a high-level policy
// calls into — so a type with no behavior has nothing to invert: the structs it
// aggregates (`[]*ParamInfo`, a nested report block) are the data it stores,
// not services it uses. Such targets are reported as "DIP not applicable"
// (like dip-no-dependencies) instead of being scored on their field types,
// which systematically flagged idiomatic data models (AST nodes, DTOs,
// serialization/report structs) as total DIP violations.
//
// A struct qualifies only when it has no embedded types (embedding promotes
// the embedded type's method set, i.e. inherits behavior) and every declared
// method is one of:
//
//   - a documented Go convention method — the errors Is/As/Unwrap protocol
//     (isConventionMethod, matched by signature) or a formatting/marshalling
//     method from dataConventionMethods;
//   - a pure accessor (isAccessorMethod) — it returns a value and invokes
//     nothing while producing it.
//
// Any other method keeps the type behavioral, so genuine collaborator owners
// remain fully scored: a method that returns nothing exists for its effect
// (e.g. a Run method driving a held `[]*Worker`), and a method that calls
// functions or methods delegates work rather than exposing stored data.
func isDataType(s *model.StructInfo) bool {
	if len(s.Embeddings) > 0 {
		return false
	}
	for _, m := range s.Methods {
		if isConventionMethod(m) || dataConventionMethods[m.Name] || isAccessorMethod(m) {
			continue
		}
		return false
	}
	return true
}

// isAccessorMethod reports whether a method is a pure accessor over the
// receiver's data: it returns at least one value and calls no functions or
// methods while computing it (CalledMethods records every selector-based
// function reference in the body, so filtering, counting, or reshaping own
// fields still qualifies — delegation of any kind does not).
func isAccessorMethod(m *model.MethodInfo) bool {
	return len(m.Returns) > 0 && len(m.CalledMethods) == 0
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
//     map[string]string`, …), or a *value-element collection* of a struct
//     (`[]Message`, `map[string]Event`). These hold data records the struct
//     stores, not collaborators it calls into. A *pointer* collection of a
//     struct (e.g. `[]*PaymentService`, `workers []*Worker`) is deliberately NOT
//     skipped: collaborators are idiomatically held by pointer, so its element
//     is a concrete collaborator and remains a concrete dependency (see
//     IsValueType for the value-vs-pointer-element distinction);
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
