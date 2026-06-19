package analyzer_test

import (
	"testing"

	"github.com/harakeishi/go-solid-score/analyzer"
	"github.com/harakeishi/go-solid-score/parser"
)

// findResult returns the result for the named target, or nil if absent.
func findResult(results []analyzer.Result, name string) *analyzer.Result {
	for i := range results {
		if results[i].TargetName == name {
			return &results[i]
		}
	}
	return nil
}

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

// TestISPAnalyzer_FatEmbedInterface is the boundary case for the embedding
// bonus: an interface that embeds a single small role interface (Closer) but
// declares 10 methods directly is still a fat interface. The embed must NOT
// rescue it above the ISP violation threshold (50). Guards against the
// false negative where any embed grants the +15 bonus regardless of how many
// methods are declared directly.
func TestISPAnalyzer_FatEmbedInterface(t *testing.T) {
	pkgs, err := parser.Parse([]string{"../testdata/isp"})
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	a := analyzer.NewISPAnalyzer()
	results := a.Analyze(pkgs[0])

	r := findResult(results, "FatEmbedInterface")
	if r == nil {
		t.Fatal("FatEmbedInterface not found in results")
	}
	if r.Score >= 50 {
		t.Errorf("FatEmbedInterface ISP score %.1f should be < 50: embedding one role interface must not rescue a directly-bloated interface", r.Score)
	}
}

// TestISPAnalyzer_FatInterfaceConfidence asserts that a flagged fat interface
// is reported with at least medium-high confidence, so downstream consumers
// can trust the violation signal.
func TestISPAnalyzer_FatInterfaceConfidence(t *testing.T) {
	pkgs, err := parser.Parse([]string{"../testdata/isp"})
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	a := analyzer.NewISPAnalyzer()
	results := a.Analyze(pkgs[0])

	r := findResult(results, "FatInterface")
	if r == nil {
		t.Fatal("FatInterface not found in results")
	}
	if r.Confidence < analyzer.ConfidenceMediumHigh {
		t.Errorf("FatInterface confidence %.2f should be >= ConfidenceMediumHigh (%.2f)", r.Confidence, analyzer.ConfidenceMediumHigh)
	}
}
