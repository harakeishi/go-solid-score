package rules

import "testing"

func ptr(f float64) *float64 { return &f }
func bptr(b bool) *bool      { return &b }

// TestDefaultPresetsCompile guards the embedded preset file: it must parse and
// compile (every condition valid). DefaultEngine panics otherwise.
func TestDefaultPresetsCompile(t *testing.T) {
	e := DefaultEngine()
	if !e.HasInterfaceRules("ISP") {
		t.Error("ISP should have interface-targeted rules")
	}
	if e.HasInterfaceRules("DIP") {
		t.Error("DIP should not have interface-targeted rules")
	}
}

func TestEvaluate_PenaltyAndCap(t *testing.T) {
	rs := RuleSet{
		Defaults: map[string]Defaults{"OCP": {BaseScore: 100, BaseConfidence: 0.7}},
		Rules: []Rule{{
			ID: "ts", Principle: "OCP", Metric: "type_switch_count",
			When: "> 0", Effect: EffectPenalty, FromMetric: true, Scale: 15, Cap: ptr(40),
		}},
	}
	e, err := NewEngine(rs)
	if err != nil {
		t.Fatal(err)
	}
	// 1 switch -> -15 -> 85
	if got := e.Evaluate("OCP", false, Metrics{"type_switch_count": 1}).Score; got != 85 {
		t.Errorf("score = %v, want 85", got)
	}
	// 5 switches -> -75 capped at -40 -> 60
	if got := e.Evaluate("OCP", false, Metrics{"type_switch_count": 5}).Score; got != 60 {
		t.Errorf("capped score = %v, want 60", got)
	}
}

func TestEvaluate_Bands(t *testing.T) {
	rs := RuleSet{
		Rules: []Rule{{
			ID: "size", Principle: "ISP", Target: TargetInterface, Metric: "total_methods",
			Effect: EffectSet,
			Bands: []Band{
				{When: "> 15", Value: 20},
				{When: "> 10", Value: 40},
				{When: "> 5", Value: 75},
			},
		}},
	}
	e, _ := NewEngine(rs)
	if got := e.Evaluate("ISP", true, Metrics{"total_methods": 12}).Score; got != 40 {
		t.Errorf("12 methods -> %v, want 40", got)
	}
	if got := e.Evaluate("ISP", true, Metrics{"total_methods": 3}).Score; got != 100 {
		t.Errorf("3 methods -> %v, want 100 (no band)", got)
	}
}

func TestEvaluate_WhereAndStop(t *testing.T) {
	rs := RuleSet{
		Rules: []Rule{
			{ID: "floor", Principle: "DIP", Metric: "iface_dep_ratio",
				Where: []string{"structural_dep_total == 0"}, When: "< 0.5",
				Effect: EffectSet, Value: 50, Stop: true},
			{ID: "after", Principle: "DIP", Metric: "iface_dep_ratio",
				Effect: EffectSet, Value: 0},
		},
	}
	e, _ := NewEngine(rs)
	// where holds, when holds -> set 50, stop (the "after" rule must not run)
	out := e.Evaluate("DIP", false, Metrics{"iface_dep_ratio": 0, "structural_dep_total": 0})
	if out.Score != 50 {
		t.Errorf("score = %v, want 50 (stop should prevent 'after')", out.Score)
	}
	// where fails (structural>0) -> floor skipped -> "after" sets 0
	out = e.Evaluate("DIP", false, Metrics{"iface_dep_ratio": 0, "structural_dep_total": 2})
	if out.Score != 0 {
		t.Errorf("score = %v, want 0", out.Score)
	}
}

func TestEvaluate_ConfidenceOverride(t *testing.T) {
	rs := RuleSet{
		Defaults: map[string]Defaults{"LSP": {BaseScore: 100, BaseConfidence: 0.7}},
		Rules: []Rule{{
			ID: "impl", Principle: "LSP", Metric: "implements_interface",
			When: ">= 1", Effect: EffectNone, Confidence: ptr(0.85),
		}},
	}
	e, _ := NewEngine(rs)
	if got := e.Evaluate("LSP", false, Metrics{"implements_interface": 1}).Confidence; got != 0.85 {
		t.Errorf("confidence = %v, want 0.85", got)
	}
	if got := e.Evaluate("LSP", false, Metrics{"implements_interface": 0}).Confidence; got != 0.7 {
		t.Errorf("confidence = %v, want 0.7 (base)", got)
	}
}

func TestEvaluate_DisabledRuleSkipped(t *testing.T) {
	rs := RuleSet{
		Rules: []Rule{{
			ID: "p", Principle: "SRP", Metric: "method_count", When: "> 0",
			Effect: EffectPenalty, Value: 50, Enabled: bptr(false),
		}},
	}
	e, _ := NewEngine(rs)
	if got := e.Evaluate("SRP", false, Metrics{"method_count": 3}).Score; got != 100 {
		t.Errorf("disabled rule applied: score = %v, want 100", got)
	}
}

func TestNewEngine_BadConditionErrors(t *testing.T) {
	_, err := NewEngine(RuleSet{Rules: []Rule{{ID: "x", Principle: "SRP", When: "~~"}}})
	if err == nil {
		t.Error("expected error for malformed condition")
	}
}
