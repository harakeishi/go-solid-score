// Package rules implements a declarative scoring engine for go-solid-score.
//
// Historically every SOLID principle was scored by bespoke Go code with the
// thresholds, penalties, and bonuses hard-coded inside each analyzer. This
// package replaces that with a data-driven model: the scoring logic is a list
// of [Rule] values that operate over a flat set of named metrics computed from
// the source. The five built-in principles ship as an embedded default
// rule set (see preset.go and presets.yaml) that reproduces the original
// behavior, and users can disable presets, retune their parameters, or add
// entirely new rules of their own through configuration — all without
// recompiling.
package rules

import "strings"

// Effect kinds a rule (or band) can have on a target's running score.
const (
	EffectPenalty = "penalty" // subtract the value from the score (the default)
	EffectBonus   = "bonus"   // add the value to the score
	EffectSet     = "set"     // set the score to the value
	EffectNone    = "none"    // leave the score unchanged (e.g. confidence-only)
)

// Target kinds a rule can apply to.
const (
	TargetStruct    = "struct"    // struct definitions (the default)
	TargetInterface = "interface" // interface definitions
	TargetBoth      = "both"      // both structs and interfaces
)

// Band is one entry of a rule's banded effect. The first band (in order) whose
// `when` condition matches the rule's metric value is applied; later bands are
// ignored. Bands always use literal values; use a band-less rule with
// from_metric to derive the value from the metric itself.
type Band struct {
	When    string  `yaml:"when"`    // condition on the rule's metric, e.g. "> 40"
	Effect  string  `yaml:"effect"`  // overrides the rule's effect; defaults to it
	Value   float64 `yaml:"value"`   // the literal penalty/bonus/set amount
	Message string  `yaml:"message"` // optional detail line; %v is the metric value
}

// Rule is a single declarative scoring rule. A rule reads one metric, checks an
// optional condition (and optional preconditions), and applies an effect to the
// target's score. Rules for a principle are evaluated in order; each starts from
// the running score left by the previous rule.
type Rule struct {
	// ID uniquely identifies the rule. User rules with a matching ID replace the
	// preset rule of the same ID in place; new IDs are appended.
	ID string `yaml:"id"`
	// Principle is the SOLID principle this rule contributes to (SRP/OCP/LSP/ISP/DIP).
	Principle string `yaml:"principle"`
	// Target selects the kind of definition the rule applies to (default struct).
	Target string `yaml:"target"`
	// Enabled toggles the rule. Nil means enabled; set false to disable a preset.
	Enabled *bool `yaml:"enabled"`

	// Metric is the name of the metric this rule reads (see analyzer.Metrics).
	Metric string `yaml:"metric"`
	// Where lists extra preconditions ("<metric> <op> <number>"); all must hold.
	Where []string `yaml:"where"`
	// When is the condition on Metric ("<op> <number>"). Empty means always.
	When string `yaml:"when"`

	// Effect selects how a matched rule changes the score (default penalty).
	Effect string `yaml:"effect"`
	// Value is the literal amount for the effect (ignored when FromMetric is set).
	Value float64 `yaml:"value"`
	// FromMetric derives the effect amount from the metric value (× Scale).
	FromMetric bool `yaml:"from_metric"`
	// Scale multiplies the metric value when FromMetric is set (default 1).
	Scale float64 `yaml:"scale"`
	// Cap limits the magnitude of a penalty/bonus amount (nil means no cap).
	Cap *float64 `yaml:"cap"`

	// Bands is an ordered list of banded effects; the first match applies.
	Bands []Band `yaml:"bands"`

	// Confidence, when non-nil and the rule matches, sets the result confidence.
	Confidence *float64 `yaml:"confidence"`
	// Message is an optional detail line added when the rule matches; %v is the
	// metric value.
	Message string `yaml:"message"`
	// Stop ends evaluation of further rules for the target when this rule matches.
	Stop bool `yaml:"stop"`
}

// IsEnabled reports whether the rule is active (nil Enabled means enabled).
func (r Rule) IsEnabled() bool { return r.Enabled == nil || *r.Enabled }

