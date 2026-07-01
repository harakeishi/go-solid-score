package analyzer

import (
	"github.com/harakeishi/go-solid-score/model"
)

// This file holds the cohesion primitives the metric layer reuses: LSCC (the
// SRP cohesion metric) and LCOM4 (retained for ISP's public_lcom4). The SRP
// scoring behavior itself lives in the declarative rule set (see
// rules/presets.yaml), computed from the metrics in metrics.go.

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

// effectiveCohesionMethods returns the method set used for LSCC cohesion,
// excluding Go convention methods (errors.Is/As/Unwrap). They are
// framework-protocol methods that idiomatically inspect their argument rather
// than the receiver's fields, so counting them as non-sharing methods
// artificially deflates cohesion. The LSCC paper treats the composition of the
// method set as an application-level decision; the Go errors docs make these
// signatures a documented convention. Other method kinds (accessors,
// constructors) are intentionally NOT excluded.
func effectiveCohesionMethods(methods []*model.MethodInfo) []*model.MethodInfo {
	effective := make([]*model.MethodInfo, 0, len(methods))
	for _, m := range methods {
		if isConventionMethod(m) {
			continue
		}
		effective = append(effective, m)
	}
	return effective
}

// calculateLSCC computes the LSCC (Low-level Similarity-based Class Cohesion,
// Al Dallal & Briand 2012) cohesion metric in [0,1], where 1 is maximally
// cohesive. For each of the receiver's OWN named fields f accessed by x_f
// methods, the metric sums x_f*(x_f-1) and normalizes by k*l*(l-1) (l effective
// methods, k own named fields). Only fields in ownFields are counted: a
// method's AccessedFields also contains names read through other objects,
// package variables, or promoted embedded fields, and counting those against a
// denominator built solely from own fields let LSCC exceed 1 and masked low
// cohesion. Restricting to own fields keeps the numerator and denominator over
// the same set, guaranteeing LSCC in [0,1].
//
// It returns 0 when the metric is undefined (l <= 1 or no own fields). The
// second return, ownFieldMethodCount, is the number of effective methods that
// read at least one own field — the applicability signal for the srp-cohesion
// rule. When it is < 2 no field can be shared, so cohesion is not merely low
// but undefined; the rule guards on it (own_field_access_method_count >= 2) so
// that a struct whose methods operate purely on parameters is not penalized as
// "low cohesion" the way the unfiltered metric was.
func calculateLSCC(methods []*model.MethodInfo, ownFields map[string]bool) (lscc float64, ownFieldMethodCount int) {
	effective := effectiveCohesionMethods(methods)

	l := len(effective)
	if l <= 1 || len(ownFields) == 0 {
		return 0, 0
	}
	accessCount := make(map[string]int)
	for _, m := range effective {
		touched := false
		for _, f := range m.AccessedFields {
			if ownFields[f] {
				accessCount[f]++
				touched = true
			}
		}
		if touched {
			ownFieldMethodCount++
		}
	}
	numerator := 0.0
	for _, x := range accessCount {
		numerator += float64(x) * float64(x-1)
	}
	denominator := float64(len(ownFields)) * float64(l) * float64(l-1)
	return numerator / denominator, ownFieldMethodCount
}

// isConventionMethod reports whether m matches one of Go's error-handling
// convention method signatures: Is(error) bool, As(any) bool, Unwrap() error,
// or Unwrap() []error. These are framework-protocol methods (documented in the
// errors package) that inspect their argument or return wrapped state rather
// than reading the receiver's own fields, so they should not count toward a
// type's cohesion. Matching is by signature, not name alone, so an unrelated
// method that merely happens to be named Is/As/Unwrap is not excluded.
func isConventionMethod(m *model.MethodInfo) bool {
	param := func(i int) string {
		if i < len(m.Params) {
			return m.Params[i].TypeName
		}
		return ""
	}
	ret := func(i int) string {
		if i < len(m.Returns) {
			return m.Returns[i].TypeName
		}
		return ""
	}
	switch m.Name {
	case "Is":
		return len(m.Params) == 1 && param(0) == "error" &&
			len(m.Returns) == 1 && ret(0) == "bool"
	case "As":
		return len(m.Params) == 1 && (param(0) == "any" || param(0) == "interface{}") &&
			len(m.Returns) == 1 && ret(0) == "bool"
	case "Unwrap":
		return len(m.Params) == 0 &&
			len(m.Returns) == 1 && (ret(0) == "error" || ret(0) == "[]error")
	}
	return false
}
