package analyzer_test

import (
	"testing"

	"github.com/harakeishi/go-solid-score/analyzer"
	"github.com/harakeishi/go-solid-score/parser"
)

func TestSRPAnalyzer_Good(t *testing.T) {
	pkgs, err := parser.Parse([]string{"../testdata/srp"})
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if len(pkgs) == 0 {
		t.Fatal("no packages parsed")
	}

	a := analyzer.NewSRPAnalyzer()
	results := a.Analyze(pkgs[0])

	for _, r := range results {
		if r.TargetName == "TaxCalculator" {
			if r.Score < 80 {
				t.Errorf("TaxCalculator SRP score %.1f should be >= 80", r.Score)
			}
			return
		}
	}
	t.Error("TaxCalculator not found in results")
}

func TestSRPAnalyzer_Bad(t *testing.T) {
	pkgs, err := parser.Parse([]string{"../testdata/srp"})
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	a := analyzer.NewSRPAnalyzer()
	results := a.Analyze(pkgs[0])

	for _, r := range results {
		if r.TargetName == "GodStruct" {
			if r.Score >= 70 {
				t.Errorf("GodStruct SRP score %.1f should be < 70", r.Score)
			}
			return
		}
	}
	t.Error("GodStruct not found in results")
}

// TestSRPAnalyzer_LSCCSeparatesCohesion verifies that LSCC-based scoring
// separates a cohesive type from a fragmented one: the cohesive TaxCalculator
// (all methods share the rate/discount fields) must score well above the
// fragmented GodStruct (methods cluster over disjoint field groups, sharing
// nothing).
func TestSRPAnalyzer_LSCCSeparatesCohesion(t *testing.T) {
	pkgs, err := parser.Parse([]string{"../testdata/srp"})
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	a := analyzer.NewSRPAnalyzer()
	results := a.Analyze(pkgs[0])

	var cohesive, fragmented float64
	var cohesiveFound, fragmentedFound bool
	for _, r := range results {
		switch r.TargetName {
		case "TaxCalculator":
			cohesive, cohesiveFound = r.Score, true
		case "GodStruct":
			fragmented, fragmentedFound = r.Score, true
		}
	}
	if !cohesiveFound {
		t.Fatal("TaxCalculator not found in results")
	}
	if !fragmentedFound {
		t.Fatal("GodStruct not found in results")
	}
	if cohesive <= fragmented {
		t.Errorf("cohesive TaxCalculator SRP %.1f should exceed fragmented GodStruct SRP %.1f", cohesive, fragmented)
	}
	if fragmented >= 70 {
		t.Errorf("fragmented GodStruct SRP %.1f should be < 70 (low LSCC cohesion)", fragmented)
	}
}

// TestSRPAnalyzer_StatelessConventionMethod verifies the cohesive error type
// (ParseError) is not driven below the SRP threshold by the stateless errors.Is
// convention method. LSCC excludes Is/As/Unwrap from the method set, so the
// remaining cohesive Error method (or a too-small effective set) keeps the type
// acceptable rather than reporting a spurious low-cohesion penalty.
func TestSRPAnalyzer_StatelessConventionMethod(t *testing.T) {
	pkgs, err := parser.Parse([]string{"../testdata/srp"})
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	a := analyzer.NewSRPAnalyzer()
	results := a.Analyze(pkgs[0])

	for _, r := range results {
		if r.TargetName == "ParseError" {
			if r.Score < 70 {
				t.Errorf("ParseError SRP score %.1f should stay >= 70 "+
					"(a stateless errors.Is convention method must not drive a cohesive type below threshold)", r.Score)
			}
			return
		}
	}
	t.Error("ParseError not found in results")
}

// TestSRPAnalyzer_NoOwnFieldAccessNotPenalized verifies that a struct whose
// methods read none of its own fields (pure calculators over their parameters)
// is not hit by a false low-cohesion penalty. LSCC is undefined there (no field
// can be shared), so the cohesion rule's own_field_access_method_count >= 2
// guard skips it and the type keeps a perfect SRP score.
func TestSRPAnalyzer_NoOwnFieldAccessNotPenalized(t *testing.T) {
	pkgs, err := parser.Parse([]string{"../testdata/srp"})
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	a := analyzer.NewSRPAnalyzer()
	results := a.Analyze(pkgs[0])

	for _, r := range results {
		if r.TargetName == "MathKit" {
			if r.Score != 100 {
				t.Errorf("MathKit SRP score %.1f should be 100 "+
					"(methods read no own field; cohesion is not applicable, not low)", r.Score)
			}
			return
		}
	}
	t.Error("MathKit not found in results")
}
