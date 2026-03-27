package analyzer_test

import (
	"testing"

	"github.com/harakeishi/go-solid-score/analyzer"
	"github.com/harakeishi/go-solid-score/parser"
)

func TestDIPAnalyzer_Good(t *testing.T) {
	pkgs, err := parser.Parse([]string{"../testdata/dip"})
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if len(pkgs) == 0 {
		t.Fatal("no packages parsed")
	}

	a := analyzer.NewDIPAnalyzer(nil)
	results := a.Analyze(pkgs[0])

	for _, r := range results {
		if r.TargetName == "Service" {
			if r.Score < 80 {
				t.Errorf("Service DIP score %.1f should be >= 80", r.Score)
			}
			return
		}
	}
	t.Error("Service not found in results")
}

func TestDIPAnalyzer_Bad(t *testing.T) {
	pkgs, err := parser.Parse([]string{"../testdata/dip"})
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	a := analyzer.NewDIPAnalyzer(nil)
	results := a.Analyze(pkgs[0])

	for _, r := range results {
		if r.TargetName == "BadService" {
			if r.Score >= 80 {
				t.Errorf("BadService DIP score %.1f should be < 80", r.Score)
			}
			return
		}
	}
	t.Error("BadService not found in results")
}
