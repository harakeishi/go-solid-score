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
