// Package scorer orchestrates SOLID principle analyzers and computes
// weighted aggregate scores for each target in a Go package.
package scorer

import (
	"math"

	"github.com/harakeishi/go-solid-score/analyzer"
	"github.com/harakeishi/go-solid-score/model"
)

// ScoreResult holds SOLID scores for a single analyzed struct.
type ScoreResult struct {
	// TargetPkg is the import path of the package the target belongs to.
	// Together with TargetName it forms the stable identity used to match
	// the same target across runs (e.g. for diffing two commits).
	TargetPkg  string
	TargetName string
	TargetFile string
	TargetLine int
	// IsInterface is true when the target is an interface definition rather
	// than a struct. Interfaces are scored on ISP alone, so their Total is not
	// comparable to a struct's five-principle Total; formatters use this to
	// present the two kinds in separate sections.
	IsInterface bool
	Scores      map[analyzer.Principle]float64
	Total       float64
	Confidence  map[analyzer.Principle]float64
	Details     map[analyzer.Principle][]string
}

// targetID computes the canonical identity for a target. It is the single
// source of truth for how targets are identified, used both as the internal
// merge key during scoring and as the public diff key exposed via
// ScoreResult.TargetID — keeping the two in lockstep.
//
// When the package path is known, the ID is "<pkgPath>.<name>", which is
// independent of the absolute file path and therefore survives file renames
// and moves within the same package. When the package path is unknown (e.g.
// an unresolved package), it falls back to "<file>:<name>" so that targets in
// different files do not silently collapse into one ID.
func targetID(pkgPath, name, file string) string {
	if pkgPath == "" {
		return file + ":" + name
	}
	return pkgPath + "." + name
}

// TargetID returns the stable identifier for this target (see [targetID]).
// It is suitable as a join key when diffing scores across runs.
func (r *ScoreResult) TargetID() string {
	return targetID(r.TargetPkg, r.TargetName, r.TargetFile)
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
			// Use the canonical target identity as the merge key so that the
			// same target maps to one ScoreResult across analyzers, and so the
			// internal key matches the public diff key exactly.
			key := targetID(r.TargetPkg, r.TargetName, r.TargetFile)
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
			// Only the ISP analyzer ever sets TargetIsInterface, and a struct
			// and an interface cannot share a name within one package, so this
			// flag is consistent across analyzers regardless of their order.
			// OR-ing it in (rather than overwriting) keeps it stable even if a
			// struct-only analyzer reports the same key first with false.
			if r.TargetIsInterface {
				sr.IsInterface = true
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
