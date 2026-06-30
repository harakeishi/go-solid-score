package analyzer

import (
	"github.com/harakeishi/go-solid-score/model"
)

// This file holds the SRP cohesion primitives (LCOM4 and its base penalty) that
// the metric layer reuses. The SRP scoring behavior itself now lives in the
// declarative rule set (see rules/presets.yaml), computed from the metrics in
// metrics.go.

// baseLCOM4Penalty maps an LCOM4 component count to the base cohesion penalty.
// It ramps linearly — 40 at two disconnected groups, +15 for each additional
// group — and saturates at 70. A ramp (rather than a 40→70 step at three
// groups) gives the metric resolution above three groups and keeps the final,
// size-attenuated penalty from jumping discontinuously as the group count rises.
func baseLCOM4Penalty(lcom4 int) float64 {
	if lcom4 <= 1 {
		return 0
	}
	const at2, perGroup, maxPenalty = 40.0, 15.0, 70.0
	p := at2 + perGroup*float64(lcom4-2)
	if p > maxPenalty {
		p = maxPenalty
	}
	return p
}

// calculateLCOM4 computes the LCOM4 metric: number of connected components
// in the method-field access graph.
func calculateLCOM4(methods []*model.MethodInfo) int {
	n := len(methods)
	if n == 0 {
		return 0
	}

	// Build adjacency: two methods are connected if they share a field
	adj := make([][]bool, n)
	for i := range adj {
		adj[i] = make([]bool, n)
	}

	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			if shareField(methods[i], methods[j]) || callsEachOther(methods[i], methods[j]) {
				adj[i][j] = true
				adj[j][i] = true
			}
		}
	}

	// BFS to count connected components, excluding singleton components whose
	// sole method accesses no receiver field. Such a method is stateless with
	// respect to the struct — a common example is an errors.Is/As convention
	// method that only inspects its argument, or any helper that operates
	// purely on parameters — and does not represent a distinct responsibility
	// over the struct's data. Counting it inflated LCOM4 and produced
	// false-positive SRP penalties on otherwise cohesive types. Methods that
	// access a field, or that are coupled to a sibling via a call, still count.
	visited := make([]bool, n)
	components := 0
	for i := 0; i < n; i++ {
		if visited[i] {
			continue
		}
		queue := []int{i}
		visited[i] = true
		members := 0
		for len(queue) > 0 {
			curr := queue[0]
			queue = queue[1:]
			members++
			for j := 0; j < n; j++ {
				if !visited[j] && adj[curr][j] {
					visited[j] = true
					queue = append(queue, j)
				}
			}
		}
		if members == 1 && len(methods[i].AccessedFields) == 0 {
			continue // stateless, uncoupled method: not a data responsibility
		}
		components++
	}
	return components
}

func shareField(a, b *model.MethodInfo) bool {
	set := make(map[string]bool, len(a.AccessedFields))
	for _, f := range a.AccessedFields {
		set[f] = true
	}
	for _, f := range b.AccessedFields {
		if set[f] {
			return true
		}
	}
	return false
}

func callsEachOther(a, b *model.MethodInfo) bool {
	for _, m := range a.CalledMethods {
		if m == b.Name {
			return true
		}
	}
	for _, m := range b.CalledMethods {
		if m == a.Name {
			return true
		}
	}
	return false
}

// calculateLSCC computes the LSCC (Low-level Similarity-based Class Cohesion,
// Al Dallal & Briand 2012) cohesion metric in [0,1], where 1 is maximally
// cohesive. For each named field f accessed by x_f methods, the metric sums
// x_f*(x_f-1) and normalizes by k*l*(l-1) (l methods, k named fields). It
// returns 0 when the metric is undefined (l <= 1 or k <= 0). Unlike LCOM4 this
// is a normalized ratio, so a single stateless method dilutes rather than
// fragments the score; false-positive control is left to the rule thresholds.
func calculateLSCC(methods []*model.MethodInfo, namedFieldCount int) float64 {
	l := len(methods)
	if l <= 1 || namedFieldCount <= 0 {
		return 0
	}
	accessCount := make(map[string]int)
	for _, m := range methods {
		for _, f := range m.AccessedFields {
			accessCount[f]++
		}
	}
	numerator := 0.0
	for _, x := range accessCount {
		numerator += float64(x) * float64(x-1)
	}
	denominator := float64(namedFieldCount) * float64(l) * float64(l-1)
	return numerator / denominator
}
