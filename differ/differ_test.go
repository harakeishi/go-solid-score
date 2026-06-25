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

func entryOf(r differ.Report, id string) *differ.Entry {
	for i := range r.Entries {
		if r.Entries[i].ID == id {
			return &r.Entries[i]
		}
	}
	return nil
}

// TestDiff_NoPrincipleDeltasWhenSideMissing verifies the backward-compat
// degradation: if either side lacks per-principle data (e.g. a legacy baseline
// with no srp/ocp/... keys), no breakdown is produced rather than diffing
// against phantom zero scores.
func TestDiff_NoPrincipleDeltasWhenSideMissing(t *testing.T) {
	// base has no per-principle data; head does.
	base := []differ.Snapshot{{ID: "pkg.Svc", Name: "Svc", Package: "pkg", Total: 100}}
	head := []differ.Snapshot{{
		ID: "pkg.Svc", Name: "Svc", Package: "pkg", Total: 75,
		Principles: map[string]float64{"SRP": 60, "OCP": 100},
	}}

	r := differ.Diff(base, head, differ.Options{MaxDrop: 5})
	e := entryOf(r, "pkg.Svc")
	if e == nil {
		t.Fatal("pkg.Svc not found")
	}
	if e.Status != differ.StatusRegressed {
		t.Errorf("expected REGRESSED on total, got %q", e.Status)
	}
	if len(e.PrincipleDeltas) != 0 {
		t.Errorf("legacy baseline must yield no principle deltas, got %+v", e.PrincipleDeltas)
	}
}

