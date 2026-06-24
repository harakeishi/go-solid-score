package analyzer

import (
	"github.com/harakeishi/go-solid-score/model"
)

// ISP (Interface Segregation) scoring is now declarative: public-surface size,
// large-interface implementation, composition bonuses, and cohesion are
// computed in metrics.go and combined by the rules in rules/presets.yaml. This
// file retains the pattern-detection helpers the metric layer depends on.

// isDecoratorPattern detects if a struct wraps a single dependency and
// all public methods access only that one field (delegation pattern). In that
// case the large public surface is dictated by the wrapped type, not by poor
// design, so ISP scoring treats it leniently.
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
		for _, af := range m.AccessedFields {
			if af == targetField {
				delegating++
				break
			}
		}
	}

	// At least 70% of public methods must delegate to the single field
	return float64(delegating)/float64(len(pubMethods)) >= 0.7
}

// structImplements reports whether the struct implements every method of the
// interface (and the interface declares at least one method).
func structImplements(s *model.StructInfo, iface *model.InterfaceInfo) bool {
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
