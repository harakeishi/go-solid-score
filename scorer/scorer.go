// Package scorer orchestrates SOLID principle analyzers and computes
// weighted aggregate scores for each target in a Go package.
package scorer

import (
	"math"

	"github.com/harakeishi/go-solid-score/analyzer"
	"github.com/harakeishi/go-solid-score/model"
)

// ScoreResult holds SOLID scores for a single target (struct or function).
type ScoreResult struct {
	// TargetPkg is the import path of the package the target belongs to.
	// Together with TargetName it forms the stable identity used to match
	// the same target across runs (e.g. for diffing two commits).
	TargetPkg  string
	TargetName string
	TargetFile string
	TargetLine int
	Scores     map[analyzer.Principle]float64
	Total      float64
	Confidence map[analyzer.Principle]float64
	Details    map[analyzer.Principle][]string
}

// TargetID returns the stable identifier for this target: the package import
// path joined with the target name (e.g. "github.com/foo/bar.MyStruct").
// It is independent of the absolute file path, so it survives file renames
// and moves within the same package — making it suitable as a diff key.
func (r *ScoreResult) TargetID() string {
	if r.TargetPkg == "" {
		return r.TargetName
	}
	return r.TargetPkg + "." + r.TargetName
}

// Scorer orchestrates all SOLID analyzers and computes weighted totals.
type Scorer struct {
	Analyzers []analyzer.Analyzer
	Weights   map[string]float64
}

// New creates a Scorer with all provided analyzers and weights.
func New(analyzers []analyzer.Analyzer, weights map[string]float64) *Scorer {
	return &Scorer{
		Analyzers: analyzers,
		Weights:   weights,
	}
}

// Score analyzes a package and returns per-target score results.
func (s *Scorer) Score(pkg *model.PackageInfo) []*ScoreResult {
	// Collect all results from all analyzers
	resultsByTarget := make(map[string]*ScoreResult)

	for _, a := range s.Analyzers {
		results := a.Analyze(pkg)
		for _, r := range results {
			// Identify a target by its package path + name so that the same
			// target maps to the same key even if its source file is renamed
			// or moved. Fall back to file path when no package path is known.
			key := r.TargetPkg + "." + r.TargetName
			if r.TargetPkg == "" {
				key = r.TargetFile + ":" + r.TargetName
			}
			sr, ok := resultsByTarget[key]
			if !ok {
				sr = &ScoreResult{
					TargetPkg:  r.TargetPkg,
					TargetName: r.TargetName,
					TargetFile: r.TargetFile,
					TargetLine: r.TargetLine,
					Scores:     make(map[analyzer.Principle]float64),
					Confidence: make(map[analyzer.Principle]float64),
					Details:    make(map[analyzer.Principle][]string),
				}
				resultsByTarget[key] = sr
			}
			sr.Scores[r.Principle] = r.Score
			sr.Confidence[r.Principle] = r.Confidence
			sr.Details[r.Principle] = r.Details
		}
	}

	// Compute weighted totals
	results := make([]*ScoreResult, 0, len(resultsByTarget))
	for _, sr := range resultsByTarget {
		sr.Total = s.computeTotal(sr.Scores)
		results = append(results, sr)
	}

	return results
}

func (s *Scorer) computeTotal(scores map[analyzer.Principle]float64) float64 {
	var total, weightSum float64
	for principle, score := range scores {
		w := s.Weights[string(principle)]
		if w == 0 {
			continue
		}
		total += score * w
		weightSum += w
	}
	if weightSum == 0 {
		return 0
	}
	return math.Round(total/weightSum*10) / 10
}
