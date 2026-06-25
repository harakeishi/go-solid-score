package stability

import (
	"fmt"
	"math"
	"sort"

	"github.com/harakeishi/go-solid-score/analyzer"
	"github.com/harakeishi/go-solid-score/scorer"
)

// Divergence is one way a target's score moved between two scorings that should
// have been identical. Principle is "total" for the aggregate, or a principle
// name (e.g. "SRP") for a per-principle score.
type Divergence struct {
	TargetID  string
	Principle string
	Base      float64
	Head      float64
}

func (d Divergence) String() string {
	return fmt.Sprintf("%s %s: %.1f -> %.1f", d.TargetID, d.Principle, d.Base, d.Head)
}

// Compare reports every score that moved between base and head at display
// resolution (0.1, the precision the tool reports and the differ compares at).
// A target present in one side but not the other is itself a divergence: a
// semantics-preserving transform must not make a type appear or disappear.
//
// Comparison is at 0.1 because that is the instrument's stated resolution —
// sub-0.1 float wobble from, e.g., map-iteration-order-dependent summation is
// absorbed by the same rounding the scorer applies to its reported numbers, so
// it is by design not a signal. Anything that survives rounding is a real
// precision defect.
func Compare(base, head []*scorer.ScoreResult) []Divergence {
	baseByID := indexByID(base)
	headByID := indexByID(head)

	var divs []Divergence
	for id, b := range baseByID {
		h, ok := headByID[id]
		if !ok {
			divs = append(divs, Divergence{TargetID: id, Principle: "presence", Base: 1, Head: 0})
			continue
		}
		if !eq(b.Total, h.Total) {
			divs = append(divs, Divergence{TargetID: id, Principle: "total", Base: b.Total, Head: h.Total})
		}
		for _, p := range allPrinciples(b.Scores, h.Scores) {
			if !eq(b.Scores[p], h.Scores[p]) {
				divs = append(divs, Divergence{
					TargetID:  id,
					Principle: string(p),
					Base:      b.Scores[p],
					Head:      h.Scores[p],
				})
			}
		}
	}
	for id := range headByID {
		if _, ok := baseByID[id]; !ok {
			divs = append(divs, Divergence{TargetID: id, Principle: "presence", Base: 0, Head: 1})
		}
	}

	sort.Slice(divs, func(i, j int) bool {
		if divs[i].TargetID != divs[j].TargetID {
			return divs[i].TargetID < divs[j].TargetID
		}
		return divs[i].Principle < divs[j].Principle
	})
	return divs
}

func indexByID(results []*scorer.ScoreResult) map[string]*scorer.ScoreResult {
	m := make(map[string]*scorer.ScoreResult, len(results))
	for _, r := range results {
		m[r.TargetID()] = r
	}
	return m
}

func allPrinciples(a, b map[analyzer.Principle]float64) []analyzer.Principle {
	seen := make(map[analyzer.Principle]bool)
	for p := range a {
		seen[p] = true
	}
	for p := range b {
		seen[p] = true
	}
	ps := make([]analyzer.Principle, 0, len(seen))
	for p := range seen {
		ps = append(ps, p)
	}
	sort.Slice(ps, func(i, j int) bool { return ps[i] < ps[j] })
	return ps
}

// eq compares two scores at the tool's reported 0.1 resolution.
func eq(a, b float64) bool {
	return math.Round(a*10) == math.Round(b*10)
}
