package analyzer_test

import (
	"testing"

	"github.com/harakeishi/go-solid-score/analyzer"
	"github.com/harakeishi/go-solid-score/parser"
)

func TestISPAnalyzer_Good(t *testing.T) {
	pkgs, err := parser.Parse([]string{"../testdata/isp"})
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if len(pkgs) == 0 {
		t.Fatal("no packages parsed")
	}

	a := analyzer.NewISPAnalyzer()
	results := a.Analyze(pkgs[0])

	for _, r := range results {
		if r.TargetName == "SimpleReader" {
			if r.Score < 80 {
				t.Errorf("SimpleReader ISP score %.1f should be >= 80", r.Score)
			}
			return
		}
	}
	t.Error("SimpleReader not found in results")
}

func TestISPAnalyzer_Bad(t *testing.T) {
	pkgs, err := parser.Parse([]string{"../testdata/isp"})
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	a := analyzer.NewISPAnalyzer()
	results := a.Analyze(pkgs[0])

	for _, r := range results {
		if r.TargetName == "FatImpl" {
			if r.Score >= 80 {
				t.Errorf("FatImpl ISP score %.1f should be < 80", r.Score)
			}
			return
		}
	}
	t.Error("FatImpl not found in results")
}
