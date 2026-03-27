package analyzer

import (
	"fmt"

	"github.com/harakeishi/go-solid-score/model"
)

// SRPAnalyzer checks the Single Responsibility Principle using LCOM4.
type SRPAnalyzer struct{}

func NewSRPAnalyzer() *SRPAnalyzer { return &SRPAnalyzer{} }

func (a *SRPAnalyzer) Principle() Principle { return SRP }

func (a *SRPAnalyzer) Analyze(pkg *model.PackageInfo) []Result {
	var results []Result
	for _, s := range pkg.Structs {
		results = append(results, a.analyzeStruct(s))
	}
	return results
}

func (a *SRPAnalyzer) analyzeStruct(s *model.StructInfo) Result {
	r := Result{
		Principle:  SRP,
		TargetName: s.Name,
		TargetFile: s.File,
		TargetLine: s.Line,
		Score:      100,
		Confidence: ConfidenceHigh,
	}

	methods := s.Methods
	if len(methods) <= 1 {
		r.Confidence = ConfidenceLow
		r.Details = append(r.Details, "too few methods for meaningful LCOM4 analysis")
		return r
	}

	// Mitigation: structs with no fields and few methods (e.g., interceptors,
	// stateless handlers) get a minimum score since LCOM4 is unreliable
	// when there are no shared fields to form connections.
	hasFields := false
	for _, f := range s.Fields {
		if f.Name != "" { // skip embedded
			hasFields = true
			break
		}
	}
	if !hasFields && len(methods) <= 5 {
		r.Score = 80
		r.Confidence = ConfidenceLow
		r.Details = append(r.Details, "stateless struct (no fields): LCOM4 not applicable, minimum score applied")
		return r
	}

	if len(methods) <= 3 {
		r.Confidence = ConfidenceLowMedium
	} else if len(methods) >= 5 {
		r.Confidence = ConfidenceHigh
	} else {
		r.Confidence = ConfidenceMedium
	}

	// Compute LCOM4
	lcom4 := calculateLCOM4(methods)
	switch {
	case lcom4 <= 1:
		// fully cohesive
	case lcom4 == 2:
		r.Score -= 40
		r.Details = append(r.Details, fmt.Sprintf("LCOM4=%d: struct has %d disconnected responsibility groups", lcom4, lcom4))
	default:
		r.Score -= 70
		r.Details = append(r.Details, fmt.Sprintf("LCOM4=%d: struct has %d disconnected responsibility groups", lcom4, lcom4))
	}

	// Penalty: total cyclomatic complexity
	totalComplexity := 0
	for _, m := range methods {
		totalComplexity += m.CyclomaticComplexity
	}
	if totalComplexity > 40 {
		r.Score -= 20
		r.Details = append(r.Details, fmt.Sprintf("high total complexity: %d", totalComplexity))
	} else if totalComplexity > 20 {
		r.Score -= 10
		r.Details = append(r.Details, fmt.Sprintf("moderate total complexity: %d", totalComplexity))
	}

	// Penalty: too many methods
	if len(methods) > 15 {
		r.Score -= 15
		r.Details = append(r.Details, fmt.Sprintf("too many methods: %d", len(methods)))
	} else if len(methods) > 10 {
		r.Score -= 5
		r.Details = append(r.Details, fmt.Sprintf("many methods: %d", len(methods)))
	}

	r.Score = Clamp(r.Score)
	return r
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

	// BFS to count connected components
	visited := make([]bool, n)
	components := 0
	for i := 0; i < n; i++ {
		if visited[i] {
			continue
		}
		components++
		queue := []int{i}
		visited[i] = true
		for len(queue) > 0 {
			curr := queue[0]
			queue = queue[1:]
			for j := 0; j < n; j++ {
				if !visited[j] && adj[curr][j] {
					visited[j] = true
					queue = append(queue, j)
				}
			}
		}
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
