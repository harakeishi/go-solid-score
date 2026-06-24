package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/harakeishi/go-solid-score/config"
)

// TestRuleSet_CustomRuleAppended verifies that a user-defined rule in the
// config is merged on top of the built-in presets.
func TestRuleSet_CustomRuleAppended(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".go-solid-score.yaml")
	content := `
disable_rules:
  - srp-method-count
rules:
  - id: my-field-rule
    principle: SRP
    metric: field_count
    when: "> 30"
    effect: penalty
    value: 25
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}

	rs := cfg.RuleSet()
	var foundCustom, foundDisabled bool
	for _, r := range rs.Rules {
		if r.ID == "my-field-rule" {
			foundCustom = true
		}
		if r.ID == "srp-method-count" && !r.IsEnabled() {
			foundDisabled = true
		}
	}
	if !foundCustom {
		t.Error("custom rule my-field-rule not present in merged rule set")
	}
	if !foundDisabled {
		t.Error("srp-method-count should be disabled")
	}

	// The merged set must still compile into an engine.
	if _, err := cfg.Engine(); err != nil {
		t.Fatalf("Engine() error: %v", err)
	}
}

// TestRuleSet_OverrideAffectsScore verifies that overriding a preset rule by id
// actually changes the score the engine produces.
func TestRuleSet_OverrideAffectsScore(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".go-solid-score.yaml")
	// Override the OCP type-switch penalty to be much larger.
	content := `
rules:
  - id: ocp-type-switch
    principle: OCP
    metric: type_switch_count
    when: "> 0"
    effect: penalty
    value: 90
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	engine, err := cfg.Engine()
	if err != nil {
		t.Fatal(err)
	}
	out := engine.Evaluate("OCP", false, map[string]float64{
		"method_count": 3, "type_switch_count": 1,
	})
	if out.Score != 10 {
		t.Errorf("overridden type-switch penalty: score = %v, want 10 (100-90)", out.Score)
	}
}

// TestRuleSet_BadConditionSurfacesError ensures a malformed user condition is
// reported rather than silently ignored.
func TestRuleSet_BadConditionSurfacesError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".go-solid-score.yaml")
	content := `
rules:
  - id: broken
    principle: SRP
    metric: method_count
    when: "?? 3"
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := cfg.Engine(); err == nil {
		t.Error("expected Engine() to fail on malformed condition")
	}
}