// isNoop reports whether the rule can never change a target's score, confidence,
// or control flow — i.e. it does literally nothing. This is almost always a
// configuration mistake: the classic case is overriding a preset by id while
// supplying only the field you meant to tweak (e.g. just `cap`), which replaces
// the whole rule and drops its metric/effect, silently turning it into a no-op.
// The engine rejects such rules at construction so the mistake is surfaced
// rather than quietly dropping the preset's behavior.
func (r Rule) isNoop() bool {
	switch {
	case r.Effect == EffectSet: // set always assigns the score, even to 0
		return false
	case r.FromMetric: // derives a (possibly non-zero) amount from the metric
		return false
	case r.Value != 0: // a literal penalty/bonus
		return false
	case len(r.Bands) > 0: // bands apply their own effects
		return false
	case r.Confidence != nil: // adjusts confidence
		return false
	case r.Stop: // controls evaluation flow
		return false
	}
	return true
}

// usesMetric reports whether the rule actually reads its Metric value (through a
// condition, banded thresholds, or a from-metric effect). A rule that does not
// use its metric may legitimately leave Metric empty (e.g. an unconditional
// confidence-only rule).
func (r Rule) usesMetric() bool {
	return strings.TrimSpace(r.When) != "" || len(r.Bands) > 0 || r.FromMetric
}

// AppliesTo reports whether the rule targets the given kind of definition.
func (r Rule) AppliesTo(isInterface bool) bool {
	switch r.Target {
	case TargetInterface:
		return isInterface
	case TargetBoth:
		return true
	default: // TargetStruct or empty
		return !isInterface
	}
}

// Defaults holds the starting score and confidence for a principle before any
// rule runs.
type Defaults struct {
	BaseScore      float64 `yaml:"base_score"`
	BaseConfidence float64 `yaml:"base_confidence"`
}

// RuleSet is the complete declarative scoring configuration: per-principle
// starting values plus the ordered list of rules.
type RuleSet struct {
	Defaults map[string]Defaults `yaml:"defaults"`
	Rules    []Rule              `yaml:"rules"`
}

// Merge overlays user customizations onto a base rule set and returns the
// result. The base set is not mutated. The merge rules are:
//
//   - A user rule whose ID matches a base rule replaces that base rule in place,
//     preserving its position so evaluation order is stable.
//   - A user rule with a new ID is appended after the base rules.
//   - IDs listed in disable are turned off (Enabled=false) after merging, so a
//     preset can be switched off without redefining it.
//   - User Defaults override base Defaults per principle.
func Merge(base RuleSet, user RuleSet, disable []string) RuleSet {
	out := RuleSet{
		Defaults: make(map[string]Defaults, len(base.Defaults)),
		Rules:    make([]Rule, len(base.Rules)),
	}
	for k, v := range base.Defaults {
		out.Defaults[k] = v
	}
	// Overlay user defaults field by field so a partial override (e.g. setting
	// only base_score) keeps the base value for the unspecified field rather
	// than zeroing it. A zero value means "not set" here, since neither a
	// starting score of 0 nor a confidence of 0 is a meaningful configuration.
	for k, v := range user.Defaults {
		d := out.Defaults[k]
		if v.BaseScore != 0 {
			d.BaseScore = v.BaseScore
		}
		if v.BaseConfidence != 0 {
			d.BaseConfidence = v.BaseConfidence
		}
		out.Defaults[k] = d
	}
	copy(out.Rules, base.Rules)

	indexByID := make(map[string]int, len(out.Rules))
	for i, r := range out.Rules {
		if r.ID != "" {
			indexByID[r.ID] = i
		}
	}
	for _, ur := range user.Rules {
		if i, ok := indexByID[ur.ID]; ok && ur.ID != "" {
			out.Rules[i] = ur
			continue
		}
		indexByID[ur.ID] = len(out.Rules)
		out.Rules = append(out.Rules, ur)
	}

	if len(disable) > 0 {
		off := false
		disableSet := make(map[string]bool, len(disable))
		for _, id := range disable {
			disableSet[id] = true
		}
		for i := range out.Rules {
			if disableSet[out.Rules[i].ID] {
				out.Rules[i].Enabled = &off
			}
		}
	}

	return out
}
