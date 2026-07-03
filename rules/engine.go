package rules

import (
	"fmt"
	"math"
	"strings"
)

// Metrics is the flat set of named, numeric facts computed for a single target
// (a struct or interface). Boolean facts are encoded as 0 or 1. Rules read
// these by name. A missing metric is treated as 0 during evaluation.
type Metrics map[string]float64

// Outcome is the result of evaluating one principle's rules against a target.
type Outcome struct {
	Score      float64
	Confidence float64
	Details    []string
}

// compiledRule is a Rule with its condition strings parsed once at engine
// construction, so evaluation does no string parsing and construction surfaces
// malformed conditions as errors up front.
type compiledRule struct {
	rule  Rule
	when  *comparison   // nil means "always"
	where []whereClause // all must hold
	bands []compiledBand
}

type compiledBand struct {
	band Band
	when comparison
}

// Engine evaluates a [RuleSet] against target metrics. It is safe for
// concurrent use after construction.
type Engine struct {
	defaults    map[string]Defaults
	byprinciple map[string][]compiledRule
	ifaceRules  map[string]bool
}

// defaultBaseScore and defaultBaseConfidence are used for a principle that has
// no explicit defaults entry.
const (
	defaultBaseScore      = 100.0
	defaultBaseConfidence = 0.7
)

// NewEngine compiles a rule set into an Engine, validating it up front. Every
// condition string must parse, and every enabled rule must actually do
// something (no silent no-ops). If knownMetrics is non-empty, each enabled
// rule's metric references are checked against it, so a typo'd or dropped
// metric name is reported rather than silently read as 0. A failure returns an
// error that names the offending rule.
func NewEngine(rs RuleSet, knownMetrics ...string) (*Engine, error) {
	var known map[string]bool
	if len(knownMetrics) > 0 {
		known = make(map[string]bool, len(knownMetrics))
		for _, name := range knownMetrics {
			known[name] = true
		}
	}

	e := &Engine{
		defaults:    rs.Defaults,
		byprinciple: make(map[string][]compiledRule),
		ifaceRules:  make(map[string]bool),
	}
	for _, r := range rs.Rules {
		// Disabled rules never run, so they are not validated — this lets a user
		// park an intentionally-empty placeholder behind `enabled: false`.
		if r.IsEnabled() {
			if r.isNoop() {
				return nil, fmt.Errorf("rule %q has no effect: it neither changes the score nor sets confidence nor stops; if you meant to override a preset, copy all of its fields, or disable it via disable_rules", r.ID)
			}
			if known != nil {
				if r.usesMetric() && !known[r.Metric] {
					return nil, fmt.Errorf("rule %q references unknown metric %q", r.ID, r.Metric)
				}
				for _, w := range r.Where {
					if name := whereMetric(w); name != "" && !known[name] {
						return nil, fmt.Errorf("rule %q where clause references unknown metric %q", r.ID, name)
					}
				}
			}
		}

		cr := compiledRule{rule: r}
		if strings.TrimSpace(r.When) != "" {
			c, err := parseComparison(r.When)
			if err != nil {
				return nil, fmt.Errorf("rule %q: when: %w", r.ID, err)
			}
			cr.when = &c
		}
		for _, w := range r.Where {
			wc, err := parseWhere(w)
			if err != nil {
				return nil, fmt.Errorf("rule %q: where: %w", r.ID, err)
			}
			cr.where = append(cr.where, wc)
		}
		for _, b := range r.Bands {
			c, err := parseComparison(b.When)
			if err != nil {
				return nil, fmt.Errorf("rule %q: band: %w", r.ID, err)
			}
			cr.bands = append(cr.bands, compiledBand{band: b, when: c})
		}
		e.byprinciple[r.Principle] = append(e.byprinciple[r.Principle], cr)
		if r.IsEnabled() && r.AppliesTo(true) {
			e.ifaceRules[r.Principle] = true
		}
	}
	return e, nil
}

