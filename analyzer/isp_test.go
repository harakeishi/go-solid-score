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

// TestISPAnalyzer_FatInterface checks that a fat interface definition is scored
// below the ISP threshold (50) as its own target — the principle's true subject.
func TestISPAnalyzer_FatInterface(t *testing.T) {
	pkgs, err := parser.Parse([]string{"../testdata/isp"})
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	a := analyzer.NewISPAnalyzer()
	results := a.Analyze(pkgs[0])

	for _, r := range results {
		if r.TargetName == "FatInterface" {
			if r.Score >= 50 {
				t.Errorf("FatInterface ISP score %.1f should be < 50 (flagged as a violation)", r.Score)
			}
			return
		}
	}
	t.Error("FatInterface not found in results — interfaces are not being scored")
}

// TestISPAnalyzer_SmallInterface checks that a small, focused interface scores
// at the top (Go idiom: io.Reader-style single-method interfaces).
func TestISPAnalyzer_SmallInterface(t *testing.T) {
	pkgs, err := parser.Parse([]string{"../testdata/isp"})
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	a := analyzer.NewISPAnalyzer()
	results := a.Analyze(pkgs[0])

	for _, r := range results {
		if r.TargetName == "Reader" {
			if r.Score < 90 {
				t.Errorf("Reader (1 method) ISP score %.1f should be >= 90", r.Score)
			}
			return
		}
	}
	t.Error("Reader interface not found in results")
}

// TestISPAnalyzer_ComposedInterface checks that an interface composed of small
// role interfaces via embedding is not flagged (it is ISP-faithful).
func TestISPAnalyzer_ComposedInterface(t *testing.T) {
	pkgs, err := parser.Parse([]string{"../testdata/isp"})
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	a := analyzer.NewISPAnalyzer()
	results := a.Analyze(pkgs[0])

	for _, r := range results {
		if r.TargetName == "ReadWriter" {
			if r.Score < 90 {
				t.Errorf("ReadWriter (composed) ISP score %.1f should be >= 90 (no FP)", r.Score)
			}
			return
		}
	}
	t.Error("ReadWriter interface not found in results")
}
