package analyzer

import (
	"github.com/harakeishi/go-solid-score/model"
	"github.com/harakeishi/go-solid-score/rules"
)

// RuleAnalyzer scores one SOLID principle by evaluating that principle's rules
// (from a declarative [rules.Engine]) against metrics computed from the source.
// It implements [Analyzer], so it is a drop-in replacement for the former
// hand-written per-principle analyzers — the scoring numbers now live in the
// rule set rather than in Go code, which is what makes them configurable.
type RuleAnalyzer struct {
	principle Principle
	engine    *rules.Engine
	// whitelist is the DIP dependency whitelist; it only affects the dependency
	// metrics and is nil for the other principles.
	whitelist []string
}

// NewRuleAnalyzer builds a RuleAnalyzer for a principle backed by the given
// engine. whitelist is forwarded to metric computation (used by DIP only).
func NewRuleAnalyzer(principle Principle, engine *rules.Engine, whitelist []string) *RuleAnalyzer {
	return &RuleAnalyzer{principle: principle, engine: engine, whitelist: whitelist}
}

func (a *RuleAnalyzer) Principle() Principle { return a.principle }

// Analyze scores every struct (and, when the principle has interface-targeted
// rules, every interface) in the package against this principle's rules.
func (a *RuleAnalyzer) Analyze(pkg *model.PackageInfo) []Result {
	p := string(a.principle)
	var results []Result

	for _, s := range pkg.Structs {
		m := StructMetrics(s, pkg, a.whitelist)
		out := a.engine.Evaluate(p, false, m)
		results = append(results, Result{
			Principle:  a.principle,
			TargetPkg:  pkg.PkgPath,
			TargetName: s.Name,
			TargetFile: s.File,
			TargetLine: s.Line,
			Score:      out.Score,
			Confidence: out.Confidence,
			Details:    out.Details,
		})
	}

	if a.engine.HasInterfaceRules(p) {
		for _, iface := range pkg.Interfaces {
			m := InterfaceMetrics(iface)
			out := a.engine.Evaluate(p, true, m)
			results = append(results, Result{
				Principle:         a.principle,
				TargetPkg:         pkg.PkgPath,
				TargetName:        iface.Name,
				TargetFile:        iface.File,
				TargetLine:        iface.Line,
				TargetIsInterface: true,
				Score:             out.Score,
				Confidence:        out.Confidence,
				Details:           out.Details,
			})
		}
	}

	return results
}

// The constructors below preserve the original analyzer API. Each returns a
// RuleAnalyzer driven by the built-in default rule set, so existing callers
// (and the golangci-lint plugin) keep working unchanged while the scoring logic
// is now data-driven. To score with customized rules, build an engine from a
// merged rule set and use [NewRuleAnalyzer] directly.

// NewSRPAnalyzer returns the default SRP analyzer.
func NewSRPAnalyzer() *RuleAnalyzer { return NewRuleAnalyzer(SRP, rules.DefaultEngine(), nil) }

// NewOCPAnalyzer returns the default OCP analyzer.
func NewOCPAnalyzer() *RuleAnalyzer { return NewRuleAnalyzer(OCP, rules.DefaultEngine(), nil) }

// NewLSPAnalyzer returns the default LSP analyzer.
func NewLSPAnalyzer() *RuleAnalyzer { return NewRuleAnalyzer(LSP, rules.DefaultEngine(), nil) }

// NewISPAnalyzer returns the default ISP analyzer.
func NewISPAnalyzer() *RuleAnalyzer { return NewRuleAnalyzer(ISP, rules.DefaultEngine(), nil) }

// NewDIPAnalyzer returns the default DIP analyzer using the given dependency
// whitelist.
func NewDIPAnalyzer(userWhitelist []string) *RuleAnalyzer {
	return NewRuleAnalyzer(DIP, rules.DefaultEngine(), userWhitelist)
}
