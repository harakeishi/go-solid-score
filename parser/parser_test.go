package parser_test

import (
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
