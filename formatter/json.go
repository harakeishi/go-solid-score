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
type JSONResult struct {
	// ID is the stable identifier (package path + name) for diffing scores
	// across runs; it is unaffected by file renames or moves.
	ID         string             `json:"id"`
	Name       string             `json:"name"`
	Package    string             `json:"package"`
	File       string             `json:"file"`
	Line       int                `json:"line"`
	SRP        float64            `json:"srp"`
	OCP        float64            `json:"ocp"`
	LSP        float64            `json:"lsp"`
	ISP        float64            `json:"isp"`
	DIP        float64            `json:"dip"`
	Total      float64            `json:"total"`
	Confidence map[string]float64 `json:"confidence"`
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
		jr := JSONResult{
			ID:         r.TargetID(),
			Name:       r.TargetName,
			Package:    r.TargetPkg,
			File:       r.TargetFile,
			Line:       r.TargetLine,
			SRP:        r.Scores[analyzer.SRP],
			OCP:        r.Scores[analyzer.OCP],
			LSP:        r.Scores[analyzer.LSP],
			ISP:        r.Scores[analyzer.ISP],
			DIP:        r.Scores[analyzer.DIP],
			Total:      r.Total,
			Confidence: conf,
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
