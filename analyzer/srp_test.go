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

// TestSRPAnalyzer_GraduatedLCOM4 verifies that the LCOM4 cohesion penalty is
// graduated by average component size. A large aggregate whose methods cluster
// into a few cohesive groups (LargeFacade: 16 methods, LCOM4=2) is a far weaker
// SRP signal than a small type whose methods are mostly disconnected islands
// (GodStruct: 8 methods, LCOM4=5). The facade must therefore score
// meaningfully higher than the fragmented type, and higher than the flat
// per-LCOM4 penalty alone would yield (a flat LCOM4=2 hit of -40 plus the
// -15 method-count penalty would floor it at 45), while still staying below a
// perfect score because the method-count penalty keeps a large type honest.
func TestSRPAnalyzer_GraduatedLCOM4(t *testing.T) {
	pkgs, err := parser.Parse([]string{"../testdata/srp"})
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	a := analyzer.NewSRPAnalyzer()
	results := a.Analyze(pkgs[0])

	var facade, god float64
	var facadeFound, godFound bool
	var facadeConf float64
	for _, r := range results {
		switch r.TargetName {
		case "LargeFacade":
			facade, facadeFound, facadeConf = r.Score, true, r.Confidence
		case "GodStruct":
			god, godFound = r.Score, true
		}
	}
	if !facadeFound {
		t.Fatal("LargeFacade not found in results")
	}
	if !godFound {
		t.Fatal("GodStruct not found in results")
	}

	if facade <= god {
		t.Errorf("LargeFacade SRP %.1f should exceed fragmented GodStruct SRP %.1f", facade, god)
	}
	if facade <= 45 {
		t.Errorf("LargeFacade SRP %.1f should exceed the flat-penalty floor (45): "+
			"the cohesion penalty must be attenuated for a large cohesive aggregate", facade)
	}
	if facade >= 90 {
		t.Errorf("LargeFacade SRP %.1f should stay below 90: a large type is still "+
			"penalized by method count even when its cohesion penalty is attenuated", facade)
	}
	// Attenuated aggregates are a weaker SRP signal; confidence is reduced.
	if facadeConf > analyzer.ConfidenceMedium {
		t.Errorf("LargeFacade SRP confidence %.2f should be reduced (<= %.2f) when the "+
			"cohesion penalty is attenuated", facadeConf, analyzer.ConfidenceMedium)
	}
}

// TestSRPAnalyzer_StatelessConventionMethod verifies that a method which
// accesses no receiver field (e.g. an errors.Is convention method) does not
// fragment LCOM4 and penalize an otherwise cohesive type.
func TestSRPAnalyzer_StatelessConventionMethod(t *testing.T) {
	pkgs, err := parser.Parse([]string{"../testdata/srp"})
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	a := analyzer.NewSRPAnalyzer()
	results := a.Analyze(pkgs[0])

	for _, r := range results {
		if r.TargetName == "ParseError" {
			if r.Score < 100 {
				t.Errorf("ParseError SRP score %.1f should be 100 "+
					"(a stateless Is method must not fragment LCOM4)", r.Score)
			}
			return
		}
	}
	t.Error("ParseError not found in results")
}
