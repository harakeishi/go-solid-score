package scorer

import (
	"math"

	"github.com/harakeishi/go-solid-score/analyzer"
	"github.com/harakeishi/go-solid-score/model"
)

// ScoreResult holds SOLID scores for a single target (struct or function).
type ScoreResult struct {
	TargetName string
	TargetFile string
	TargetLine int
	Scores     map[analyzer.Principle]float64
	Total      float64
	Confidence map[analyzer.Principle]float64
	Details    map[analyzer.Principle][]string
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
			key := r.TargetFile + ":" + r.TargetName
			sr, ok := resultsByTarget[key]
			if !ok {
				sr = &ScoreResult{
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
