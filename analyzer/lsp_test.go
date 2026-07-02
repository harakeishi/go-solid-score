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

// TestLSPAnalyzer_GuardPanicNotPenalized verifies that a method which panics
// only inside an argument guard (fail-fast, idiomatic Go) is not treated as an
// LSP violation — unlike an unconditional "not implemented" panic.
func TestLSPAnalyzer_GuardPanicNotPenalized(t *testing.T) {
	pkgs, err := parser.Parse([]string{"../testdata/lsp"})
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	a := analyzer.NewLSPAnalyzer()
	results := a.Analyze(pkgs[0])

	for _, r := range results {
		if r.TargetName == "GuardedSaver" {
			if r.Score < 100 {
				t.Errorf("GuardedSaver LSP score %.1f should be 100 "+
					"(guard panics are fail-fast, not LSP violations)", r.Score)
			}
			return
		}
	}
	t.Error("GuardedSaver not found in results")
}

// TestLSPAnalyzer_EmbeddedInterfaceUnderOverride verifies that a struct which
// satisfies an in-package interface only by embedding it is still held to that
// contract: without this, the no-interface stop rule would fire (the struct
// declares almost none of the methods itself) and the embed-missing-override
// penalty could never apply.
func TestLSPAnalyzer_EmbeddedInterfaceUnderOverride(t *testing.T) {
	pkgs, err := parser.Parse([]string{"../testdata/lsp"})
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	a := analyzer.NewLSPAnalyzer()
	results := a.Analyze(pkgs[0])

	for _, r := range results {
		if r.TargetName == "LazyCodec" {
			if r.Score >= 50 {
				t.Errorf("LazyCodec LSP score %.1f should be < 50 "+
					"(embeds Codec but overrides 1 of 7 methods)", r.Score)
			}
			return
		}
	}
	t.Error("LazyCodec not found in results")
}
