package analyzer

import (
	"strings"

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

// embedsInPackageInterface reports whether the struct embeds an in-package
// interface. Such a struct satisfies that interface without declaring its
// methods, so it must count as "implements an interface" for LSP: otherwise
// the lsp-no-interface stop rule fires and the embed-missing-override signal
// (the whole point of that pattern) can never be scored.
func embedsInPackageInterface(s *model.StructInfo, pkg *model.PackageInfo) bool {
	for _, embed := range s.Embeddings {
		for _, iface := range pkg.Interfaces {
			if iface.Name == embed {
				return true
			}
		}
	}
	return false
}

// embeddedIfaceInjected reports whether a constructor takes one of the
// struct's embedded in-package interfaces as a parameter. That is the static
// signature of a legitimate decorator/middleware — the embedded interface is
// handed a real implementation to delegate to — as opposed to embedding an
// interface merely to claim conformance and leaving it nil, where every
// non-overridden method panics at runtime. The two shapes are otherwise
// identical in the AST, so this is the discriminator the embed-missing-override
// rule uses to spare decorators.
func embeddedIfaceInjected(s *model.StructInfo, pkg *model.PackageInfo) bool {
	constructor := findConstructor(s.Name, pkg)
	if constructor == nil {
		return false
	}
	embeds := make(map[string]bool, len(s.Embeddings))
	for _, embed := range s.Embeddings {
		for _, iface := range pkg.Interfaces {
			if iface.Name == embed {
				embeds[embed] = true
			}
		}
	}
	for _, p := range constructor.Params {
		if embeds[strings.TrimPrefix(p.TypeName, "*")] {
			return true
		}
	}
	return false
}
