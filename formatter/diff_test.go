package formatter_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/harakeishi/go-solid-score/differ"
	"github.com/harakeishi/go-solid-score/formatter"
)

func sampleReport() differ.Report {
	f := func(v float64) *float64 { return &v }
	return differ.Report{
		Entries: []differ.Entry{
			{ID: "pkg.Reg", Name: "Reg", Package: "pkg", Status: differ.StatusRegressed, Base: f(72), Head: f(58)},
			{ID: "pkg.New", Name: "New", Package: "pkg", Status: differ.StatusNewLow, Head: f(45)},
			{ID: "pkg.Same", Name: "Same", Package: "pkg", Status: differ.StatusUnchanged, Base: f(80), Head: f(80)},
		},
		Counts:    map[differ.Status]int{differ.StatusRegressed: 1, differ.StatusNewLow: 1, differ.StatusUnchanged: 1},
		Regressed: true,
	}
}

func TestFormatDiffText(t *testing.T) {
	out := formatter.FormatDiffText(sampleReport(), "base.json")
	if !strings.Contains(out, "REGRESSED") || !strings.Contains(out, "pkg.Reg") {
		t.Errorf("missing regressed line:\n%s", out)
	}
	if !strings.Contains(out, "-14.0") {
		t.Errorf("missing diff value:\n%s", out)
	}
	if !strings.Contains(out, "1 regressed") {
		t.Errorf("missing summary counts:\n%s", out)
	}
	// UNCHANGED should not be listed individually.
	if strings.Contains(out, "UNCHANGED  pkg.Same") {
		t.Errorf("UNCHANGED should be summarized, not listed:\n%s", out)
	}
}

func TestFormatDiffMarkdown(t *testing.T) {
	out := formatter.FormatDiffMarkdown(sampleReport())
	if !strings.Contains(out, "<!-- go-solid-score-diff -->") {
		t.Errorf("missing marker comment:\n%s", out)
	}
	if !strings.Contains(out, "REGRESSED") || !strings.Contains(out, "`pkg.Reg`") {
		t.Errorf("missing regressed row:\n%s", out)
	}
	if !strings.Contains(out, "<details>") {
		t.Errorf("missing details fold:\n%s", out)
	}
}

func TestFormatDiffJSON(t *testing.T) {
	out := formatter.FormatDiffJSON(sampleReport())
	var parsed struct {
		Results []struct {
			ID     string  `json:"id"`
			Status string  `json:"status"`
			Diff   float64 `json:"diff"`
		} `json:"results"`
		Summary struct {
			Regressed bool `json:"regressed"`
		} `json:"summary"`
	}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if !parsed.Summary.Regressed {
		t.Error("expected regressed=true in summary")
	}
	var foundReg bool
	for _, r := range parsed.Results {
		if r.ID == "pkg.Reg" {
			foundReg = true
			if r.Status != "REGRESSED" || r.Diff != -14 {
				t.Errorf("pkg.Reg: status=%s diff=%v", r.Status, r.Diff)
			}
		}
	}
	if !foundReg {
		t.Error("pkg.Reg not in JSON results")
	}
}
