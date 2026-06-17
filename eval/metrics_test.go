package eval

import (
	"math"
	"testing"

	"github.com/harakeishi/go-solid-score/analyzer"
)

func approx(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func TestMetrics_FromConfusion(t *testing.T) {
	// TP=8, FP=2, FN=4 -> P=0.8, R=8/12, F1=2PR/(P+R)
	m := MetricsFromConfusion(Confusion{TP: 8, FP: 2, FN: 4, TN: 6})
	if !approx(m.Precision, 0.8) {
		t.Errorf("Precision = %v, want 0.8", m.Precision)
	}
	if !approx(m.Recall, 8.0/12.0) {
		t.Errorf("Recall = %v, want %v", m.Recall, 8.0/12.0)
	}
	wantF1 := 2 * 0.8 * (8.0 / 12.0) / (0.8 + 8.0/12.0)
	if !approx(m.F1, wantF1) {
		t.Errorf("F1 = %v, want %v", m.F1, wantF1)
	}
	if m.RecallDenominator != 12 {
		t.Errorf("RecallDenominator = %d, want 12 (TP+FN)", m.RecallDenominator)
	}
}

func TestMetrics_ZeroDenominators(t *testing.T) {
	// No positives predicted and none actual: precision/recall undefined (NaN),
	// not a divide-by-zero panic.
	m := MetricsFromConfusion(Confusion{TN: 5})
	if !math.IsNaN(m.Precision) {
		t.Errorf("Precision with no predicted positives should be NaN, got %v", m.Precision)
	}
	if !math.IsNaN(m.Recall) {
		t.Errorf("Recall with no actual positives should be NaN, got %v", m.Recall)
	}
}

func TestMetrics_Perfect(t *testing.T) {
	m := MetricsFromConfusion(Confusion{TP: 5, TN: 5})
	if !approx(m.Precision, 1) || !approx(m.Recall, 1) || !approx(m.F1, 1) {
		t.Errorf("perfect classifier should give P=R=F1=1, got %+v", m)
	}
}

func TestBootstrap_Deterministic(t *testing.T) {
	units := []ConfusionUnit{
		{Outcome: TP}, {Outcome: TP}, {Outcome: FP},
		{Outcome: FN}, {Outcome: TN}, {Outcome: TP},
	}
	a := Bootstrap(units, 200, 42)
	b := Bootstrap(units, 200, 42)
	if a.F1Low != b.F1Low || a.F1High != b.F1High {
		t.Errorf("bootstrap with same seed must be deterministic: %+v vs %+v", a, b)
	}
	// CI must bracket the point estimate.
	if a.F1Low > a.F1Point || a.F1High < a.F1Point {
		t.Errorf("CI [%v,%v] does not bracket point %v", a.F1Low, a.F1High, a.F1Point)
	}
}

func TestBootstrap_PerfectHasTightCI(t *testing.T) {
	units := []ConfusionUnit{{Outcome: TP}, {Outcome: TP}, {Outcome: TN}, {Outcome: TN}}
	ci := Bootstrap(units, 200, 1)
	if !approx(ci.F1Low, 1) || !approx(ci.F1High, 1) {
		t.Errorf("a perfectly-separated set should have CI [1,1], got [%v,%v]", ci.F1Low, ci.F1High)
	}
}

// guard: the principle constant is used so the import is meaningful in future
// expansion; keep a trivial reference.
var _ = analyzer.ISP
