// Package differ compares two sets of SOLID score snapshots (a baseline and a
// head) and classifies each target as regressed, improved, unchanged, new,
// new-low, or removed. The core Diff function is pure: it performs no I/O.
package differ

import (
	"math"
	"sort"
)

// Snapshot is the minimal projection of a scored target needed for diffing.
// Principles holds the per-principle scores (keyed "SRP", "OCP", ...) so the
// diff can explain which principle moved, not just the aggregate total. It may
// be nil for baselines that predate per-principle output.
type Snapshot struct {
	ID         string
	Name       string
	Package    string
	Total      float64
	Principles map[string]float64
}

// PrincipleDelta records the change in one principle's score between base and
// head for a target present in both.
type PrincipleDelta struct {
	Principle string
	Base      float64
	Head      float64
}

// Delta returns Head - Base for the principle.
func (d PrincipleDelta) Delta() float64 { return d.Head - d.Base }

// Status is the classification of a target between base and head.
type Status string

const (
	StatusRegressed Status = "REGRESSED"
	StatusImproved  Status = "IMPROVED"
	StatusUnchanged Status = "UNCHANGED"
	StatusNew       Status = "NEW"
	StatusNewLow    Status = "NEW-LOW"
	StatusRemoved   Status = "REMOVED"
)

// Entry is the diff result for a single target. Base/Head are nil when the
// target is absent from that side (NEW/NEW-LOW have no Base; REMOVED has no Head).
type Entry struct {
	ID      string
	Name    string
	Package string
	Status  Status
	Base    *float64
	Head    *float64
	// PrincipleDeltas lists the per-principle score changes for targets present
	// in both base and head. Only principles whose score changed are included,
	// sorted by principle name. Empty when a side lacks per-principle data.
	PrincipleDeltas []PrincipleDelta
}

// Diff returns the head total minus the base total. Only meaningful when both
// are present; callers guard via Status.
func (e *Entry) Diff() float64 {
	if e.Base == nil || e.Head == nil {
		return 0
	}
	return *e.Head - *e.Base
}

// Report is the full diff outcome.
type Report struct {
	Entries   []Entry
	Counts    map[Status]int
	Regressed bool // true if any REGRESSED or NEW-LOW exists
}

// Options tunes the classification thresholds.
type Options struct {
	MaxDrop  float64 // a total drop strictly greater than this is a regression
	MinScore float64 // a new target below this is NEW-LOW; 0 disables
	// NoiseFloor is the measurement noise floor: a change whose magnitude is at
	// or below it is treated as no change at all (UNCHANGED), in both
	// directions and for every per-principle delta. It is the minimum detectable
	// change of the instrument — below it, a score move is not a real signal but
	// measurement wobble (e.g. when comparing two scores produced by different
	// tool builds). 0 disables suppression and preserves the prior behavior
	// where any non-zero move is reported.
	//
	// NoiseFloor gates "is this change real at all"; MaxDrop then decides whether
	// a real drop is large enough to be a regression. The noise gate is applied
	// first, so a NoiseFloor larger than MaxDrop wins (a sub-floor move is never
	// a regression no matter how MaxDrop is set).
	NoiseFloor float64
}

// Diff compares base and head snapshots by ID and classifies each target.
// It is a pure function with no side effects.
func Diff(base, head []Snapshot, opts Options) Report {
	baseByID := make(map[string]Snapshot, len(base))
	for _, s := range base {
		baseByID[s.ID] = s
	}
	headByID := make(map[string]Snapshot, len(head))
	for _, s := range head {
		headByID[s.ID] = s
	}

	r := Report{Counts: make(map[Status]int)}

	for _, h := range head {
		hv := h.Total
		e := Entry{ID: h.ID, Name: h.Name, Package: h.Package, Head: &hv}
		if b, ok := baseByID[h.ID]; ok {
			bv := b.Total
			e.Base = &bv
			// Quantize the delta to 0.1 (scores are already rounded to one
			// decimal at scoring time) before comparing, so floating-point
			// representation noise can't tip a target across a boundary in
			// either direction. A positive delta is an improvement, negative a
			// drop.
			delta := math.Round((h.Total-b.Total)*10) / 10
			drop := -delta
			switch {
			case math.Abs(delta) <= opts.NoiseFloor:
				// Within the noise floor: not a real change in either direction.
				e.Status = StatusUnchanged
			case drop > opts.MaxDrop:
				e.Status = StatusRegressed
			case delta > 0:
				e.Status = StatusImproved
			default:
				// A real drop, but within the tolerated MaxDrop band.
				e.Status = StatusUnchanged
			}
			e.PrincipleDeltas = principleDeltas(b.Principles, h.Principles, opts.NoiseFloor)
		} else {
			if opts.MinScore > 0 && h.Total < opts.MinScore {
				e.Status = StatusNewLow
			} else {
				e.Status = StatusNew
			}
		}
		r.Entries = append(r.Entries, e)
		r.Counts[e.Status]++
	}

	for _, b := range base {
		if _, ok := headByID[b.ID]; ok {
			continue
		}
		bv := b.Total
		e := Entry{ID: b.ID, Name: b.Name, Package: b.Package, Base: &bv, Status: StatusRemoved}
		r.Entries = append(r.Entries, e)
		r.Counts[e.Status]++
	}

	r.Regressed = r.Counts[StatusRegressed] > 0 || r.Counts[StatusNewLow] > 0
	return r
}

// principleDeltas returns the per-principle changes between base and head,
// including only principles whose score moved by more than noiseFloor, sorted
// by principle name. Returns nil when either side lacks per-principle data, so
// callers can gracefully omit the breakdown for older baselines.
func principleDeltas(base, head map[string]float64, noiseFloor float64) []PrincipleDelta {
	if len(base) == 0 || len(head) == 0 {
		return nil
	}
	var deltas []PrincipleDelta
	for p, hv := range head {
		bv, ok := base[p]
		if !ok {
			continue
		}
		// Quantize to 0.1 like the total, then drop sub-noise-floor moves so a
		// breakdown never reports a change the top-line classification ignored.
		if math.Abs(math.Round((hv-bv)*10)/10) <= noiseFloor {
			continue
		}
		deltas = append(deltas, PrincipleDelta{Principle: p, Base: bv, Head: hv})
	}
	sort.Slice(deltas, func(i, j int) bool {
		return deltas[i].Principle < deltas[j].Principle
	})
	return deltas
}
