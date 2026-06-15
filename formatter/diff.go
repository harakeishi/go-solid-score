package formatter

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/harakeishi/go-solid-score/differ"
)

// diffMarker is the leading HTML comment used by CI to find and update the
// previous PR comment.
const diffMarker = "<!-- go-solid-score-diff -->"

// statusOrder controls the display order of statuses in diff output.
var statusOrder = []differ.Status{
	differ.StatusRegressed, differ.StatusNewLow, differ.StatusImproved,
	differ.StatusNew, differ.StatusRemoved, differ.StatusUnchanged,
}

// statusEmoji maps a status to its Markdown marker.
var statusEmoji = map[differ.Status]string{
	differ.StatusRegressed: "🔻",
	differ.StatusNewLow:    "⚠️",
	differ.StatusImproved:  "🔺",
	differ.StatusNew:       "✨",
	differ.StatusRemoved:   "🗑",
	differ.StatusUnchanged: "▫️",
}

// sortedEntries returns entries grouped by statusOrder, then by ID.
func sortedEntries(r differ.Report) []differ.Entry {
	es := make([]differ.Entry, len(r.Entries))
	copy(es, r.Entries)
	rank := make(map[differ.Status]int, len(statusOrder))
	for i, s := range statusOrder {
		rank[s] = i
	}
	sort.SliceStable(es, func(i, j int) bool {
		if rank[es[i].Status] != rank[es[j].Status] {
			return rank[es[i].Status] < rank[es[j].Status]
		}
		return es[i].ID < es[j].ID
	})
	return es
}

// summaryLine renders "1 regressed, 1 new-low, ..." in a stable order.
func summaryLine(r differ.Report) string {
	labels := map[differ.Status]string{
		differ.StatusRegressed: "regressed", differ.StatusNewLow: "new-low",
		differ.StatusImproved: "improved", differ.StatusNew: "new",
		differ.StatusRemoved: "removed", differ.StatusUnchanged: "unchanged",
	}
	var parts []string
	for _, s := range statusOrder {
		parts = append(parts, fmt.Sprintf("%d %s", r.Counts[s], labels[s]))
	}
	return strings.Join(parts, ", ")
}

// principleBreakdown renders an entry's per-principle changes as
// "OCP 100.0->50.0 (-50.0), SRP 60.0->55.0 (-5.0)". Empty when there are none.
func principleBreakdown(e differ.Entry) string {
	if len(e.PrincipleDeltas) == 0 {
		return ""
	}
	parts := make([]string, 0, len(e.PrincipleDeltas))
	for _, d := range e.PrincipleDeltas {
		parts = append(parts, fmt.Sprintf("%s %.1f->%.1f (%+.1f)", d.Principle, d.Base, d.Head, d.Delta()))
	}
	return strings.Join(parts, ", ")
}

// FormatDiffText renders a human-readable diff report. UNCHANGED targets are
// summarized in the count line rather than listed. minScore is shown next to
// NEW-LOW entries to make the violated floor explicit.
func FormatDiffText(r differ.Report, basePath string, minScore float64) string {
	var b strings.Builder
	fmt.Fprintf(&b, "go-solid-score diff (base: %s)\n", basePath)
	b.WriteString(strings.Repeat("=", 52) + "\n")
	for _, e := range sortedEntries(r) {
		switch e.Status {
		case differ.StatusUnchanged:
			continue
		case differ.StatusRegressed, differ.StatusImproved:
			fmt.Fprintf(&b, "%-10s %s  %.1f -> %.1f (%+.1f)\n",
				e.Status, e.ID, *e.Base, *e.Head, e.Diff())
			if bd := principleBreakdown(e); bd != "" {
				fmt.Fprintf(&b, "  ↳ %s\n", bd)
			}
		case differ.StatusNewLow:
			fmt.Fprintf(&b, "%-10s %s  %.1f (< min %.1f)\n", e.Status, e.ID, *e.Head, minScore)
		case differ.StatusNew:
			fmt.Fprintf(&b, "%-10s %s  %.1f\n", e.Status, e.ID, *e.Head)
		case differ.StatusRemoved:
			fmt.Fprintf(&b, "%-10s %s\n", e.Status, e.ID)
		}
	}
	b.WriteString(strings.Repeat("-", 52) + "\n")
	b.WriteString(summaryLine(r) + "\n")
	return b.String()
}

