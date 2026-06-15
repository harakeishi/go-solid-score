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

// TestDIPAnalyzer_ConcreteCollection verifies that a collection of a concrete
// struct (e.g. []*stage) is still counted as a concrete dependency, so the
// value-type exclusion does not silently drop genuine concrete collaborators.
func TestDIPAnalyzer_ConcreteCollection(t *testing.T) {
	pkgs, err := parser.Parse([]string{"../testdata/dip"})
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	a := analyzer.NewDIPAnalyzer(nil)
	results := a.Analyze(pkgs[0])

	for _, r := range results {
		if r.TargetName == "Pipeline" {
			// `stages []*stage` is a collection of a concrete struct, i.e. a
			// genuine concrete dependency. It must still be penalized — value
			// containers (map[string]string) are excluded, concrete-struct
			// containers are not.
			if r.Score >= 80 {
				t.Errorf("Pipeline DIP score %.1f should be < 80 "+
					"([]*stage is a concrete collaborator dependency)", r.Score)
			}
			return
		}
	}
	t.Error("Pipeline not found in results")
}

// TestDIPAnalyzer_NoOwnedDependencies verifies that a type whose only
// non-value dependency arrives as a concrete method parameter (call-time data,
// not an owned collaborator) is scored neutrally with low confidence: not
// penalized to zero (a DTO-taking method is not a false positive) and not
// absolved at 100 (a concrete service coupling is not a false negative).
func TestDIPAnalyzer_NoOwnedDependencies(t *testing.T) {
	pkgs, err := parser.Parse([]string{"../testdata/dip"})
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	a := analyzer.NewDIPAnalyzer(nil)
	results := a.Analyze(pkgs[0])

	for _, r := range results {
		if r.TargetName == "Printer" {
			// Printer.Print(t *Tree) — concrete param, no owned deps.
			if r.Score <= 0 || r.Score >= 80 {
				t.Errorf("Printer DIP score %.1f should be neutral "+
					"(method-param-only concrete dep: neither 0 nor 100)", r.Score)
			}
			if r.Confidence > analyzer.ConfidenceLowMedium {
				t.Errorf("Printer DIP confidence %.2f should be low "+
					"(DIP weakly applicable without owned dependencies)", r.Confidence)
			}
			return
		}
	}
	t.Error("Printer not found in results")
}
