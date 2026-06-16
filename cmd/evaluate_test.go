package cmd

import (
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/harakeishi/go-solid-score/eval"
)

// testdataPkgs is the explicit list of testdata packages. The go tool excludes
// directories named "testdata" from a "./testdata/..." glob, so the evaluate
// command (and these tests) must enumerate the packages by path. Real targets
// outside a testdata directory are matched by "./..." normally.
var testdataPkgs = []string{
	"github.com/harakeishi/go-solid-score/testdata/srp",
	"github.com/harakeishi/go-solid-score/testdata/ocp",
	"github.com/harakeishi/go-solid-score/testdata/lsp",
	"github.com/harakeishi/go-solid-score/testdata/isp",
	"github.com/harakeishi/go-solid-score/testdata/dip",
}

// runEvaluateCapture runs the evaluate command end-to-end against the given
// packages, capturing whatever it writes to os.Stdout (the command writes its
// report to stdout directly, not to cobra's out writer).
func runEvaluateCapture(t *testing.T, extraArgs []string, pkgs ...string) (string, error) {
	t.Helper()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	orig := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = orig }()

	cmd := newEvaluateCmd()
	args := append([]string{}, extraArgs...)
	args = append(args, pkgs...)
	cmd.SetArgs(args)
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	runErr := cmd.Execute()

	if cerr := w.Close(); cerr != nil {
		t.Fatal(cerr)
	}
	out, rerr := io.ReadAll(r)
	if rerr != nil {
		t.Fatal(rerr)
	}
	return string(out), runErr
}

// decodeReport runs evaluate with JSON output and decodes it.
func decodeReport(t *testing.T, pkgs ...string) eval.ReportJSON {
	t.Helper()
	out, err := runEvaluateCapture(t, []string{"-f", "json"}, pkgs...)
	if err != nil {
		t.Fatalf("evaluate failed: %v", err)
	}
	var rep eval.ReportJSON
	if err := json.Unmarshal([]byte(out), &rep); err != nil {
		t.Fatalf("evaluate JSON did not parse: %v\noutput:\n%s", err, out)
	}
	return rep
}

// TestEvaluate_ISPRecall checks that ISP recall captures the known true
// positive (FatImpl). This is the recall floor the harness must hold: a known
// fat interface must not silently slip below the measured violations.
func TestEvaluate_ISPRecall(t *testing.T) {
	rep := decodeReport(t, testdataPkgs...)
	isp, ok := rep.PerPrinciple["ISP"]
	if !ok {
		t.Fatal("ISP missing from report")
	}
	if isp.TP < 1 {
		t.Errorf("expected ISP to catch the known violation (FatImpl), got TP=%d", isp.TP)
	}
	if isp.Recall == nil || *isp.Recall < 1.0 {
		t.Errorf("expected ISP recall 1.0, got %v", isp.Recall)
	}
}

// TestEvaluate_NoFalsePositives checks that no principle flags a type labelled
// OK — i.e. the good/sound testdata types produce no false positives. A nonzero
// FP anywhere would mean the scorer penalized a type the ground truth says is
// fine.
func TestEvaluate_NoFalsePositives(t *testing.T) {
	rep := decodeReport(t, testdataPkgs...)
	for principle, pr := range rep.PerPrinciple {
		if pr.FP != 0 {
			t.Errorf("principle %s produced %d false positive(s); good types must not be flagged", principle, pr.FP)
		}
	}
}

// TestEvaluate_KnownFalseNegatives is the core Phase 1 assertion: the harness
// must make the documented misses visible as false negatives rather than hiding
// them. OCP.Router and LSP.{ReadOnlySaver,NoopSaver} score above threshold today
// (handoff §2), so they are real violations the scorer fails to flag. The point
// of the measurement basis is that these show up as FN, not vanish.
func TestEvaluate_KnownFalseNegatives(t *testing.T) {
	rep := decodeReport(t, testdataPkgs...)

	ocp, ok := rep.PerPrinciple["OCP"]
	if !ok {
		t.Fatal("OCP missing from report")
	}
	if ocp.FN < 1 {
		t.Errorf("expected OCP to surface the known miss (Router) as a false negative, got FN=%d", ocp.FN)
	}

	lsp, ok := rep.PerPrinciple["LSP"]
	if !ok {
		t.Fatal("LSP missing from report")
	}
	if lsp.FN < 2 {
		t.Errorf("expected LSP to surface the known misses (ReadOnlySaver, NoopSaver) as false negatives, got FN=%d", lsp.FN)
	}
}

// TestEvaluate_RecallDenominatorReported checks that the recall denominator
// (the count of known true violations: TP+FN) is exposed per principle, so a
// report states how many real violations the measurement rests on.
func TestEvaluate_RecallDenominatorReported(t *testing.T) {
	rep := decodeReport(t, testdataPkgs...)
	for principle, pr := range rep.PerPrinciple {
		if pr.RecallDenominator != pr.TP+pr.FN {
			t.Errorf("principle %s: recall_denominator=%d but TP+FN=%d", principle, pr.RecallDenominator, pr.TP+pr.FN)
		}
	}
	// At least one principle must have a nonzero denominator, otherwise the
	// harness measured nothing.
	total := 0
	for _, pr := range rep.PerPrinciple {
		total += pr.RecallDenominator
	}
	if total == 0 {
		t.Error("no known violations were measured; the label/score join produced an empty matrix")
	}
}

// TestEvaluate_TextFormat checks the default text output renders the table
// header and at least one principle row.
func TestEvaluate_TextFormat(t *testing.T) {
	out, err := runEvaluateCapture(t, nil, testdataPkgs...)
	if err != nil {
		t.Fatalf("evaluate failed: %v", err)
	}
	if !strings.Contains(out, "go-solid-score evaluate") {
		t.Errorf("text output missing header, got:\n%s", out)
	}
	if !strings.Contains(out, "recall") {
		t.Errorf("text output missing column labels, got:\n%s", out)
	}
}

// TestEvaluate_UnknownFormat rejects an unrecognized -f value before analysis.
func TestEvaluate_UnknownFormat(t *testing.T) {
	cmd := newEvaluateCmd()
	cmd.SetArgs([]string{"-f", "xml", testdataPkgs[0]})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "unknown format") {
		t.Errorf("expected unknown-format error, got: %v", err)
	}
}

// TestEvaluate_UnknownSplit rejects an unrecognized --split value.
func TestEvaluate_UnknownSplit(t *testing.T) {
	cmd := newEvaluateCmd()
	cmd.SetArgs([]string{"--split", "holdout", testdataPkgs[0]})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "unknown split") {
		t.Errorf("expected unknown-split error, got: %v", err)
	}
}