// FormatDiffMarkdown renders an octocov-style Markdown report for PR comments.
// Notable targets go in the top table; the full list is folded in <details>.
func FormatDiffMarkdown(r differ.Report) string {
	var b strings.Builder
	b.WriteString(diffMarker + "\n")
	b.WriteString("## go-solid-score\n\n")
	fmt.Fprintf(&b, "%s.\n\n", summaryLine(r))

	writeRow := func(e differ.Entry) {
		base, head, diff := "–", "–", "–"
		if e.Base != nil {
			base = fmt.Sprintf("%.1f", *e.Base)
		}
		if e.Head != nil {
			head = fmt.Sprintf("%.1f", *e.Head)
		}
		if e.Base != nil && e.Head != nil {
			diff = fmt.Sprintf("%+.1f", e.Diff())
		}
		principles := "–"
		if bd := principleBreakdown(e); bd != "" {
			principles = bd
		}
		fmt.Fprintf(&b, "| %s %s | `%s` | %s | %s | %s | %s |\n",
			statusEmoji[e.Status], e.Status, e.ID, base, head, diff, principles)
	}

	header := "| | target | base | head | diff | principles |\n|--|--|--|--|--|--|\n"
	b.WriteString(header)
	for _, e := range sortedEntries(r) {
		if e.Status == differ.StatusUnchanged {
			continue
		}
		writeRow(e)
	}

	if r.Counts[differ.StatusUnchanged] > 0 {
		fmt.Fprintf(&b, "\n<details><summary>All targets (incl. %d unchanged)</summary>\n\n",
			r.Counts[differ.StatusUnchanged])
		b.WriteString(header)
		for _, e := range sortedEntries(r) {
			writeRow(e)
		}
		b.WriteString("\n</details>\n")
	}
	return b.String()
}

// diffPrincipleDelta is the machine-readable per-principle change.
type diffPrincipleDelta struct {
	Principle string  `json:"principle"`
	Base      float64 `json:"base"`
	Head      float64 `json:"head"`
	Diff      float64 `json:"diff"`
}

// diffJSONResult is the machine-readable per-target diff record.
type diffJSONResult struct {
	ID         string               `json:"id"`
	Name       string               `json:"name"`
	Package    string               `json:"package"`
	Status     string               `json:"status"`
	Base       *float64             `json:"base"`
	Head       *float64             `json:"head"`
	Diff       *float64             `json:"diff"`
	Principles []diffPrincipleDelta `json:"principles,omitempty"`
}

// FormatDiffJSON renders the diff report as machine-readable JSON.
func FormatDiffJSON(r differ.Report) string {
	type summary struct {
		Counts    map[string]int `json:"counts"`
		Regressed bool           `json:"regressed"`
	}
	type doc struct {
		Results []diffJSONResult `json:"results"`
		Summary summary          `json:"summary"`
	}

	d := doc{
		Results: []diffJSONResult{}, // emit [] not null for an empty diff
		Summary: summary{Counts: map[string]int{}, Regressed: r.Regressed},
	}
	for s, c := range r.Counts {
		d.Summary.Counts[string(s)] = c
	}
	for _, e := range sortedEntries(r) {
		jr := diffJSONResult{
			ID: e.ID, Name: e.Name, Package: e.Package, Status: string(e.Status),
			Base: e.Base, Head: e.Head,
		}
		if e.Base != nil && e.Head != nil {
			dv := e.Diff()
			jr.Diff = &dv
		}
		for _, pd := range e.PrincipleDeltas {
			jr.Principles = append(jr.Principles, diffPrincipleDelta{
				Principle: pd.Principle, Base: pd.Base, Head: pd.Head, Diff: pd.Delta(),
			})
		}
		d.Results = append(d.Results, jr)
	}
	out, _ := json.MarshalIndent(d, "", "  ")
	return string(out) + "\n"
}
