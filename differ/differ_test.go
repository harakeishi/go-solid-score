package differ_test

import (
	"testing"

	"github.com/harakeishi/go-solid-score/differ"
)

func snap(id string, total float64) differ.Snapshot {
	return differ.Snapshot{ID: id, Name: id, Package: "pkg", Total: total}
}

func statusOf(r differ.Report, id string) differ.Status {
	for _, e := range r.Entries {
		if e.ID == id {
			return e.Status
		}
	}
	return ""
}

func TestDiff_Classifications(t *testing.T) {
	base := []differ.Snapshot{
		snap("pkg.Reg", 72),
		snap("pkg.Imp", 60),
		snap("pkg.Same", 80),
		snap("pkg.SmallDrop", 80),
		snap("pkg.Removed", 90),
	}
	head := []differ.Snapshot{
		snap("pkg.Reg", 58),
		snap("pkg.Imp", 80),
		snap("pkg.Same", 80),
		snap("pkg.SmallDrop", 77),
		snap("pkg.NewLow", 45),
		snap("pkg.NewOk", 90),
	}

	r := differ.Diff(base, head, differ.Options{MaxDrop: 5, MinScore: 70})

	cases := map[string]differ.Status{
		"pkg.Reg":       differ.StatusRegressed,
		"pkg.Imp":       differ.StatusImproved,
		"pkg.Same":      differ.StatusUnchanged,
		"pkg.SmallDrop": differ.StatusUnchanged,
		"pkg.Removed":   differ.StatusRemoved,
		"pkg.NewLow":    differ.StatusNewLow,
		"pkg.NewOk":     differ.StatusNew,
	}
	for id, want := range cases {
		if got := statusOf(r, id); got != want {
			t.Errorf("%s: got %q, want %q", id, got, want)
		}
	}
	if !r.Regressed {
		t.Error("expected Regressed=true (has REGRESSED and NEW-LOW)")
	}
	if r.Counts[differ.StatusRegressed] != 1 {
		t.Errorf("regressed count: got %d, want 1", r.Counts[differ.StatusRegressed])
	}
}

func TestDiff_MaxDropBoundary(t *testing.T) {
	base := []differ.Snapshot{snap("pkg.A", 80), snap("pkg.B", 80)}
	head := []differ.Snapshot{snap("pkg.A", 75), snap("pkg.B", 74)}

	r := differ.Diff(base, head, differ.Options{MaxDrop: 5})

	if got := statusOf(r, "pkg.A"); got != differ.StatusUnchanged {
		t.Errorf("drop == maxDrop must be UNCHANGED, got %q", got)
	}
	if got := statusOf(r, "pkg.B"); got != differ.StatusRegressed {
		t.Errorf("drop just over maxDrop must be REGRESSED, got %q", got)
	}
}

func TestDiff_MinScoreDisabled(t *testing.T) {
	head := []differ.Snapshot{snap("pkg.New", 10)}
	r := differ.Diff(nil, head, differ.Options{MaxDrop: 5, MinScore: 0})
	if got := statusOf(r, "pkg.New"); got != differ.StatusNew {
		t.Errorf("minScore=0 must yield NEW (not NEW-LOW), got %q", got)
	}
	if r.Regressed {
		t.Error("a lone NEW must not mark the report regressed")
	}
}
