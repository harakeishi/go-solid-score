package analyzer_test

import (
	"testing"

	"github.com/harakeishi/go-solid-score/analyzer"
	"github.com/harakeishi/go-solid-score/parser"
	"github.com/harakeishi/go-solid-score/rules"
)

// TestMetricNamesCoverEmitted verifies that every metric key StructMetrics and
// InterfaceMetrics actually emit is registered in MetricNames(). If a new
// metric is added to the computation but not to the vocabulary, the engine
// would reject user rules that reference it — this test fails first.
func TestMetricNamesCoverEmitted(t *testing.T) {
	known := make(map[string]bool)
	for _, n := range analyzer.MetricNames() {
		known[n] = true
	}

	// Exercise many branches (conditional metrics like iface_dep_ratio,
	// type_check_density, public_lcom4, srp_avg_component_size) across the
	// principle testdata packages.
	pkgs, err := parser.Parse([]string{
		"../testdata/srp", "../testdata/ocp", "../testdata/lsp",
		"../testdata/isp", "../testdata/dip",
	})
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	for _, pkg := range pkgs {
		for _, s := range pkg.Structs {
			for key := range analyzer.StructMetrics(s, pkg, nil) {
				if !known[key] {
					t.Errorf("StructMetrics emits %q which is not in MetricNames()", key)
				}
			}
		}
		for _, iface := range pkg.Interfaces {
			for key := range analyzer.InterfaceMetrics(iface) {
				if !known[key] {
					t.Errorf("InterfaceMetrics emits %q which is not in MetricNames()", key)
				}
			}
		}
	}
}

// TestPresetsValidateAgainstMetricNames ensures every metric the built-in
// preset rules reference exists in the vocabulary — guarding against a rename
// that leaves a preset pointing at a metric that is no longer computed.
func TestPresetsValidateAgainstMetricNames(t *testing.T) {
	if _, err := rules.NewEngine(rules.DefaultRuleSet(), analyzer.MetricNames()...); err != nil {
		t.Fatalf("preset rules reference unknown metric(s): %v", err)
	}
}
