package formatter

import (
	"encoding/json"

	"github.com/harakeishi/go-solid-score/analyzer"
	"github.com/harakeishi/go-solid-score/scorer"
)

// JSONFormatter outputs machine-readable JSON.
type JSONFormatter struct {
	// Verbose includes the per-principle detail lines (the reasons behind each
	// score) as a "details" field on every result. Off by default so baselines
	// stay minimal; the field is omitted (not null) when absent, which keeps
	// verbose and non-verbose documents mutually decodable.
	Verbose bool
}

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
	// Details carries the per-principle reasons behind each score, keyed by
	// principle name. Populated only in verbose mode and omitted otherwise, so
	// pre-existing baselines (and non-verbose output) are unaffected.
	Details map[string][]string `json:"details,omitempty"`
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

// JSONSummary is the aggregate block of the JSON document. Structs and
// interfaces are summarized separately: an interface is scored on ISP alone, so
// its Total is not comparable to a struct's five-principle Total, and blending
// the two (or counting interfaces under total_structs) is meaningless. This
// mirrors the two-section split in the text formatter.
type JSONSummary struct {
	TotalStructs          int     `json:"total_structs"`
	AverageScore          float64 `json:"average_score"`
	TotalInterfaces       int     `json:"total_interfaces"`
	InterfaceAverageScore float64 `json:"interface_average_score"`
}

func (f *JSONFormatter) Format(results []*scorer.ScoreResult) (string, error) {
	out := JSONOutput{
		Results: make([]JSONResult, 0, len(results)),
	}

	var structSum, ifaceSum float64
	var structCount, ifaceCount int
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
		if f.Verbose {
			details := make(map[string][]string)
			for p, d := range r.Details {
				if len(d) > 0 {
					details[string(p)] = d
				}
			}
			if len(details) > 0 {
				jr.Details = details
			}
		}
		out.Results = append(out.Results, jr)
		if r.IsInterface {
			ifaceSum += r.Total
			ifaceCount++
		} else {
			structSum += r.Total
			structCount++
		}
	}

	out.Summary = JSONSummary{
		TotalStructs:          structCount,
		AverageScore:          roundAvg(structSum, structCount),
		TotalInterfaces:       ifaceCount,
		InterfaceAverageScore: roundAvg(ifaceSum, ifaceCount),
	}

	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data) + "\n", nil
}
