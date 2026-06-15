package scorer_test

import (
	"testing"

	"github.com/harakeishi/go-solid-score/analyzer"
	"github.com/harakeishi/go-solid-score/config"
	"github.com/harakeishi/go-solid-score/model"
	"github.com/harakeishi/go-solid-score/scorer"
)

type stubAnalyzer struct {
	principle analyzer.Principle
	results   []analyzer.Result
}

func (s *stubAnalyzer) Principle() analyzer.Principle { return s.principle }
func (s *stubAnalyzer) Analyze(pkg *model.PackageInfo) []analyzer.Result {
	return s.results
}

func TestScorer_WeightedTotal(t *testing.T) {
	analyzers := []analyzer.Analyzer{
		&stubAnalyzer{
			principle: analyzer.SRP,
			results: []analyzer.Result{{
				Principle: analyzer.SRP, TargetName: "Foo", TargetFile: "foo.go", Score: 100,
			}},
		},
		&stubAnalyzer{
			principle: analyzer.DIP,
			results: []analyzer.Result{{
				Principle: analyzer.DIP, TargetName: "Foo", TargetFile: "foo.go", Score: 0,
			}},
		},
	}

	weights := config.DefaultWeights()
	s := scorer.New(analyzers, weights)

	pkg := &model.PackageInfo{Name: "test"}
	results := s.Score(pkg)

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	r := results[0]
	if r.TargetName != "Foo" {
		t.Errorf("expected target Foo, got %s", r.TargetName)
	}
	// SRP=100*0.30 + DIP=0*0.25 = 30, weightSum=0.55, total=30/0.55=54.5
	if r.Total < 54.0 || r.Total > 55.0 {
		t.Errorf("expected total ~54.5, got %.1f", r.Total)
	}
}

func TestScorer_EmptyPackage(t *testing.T) {
	analyzers := []analyzer.Analyzer{
		analyzer.NewSRPAnalyzer(),
	}
	s := scorer.New(analyzers, config.DefaultWeights())
	pkg := &model.PackageInfo{Name: "empty"}
	results := s.Score(pkg)
	if len(results) != 0 {
		t.Errorf("expected 0 results for empty package, got %d", len(results))
	}
}

// TestScorer_StableKeyAcrossFileMove verifies that two analyzers reporting the
// same target (same package + name) are merged even when they disagree on the
// source file path — as happens when a type is moved to a different file
// between commits. This is the property that makes scores diff-able.
func TestScorer_StableKeyAcrossFileMove(t *testing.T) {
	analyzers := []analyzer.Analyzer{
		&stubAnalyzer{
			principle: analyzer.SRP,
			results: []analyzer.Result{{
				Principle: analyzer.SRP, TargetPkg: "example.com/pkg", TargetName: "Foo",
				TargetFile: "old.go", Score: 100,
			}},
		},
		&stubAnalyzer{
			principle: analyzer.DIP,
			results: []analyzer.Result{{
				Principle: analyzer.DIP, TargetPkg: "example.com/pkg", TargetName: "Foo",
				TargetFile: "new.go", Score: 0,
			}},
		},
	}

	s := scorer.New(analyzers, config.DefaultWeights())
	results := s.Score(&model.PackageInfo{Name: "pkg", PkgPath: "example.com/pkg"})

	if len(results) != 1 {
		t.Fatalf("expected 1 merged result despite differing files, got %d", len(results))
	}
	r := results[0]
	if r.TargetID() != "example.com/pkg.Foo" {
		t.Errorf("expected TargetID example.com/pkg.Foo, got %s", r.TargetID())
	}
	if _, ok := r.Scores[analyzer.SRP]; !ok {
		t.Error("expected SRP score to be present on merged result")
	}
	if _, ok := r.Scores[analyzer.DIP]; !ok {
		t.Error("expected DIP score to be present on merged result")
	}
}

// TestScorer_DistinctPackagesSameNameNotMerged verifies that two types with the
// same name in different packages are NOT collapsed into one target.
func TestScorer_DistinctPackagesSameNameNotMerged(t *testing.T) {
	mk := func(pkgPath string) analyzer.Analyzer {
		return &stubAnalyzer{
			principle: analyzer.SRP,
			results: []analyzer.Result{{
				Principle: analyzer.SRP, TargetPkg: pkgPath, TargetName: "Handler",
				TargetFile: "h.go", Score: 90,
			}},
		}
	}

	s := scorer.New(nil, config.DefaultWeights())
	s.Analyzers = []analyzer.Analyzer{mk("example.com/a")}
	resA := s.Score(&model.PackageInfo{PkgPath: "example.com/a"})
	s.Analyzers = []analyzer.Analyzer{mk("example.com/b")}
	resB := s.Score(&model.PackageInfo{PkgPath: "example.com/b"})

	if resA[0].TargetID() == resB[0].TargetID() {
		t.Errorf("expected distinct IDs for same-named types in different packages, both were %s", resA[0].TargetID())
	}
}

// TestScoreResult_TargetIDFallback verifies the fallback to bare name when no
// package path is available (e.g. unresolved package).
func TestScoreResult_TargetIDFallback(t *testing.T) {
	r := &scorer.ScoreResult{TargetName: "Foo"}
	if got := r.TargetID(); got != "Foo" {
		t.Errorf("expected fallback TargetID Foo, got %s", got)
	}
}

func TestScorer_MultipleStructs(t *testing.T) {
	analyzers := []analyzer.Analyzer{
		&stubAnalyzer{
			principle: analyzer.SRP,
			results: []analyzer.Result{
				{Principle: analyzer.SRP, TargetName: "A", TargetFile: "a.go", Score: 80},
				{Principle: analyzer.SRP, TargetName: "B", TargetFile: "b.go", Score: 60},
			},
		},
	}

	s := scorer.New(analyzers, config.DefaultWeights())
	pkg := &model.PackageInfo{Name: "test"}
	results := s.Score(pkg)

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
}
