package eval

import (
	"fmt"
	"sort"
	"strings"

	"github.com/harakeishi/go-solid-score/analyzer"
)

// principleOrder fixes the reporting order of principles.
var principleOrder = []analyzer.Principle{
	analyzer.SRP, analyzer.OCP, analyzer.LSP, analyzer.ISP, analyzer.DIP,
}

// Report is the full evaluation result: per-principle metrics with confidence
// intervals, computed over a chosen split.
type Report struct {
	Split        Split
	BootstrapN   int
	PerPrinciple map[analyzer.Principle]PrincipleReport
}

// PrincipleReport bundles a principle's metrics and F1 confidence interval.
type PrincipleReport struct {
	Metrics Metrics
	CI      CI
}

// BuildReport joins labels to scores, builds per-principle confusion matrices
// over the given split, and computes metrics with a bootstrap CI for F1. seed
// makes the bootstrap deterministic.
func BuildReport(
	labels []Label,
	scored map[string]map[analyzer.Principle]float64,
	thresholds map[analyzer.Principle]float64,
	split Split,
	bootstrapN int,
	seed int64,
) Report {
	rep := Report{
		Split:        split,
		BootstrapN:   bootstrapN,
		PerPrinciple: map[analyzer.Principle]PrincipleReport{},
	}
	unitsByPrinciple := classifyUnits(labels, scored, thresholds, split)
	for p, units := range unitsByPrinciple {
		conf := confusionOf(units)
		rep.PerPrinciple[p] = PrincipleReport{
			Metrics: MetricsFromConfusion(conf),
			CI:      Bootstrap(units, bootstrapN, seed),
		}
	}
	return rep
}

// classifyUnits is like ConfusionByPrinciple but keeps the per-type outcomes so
// the bootstrap can resample at the type level.
func classifyUnits(
	labels []Label,
	scored map[string]map[analyzer.Principle]float64,
	thresholds map[analyzer.Principle]float64,
	split Split,
) map[analyzer.Principle][]ConfusionUnit {
	out := map[analyzer.Principle][]ConfusionUnit{}
	for _, l := range labels {
		if l.Split != split {
			continue
		}
		threshold, ok := thresholds[l.Principle]
		if !ok {
			continue
		}
		ps, ok := scored[l.ID]
		if !ok {
			continue
		}
		score, ok := ps[l.Principle]
		if !ok {
			continue
		}
		o := classifyOne(l.Expect, score, threshold)
		if o == Excluded {
			continue
		}
		out[l.Principle] = append(out[l.Principle], ConfusionUnit{Outcome: o})
	}
	return out
}

// FormatText renders a Report as a fixed-width table. NaN metrics (undefined
// for lack of data) render as "n/a".
func (r Report) FormatText() string {
	var b strings.Builder
	fmt.Fprintf(&b, "go-solid-score evaluate (split=%s, bootstrap=%d)\n", r.Split, r.BootstrapN)
	fmt.Fprintln(&b, strings.Repeat("=", 64))
	fmt.Fprintf(&b, "%-5s %8s %8s %8s %16s %s\n", "prin", "prec", "recall", "F1", "F1 95% CI", "n(TP+FN)")
	fmt.Fprintln(&b, strings.Repeat("-", 64))

	principles := orderedPrinciples(r.PerPrinciple)
	for _, p := range principles {
		pr := r.PerPrinciple[p]
		m := pr.Metrics
		fmt.Fprintf(&b, "%-5s %8s %8s %8s %16s %d\n",
			p,
			num(m.Precision), num(m.Recall), num(m.F1),
			ciStr(pr.CI), m.RecallDenominator,
		)
	}
	return b.String()
}

func orderedPrinciples(m map[analyzer.Principle]PrincipleReport) []analyzer.Principle {
	var ps []analyzer.Principle
	for _, p := range principleOrder {
		if _, ok := m[p]; ok {
			ps = append(ps, p)
		}
	}
	// Append any principle not in the canonical order (defensive).
	for p := range m {
		if !containsPrinciple(ps, p) {
			ps = append(ps, p)
		}
	}
	sort.SliceStable(ps, func(i, j int) bool {
		return orderIndex(ps[i]) < orderIndex(ps[j])
	})
	return ps
}

func orderIndex(p analyzer.Principle) int {
	for i, q := range principleOrder {
		if q == p {
			return i
		}
	}
	return len(principleOrder)
}

func containsPrinciple(ps []analyzer.Principle, p analyzer.Principle) bool {
	for _, q := range ps {
		if q == p {
			return true
		}
	}
	return false
}

func num(v float64) string {
	if v != v { // NaN
		return "n/a"
	}
	return fmt.Sprintf("%.3f", v)
}

func ciStr(ci CI) string {
	if ci.F1Low != ci.F1Low || ci.F1High != ci.F1High { // NaN
		return "n/a"
	}
	return fmt.Sprintf("[%.2f, %.2f]", ci.F1Low, ci.F1High)
}
