package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

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
	baseline  string
	failOnReg bool
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
	cmd.Flags().StringVar(&f.baseline, "baseline", "", "Committed baseline report JSON to check for regressions against")
	cmd.Flags().BoolVar(&f.failOnReg, "fail-on-regression", false, "Exit 1 if any principle regressed below --baseline (recall floor or new false positive)")
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

	// Guard against a hollow evaluation: if no principle was measured, the
	// label/score join produced nothing (e.g. patterns matched no annotated
	// packages, or shell quoting collapsed several patterns into one). Returning
	// an empty report here would let a regression gate pass green having checked
	// nothing — the worst failure mode for an accuracy harness — so fail loudly.
	if len(report.PerPrinciple) == 0 {
		return fmt.Errorf("no labels were measured for patterns %v; "+
			"the packages matched no `// solid:want` labels (note: the go tool "+
			"excludes `testdata` from `./...` globs — list those packages explicitly)", patterns)
	}

	out, err := formatEvaluation(report, f.format)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprint(os.Stdout, out); err != nil {
		return fmt.Errorf("writing output: %w", err)
	}

	// Baseline regression check. A detected regression is a normal CI outcome,
	// not a usage error, so SilenceUsage keeps the help text from printing on a
	// non-nil return.
	if f.baseline != "" {
		regs, err := checkRegressions(report, f.baseline)
		if err != nil {
			return err
		}
		if len(regs) > 0 {
			printRegressions(regs)
			if f.failOnReg {
				return fmt.Errorf("%d principle regression(s) against baseline %s", len(regs), f.baseline)
			}
		}
	}
	return nil
}

// checkRegressions loads the committed baseline and compares the current report
// against it.
func checkRegressions(report eval.Report, baselinePath string) ([]eval.Regression, error) {
	data, err := os.ReadFile(baselinePath)
	if err != nil {
		return nil, fmt.Errorf("reading baseline: %w", err)
	}
	base, err := eval.LoadBaselineJSON(data)
	if err != nil {
		return nil, err
	}
	return eval.CompareToBaseline(eval.NewReportJSON(report), base), nil
}

// printRegressions writes the regressions to stderr so they are visible in CI
// logs without polluting the report on stdout.
func printRegressions(regs []eval.Regression) {
	fmt.Fprintln(os.Stderr, "accuracy regression(s) against baseline:")
	for _, r := range regs {
		fmt.Fprintf(os.Stderr, "  [%s] %s: %s\n", r.Kind, r.Principle, r.Detail)
	}
}

// buildEvaluation parses the patterns once, then derives both the scores and
// the inline labels from that single parse — so the labels and the scores are
// guaranteed to describe the same source and the (heavy) go/packages build runs
// once, not twice. External YAML labels, if any, are appended.
func buildEvaluation(cfg *config.Config, patterns []string, f *evaluateFlags, split eval.Split) (eval.Report, error) {
	pkgs, err := parsePackages(patterns)
	if err != nil {
		return eval.Report{}, err
	}

	results, err := scorePackages(cfg, pkgs)
	if err != nil {
		return eval.Report{}, err
	}
	scored := eval.CollectScores(results)

	labels, err := eval.CollectDocLabels(pkgs)
	if err != nil {
		return eval.Report{}, fmt.Errorf("collecting inline labels: %w", err)
	}
	if f.labels != "" {
		ext, err := externalLabels(f.labels)
		if err != nil {
			return eval.Report{}, err
		}
		// An external label whose ID matches no analyzed target would silently
		// leave the join — and with it the confusion matrix. For TP/FN labels the
		// baseline's recall-denominator check would eventually notice, but ok
		// labels (TN/FP rows) would just vanish, and a vanished FP row keeps the
		// baseline's FP count as a high-water mark that masks the next new false
		// positive. Corpus drift (a version bump renaming or removing a labelled
		// type) must therefore be loud, not silent.
		if missing := unmatchedLabelIDs(ext, scored); len(missing) > 0 {
			return eval.Report{}, fmt.Errorf(
				"label file %s: %d label ID(s) matched no analyzed target "+
					"(type renamed/removed by a corpus version bump, or outside the given patterns?): %s",
				f.labels, len(missing), strings.Join(missing, ", "))
		}
		labels = append(labels, ext...)
	}

	thresholds := principleThresholds(cfg)

	return eval.BuildReport(labels, scored, thresholds, split, f.bootstrap, f.seed), nil
}

// unmatchedLabelIDs returns the sorted, deduplicated IDs of external labels
// that join to no scored target. Inline labels are exempt from this check by
// construction (they are parsed out of the analyzed source itself); external
// labels have no such anchor, so an unmatched ID is evidence of corpus drift.
func unmatchedLabelIDs(ext []eval.Label, scored map[string]map[analyzer.Principle]float64) []string {
	seen := make(map[string]bool)
	var missing []string
	for _, l := range ext {
		if _, ok := scored[l.ID]; ok || seen[l.ID] {
			continue
		}
		seen[l.ID] = true
		missing = append(missing, l.ID)
	}
	sort.Strings(missing)
	return missing
}

// externalLabels reads ground-truth labels from an external YAML file (for
// corpora that cannot be annotated inline).
func externalLabels(yamlPath string) ([]eval.Label, error) {
	data, err := os.ReadFile(yamlPath)
	if err != nil {
		return nil, fmt.Errorf("reading label file: %w", err)
	}
	ext, err := eval.ParseYAMLLabels(data)
	if err != nil {
		return nil, fmt.Errorf("parsing label file %s: %w", yamlPath, err)
	}
	return ext, nil
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
