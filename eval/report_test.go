package eval

import (
	"strings"
	"testing"

	"github.com/harakeishi/go-solid-score/analyzer"
)

func TestBuildReport(t *testing.T) {
	thresholds := map[analyzer.Principle]float64{analyzer.ISP: 50}
	scored := map[string]map[analyzer.Principle]float64{
		"pkg.Fat":  {analyzer.ISP: 25},  // violation caught
		"pkg.Slim": {analyzer.ISP: 100}, // ok, not flagged
	}
	labels := []Label{
		{ID: "pkg.Fat", Principle: analyzer.ISP, Expect: Violation, Split: SplitTest},
		{ID: "pkg.Slim", Principle: analyzer.ISP, Expect: OK, Split: SplitTest},
	}

	rep := BuildReport(labels, scored, thresholds, SplitTest, 100, 7)
	pr, ok := rep.PerPrinciple[analyzer.ISP]
	if !ok {
		t.Fatal("ISP missing from report")
	}
	if pr.Metrics.RecallDenominator != 1 {
		t.Errorf("recall denominator = %d, want 1", pr.Metrics.RecallDenominator)
	}
	if !approx(pr.Metrics.Recall, 1) {
		t.Errorf("recall = %v, want 1 (the one violation was caught)", pr.Metrics.Recall)
	}

	text := rep.FormatText()
	if !strings.Contains(text, "ISP") {
		t.Errorf("rendered report missing ISP row:\n%s", text)
	}
	if !strings.Contains(text, "split=test") {
		t.Errorf("rendered report missing split header:\n%s", text)
	}
}

func TestReport_FormatText_NA(t *testing.T) {
	// A principle with only an OK label has no actual positives -> recall NaN.
	thresholds := map[analyzer.Principle]float64{analyzer.SRP: 60}
	scored := map[string]map[analyzer.Principle]float64{"pkg.Good": {analyzer.SRP: 100}}
	labels := []Label{{ID: "pkg.Good", Principle: analyzer.SRP, Expect: OK, Split: SplitTest}}

	rep := BuildReport(labels, scored, thresholds, SplitTest, 50, 1)
	text := rep.FormatText()
	if !strings.Contains(text, "n/a") {
		t.Errorf("expected n/a for undefined recall, got:\n%s", text)
	}
}
