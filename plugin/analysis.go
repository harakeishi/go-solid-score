// Package plugin provides [analysis.Analyzer] definitions for integration
// with golangci-lint and other go/analysis-based tools.
package plugin

import (
	"fmt"
	"go/token"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/harakeishi/go-solid-score/analyzer"
	"github.com/harakeishi/go-solid-score/config"
	"github.com/harakeishi/go-solid-score/scorer"
)

// Analyzer is the combined SOLID score analyzer for golangci-lint integration.
var Analyzer = &analysis.Analyzer{
	Name: "solidscore",
	Doc:  "checks SOLID principle compliance for Go structs",
	Run:  runAll,
}

// SRPAnalyzer checks only the Single Responsibility Principle.
var SRPAnalyzer = &analysis.Analyzer{
	Name: "solidsrp",
	Doc:  "checks Single Responsibility Principle compliance",
	Run:  makePrincipleRunner(analyzer.SRP),
}

// OCPAnalyzer checks only the Open/Closed Principle.
var OCPAnalyzer = &analysis.Analyzer{
	Name: "solidocp",
	Doc:  "checks Open/Closed Principle compliance",
	Run:  makePrincipleRunner(analyzer.OCP),
}

// LSPAnalyzer checks only the Liskov Substitution Principle.
var LSPAnalyzer = &analysis.Analyzer{
	Name: "solidlsp",
	Doc:  "checks Liskov Substitution Principle compliance",
	Run:  makePrincipleRunner(analyzer.LSP),
}

// ISPAnalyzer checks only the Interface Segregation Principle.
var ISPAnalyzer = &analysis.Analyzer{
	Name: "solidisp",
	Doc:  "checks Interface Segregation Principle compliance",
	Run:  makePrincipleRunner(analyzer.ISP),
}

// DIPAnalyzer checks only the Dependency Inversion Principle.
var DIPAnalyzer = &analysis.Analyzer{
	Name: "soliddip",
	Doc:  "checks Dependency Inversion Principle compliance",
	Run:  makePrincipleRunner(analyzer.DIP),
}

func runAll(pass *analysis.Pass) (interface{}, error) {
	cfg := config.Default()
	pi := PackageInfoFromPass(pass)

	engine, err := cfg.Engine()
	if err != nil {
		return nil, err
	}
	analyzers := []analyzer.Analyzer{
		analyzer.NewRuleAnalyzer(analyzer.SRP, engine, nil),
		analyzer.NewRuleAnalyzer(analyzer.OCP, engine, nil),
		analyzer.NewRuleAnalyzer(analyzer.LSP, engine, nil),
		analyzer.NewRuleAnalyzer(analyzer.ISP, engine, nil),
		analyzer.NewRuleAnalyzer(analyzer.DIP, engine, cfg.DIP.Whitelist),
	}

	s := scorer.New(analyzers, cfg.Weights)
	results := s.Score(pi)

	minScore := cfg.Thresholds["total"]
	for _, r := range results {
		if r.Total < minScore {
			var details []string
			for p, score := range r.Scores {
				details = append(details, fmt.Sprintf("%s=%.1f", p, score))
			}
			pos := posForLine(pass, r.TargetLine)
			pass.Reportf(pos,
				"SOLID total score %.1f is below minimum %.1f for %s (%s)",
				r.Total, minScore, r.TargetName, strings.Join(details, " "))
		}
	}

	return nil, nil
}

func posForLine(pass *analysis.Pass, line int) token.Pos {
	if len(pass.Files) == 0 {
		return token.NoPos
	}
	f := pass.Fset.File(pass.Files[0].Pos())
	if f == nil || line <= 0 || line > f.LineCount() {
		return pass.Files[0].Pos()
	}
	return f.LineStart(line)
}

func makePrincipleRunner(principle analyzer.Principle) func(*analysis.Pass) (interface{}, error) {
	return func(pass *analysis.Pass) (interface{}, error) {
		pi := PackageInfoFromPass(pass)

		var a analyzer.Analyzer
		switch principle {
		case analyzer.SRP:
			a = analyzer.NewSRPAnalyzer()
		case analyzer.OCP:
			a = analyzer.NewOCPAnalyzer()
		case analyzer.LSP:
			a = analyzer.NewLSPAnalyzer()
		case analyzer.ISP:
			a = analyzer.NewISPAnalyzer()
		case analyzer.DIP:
			a = analyzer.NewDIPAnalyzer(nil)
		}

		results := a.Analyze(pi)
		cfg := config.Default()
		threshold := cfg.Thresholds[string(principle)]

		for _, r := range results {
			if r.Score < threshold {
				pos := posForLine(pass, r.TargetLine)
				pass.Reportf(pos,
					"%s score %.1f is below minimum %.1f for %s: %s",
					r.Principle, r.Score, threshold, r.TargetName, strings.Join(r.Details, "; "))
			}
		}

		return nil, nil
	}
}
