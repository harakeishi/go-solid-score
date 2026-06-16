package eval

import "github.com/harakeishi/go-solid-score/analyzer"

// Outcome is the confusion-matrix cell a single labelled type falls into for
// one principle.
type Outcome int

const (
	// Excluded means the label does not contribute to the matrix (NA, or a
	// split that was filtered out, or a missing score).
	Excluded Outcome = iota
	TP               // violation, flagged
	FP               // ok, flagged
	FN               // violation, not flagged
	TN               // ok, not flagged
)

// classifyOne places a single (expectation, score) pair into a confusion cell
// given the principle's pass/fail threshold. A score strictly below the
// threshold counts as "flagged as a violation" (positive); a score at or above
// it is not flagged. NA is excluded.
func classifyOne(expect Expectation, score, threshold float64) Outcome {
	if expect == NA {
		return Excluded
	}
	flagged := score < threshold
	switch {
	case expect == Violation && flagged:
		return TP
	case expect == Violation && !flagged:
		return FN
	case expect == OK && flagged:
		return FP
	default: // OK && !flagged
		return TN
	}
}

// Confusion is a per-principle confusion matrix.
type Confusion struct {
	TP, FP, FN, TN int
}

// ConfusionByPrinciple joins labels to scored results by ID and tallies a
// confusion matrix per principle. Only labels in the requested split are
// counted; labels whose ID or principle is absent from scored, or whose
// principle has no threshold, are skipped (they cannot be classified).
func ConfusionByPrinciple(
	labels []Label,
	scored map[string]map[analyzer.Principle]float64,
	thresholds map[analyzer.Principle]float64,
	split Split,
) map[analyzer.Principle]Confusion {
	out := map[analyzer.Principle]Confusion{}
	for _, l := range labels {
		if l.Split != split {
			continue
		}
		threshold, ok := thresholds[l.Principle]
		if !ok {
			continue
		}
		principleScores, ok := scored[l.ID]
		if !ok {
			continue
		}
		score, ok := principleScores[l.Principle]
		if !ok {
			continue
		}
		c := out[l.Principle]
		switch classifyOne(l.Expect, score, threshold) {
		case TP:
			c.TP++
		case FP:
			c.FP++
		case FN:
			c.FN++
		case TN:
			c.TN++
		case Excluded:
			continue
		}
		out[l.Principle] = c
	}
	return out
}
