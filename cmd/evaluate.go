package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/harakeishi/go-solid-score/analyzer"
	"github.com/harakeishi/go-solid-score/config"
	"github.com/harakeishi/go-solid-score/eval"
	"github.com/spf13/cobra"
)

// defaultBootstrapSeed fixes the bootstrap resampling so the reported
// confidence intervals are reproducible run-to-run (and testable).
const defaultBootstrapSeed = 42

// evaluateFlags holds the parsed flags for one invocation of the evaluate
// command, kept in a struct (not package globals) so instances don't share
// mutable state — matching the diff command's pattern.
type evaluateFlags struct {
	labels    string
	split     string
	format    string
	bootstrap int
	seed      int64
	cfgFile   string
}

func newEvaluateCmd() *cobra.Command {
	var f evaluateFlags

	cmd := &cobra.Command{
		Use:   "evaluate [packages...]",
		Short: "Measure SOLID-scoring precision/recall against ground-truth labels",
		Long: "evaluate scores the given packages, joins each target to its " +
			"ground-truth `// solid:want` labels (and optionally an external " +
			"YAML label file), and reports per-principle precision, recall and " +
			"F1 with bootstrap confidence intervals. recall's denominator (the " +
			"count of known true violations) is shown so the measurement's " +
			"basis is explicit.",
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runEvaluate(&f, args)
		},
		SilenceUsage: true,
	}
	cmd.Flags().StringVar(&f.labels, "labels", "", "External YAML label file (optional; for corpora that cannot be annotated inline)")
	cmd.Flags().StringVar(&f.split, "split", string(eval.SplitTest), "Label split to evaluate: test, train")
	cmd.Flags().StringVarP(&f.format, "format", "f", "text", "Output format: text, json")
	cmd.Flags().IntVar(&f.bootstrap, "bootstrap", 1000, "Bootstrap resamples for the F1 confidence interval")
	cmd.Flags().Int64Var(&f.seed, "seed", defaultBootstrapSeed, "Bootstrap RNG seed (fixed for reproducibility)")
	cmd.Flags().StringVarP(&f.cfgFile, "config", "c", ".go-solid-score.yaml", "Config file path")
	return cmd
}

func runEvaluate(f *evaluateFlags, args []string) error {
	// Validate up front so a bad format/split fails before any analysis work.
	switch f.format {
	case "text", "json":
	default:
		return fmt.Errorf("unknown format %q (want text or json)", f.format)
	}
	split := eval.Split(f.split)
	switch split {
	case eval.SplitTest, eval.SplitTrain:
	default:
		return fmt.Errorf("unknown split %q (want test or train)", f.split)
	}

	cfg, err := config.Load(f.cfgFile)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	patterns := cfg.Paths
	if len(args) > 0 {
		patterns = args
	}

	// Parse once and reuse the packages for both scoring and label collection,
	// so the inline labels and the scores are guaranteed to describe the same
	// source.
	report, err := buildEvaluation(cfg, patterns, f, split)
	if err != nil {
		return err
	}

	out, err := formatEvaluation(report, f.format)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprint(os.Stdout, out); err != nil {
		return fmt.Errorf("writing output: %w", err)
	}
	return nil
}

// buildEvaluation parses, scores, collects labels (inline + optional YAML) and
// joins them into an eval.Report over the chosen split.
func buildEvaluation(cfg *config.Config, patterns []string, f *evaluateFlags, split eval.Split) (eval.Report, error) {
	results, err := analyze(cfg, patterns)
	if err != nil {
		return eval.Report{}, err
	}
	scored := eval.CollectScores(results)

	labels, err := collectLabels(cfg, patterns, f.labels)
	if err != nil {
		return eval.Report{}, err
	}

	thresholds := principleThresholds(cfg)

	return eval.BuildReport(labels, scored, thresholds, split, f.bootstrap, f.seed), nil
}

// collectLabels gathers the inline `solid:want` labels from the source under
// the given patterns and appends any labels from an external YAML file.
func collectLabels(cfg *config.Config, patterns []string, yamlPath string) ([]eval.Label, error) {
	pkgs, err := parsePackages(patterns)
	if err != nil {
		return nil, err
	}
	labels, err := eval.CollectDocLabels(pkgs)
	if err != nil {
		return nil, fmt.Errorf("collecting inline labels: %w", err)
	}
	if yamlPath != "" {
		data, err := os.ReadFile(yamlPath)
		if err != nil {
			return nil, fmt.Errorf("reading label file: %w", err)
		}
		ext, err := eval.ParseYAMLLabels(data)
		if err != nil {
			return nil, fmt.Errorf("parsing label file %s: %w", yamlPath, err)
		}
		labels = append(labels, ext...)
	}
	return labels, nil
}

// principleThresholds converts the config's string-keyed thresholds into the
// principle-keyed map the evaluation uses. Per the accuracy-improvement design
// (§2.2), this is the first legitimate place evaluation consults Thresholds:
// the threshold defines what counts as "flagged as a violation". A principle
// absent from the config simply gets no entry, and labels for it are skipped at
// join time.
func principleThresholds(cfg *config.Config) map[analyzer.Principle]float64 {
	out := make(map[analyzer.Principle]float64)
	for _, p := range []analyzer.Principle{analyzer.SRP, analyzer.OCP, analyzer.LSP, analyzer.ISP, analyzer.DIP} {
		if v, ok := cfg.Thresholds[string(p)]; ok {
			out[p] = v
		}
	}
	return out
}

// formatEvaluation renders a report as text or as JSON.
func formatEvaluation(report eval.Report, format string) (string, error) {
	if format == "json" {
		data, err := json.MarshalIndent(eval.NewReportJSON(report), "", "  ")
		if err != nil {
			return "", fmt.Errorf("encoding report JSON: %w", err)
		}
		return string(data) + "\n", nil
	}
	return report.FormatText(), nil
}
