package eval

import (
	"testing"

	"github.com/harakeishi/go-solid-score/analyzer"
)

func TestClassify(t *testing.T) {
	// Threshold 50: score < 50 means "flagged as a violation" (positive).
	thresholds := map[analyzer.Principle]float64{analyzer.ISP: 50}

	tests := []struct {
		name   string
		expect Expectation
		score  float64
		want   Outcome
	}{
		{"violation caught (TP)", Violation, 25, TP},
		{"violation missed (FN)", Violation, 80, FN},
		{"ok flagged (FP)", OK, 30, FP},
		{"ok not flagged (TN)", OK, 90, TN},
		{"exactly at threshold is not a violation (TN)", OK, 50, TN},
		{"just below threshold flags (FP)", OK, 49.9, FP},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyOne(tt.expect, tt.score, thresholds[analyzer.ISP])
			if got != tt.want {
				t.Errorf("classifyOne(%s, %.1f, 50) = %v, want %v", tt.expect, tt.score, got, tt.want)
			}
		})
	}
}

func TestClassify_NAExcluded(t *testing.T) {
	got := classifyOne(NA, 10, 50)
	if got != Excluded {
		t.Errorf("NA label should be Excluded, got %v", got)
	}
}

func TestConfusionByPrinciple(t *testing.T) {
	thresholds := map[analyzer.Principle]float64{
		analyzer.ISP: 50,
		analyzer.SRP: 60,
	}
	scored := map[string]map[analyzer.Principle]float64{
		"pkg.Fat":    {analyzer.ISP: 25},  // violation, caught -> TP
		"pkg.Slim":   {analyzer.ISP: 100}, // ok, not flagged -> TN
		"pkg.Missed": {analyzer.ISP: 80},  // violation, missed -> FN
		"pkg.God":    {analyzer.SRP: 30},  // violation, caught -> TP
		"pkg.Facade": {analyzer.SRP: 55},  // na -> excluded
	}
	labels := []Label{
		{ID: "pkg.Fat", Principle: analyzer.ISP, Expect: Violation, Split: SplitTest},
		{ID: "pkg.Slim", Principle: analyzer.ISP, Expect: OK, Split: SplitTest},
		{ID: "pkg.Missed", Principle: analyzer.ISP, Expect: Violation, Split: SplitTest},
		{ID: "pkg.God", Principle: analyzer.SRP, Expect: Violation, Split: SplitTest},
		{ID: "pkg.Facade", Principle: analyzer.SRP, Expect: NA, Split: SplitTest},
	}

	conf := ConfusionByPrinciple(labels, scored, thresholds, SplitTest)

	isp := conf[analyzer.ISP]
	if isp.TP != 1 || isp.FN != 1 || isp.TN != 1 || isp.FP != 0 {
		t.Errorf("ISP confusion = %+v, want TP=1 FN=1 TN=1 FP=0", isp)
	}
	srp := conf[analyzer.SRP]
	if srp.TP != 1 || srp.FN != 0 || srp.TN != 0 || srp.FP != 0 {
		t.Errorf("SRP confusion = %+v, want TP=1 (Facade excluded as NA)", srp)
	}
}

func TestConfusionByPrinciple_SplitFilter(t *testing.T) {
	thresholds := map[analyzer.Principle]float64{analyzer.ISP: 50}
	scored := map[string]map[analyzer.Principle]float64{
		"pkg.A": {analyzer.ISP: 25},
		"pkg.B": {analyzer.ISP: 25},
	}
	labels := []Label{
		{ID: "pkg.A", Principle: analyzer.ISP, Expect: Violation, Split: SplitTest},
		{ID: "pkg.B", Principle: analyzer.ISP, Expect: Violation, Split: SplitTrain},
	}
	// Only the test-split label should count.
	conf := ConfusionByPrinciple(labels, scored, thresholds, SplitTest)
	if conf[analyzer.ISP].TP != 1 {
		t.Errorf("with split=test, TP = %d, want 1 (train label excluded)", conf[analyzer.ISP].TP)
	}
}
