package formatter

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/harakeishi/go-solid-score/analyzer"
	"github.com/harakeishi/go-solid-score/scorer"
)

// TextFormatter outputs human-readable table format.
type TextFormatter struct{}

var principles = []analyzer.Principle{
	analyzer.SRP, analyzer.OCP, analyzer.LSP, analyzer.ISP, analyzer.DIP,
}

func (f *TextFormatter) Format(results []*scorer.ScoreResult) (string, error) {
	if len(results) == 0 {
		return "No structs found to analyze.\n", nil
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Total < results[j].Total
	})

	var b strings.Builder

	b.WriteString("go-solid-score\n")
	b.WriteString(strings.Repeat("=", 100) + "\n")

	// Header
	fmt.Fprintf(&b, "%-40s %6s %6s %6s %6s %6s %7s\n",
		"Struct", "SRP", "OCP", "LSP", "ISP", "DIP", "Total")
	b.WriteString(strings.Repeat("-", 100) + "\n")

	var totalSum float64
	for _, r := range results {
		name := r.TargetName
		if len(name) > 40 {
			name = name[:37] + "..."
		}
		fmt.Fprintf(&b, "%-40s", name)
		for _, p := range principles {
			fmt.Fprintf(&b, " %6.1f", r.Scores[p])
		}
		fmt.Fprintf(&b, " %7.1f\n", r.Total)
		totalSum += r.Total
	}

	b.WriteString(strings.Repeat("-", 100) + "\n")
	avg := math.Round(totalSum/float64(len(results))*10) / 10
	fmt.Fprintf(&b, "%-40s %6s %6s %6s %6s %6s %7.1f\n",
		"Average", "", "", "", "", "", avg)
	b.WriteString(strings.Repeat("=", 100) + "\n")

	return b.String(), nil
}