// TestDiff_PrincipleDeltas verifies that, for a target present in both base and
// head, the per-principle changes are computed so callers can show *what* moved
// (not just the total). Only principles that actually changed are reported,
// sorted by principle name.
func TestDiff_PrincipleDeltas(t *testing.T) {
	base := []differ.Snapshot{{
		ID: "pkg.Svc", Name: "Svc", Package: "pkg", Total: 75,
		Principles: map[string]float64{"SRP": 60, "OCP": 100, "DIP": 50},
	}}
	head := []differ.Snapshot{{
		ID: "pkg.Svc", Name: "Svc", Package: "pkg", Total: 55,
		Principles: map[string]float64{"SRP": 55, "OCP": 50, "DIP": 50}, // OCP -50, SRP -5, DIP same
	}}

	r := differ.Diff(base, head, differ.Options{MaxDrop: 5})
	e := entryOf(r, "pkg.Svc")
	if e == nil {
		t.Fatal("pkg.Svc not found")
	}

	// Expect only changed principles (DIP unchanged is excluded), sorted by name.
	want := []differ.PrincipleDelta{
		{Principle: "OCP", Base: 100, Head: 50},
		{Principle: "SRP", Base: 60, Head: 55},
	}
	if len(e.PrincipleDeltas) != len(want) {
		t.Fatalf("got %d principle deltas, want %d: %+v", len(e.PrincipleDeltas), len(want), e.PrincipleDeltas)
	}
	for i, w := range want {
		got := e.PrincipleDeltas[i]
		if got.Principle != w.Principle || got.Base != w.Base || got.Head != w.Head {
			t.Errorf("delta[%d]: got %+v, want %+v", i, got, w)
		}
	}
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

func TestDiff_MaxDropFloatNoise(t *testing.T) {
	// 80.0 - 79.7 == 0.30000000000000004 in float64. With maxDrop 0.3 the raw
	// comparison 0.3000...4 > 0.3 would wrongly flag a regression; quantizing
	// the drop to 0.1 keeps it UNCHANGED.
	base := []differ.Snapshot{snap("pkg.A", 80.0)}
	head := []differ.Snapshot{snap("pkg.A", 79.7)}

	r := differ.Diff(base, head, differ.Options{MaxDrop: 0.3})
	if got := statusOf(r, "pkg.A"); got != differ.StatusUnchanged {
		t.Errorf("float-noise drop of exactly maxDrop must be UNCHANGED, got %q", got)
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

// snapP builds a snapshot with per-principle scores for noise-floor tests.
func snapP(id string, total float64, principles map[string]float64) differ.Snapshot {
	return differ.Snapshot{ID: id, Name: id, Package: "pkg", Total: total, Principles: principles}
}

// TestDiff_NoiseFloorSuppressesSubFloorDrop: a total drop at or below the noise
// floor is UNCHANGED, not REGRESSED, even when MaxDrop would otherwise flag it.
func TestDiff_NoiseFloorSuppressesSubFloorDrop(t *testing.T) {
	base := []differ.Snapshot{snap("pkg.A", 80.0)}
	head := []differ.Snapshot{snap("pkg.A", 79.6)} // 0.4 drop

	// With MaxDrop=0 the 0.4 drop would be REGRESSED; a 0.5 noise floor absorbs it.
	r := differ.Diff(base, head, differ.Options{MaxDrop: 0, NoiseFloor: 0.5})
	if got := statusOf(r, "pkg.A"); got != differ.StatusUnchanged {
		t.Errorf("sub-floor drop should be UNCHANGED, got %q", got)
	}
	if r.Regressed {
		t.Error("report should not be marked regressed for a sub-floor drop")
	}
}

// TestDiff_NoiseFloorSuppressesSubFloorImprovement: improvements are gated
// symmetrically — a tiny gain within the floor is not reported as IMPROVED.
func TestDiff_NoiseFloorSuppressesSubFloorImprovement(t *testing.T) {
	base := []differ.Snapshot{snap("pkg.A", 80.0)}
	head := []differ.Snapshot{snap("pkg.A", 80.4)} // 0.4 gain

	r := differ.Diff(base, head, differ.Options{NoiseFloor: 0.5})
	if got := statusOf(r, "pkg.A"); got != differ.StatusUnchanged {
		t.Errorf("sub-floor improvement should be UNCHANGED, got %q", got)
	}
}

// TestDiff_NoiseFloorStillFlagsRealMove: a move beyond the floor is classified
// as usual — the floor only suppresses sub-floor wobble.
func TestDiff_NoiseFloorStillFlagsRealMove(t *testing.T) {
	base := []differ.Snapshot{snap("pkg.A", 80.0)}
	head := []differ.Snapshot{snap("pkg.A", 73.0)} // 7.0 drop > floor and > MaxDrop

	r := differ.Diff(base, head, differ.Options{MaxDrop: 5, NoiseFloor: 0.5})
	if got := statusOf(r, "pkg.A"); got != differ.StatusRegressed {
		t.Errorf("supra-floor drop beyond MaxDrop should be REGRESSED, got %q", got)
	}
}

// TestDiff_NoiseFloorDominatesMaxDrop: when the floor exceeds MaxDrop, the floor
// wins — a sub-floor move is never a regression regardless of MaxDrop.
func TestDiff_NoiseFloorDominatesMaxDrop(t *testing.T) {
	base := []differ.Snapshot{snap("pkg.A", 80.0)}
	head := []differ.Snapshot{snap("pkg.A", 78.5)} // 1.5 drop

	r := differ.Diff(base, head, differ.Options{MaxDrop: 1.0, NoiseFloor: 2.0})
	if got := statusOf(r, "pkg.A"); got != differ.StatusUnchanged {
		t.Errorf("floor (2.0) should dominate MaxDrop (1.0): want UNCHANGED, got %q", got)
	}
}

// TestDiff_NoiseFloorFiltersPrincipleDeltas: a per-principle move within the
// floor is omitted from the breakdown, while a supra-floor one is kept.
func TestDiff_NoiseFloorFiltersPrincipleDeltas(t *testing.T) {
	base := []differ.Snapshot{snapP("pkg.A", 80.0, map[string]float64{"SRP": 60, "DIP": 90})}
	head := []differ.Snapshot{snapP("pkg.A", 74.0, map[string]float64{"SRP": 59.6, "DIP": 80})}

	r := differ.Diff(base, head, differ.Options{MaxDrop: 5, NoiseFloor: 0.5})
	e := entryOf(r, "pkg.A")
	if e == nil {
		t.Fatal("pkg.A not found")
	}
	if len(e.PrincipleDeltas) != 1 || e.PrincipleDeltas[0].Principle != "DIP" {
		t.Errorf("expected only the DIP delta to survive the floor, got %+v", e.PrincipleDeltas)
	}
}

// TestDiff_ZeroNoiseFloorIsBackwardCompatible: with the floor disabled (0), any
// non-zero move is reported exactly as before.
func TestDiff_ZeroNoiseFloorIsBackwardCompatible(t *testing.T) {
	base := []differ.Snapshot{snap("pkg.A", 80.0)}
	head := []differ.Snapshot{snap("pkg.A", 79.9)} // 0.1 drop

	r := differ.Diff(base, head, differ.Options{MaxDrop: 0, NoiseFloor: 0})
	if got := statusOf(r, "pkg.A"); got != differ.StatusRegressed {
		t.Errorf("with floor disabled, a 0.1 drop past MaxDrop=0 is REGRESSED, got %q", got)
	}
}
