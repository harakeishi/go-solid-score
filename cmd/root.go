package cmd

import (
	"fmt"
	"os"

	"github.com/harakeishi/go-solid-score/analyzer"
	"github.com/harakeishi/go-solid-score/config"
	"github.com/harakeishi/go-solid-score/formatter"
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

	// Parse
	pkgs, err := parser.Parse(patterns)
	if err != nil {
		return fmt.Errorf("parsing packages: %w", err)
	}

	// Build analyzers
	analyzers := []analyzer.Analyzer{
		analyzer.NewSRPAnalyzer(),
		analyzer.NewOCPAnalyzer(),
		analyzer.NewLSPAnalyzer(),
		analyzer.NewISPAnalyzer(),
		analyzer.NewDIPAnalyzer(cfg.DIP.Whitelist),
	}

	// Score
	s := scorer.New(analyzers, cfg.Weights)
	var allResults []*scorer.ScoreResult
	for _, pkg := range pkgs {
		results := s.Score(pkg)
		allResults = append(allResults, results...)
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
	fmt.Fprint(os.Stdout, output)

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
