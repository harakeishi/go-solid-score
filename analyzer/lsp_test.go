package analyzer_test

import (
	"testing"

	"github.com/harakeishi/go-solid-score/analyzer"
	"github.com/harakeishi/go-solid-score/parser"
)

func TestLSPAnalyzer_Good(t *testing.T) {
	pkgs, err := parser.Parse([]string{"../testdata/lsp"})
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if len(pkgs) == 0 {
		t.Fatal("no packages parsed")
	}

	a := analyzer.NewLSPAnalyzer()
	results := a.Analyze(pkgs[0])

	for _, r := range results {
		if r.TargetName == "Dog" {
			if r.Score < 80 {
				t.Errorf("Dog LSP score %.1f should be >= 80", r.Score)
			}
			return
		}
	}
	t.Error("Dog not found in results")
}

func TestLSPAnalyzer_Bad(t *testing.T) {
	pkgs, err := parser.Parse([]string{"../testdata/lsp"})
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	a := analyzer.NewLSPAnalyzer()
	results := a.Analyze(pkgs[0])

	for _, r := range results {
		if r.TargetName == "ReadOnlySaver" {
			if r.Score >= 90 {
				t.Errorf("ReadOnlySaver LSP score %.1f should be < 90 (panics in Save)", r.Score)
			}
			return
		}
	}
	t.Error("ReadOnlySaver not found in results")
}
