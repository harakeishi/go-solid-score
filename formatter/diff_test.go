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
			{ID: "pkg.Reg", Name: "Reg", Package: "pkg", Status: differ.StatusRegressed, Base: f(72), Head: f(58),
				PrincipleDeltas: []differ.PrincipleDelta{
					{Principle: "OCP", Base: 100, Head: 50},
					{Principle: "SRP", Base: 60, Head: 55},
				}},
			{ID: "pkg.New", Name: "New", Package: "pkg", Status: differ.StatusNewLow, Head: f(45)},
			{ID: "pkg.Same", Name: "Same", Package: "pkg", Status: differ.StatusUnchanged, Base: f(80), Head: f(80)},
		},
		Counts:    map[differ.Status]int{differ.StatusRegressed: 1, differ.StatusNewLow: 1, differ.StatusUnchanged: 1},
		Regressed: true,
	}
}

func TestFormatDiffText(t *testing.T) {
	out := formatter.FormatDiffText(sampleReport(), "base.json", 70)
	if !strings.Contains(out, "REGRESSED") || !strings.Contains(out, "pkg.Reg") {
		t.Errorf("missing regressed line:\n%s", out)
	}
	if !strings.Contains(out, "-14.0") {
		t.Errorf("missing diff value:\n%s", out)
	}
	if !strings.Contains(out, "1 regressed") {
		t.Errorf("missing summary counts:\n%s", out)
	}
	// NEW-LOW must show the violated floor.
	if !strings.Contains(out, "< min 70.0") {
		t.Errorf("NEW-LOW line should show the min threshold:\n%s", out)
	}
	// UNCHANGED should not be listed individually.
	if strings.Contains(out, "UNCHANGED  pkg.Same") {
		t.Errorf("UNCHANGED should be summarized, not listed:\n%s", out)
	}
	// The regressed line must explain which principles moved.
	if !strings.Contains(out, "OCP 100.0->50.0") {
		t.Errorf("regressed line should show the per-principle breakdown:\n%s", out)
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
	// The regressed row must explain which principles moved.
	if !strings.Contains(out, "OCP") || !strings.Contains(out, "100.0") {
		t.Errorf("regressed row should show the per-principle breakdown:\n%s", out)
	}
}

func TestFormatDiffJSON(t *testing.T) {
	out := formatter.FormatDiffJSON(sampleReport())
	type principle struct {
		Principle string  `json:"principle"`
		Base      float64 `json:"base"`
		Head      float64 `json:"head"`
		Diff      float64 `json:"diff"`
	}
	var parsed struct {
		Results []struct {
			ID         string      `json:"id"`
			Status     string      `json:"status"`
			Diff       float64     `json:"diff"`
			Principles []principle `json:"principles"`
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
	var foundReg, foundNewLow bool
	for _, r := range parsed.Results {
		switch r.ID {
		case "pkg.Reg":
			foundReg = true
			if r.Status != "REGRESSED" || r.Diff != -14 {
				t.Errorf("pkg.Reg: status=%s diff=%v", r.Status, r.Diff)
			}
			// The per-principle breakdown must be present and correct.
			want := []principle{
				{Principle: "OCP", Base: 100, Head: 50, Diff: -50},
				{Principle: "SRP", Base: 60, Head: 55, Diff: -5},
			}
			if len(r.Principles) != len(want) {
				t.Fatalf("pkg.Reg principles: got %d, want %d: %+v", len(r.Principles), len(want), r.Principles)
			}
			for i, w := range want {
				if r.Principles[i] != w {
					t.Errorf("pkg.Reg principles[%d]: got %+v, want %+v", i, r.Principles[i], w)
				}
			}
		case "pkg.New":
			foundNewLow = true
			// NEW-LOW has no base, so no breakdown — omitempty drops the key.
			if len(r.Principles) != 0 {
				t.Errorf("pkg.New should have no principles, got %+v", r.Principles)
			}
		}
	}
	if !foundReg {
		t.Error("pkg.Reg not in JSON results")
	}
	if !foundNewLow {
		t.Error("pkg.New not in JSON results")
	}
}

// TestFormatDiffJSON_EmptyResultsIsArray guards that an empty diff emits
// "results": [] (not null), matching the main JSONFormatter contract so
// consumers can always iterate the array.
func TestFormatDiffJSON_EmptyResultsIsArray(t *testing.T) {
	out := formatter.FormatDiffJSON(differ.Report{Counts: map[differ.Status]int{}})
	if !strings.Contains(out, `"results": []`) {
		t.Errorf("empty diff should emit results: [], got:\n%s", out)
	}
}
