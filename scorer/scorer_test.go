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
