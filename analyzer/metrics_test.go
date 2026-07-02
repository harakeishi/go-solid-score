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
	// type_check_density, public_lcom4) across the principle testdata packages.
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

// structMetricsFor parses the given testdata package and returns the metrics
// for one named struct.
func structMetricsFor(t *testing.T, pkgPath, structName string) rules.Metrics {
	t.Helper()
	pkgs, err := parser.Parse([]string{pkgPath})
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	for _, pkg := range pkgs {
		for _, s := range pkg.Structs {
			if s.Name == structName {
				return analyzer.StructMetrics(s, pkg, nil)
			}
		}
	}
	t.Fatalf("struct %s not found in %s", structName, pkgPath)
	return nil
}

// TestIfaceParamCount_ExcludesEmptyInterface verifies that empty-interface
// parameters (any / interface{}) do not count toward the OCP
// interface-parameter bonus: Router's methods take only interface{} params —
// the very values its type switches downcast — so rewarding them would offset
// the penalty for the switches themselves.
func TestIfaceParamCount_ExcludesEmptyInterface(t *testing.T) {
	m := structMetricsFor(t, "../testdata/ocp", "Router")
	if got := m["iface_param_count"]; got != 0 {
		t.Errorf("Router iface_param_count = %v, want 0 (interface{} params must not earn the bonus)", got)
	}
}

// TestNoopCount_DetectsZeroValueReturns verifies that contract methods whose
// body only returns zero values (the silent no-op) are counted, not just empty
// bodies and bare returns.
func TestNoopCount_DetectsZeroValueReturns(t *testing.T) {
	m := structMetricsFor(t, "../testdata/lsp", "NoopSaver")
	if got := m["noop_count"]; got != 2 {
		t.Errorf("NoopSaver noop_count = %v, want 2 (Save/Load return only zero values)", got)
	}

	m = structMetricsFor(t, "../testdata/lsp", "GuardedSaver")
	if got := m["noop_count"]; got != 0 {
		t.Errorf("GuardedSaver noop_count = %v, want 0 (guarded methods do real work)", got)
	}
}
