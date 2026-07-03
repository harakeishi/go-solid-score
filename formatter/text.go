package formatter

import (
	"fmt"
	"sort"
	"strings"

	"github.com/harakeishi/go-solid-score/analyzer"
	"github.com/harakeishi/go-solid-score/scorer"
)

// TextFormatter outputs human-readable table format.
type TextFormatter struct {
	// Verbose adds a per-principle detail breakdown (the reasons behind each
	// score) under every target row.
	Verbose bool
}

var principles = []analyzer.Principle{
	analyzer.SRP, analyzer.OCP, analyzer.LSP, analyzer.ISP, analyzer.DIP,
}

func (f *TextFormatter) Format(results []*scorer.ScoreResult) (string, error) {
	if len(results) == 0 {
		return "No targets found to analyze.\n", nil
	}

	// Structs are scored across all five principles; interfaces are scored on
	// ISP alone, so their Total is not comparable to a struct's. Present them
	// in separate sections so each Total is only ranked against like-for-like.
	var structs, interfaces []*scorer.ScoreResult
	for _, r := range results {
		if r.IsInterface {
			interfaces = append(interfaces, r)
		} else {
			structs = append(structs, r)
		}
	}

	var b strings.Builder
	b.WriteString("go-solid-score\n")

	if len(structs) > 0 {
		writeStructSection(&b, structs, f.Verbose)
	}
	if len(interfaces) > 0 {
		if len(structs) > 0 {
			b.WriteString("\n")
		}
		writeInterfaceSection(&b, interfaces, f.Verbose)
	}

	return b.String(), nil
}

// writeStructSection renders the full five-principle table for struct targets.
func writeStructSection(b *strings.Builder, results []*scorer.ScoreResult, verbose bool) {
	sort.Slice(results, func(i, j int) bool {
		return results[i].Total < results[j].Total
	})

	b.WriteString(strings.Repeat("=", 100) + "\n")
	fmt.Fprintf(b, "%-40s %6s %6s %6s %6s %6s %7s\n",
		"Struct", "SRP", "OCP", "LSP", "ISP", "DIP", "Total")
	b.WriteString(strings.Repeat("-", 100) + "\n")

	var totalSum float64
	for _, r := range results {
		fmt.Fprintf(b, "%-40s", truncateName(r.TargetName))
		for _, p := range principles {
			// A principle with no entry in Scores was not evaluated for this
			// target. Render it as "-" (not applicable) rather than 0.0, which
			// would be indistinguishable from a genuine zero score.
			if v, ok := r.Scores[p]; ok {
				fmt.Fprintf(b, " %6.1f", v)
			} else {
				fmt.Fprintf(b, " %6s", "-")
			}
		}
		fmt.Fprintf(b, " %7.1f\n", r.Total)
		if verbose {
			writeDetails(b, r)
		}
		totalSum += r.Total
	}

	b.WriteString(strings.Repeat("-", 100) + "\n")
	avg := roundAvg(totalSum, len(results))
	fmt.Fprintf(b, "%-40s %6s %6s %6s %6s %6s %7.1f\n",
		"Average", "", "", "", "", "", avg)
	b.WriteString(strings.Repeat("=", 100) + "\n")
}

// writeInterfaceSection renders a slim ISP-only table for interface targets.
// Interfaces are scored on ISP alone (Total == ISP), so a five-principle table
// would be four columns of "-"; showing only ISP and Total makes the
// single-principle nature explicit.
func writeInterfaceSection(b *strings.Builder, results []*scorer.ScoreResult, verbose bool) {
	sort.Slice(results, func(i, j int) bool {
		return results[i].Total < results[j].Total
	})

	b.WriteString(strings.Repeat("=", 100) + "\n")
	fmt.Fprintf(b, "%-40s %6s %7s\n", "Interface", "ISP", "Total")
	b.WriteString(strings.Repeat("-", 100) + "\n")

	var totalSum float64
	for _, r := range results {
		isp := r.Scores[analyzer.ISP]
		fmt.Fprintf(b, "%-40s %6.1f %7.1f\n", truncateName(r.TargetName), isp, r.Total)
		if verbose {
			writeDetails(b, r)
		}
		totalSum += r.Total
	}

	b.WriteString(strings.Repeat("-", 100) + "\n")
	avg := roundAvg(totalSum, len(results))
	fmt.Fprintf(b, "%-40s %6s %7.1f\n", "Average", "", avg)
	b.WriteString(strings.Repeat("=", 100) + "\n")
}

// writeDetails renders the per-principle detail lines (the reasons behind each
// score) indented under a target row, in the fixed principle order. Principles
// with no detail lines are skipped so signal-free targets stay compact.
func writeDetails(b *strings.Builder, r *scorer.ScoreResult) {
	for _, p := range principles {
		for _, d := range r.Details[p] {
			fmt.Fprintf(b, "    %s: %s\n", p, d)
		}
	}
}

// truncateName shortens a target name to fit the fixed-width name column.
func truncateName(name string) string {
	if len(name) > 40 {
		return name[:37] + "..."
	}
	return name
}
