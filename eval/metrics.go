package eval

import (
	"math"
	"math/rand"
	"sort"
)

// Metrics is the precision/recall/F1 triple for one principle, plus the recall
// denominator (the count of known true violations) so reports can state how
// many real violations the measurement is based on.
type Metrics struct {
	Precision         float64
	Recall            float64
	F1                float64
	RecallDenominator int // TP + FN
	Confusion         Confusion
}

// MetricsFromConfusion computes precision/recall/F1 from a confusion matrix.
// Precision and recall are NaN (undefined) rather than zero when their
// denominator is zero, so a report can distinguish "scored 0" from "no data".
func MetricsFromConfusion(c Confusion) Metrics {
	m := Metrics{RecallDenominator: c.TP + c.FN, Confusion: c}
	predictedPos := c.TP + c.FP
	actualPos := c.TP + c.FN
	if predictedPos == 0 {
		m.Precision = math.NaN()
	} else {
		m.Precision = float64(c.TP) / float64(predictedPos)
	}
	if actualPos == 0 {
		m.Recall = math.NaN()
	} else {
		m.Recall = float64(c.TP) / float64(actualPos)
	}
	if math.IsNaN(m.Precision) || math.IsNaN(m.Recall) || m.Precision+m.Recall == 0 {
		m.F1 = math.NaN()
	} else {
		m.F1 = 2 * m.Precision * m.Recall / (m.Precision + m.Recall)
	}
	return m
}

// ConfusionUnit is one classified type — the resampling unit for the bootstrap.
// Resampling at the type level (not the cell level) preserves the matrix's
// dependence structure.
type ConfusionUnit struct {
	Outcome Outcome
}

// CI holds the bootstrap point estimate and 95% percentile interval for F1.
type CI struct {
	F1Point float64
	F1Low   float64
	F1High  float64
}

// Bootstrap estimates a 95% percentile confidence interval for F1 by resampling
// the classified units with replacement. The seed makes it deterministic so the
// result is reproducible across runs and testable. iters is the number of
// resamples (e.g. 1000). NaN F1 samples (degenerate resamples with no positives)
// are skipped.
func Bootstrap(units []ConfusionUnit, iters int, seed int64) CI {
	point := MetricsFromConfusion(confusionOf(units)).F1

	rng := rand.New(rand.NewSource(seed))
	n := len(units)
	samples := make([]float64, 0, iters)
	for i := 0; i < iters; i++ {
		resampled := make([]ConfusionUnit, n)
		for j := 0; j < n; j++ {
			resampled[j] = units[rng.Intn(n)]
		}
		f1 := MetricsFromConfusion(confusionOf(resampled)).F1
		if !math.IsNaN(f1) {
			samples = append(samples, f1)
		}
	}

	ci := CI{F1Point: point}
	if len(samples) == 0 {
		ci.F1Low = math.NaN()
		ci.F1High = math.NaN()
		return ci
	}
	sort.Float64s(samples)
	ci.F1Low = percentile(samples, 2.5)
	ci.F1High = percentile(samples, 97.5)
	return ci
}

// confusionOf tallies a slice of classified units into a confusion matrix.
func confusionOf(units []ConfusionUnit) Confusion {
	var c Confusion
	for _, u := range units {
		switch u.Outcome {
		case TP:
			c.TP++
		case FP:
			c.FP++
		case FN:
			c.FN++
		case TN:
			c.TN++
		}
	}
	return c
}

// percentile returns the p-th percentile (0-100) of a pre-sorted slice using
// nearest-rank interpolation.
func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return math.NaN()
	}
	if len(sorted) == 1 {
		return sorted[0]
	}
	rank := p / 100 * float64(len(sorted)-1)
	lo := int(math.Floor(rank))
	hi := int(math.Ceil(rank))
	if lo == hi {
		return sorted[lo]
	}
	frac := rank - float64(lo)
	return sorted[lo]*(1-frac) + sorted[hi]*frac
}
