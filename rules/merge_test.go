package rules

import "testing"

func TestMerge_OverrideByID(t *testing.T) {
	base := RuleSet{Rules: []Rule{
		{ID: "a", Principle: "SRP", Value: 10},
		{ID: "b", Principle: "OCP", Value: 20},
	}}
	user := RuleSet{Rules: []Rule{
		{ID: "a", Principle: "SRP", Value: 99}, // override in place
		{ID: "c", Principle: "DIP", Value: 5},  // append
	}}
	out := Merge(base, user, nil)

	if len(out.Rules) != 3 {
		t.Fatalf("len = %d, want 3", len(out.Rules))
	}
	if out.Rules[0].ID != "a" || out.Rules[0].Value != 99 {
		t.Errorf("rule a not overridden in place: %+v", out.Rules[0])
	}
	if out.Rules[1].ID != "b" {
		t.Errorf("rule b moved: %+v", out.Rules[1])
	}
	if out.Rules[2].ID != "c" {
		t.Errorf("rule c not appended: %+v", out.Rules[2])
	}
}

func TestMerge_Disable(t *testing.T) {
	base := RuleSet{Rules: []Rule{{ID: "a", Principle: "SRP"}}}
	out := Merge(base, RuleSet{}, []string{"a"})
	if out.Rules[0].IsEnabled() {
		t.Error("rule a should be disabled")
	}
	// base must not be mutated
	if !base.Rules[0].IsEnabled() {
		t.Error("Merge mutated the base rule set")
	}
}

func TestMerge_DefaultsOverride(t *testing.T) {
	base := RuleSet{Defaults: map[string]Defaults{"SRP": {BaseScore: 100, BaseConfidence: 1.0}}}
	user := RuleSet{Defaults: map[string]Defaults{"SRP": {BaseScore: 90, BaseConfidence: 0.5}}}
	out := Merge(base, user, nil)
	if out.Defaults["SRP"].BaseScore != 90 {
		t.Errorf("base score = %v, want 90", out.Defaults["SRP"].BaseScore)
	}
}

// TestMerge_DefaultsPartialOverride is the regression guard for the bug where
// setting only base_score zeroed base_confidence (silently dropping a
// principle's confidence to 0). A partial override must keep the base value for
// the field the user did not specify.
func TestMerge_DefaultsPartialOverride(t *testing.T) {
	base := RuleSet{Defaults: map[string]Defaults{"SRP": {BaseScore: 100, BaseConfidence: 1.0}}}
	user := RuleSet{Defaults: map[string]Defaults{"SRP": {BaseScore: 90}}} // confidence omitted
	out := Merge(base, user, nil)
	if got := out.Defaults["SRP"].BaseScore; got != 90 {
		t.Errorf("base score = %v, want 90", got)
	}
	if got := out.Defaults["SRP"].BaseConfidence; got != 1.0 {
		t.Errorf("base confidence = %v, want 1.0 preserved (not zeroed)", got)
	}
}
