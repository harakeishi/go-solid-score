package eval_test

import (
	"testing"

	"github.com/harakeishi/go-solid-score/eval"
)

// p builds a one-principle ReportJSON for tests.
func report(principle string, tp, fp, fn, tn int) eval.ReportJSON {
	return eval.ReportJSON{
		Split:      "test",
		BootstrapN: 0,
		PerPrinciple: map[string]eval.PrincipleJSON{
			principle: {RecallDenominator: tp + fn, TP: tp, FP: fp, FN: fn, TN: tn},
		},
	}
}

// TestCompareToBaseline_NoRegression: an identical report regresses on nothing.
func TestCompareToBaseline_NoRegression(t *testing.T) {
	base := report("ISP", 1, 0, 0, 1)
	if regs := eval.CompareToBaseline(base, base); len(regs) != 0 {
		t.Errorf("identical report should not regress, got %+v", regs)
	}
}

// TestCompareToBaseline_RecallDrop: a caught violation now missed (TP down) is a
// recall regression — the floor the gate defends.
func TestCompareToBaseline_RecallDrop(t *testing.T) {
	base := report("ISP", 2, 0, 0, 1)
	cur := report("ISP", 1, 0, 1, 1) // one TP became an FN
	regs := eval.CompareToBaseline(cur, base)
	if len(regs) != 1 || regs[0].Kind != "recall" || regs[0].Principle != "ISP" {
		t.Fatalf("expected one ISP recall regression, got %+v", regs)
	}
}

// TestCompareToBaseline_NewFalsePositive: a sound type newly flagged (FP up) is
// a precision regression.
func TestCompareToBaseline_NewFalsePositive(t *testing.T) {
	base := report("SRP", 1, 0, 0, 3)
	cur := report("SRP", 1, 1, 0, 2) // one TN became an FP
	regs := eval.CompareToBaseline(cur, base)
	if len(regs) != 1 || regs[0].Kind != "precision" {
		t.Fatalf("expected one SRP precision regression, got %+v", regs)
	}
}

// TestCompareToBaseline_ImprovementIsNotRegression: catching a previously-missed
// violation (TP up, FN down) must not be reported as a regression.
func TestCompareToBaseline_ImprovementIsNotRegression(t *testing.T) {
	base := report("OCP", 0, 0, 1, 2) // the known FN
	cur := report("OCP", 1, 0, 0, 2)  // now caught
	if regs := eval.CompareToBaseline(cur, base); len(regs) != 0 {
		t.Errorf("an improvement must not regress, got %+v", regs)
	}
}

// TestCompareToBaseline_PrincipleDisappeared: a principle present in the
// baseline but missing from the current report is a regression — its labels or
// scores vanished, which would otherwise hide a drop.
func TestCompareToBaseline_PrincipleDisappeared(t *testing.T) {
	base := report("LSP", 2, 0, 0, 1)
	cur := eval.ReportJSON{PerPrinciple: map[string]eval.PrincipleJSON{}}
	regs := eval.CompareToBaseline(cur, base)
	if len(regs) != 1 || regs[0].Principle != "LSP" {
		t.Fatalf("expected an LSP disappearance regression, got %+v", regs)
	}
}

// TestLoadBaselineJSON_RoundTrip: a report encoded by NewReportJSON decodes back
// into an equivalent baseline.
func TestLoadBaselineJSON_BadInput(t *testing.T) {
	if _, err := eval.LoadBaselineJSON([]byte("{not json")); err == nil {
		t.Error("expected error for invalid baseline JSON")
	}
}
