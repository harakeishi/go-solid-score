//go:build cgo

package parser_test

import (
	"strings"
	"testing"

	"github.com/harakeishi/go-solid-score/parser"
)

// TestParse_CgoExcludesSyntheticTypes verifies that types from cgo-generated
// files (the _Ctype_* mangled structs synthesized into the build cache) are
// not extracted as analysis targets, while real user-defined types in the same
// cgo package still are. cgo puts generated AST files in pkg.Syntax that are
// absent from pkg.GoFiles; extracting from them surfaces phantom types scored
// 100/100 that the user never wrote.
//
// This file is gated on the `cgo` build tag so it is only compiled — and the
// guarantee only asserted — when cgo is actually available. Under CGO_ENABLED=0
// the file is excluded entirely rather than silently t.Skip-ing, so a CI runner
// without cgo does not mask a regression behind a green skip; the gating is
// explicit in the build constraint instead of hidden in a runtime branch.
func TestParse_CgoExcludesSyntheticTypes(t *testing.T) {
	pkgs, err := parser.Parse([]string{"../testdata/cgopkg"})
	if err != nil {
		// With cgo available the package must load; a failure here is a real
		// failure, not a reason to skip.
		t.Fatalf("cgo package failed to load: %v", err)
	}
	if len(pkgs) == 0 {
		t.Fatal("no packages parsed")
	}

	var sawWrapper bool
	for _, pkg := range pkgs {
		for _, s := range pkg.Structs {
			if strings.HasPrefix(s.Name, "_Ctype_") || strings.HasPrefix(s.Name, "_cgo") {
				t.Errorf("cgo synthetic type %q must not be extracted", s.Name)
			}
			if s.Name == "Wrapper" {
				sawWrapper = true
			}
		}
	}
	if !sawWrapper {
		t.Error("real user type Wrapper should be detected in the cgo package")
	}
}
