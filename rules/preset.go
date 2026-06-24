package rules

import (
	_ "embed"
	"fmt"
	"sync"

	"gopkg.in/yaml.v3"
)

//go:embed presets.yaml
var presetsYAML []byte

var (
	presetOnce sync.Once
	presetSet  RuleSet
	presetErr  error
)

// loadPresets parses the embedded default rule set once. Because the YAML is
// compiled into the binary, a parse failure is a programming error in this
// package rather than a user error; callers surface it as such.
func loadPresets() (RuleSet, error) {
	presetOnce.Do(func() {
		presetErr = yaml.Unmarshal(presetsYAML, &presetSet)
	})
	return presetSet, presetErr
}

// DefaultRuleSet returns a copy of the built-in default rule set. Callers may
// merge user customizations into it via [Merge].
func DefaultRuleSet() RuleSet {
	base, err := loadPresets()
	if err != nil {
		// The embedded presets are validated by tests; a failure here means the
		// embedded YAML was edited into an invalid state.
		panic(fmt.Sprintf("rules: invalid embedded presets: %v", err))
	}
	// Return a defensive copy so callers cannot mutate the shared preset slices.
	out := RuleSet{
		Defaults: make(map[string]Defaults, len(base.Defaults)),
		Rules:    make([]Rule, len(base.Rules)),
	}
	for k, v := range base.Defaults {
		out.Defaults[k] = v
	}
	copy(out.Rules, base.Rules)
	return out
}

var (
	defaultEngineOnce sync.Once
	defaultEngine     *Engine
)

// DefaultEngine returns the shared [Engine] built from the built-in default
// rule set. The engine is immutable and safe for concurrent use, so it is
// compiled once and reused across callers.
func DefaultEngine() *Engine {
	defaultEngineOnce.Do(func() {
		e, err := NewEngine(DefaultRuleSet())
		if err != nil {
			panic(fmt.Sprintf("rules: invalid embedded presets: %v", err))
		}
		defaultEngine = e
	})
	return defaultEngine
}

// ParseRuleSet unmarshals a rule set from YAML bytes (used for the `rules:` and
// `defaults:` sections of a user config file).
func ParseRuleSet(data []byte) (RuleSet, error) {
	var rs RuleSet
	if err := yaml.Unmarshal(data, &rs); err != nil {
		return RuleSet{}, err
	}
	return rs, nil
}
