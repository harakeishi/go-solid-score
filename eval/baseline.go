package eval

import (
	"encoding/json"
	"fmt"
	"sort"
)

// Regression is one per-principle way a report fell below its committed
// baseline. The harness treats two things as regressions, mirroring the
// case-level golden-regression approach of static-analysis rule tests (PMD,
// Semgrep, go/analysis): a known true violation that started slipping through
// (recall went down — TP fell / FN rose) and a sound type that started being
// flagged (a new false positive). Both are stated in absolute counts, not
// rates, because the per-principle samples are too small for a rate delta to
// distinguish a real regression from one case of natural wobble.
type Regression struct {
	Principle string
	Kind      string // "recall" or "precision"
	Detail    string
}

// CompareToBaseline reports the ways the current report regressed against a
// committed baseline, per principle. A principle present in the baseline but
// absent from the current report is itself a regression (its labels or scores
// vanished, which would otherwise hide a drop). Principles new in the current
// report are not regressions.
//
// Recall regression: current TP < baseline TP (a previously-caught violation is
// now missed) — the recall floor the gate defends. Precision regression:
// current FP > baseline FP (a sound type is newly flagged).
func CompareToBaseline(current, baseline ReportJSON) []Regression {
	var regs []Regression

	principles := make([]string, 0, len(baseline.PerPrinciple))
	for p := range baseline.PerPrinciple {
		principles = append(principles, p)
	}
	sort.Strings(principles)

	for _, p := range principles {
		base := baseline.PerPrinciple[p]
		cur, ok := current.PerPrinciple[p]
		if !ok {
			regs = append(regs, Regression{
				Principle: p,
				Kind:      "recall",
				Detail: fmt.Sprintf(
					"principle disappeared from the report (baseline had TP=%d, FN=%d); labels or scores went missing",
					base.TP, base.FN),
			})
			continue
		}
		if cur.TP < base.TP {
			regs = append(regs, Regression{
				Principle: p,
				Kind:      "recall",
				Detail: fmt.Sprintf(
					"caught violations dropped %d -> %d (a known violation is now missed; recall floor breached)",
					base.TP, cur.TP),
			})
		}
		if cur.FP > base.FP {
			regs = append(regs, Regression{
				Principle: p,
				Kind:      "precision",
				Detail: fmt.Sprintf(
					"false positives rose %d -> %d (a sound type is newly flagged)",
					base.FP, cur.FP),
			})
		}
	}
	return regs
}

// LoadBaselineJSON decodes a committed baseline report (the JSON produced by
// `gss evaluate -f json`).
func LoadBaselineJSON(data []byte) (ReportJSON, error) {
	var r ReportJSON
	if err := json.Unmarshal(data, &r); err != nil {
		return ReportJSON{}, fmt.Errorf("parsing baseline report JSON: %w", err)
	}
	return r, nil
}
