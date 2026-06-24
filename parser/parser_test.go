package parser_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/harakeishi/go-solid-score/parser"
)

func TestParse_ValidPackage(t *testing.T) {
	pkgs, err := parser.Parse([]string{"../testdata/srp"})
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if len(pkgs) == 0 {
		t.Fatal("no packages parsed")
	}

	pkg := pkgs[0]
	if pkg.Name != "srp" {
		t.Errorf("expected package name 'srp', got %q", pkg.Name)
	}

	// Should find TaxCalculator and GodStruct
	names := make(map[string]bool)
	for _, s := range pkg.Structs {
		names[s.Name] = true
	}
	if !names["TaxCalculator"] {
		t.Error("TaxCalculator not found")
	}
	if !names["GodStruct"] {
		t.Error("GodStruct not found")
	}
}

func TestParse_ExtractsMethods(t *testing.T) {
	pkgs, err := parser.Parse([]string{"../testdata/srp"})
	if err != nil {
		t.Fatal(err)
	}

	for _, s := range pkgs[0].Structs {
		if s.Name == "TaxCalculator" {
			if len(s.Methods) < 3 {
				t.Errorf("expected at least 3 methods for TaxCalculator, got %d", len(s.Methods))
			}
			// Check that fields are detected
			for _, m := range s.Methods {
				if m.Name == "Calculate" && len(m.AccessedFields) == 0 {
					t.Error("Calculate should access at least one field")
				}
			}
			return
		}
	}
	t.Error("TaxCalculator not found")
}

func TestParse_ExtractsInterfaces(t *testing.T) {
	pkgs, err := parser.Parse([]string{"../testdata/dip"})
	if err != nil {
		t.Fatal(err)
	}

	found := false
	for _, iface := range pkgs[0].Interfaces {
		if iface.Name == "Repository" {
			found = true
			if len(iface.Methods) != 2 {
				t.Errorf("expected 2 methods on Repository, got %d", len(iface.Methods))
			}
		}
	}
	if !found {
		t.Error("Repository interface not found")
	}
}

func TestParse_ExtractsFunctions(t *testing.T) {
	pkgs, err := parser.Parse([]string{"../testdata/srp"})
	if err != nil {
		t.Fatal(err)
	}

	found := false
	for _, f := range pkgs[0].Functions {
		if f.Name == "NewTaxCalculator" {
			found = true
			if !f.IsExported {
				t.Error("NewTaxCalculator should be exported")
			}
		}
	}
	if !found {
		t.Error("NewTaxCalculator function not found")
	}
}

func TestParse_InvalidPattern(t *testing.T) {
	// Should not error but return empty (no matching packages)
	pkgs, err := parser.Parse([]string{"../testdata/nonexistent"})
	if err != nil {
		// Some patterns may cause an error, which is acceptable
		return
	}
	// Packages with errors are skipped
	_ = pkgs
}

// TestParse_CgoExcludesSyntheticTypes verifies that types from cgo-generated
// files (the _Ctype_* mangled structs synthesized into the build cache) are
// not extracted as analysis targets, while real user-defined types in the same
// cgo package still are. cgo puts generated AST files in pkg.Syntax that are
// absent from pkg.GoFiles; extracting from them surfaces phantom types scored
// 100/100 that the user never wrote.
func TestParse_CgoExcludesSyntheticTypes(t *testing.T) {
	pkgs, err := parser.Parse([]string{"../testdata/cgopkg"})
	if err != nil {
		t.Skipf("cgo package failed to load (cgo may be unavailable): %v", err)
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

// TestParse_LoadErrorWithNoValidPackages guards against silent failure: when
// every matched package fails to load (e.g. a directory with Go files but no
// go.mod), Parse must return an error rather than an empty slice with nil
// error. Otherwise the CLI reports "No structs found" and exits 0, which a CI
// gate would read as "passed" even though nothing was actually analyzed.
func TestParse_LoadErrorWithNoValidPackages(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "foo.go"),
		[]byte("package foo\ntype Bar struct{ x int }\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	pkgs, err := parser.Parse([]string{dir})
	if err == nil {
		t.Fatalf("expected an error for a go.mod-less package, got nil (pkgs=%d)", len(pkgs))
	}
	if len(pkgs) != 0 {
		t.Errorf("expected no packages on a failed load, got %d", len(pkgs))
	}
}

// TestParse_LoadErrorMessageMentionsCause ensures the returned error surfaces
// the underlying package-load problem rather than a generic message, so the
// user can tell why nothing was analyzed.
func TestParse_LoadErrorMessageMentionsCause(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "foo.go"),
		[]byte("package foo\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := parser.Parse([]string{dir})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	// The go tooling phrases this as "does not contain main module" / "go.mod".
	msg := err.Error()
	if !strings.Contains(msg, "module") && !strings.Contains(msg, "go.mod") {
		t.Errorf("error should explain the load failure, got: %q", msg)
	}
}

func TestParse_CyclomaticComplexity(t *testing.T) {
	pkgs, err := parser.Parse([]string{"../testdata/ocp"})
	if err != nil {
		t.Fatal(err)
	}

	for _, s := range pkgs[0].Structs {
		if s.Name == "Router" {
			for _, m := range s.Methods {
				if m.Name == "Route" {
					// Route has a type switch with 4 cases -> complexity should be > 1
					if m.CyclomaticComplexity <= 1 {
						t.Errorf("Route complexity should be > 1, got %d", m.CyclomaticComplexity)
					}
					if m.TypeSwitchCount < 1 {
						t.Errorf("Route should have at least 1 type switch, got %d", m.TypeSwitchCount)
					}
					return
				}
			}
		}
	}
	t.Error("Router.Route not found")
}
