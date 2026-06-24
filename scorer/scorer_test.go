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

// TestScoreResult_TargetIDFallback verifies that when no package path is
// available (e.g. an unresolved package), the ID falls back to "<file>:<name>"
// rather than a bare name — so that two same-named targets in different files
// keep distinct IDs and are never silently merged by diff tooling.
func TestScoreResult_TargetIDFallback(t *testing.T) {
	a := &scorer.ScoreResult{TargetName: "Foo", TargetFile: "a.go"}
	b := &scorer.ScoreResult{TargetName: "Foo", TargetFile: "b.go"}

	if got := a.TargetID(); got != "a.go:Foo" {
		t.Errorf("expected fallback TargetID a.go:Foo, got %s", got)
	}
	if a.TargetID() == b.TargetID() {
		t.Errorf("same-named targets in different files must not share an ID, both were %s", a.TargetID())
	}
}

// TestScorer_FallbackKeySeparatesByFile verifies that when the package path is
// unknown, same-named targets in different files are kept as separate results
// (not merged) — matching the TargetID fallback rule, so the merge key and the
// diff key stay consistent.
func TestScorer_FallbackKeySeparatesByFile(t *testing.T) {
	analyzers := []analyzer.Analyzer{
		&stubAnalyzer{
			principle: analyzer.SRP,
			results: []analyzer.Result{
				{Principle: analyzer.SRP, TargetName: "Foo", TargetFile: "a.go", Score: 100},
				{Principle: analyzer.SRP, TargetName: "Foo", TargetFile: "b.go", Score: 0},
			},
		},
	}

	s := scorer.New(analyzers, config.DefaultWeights())
	results := s.Score(&model.PackageInfo{Name: "test"}) // no PkgPath -> fallback

	if len(results) != 2 {
		t.Fatalf("expected 2 separate results in fallback mode, got %d", len(results))
	}
	ids := map[string]bool{}
	for _, r := range results {
		ids[r.TargetID()] = true
	}
	if !ids["a.go:Foo"] || !ids["b.go:Foo"] {
		t.Errorf("expected ids a.go:Foo and b.go:Foo, got %v", ids)
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

// TestScorer_PropagatesIsInterface verifies that the interface flag set by the
// ISP analyzer reaches the merged ScoreResult, and that it stays true even when
// a struct-only analyzer reports the same target first with the flag false
// (the flag is OR-ed in, not overwritten).
func TestScorer_PropagatesIsInterface(t *testing.T) {
	analyzers := []analyzer.Analyzer{
		// A struct-only analyzer reports "Iface" first with the flag false.
		&stubAnalyzer{
			principle: analyzer.LSP,
			results: []analyzer.Result{
				{Principle: analyzer.LSP, TargetName: "Iface", TargetFile: "i.go", Score: 100, TargetIsInterface: false},
				{Principle: analyzer.LSP, TargetName: "Plain", TargetFile: "p.go", Score: 100, TargetIsInterface: false},
			},
		},
		// The ISP analyzer then reports "Iface" as an interface.
		&stubAnalyzer{
			principle: analyzer.ISP,
			results: []analyzer.Result{
				{Principle: analyzer.ISP, TargetName: "Iface", TargetFile: "i.go", Score: 100, TargetIsInterface: true},
				{Principle: analyzer.ISP, TargetName: "Plain", TargetFile: "p.go", Score: 80, TargetIsInterface: false},
			},
		},
	}

	s := scorer.New(analyzers, config.DefaultWeights())
	results := s.Score(&model.PackageInfo{Name: "test"})

	byName := make(map[string]*scorer.ScoreResult)
	for _, r := range results {
		byName[r.TargetName] = r
	}
	if !byName["Iface"].IsInterface {
		t.Error("Iface should be marked IsInterface=true even when reported false first")
	}
	if byName["Plain"].IsInterface {
		t.Error("Plain should be IsInterface=false")
	}
}
