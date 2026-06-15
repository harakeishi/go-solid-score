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

// TestDIPAnalyzer_RecursiveAggregate verifies that value data, callbacks, and
// self-references are not counted as concrete dependencies. Such fields are
// the dominant false-positive source on idiomatic aggregate/config structs.
func TestDIPAnalyzer_RecursiveAggregate(t *testing.T) {
	pkgs, err := parser.Parse([]string{"../testdata/dip"})
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	a := analyzer.NewDIPAnalyzer(nil)
	results := a.Analyze(pkgs[0])

	for _, r := range results {
		if r.TargetName == "Tree" {
			// Only `handlers []Logger` is a real dependency, and it is an
			// interface, so the score should be high rather than ~0.
			if r.Score < 80 {
				t.Errorf("Tree DIP score %.1f should be >= 80 "+
					"(value/callback/self-ref fields must be excluded)", r.Score)
			}
			return
		}
	}
	t.Error("Tree not found in results")
}

// TestDIPAnalyzer_NoOwnedDependencies verifies that a type whose only
// non-value dependency arrives as a method parameter (call-time data, not an
// owned collaborator) is treated as DIP-not-applicable rather than penalized
// to zero, and is reported with low confidence.
func TestDIPAnalyzer_NoOwnedDependencies(t *testing.T) {
	pkgs, err := parser.Parse([]string{"../testdata/dip"})
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	a := analyzer.NewDIPAnalyzer(nil)
	results := a.Analyze(pkgs[0])

	for _, r := range results {
		if r.TargetName == "Printer" {
			if r.Score < 80 {
				t.Errorf("Printer DIP score %.1f should be >= 80 "+
					"(method-param-only deps must not penalize DIP)", r.Score)
			}
			if r.Confidence > analyzer.ConfidenceLowMedium {
				t.Errorf("Printer DIP confidence %.2f should be low "+
					"(DIP not applicable without owned dependencies)", r.Confidence)
			}
			return
		}
	}
	t.Error("Printer not found in results")
}
