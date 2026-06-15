package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/harakeishi/go-solid-score/config"
	"github.com/harakeishi/go-solid-score/differ"
	"github.com/harakeishi/go-solid-score/formatter"
	"github.com/harakeishi/go-solid-score/scorer"
	"github.com/spf13/cobra"
)

var (
	diffBase      string
	diffMaxDrop   float64
	diffMinScore  float64
	diffFailOnReg bool
	diffFormat    string
)

func newDiffCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "diff [packages...]",
		Short: "Compare current SOLID scores against a baseline JSON",
		Long: "diff analyzes the given packages and compares each target's score " +
			"against a baseline JSON (produced earlier with `-f json`), reporting " +
			"regressions, improvements, new and removed targets.",
		Args: cobra.ArbitraryArgs,
		RunE: runDiff,
		// A detected regression is a normal outcome, not a usage error, so
		// don't print the help/usage text on a non-nil return.
		SilenceUsage: true,
	}
	cmd.Flags().StringVar(&diffBase, "base", "", "Baseline JSON file to compare against (required)")
	cmd.Flags().Float64Var(&diffMaxDrop, "max-drop", 5.0, "A total drop greater than this is a regression")
	cmd.Flags().Float64Var(&diffMinScore, "min-score", 0, "A new target below this is NEW-LOW (0 disables)")
	cmd.Flags().BoolVar(&diffFailOnReg, "fail-on-regression", false, "Exit 1 if any regression or new-low exists")
	cmd.Flags().StringVarP(&diffFormat, "format", "f", "text", "Output format: text, json, markdown")
	cmd.Flags().StringVarP(&cfgFile, "config", "c", ".go-solid-score.yaml", "Config file path")
	_ = cmd.MarkFlagRequired("base")
	return cmd
}

// loadBaseline reads and decodes a baseline JSON file into snapshots.
func loadBaseline(path string) ([]differ.Snapshot, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading baseline: %w", err)
	}
	var doc formatter.JSONOutput
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parsing baseline JSON: %w", err)
	}
	snaps := make([]differ.Snapshot, 0, len(doc.Results))
	for _, r := range doc.Results {
		snaps = append(snaps, differ.Snapshot{
			ID: r.ID, Name: r.Name, Package: r.Package, Total: r.Total,
		})
	}
	return snaps, nil
}

// resultsToSnapshots projects scored results to diff snapshots.
func resultsToSnapshots(results []*scorer.ScoreResult) []differ.Snapshot {
	snaps := make([]differ.Snapshot, 0, len(results))
	for _, r := range results {
		snaps = append(snaps, differ.Snapshot{
			ID: r.TargetID(), Name: r.TargetName, Package: r.TargetPkg, Total: r.Total,
		})
	}
	return snaps
}

func runDiff(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	patterns := cfg.Paths
	if len(args) > 0 {
		patterns = args
	}

	base, err := loadBaseline(diffBase)
	if err != nil {
		return err
	}

	headResults, err := analyze(cfg, patterns)
	if err != nil {
		return err
	}
	head := resultsToSnapshots(headResults)

	report := differ.Diff(base, head, differ.Options{
		MaxDrop:  diffMaxDrop,
		MinScore: diffMinScore,
	})

	var out string
	switch diffFormat {
	case "json":
		out = formatter.FormatDiffJSON(report)
	case "markdown":
		out = formatter.FormatDiffMarkdown(report)
	case "text":
		out = formatter.FormatDiffText(report, diffBase, diffMinScore)
	default:
		return fmt.Errorf("unknown format %q (want text, json, or markdown)", diffFormat)
	}
	if _, err := fmt.Fprint(os.Stdout, out); err != nil {
		return fmt.Errorf("writing output: %w", err)
	}

	if diffFailOnReg && report.Regressed {
		return fmt.Errorf("regression detected")
	}
	return nil
}
