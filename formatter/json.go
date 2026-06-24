package formatter

import (
	"encoding/json"
	"math"

	"github.com/harakeishi/go-solid-score/analyzer"
	"github.com/harakeishi/go-solid-score/scorer"
)

// JSONFormatter outputs machine-readable JSON.
type JSONFormatter struct{}

// JSONOutput is the top-level JSON document emitted by JSONFormatter and is
// also the shape consumed when decoding a baseline for diffing.
type JSONOutput struct {
	Results []JSONResult `json:"results"`
	Summary JSONSummary  `json:"summary"`
}

// JSONResult is one scored target in JSON form. The stable id/package fields
// make it suitable as a diff baseline.
//
// The per-principle scores are pointers so that a baseline produced before
// per-principle output existed (where these keys are simply absent) decodes to
// nil rather than a misleading 0.0 — letting consumers distinguish "score was
// zero" from "no per-principle data". JSONFormatter always populates them, so
// freshly emitted JSON still carries every principle.
type JSONResult struct {
	// ID is the stable identifier (package path + name) for diffing scores
	// across runs; it is unaffected by file renames or moves.
	ID      string `json:"id"`
	Name    string `json:"name"`
	Package string `json:"package"`
	File    string `json:"file"`
	Line    int    `json:"line"`
	// IsInterface is true when the target is an interface definition. Such
	// targets are scored on ISP alone (other principle fields are null), so
	// consumers can filter on this to compare like-for-like.
	IsInterface bool               `json:"is_interface"`
	SRP         *float64           `json:"srp"`
	OCP         *float64           `json:"ocp"`
	LSP         *float64           `json:"lsp"`
	ISP         *float64           `json:"isp"`
	DIP         *float64           `json:"dip"`
	Total       float64            `json:"total"`
	Confidence  map[string]float64 `json:"confidence"`
}

// Principles projects the per-principle scores into a map keyed by principle
// name, including only the principles that are present (non-nil). Returns nil
// when none are present, so callers can detect a baseline that lacks
// per-principle data.
func (r JSONResult) Principles() map[string]float64 {
	m := make(map[string]float64, 5)
	for name, v := range map[string]*float64{
		string(analyzer.SRP): r.SRP,
		string(analyzer.OCP): r.OCP,
		string(analyzer.LSP): r.LSP,
		string(analyzer.ISP): r.ISP,
		string(analyzer.DIP): r.DIP,
	} {
		if v != nil {
			m[name] = *v
		}
	}
	if len(m) == 0 {
		return nil
	}
	return m
}

// JSONSummary is the aggregate block of the JSON document.
type JSONSummary struct {
	TotalStructs int     `json:"total_structs"`
	AverageScore float64 `json:"average_score"`
}

func (f *JSONFormatter) Format(results []*scorer.ScoreResult) (string, error) {
	out := JSONOutput{
		Results: make([]JSONResult, 0, len(results)),
	}

	var totalSum float64
	for _, r := range results {
		conf := make(map[string]float64)
		for p, c := range r.Confidence {
			conf[string(p)] = c
		}
		// Return a pointer only for principles that were actually evaluated for
		// this target. A missing principle (e.g. SRP/OCP/LSP/DIP on an interface
		// definition, which only ISP scores) yields nil → JSON null, which is
		// distinguishable from a real zero score. The *float64 field type and
		// Principles() (nil-aware) are designed for exactly this.
		score := func(p analyzer.Principle) *float64 {
			v, ok := r.Scores[p]
			if !ok {
				return nil
			}
			return &v
		}
		jr := JSONResult{
			ID:          r.TargetID(),
			Name:        r.TargetName,
			Package:     r.TargetPkg,
			File:        r.TargetFile,
			Line:        r.TargetLine,
			IsInterface: r.IsInterface,
			SRP:         score(analyzer.SRP),
			OCP:         score(analyzer.OCP),
			LSP:         score(analyzer.LSP),
			ISP:         score(analyzer.ISP),
			DIP:         score(analyzer.DIP),
			Total:       r.Total,
			Confidence:  conf,
		}
		out.Results = append(out.Results, jr)
		totalSum += r.Total
	}

	avg := 0.0
	if len(results) > 0 {
		avg = math.Round(totalSum/float64(len(results))*10) / 10
	}
	out.Summary = JSONSummary{
		TotalStructs: len(results),
		AverageScore: avg,
	}

	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data) + "\n", nil
}
