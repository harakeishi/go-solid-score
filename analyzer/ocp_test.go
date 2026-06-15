package analyzer_test

import (
	"testing"

	"github.com/harakeishi/go-solid-score/analyzer"
	"github.com/harakeishi/go-solid-score/parser"
)

func TestOCPAnalyzer_Good(t *testing.T) {
	pkgs, err := parser.Parse([]string{"../testdata/ocp"})
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if len(pkgs) == 0 {
		t.Fatal("no packages parsed")
	}

	a := analyzer.NewOCPAnalyzer()
	results := a.Analyze(pkgs[0])

	for _, r := range results {
		if r.TargetName == "Processor" {
			if r.Score < 80 {
				t.Errorf("Processor OCP score %.1f should be >= 80", r.Score)
			}
			return
		}
	}
	t.Error("Processor not found in results")
}

func TestOCPAnalyzer_Bad(t *testing.T) {
	pkgs, err := parser.Parse([]string{"../testdata/ocp"})
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	a := analyzer.NewOCPAnalyzer()
	results := a.Analyze(pkgs[0])

	for _, r := range results {
		if r.TargetName == "Router" {
			if r.Score >= 80 {
				t.Errorf("Router OCP score %.1f should be < 80", r.Score)
			}
			return
		}
	}
	t.Error("Router not found in results")
}

// TestOCPAnalyzer_InterfaceAssertionNotPenalized verifies that a comma-ok
// assertion to an interface (feature detection) is treated as extensible, not
// as an OCP violation — so the type keeps a perfect OCP score.
func TestOCPAnalyzer_InterfaceAssertionNotPenalized(t *testing.T) {
	pkgs, err := parser.Parse([]string{"../testdata/ocp"})
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	a := analyzer.NewOCPAnalyzer()
	results := a.Analyze(pkgs[0])

	for _, r := range results {
		if r.TargetName == "FeatureDispatcher" {
			if r.Score < 100 {
				t.Errorf("FeatureDispatcher OCP score %.1f should be 100 "+
					"(interface feature-detection is not an OCP violation)", r.Score)
			}
			return
		}
	}
	t.Error("FeatureDispatcher not found in results")
}
