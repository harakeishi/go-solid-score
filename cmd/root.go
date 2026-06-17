// Package cmd implements the command-line interface for go-solid-score.
package cmd

import (
	"fmt"
	"os"

	"github.com/harakeishi/go-solid-score/analyzer"
	"github.com/harakeishi/go-solid-score/config"
	"github.com/harakeishi/go-solid-score/formatter"
	"github.com/harakeishi/go-solid-score/model"
	"github.com/harakeishi/go-solid-score/parser"
	"github.com/harakeishi/go-solid-score/scorer"
	"github.com/spf13/cobra"
)

var (
	version    = "dev"
	cfgFile    string
	formatFlag string
	minScore   float64
	verbose    bool
)

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "go-solid-score [packages...]",
		Short: "Analyze Go code for SOLID principle compliance",
		Long:  "go-solid-score statically analyzes Go source code and scores structs against the five SOLID principles.",
		Args:  cobra.ArbitraryArgs,
		RunE:  run,
	}

	cmd.Flags().StringVarP(&cfgFile, "config", "c", ".go-solid-score.yaml", "Config file path")
	cmd.Flags().StringVarP(&formatFlag, "format", "f", "", "Output format: text, json")
	cmd.Flags().Float64Var(&minScore, "min-score", 0, "Exit with code 1 if any score below this")
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Show detailed breakdown per struct")
	cmd.Version = version

	cmd.AddCommand(newDiffCmd())
	cmd.AddCommand(newEvaluateCmd())

	return cmd
}

// Execute runs the root command and returns the exit code.
func Execute() int {
	if err := newRootCmd().Execute(); err != nil {
		return 1
	}
	return 0
}

func run(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// CLI flags override config
	if formatFlag != "" {
		cfg.Format = formatFlag
	}
	if minScore > 0 {
		cfg.MinScore = minScore
	}

	patterns := cfg.Paths
	if len(args) > 0 {
		patterns = args
	}

	allResults, err := analyze(cfg, patterns)
	if err != nil {
		return err
	}

	// Format
	var f formatter.Formatter
	switch cfg.Format {
	case "json":
		f = &formatter.JSONFormatter{}
	default:
		f = &formatter.TextFormatter{}
	}

	output, err := f.Format(allResults)
	if err != nil {
		return fmt.Errorf("formatting output: %w", err)
	}
	if _, err := fmt.Fprint(os.Stdout, output); err != nil {
		return fmt.Errorf("writing output: %w", err)
	}

	// Check thresholds
	if cfg.MinScore > 0 {
		for _, r := range allResults {
			if r.Total < cfg.MinScore {
				return fmt.Errorf("score %.1f for %s is below minimum %.1f", r.Total, r.TargetName, cfg.MinScore)
			}
		}
	}

	return nil
}

// parsePackages parses the given patterns into model packages. It wraps
// parser.Parse with a consistent error so callers needing the raw packages
// (e.g. evaluate, for label collection) share one parse path.
func parsePackages(patterns []string) ([]*model.PackageInfo, error) {
	pkgs, err := parser.Parse(patterns)
	if err != nil {
		return nil, fmt.Errorf("parsing packages: %w", err)
	}
	return pkgs, nil
}

// analyze parses the given patterns and scores every target, applying the
// scoring rules from cfg. It is the shared core used by both `run` and `diff`.
func analyze(cfg *config.Config, patterns []string) ([]*scorer.ScoreResult, error) {
	pkgs, err := parsePackages(patterns)
	if err != nil {
		return nil, err
	}
	return scorePackages(cfg, pkgs), nil
}

// scorePackages scores already-parsed packages. Splitting it from analyze lets
// a caller that also needs the parsed packages (e.g. evaluate, which collects
// inline labels from the same syntax) parse once and feed the result to both,
// rather than loading — and fully building — the same source twice.
func scorePackages(cfg *config.Config, pkgs []*model.PackageInfo) []*scorer.ScoreResult {
	analyzers := []analyzer.Analyzer{
		analyzer.NewSRPAnalyzer(),
		analyzer.NewOCPAnalyzer(),
		analyzer.NewLSPAnalyzer(),
		analyzer.NewISPAnalyzer(),
		analyzer.NewDIPAnalyzer(cfg.DIP.Whitelist),
	}

	s := scorer.New(analyzers, cfg.Weights)
	var allResults []*scorer.ScoreResult
	for _, pkg := range pkgs {
		allResults = append(allResults, s.Score(pkg)...)
	}
	return allResults
}