// whereMetric extracts just the metric name from a where clause for validation,
// without failing on a malformed clause (the full parse below reports that).
func whereMetric(clause string) string {
	fields := strings.Fields(clause)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

// HasInterfaceRules reports whether the principle has any enabled rule that
// targets interface definitions. Analyzers use this to decide whether to score
// interface targets at all.
func (e *Engine) HasInterfaceRules(principle string) bool {
	return e.ifaceRules[principle]
}

// Evaluate runs the principle's rules against the metrics of one target and
// returns its score, confidence, and detail lines. isInterface selects which
// rules apply (struct vs interface targets).
func (e *Engine) Evaluate(principle string, isInterface bool, m Metrics) Outcome {
	d := e.defaults[principle]
	// Treat an unset (zero) starting value as the built-in default. A base score
	// or confidence of 0 is never a meaningful configuration, so this guards
	// against a principle whose defaults entry is missing or only partially
	// specified silently scoring everything at 0.
	if d.BaseScore == 0 {
		d.BaseScore = defaultBaseScore
	}
	if d.BaseConfidence == 0 {
		d.BaseConfidence = defaultBaseConfidence
	}
	out := Outcome{Score: d.BaseScore, Confidence: d.BaseConfidence}

	for _, cr := range e.byprinciple[principle] {
		r := cr.rule
		if !r.IsEnabled() || !r.AppliesTo(isInterface) {
			continue
		}
		if !whereHolds(cr.where, m) {
			continue
		}

		metricVal := m[r.Metric]
		matched := false

		if len(cr.bands) > 0 {
			for _, cb := range cr.bands {
				if cb.when.eval(metricVal) {
					applyEffect(&out, effectOr(cb.band.Effect, r.Effect), cb.band.Value, false, 0, nil, metricVal)
					addDetail(&out, cb.band.Message, metricVal)
					matched = true
					break
				}
			}
		} else if cr.when == nil || cr.when.eval(metricVal) {
			applyEffect(&out, effectOf(r), r.Value, r.FromMetric, r.Scale, r.Cap, metricVal)
			addDetail(&out, r.Message, metricVal)
			matched = true
		}

		if matched {
			if r.Confidence != nil {
				out.Confidence = *r.Confidence
			}
			if r.Stop {
				break
			}
		}
	}

	out.Score = clamp(out.Score)
	return out
}

// effectOf returns the effect kind for a band-less rule, defaulting to penalty.
func effectOf(r Rule) string { return effectOr(r.Effect, EffectPenalty) }

// effectOr returns primary if non-empty, else fallback.
func effectOr(primary, fallback string) string {
	if primary != "" {
		return primary
	}
	if fallback != "" {
		return fallback
	}
	return EffectPenalty
}

// applyEffect mutates the outcome score per the effect kind. When fromMetric is
// set the amount is metricVal×scale (scale defaults to 1); otherwise it is the
// literal value. cap, when non-nil, limits the magnitude of a penalty/bonus.
func applyEffect(out *Outcome, effect string, value float64, fromMetric bool, scale float64, capLimit *float64, metricVal float64) {
	amount := value
	if fromMetric {
		s := scale
		if s == 0 {
			s = 1
		}
		amount = metricVal * s
	}
	if capLimit != nil && amount > *capLimit {
		amount = *capLimit
	}
	switch effect {
	case EffectBonus:
		out.Score += amount
	case EffectSet:
		out.Score = amount
	case EffectNone:
		// score unchanged
	default: // EffectPenalty
		out.Score -= amount
	}
}

// whereHolds reports whether every where clause is satisfied by the metrics.
func whereHolds(where []whereClause, m Metrics) bool {
	for _, w := range where {
		if !w.cmp.eval(m[w.metric]) {
			return false
		}
	}
	return true
}

// addDetail appends a formatted detail line if a message is present. A format
// verb in the message (e.g. "%v") is filled with the metric value, rounded to
// two decimals so ratio metrics don't leak full float precision into the
// detail line (LSCC=0.03, not LSCC=0.03333333333333333); whole numbers still
// render bare ("16"). If formatting fails — the "%" was a literal, or the verb
// doesn't apply to float64 — the message is kept verbatim rather than emitting
// fmt's %! error markers.
func addDetail(out *Outcome, message string, metricVal float64) {
	if message == "" {
		return
	}
	if strings.Contains(message, "%") {
		formatted := fmt.Sprintf(message, math.Round(metricVal*100)/100)
		if !strings.Contains(formatted, "%!") {
			message = formatted
		}
	}
	out.Details = append(out.Details, message)
}

// clamp constrains a score to [0, 100].
func clamp(score float64) float64 {
	if score < 0 {
		return 0
	}
	if score > 100 {
		return 100
	}
	return score
}
