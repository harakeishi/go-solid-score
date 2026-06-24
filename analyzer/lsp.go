package analyzer

import (
	"github.com/harakeishi/go-solid-score/model"
)

// LSP (Liskov Substitution) scoring is now declarative: the panic/no-op and
// embedded-override signals are computed in metrics.go and combined by the
// rules in rules/presets.yaml. This file retains the interface-matching helper
// the metric layer depends on.

// implementedInterfaceMethods returns the set of method names belonging to any
// in-package interface that the struct fully implements. A method that is part
// of an implemented interface is held to that interface's contract (so a panic
// or no-op in it is an LSP smell), whereas a struct's own non-contract methods
// are not.
func implementedInterfaceMethods(s *model.StructInfo, pkg *model.PackageInfo) map[string]bool {
	methodNames := make(map[string]bool)
	for _, m := range s.Methods {
		methodNames[m.Name] = true
	}

	ifaceMethods := make(map[string]bool)
	for _, iface := range pkg.Interfaces {
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
