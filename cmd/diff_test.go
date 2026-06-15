package cmd

import (
	"io"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

// writeBaseline writes a minimal baseline JSON with one target at the given total.
func writeBaseline(t *testing.T, dir, id, pkg, name string, total float64) string {
	t.Helper()
	content := `{"results":[{"id":"` + id + `","name":"` + name +
		`","package":"` + pkg + `","file":"x.go","line":1,` +
		`"srp":0,"ocp":0,"lsp":0,"isp":0,"dip":0,"total":` +
		floatStr(total) + `,"confidence":{}}],"summary":{"total_structs":1,"average_score":` +
		floatStr(total) + `}}`
	path := filepath.Join(dir, "base.json")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func floatStr(f float64) string {
	return strconv.FormatFloat(f, 'f', 1, 64)
}

func TestLoadBaseline(t *testing.T) {
	dir := t.TempDir()
	path := writeBaseline(t, dir, "pkg.Foo", "pkg", "Foo", 72.0)
	snaps, err := loadBaseline(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(snaps) != 1 || snaps[0].ID != "pkg.Foo" || snaps[0].Total != 72.0 {
		t.Errorf("unexpected snapshots: %+v", snaps)
	}
}

func TestLoadBaseline_Missing(t *testing.T) {
	if _, err := loadBaseline("/no/such/file.json"); err == nil {
		t.Error("expected error for missing baseline")
	}
}

func TestLoadBaseline_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := loadBaseline(path); err == nil {
		t.Error("expected error for invalid baseline JSON")
	}
}

// runDiffWith invokes the diff command end-to-end against the given packages
// with a baseline file, returning the error (non-nil means exit code 1).
func runDiffWith(t *testing.T, basePath string, failOnReg bool, pkgs ...string) error {
	t.Helper()
	cmd := newDiffCmd()
	args := []string{"--base", basePath, "-f", "json"}
	if failOnReg {
		args = append(args, "--fail-on-regression")
	}
	args = append(args, pkgs...)
	cmd.SetArgs(args)
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	return cmd.Execute()
}

// TestDiff_FailOnRegression is an end-to-end check that the command exits with
// an error (exit 1) when a regression is detected under --fail-on-regression,
// and succeeds otherwise. The baseline pins every target at 100, so analyzing
// real code (which scores lower somewhere) guarantees a regression.
func TestDiff_FailOnRegression(t *testing.T) {
	dir := t.TempDir()
	// Baseline: a target that exists in the analyzed package, pinned high.
	// Use the differ package itself as the analysis target; differ.Report
	// scores below 100 on DIP, so it regresses against a 100 baseline.
	base := writeBaseline(t, dir, "github.com/harakeishi/go-solid-score/differ.Report",
		"github.com/harakeishi/go-solid-score/differ", "Report", 100.0)

	pkg := "github.com/harakeishi/go-solid-score/differ"

	// Without --fail-on-regression: reports but exits 0.
	if err := runDiffWith(t, base, false, pkg); err != nil {
		t.Errorf("without --fail-on-regression, expected nil error, got %v", err)
	}

	// With --fail-on-regression: a regression must produce a non-nil error.
	if err := runDiffWith(t, base, true, pkg); err == nil {
		t.Error("with --fail-on-regression and a regression, expected non-nil error (exit 1)")
	}
}
