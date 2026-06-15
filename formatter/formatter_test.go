package formatter_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/harakeishi/go-solid-score/analyzer"
	"github.com/harakeishi/go-solid-score/formatter"
	"github.com/harakeishi/go-solid-score/scorer"
)

func makeResults() []*scorer.ScoreResult {
	return []*scorer.ScoreResult{
		{
			TargetPkg:  "example.com/pkg",
			TargetName: "GoodStruct",
			TargetFile: "good.go",
			TargetLine: 10,
			Scores: map[analyzer.Principle]float64{
				analyzer.SRP: 100, analyzer.OCP: 90, analyzer.LSP: 100, analyzer.ISP: 80, analyzer.DIP: 70,
			},
			Total: 88.0,
			Confidence: map[analyzer.Principle]float64{
				analyzer.SRP: 1.0, analyzer.OCP: 0.7, analyzer.LSP: 0.3, analyzer.ISP: 0.85, analyzer.DIP: 0.7,
			},
		},
		{
			TargetName: "BadStruct",
			TargetFile: "bad.go",
			TargetLine: 20,
			Scores: map[analyzer.Principle]float64{
				analyzer.SRP: 30, analyzer.OCP: 40, analyzer.LSP: 50, analyzer.ISP: 60, analyzer.DIP: 20,
			},
			Total: 38.5,
			Confidence: map[analyzer.Principle]float64{
				analyzer.SRP: 1.0, analyzer.OCP: 0.7, analyzer.LSP: 0.5, analyzer.ISP: 0.85, analyzer.DIP: 0.85,
			},
		},
	}
}

func TestTextFormatter(t *testing.T) {
	f := &formatter.TextFormatter{}
	out, err := f.Format(makeResults())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "go-solid-score") {
		t.Error("expected header")
	}
	if !strings.Contains(out, "GoodStruct") {
		t.Error("expected GoodStruct in output")
	}
	if !strings.Contains(out, "BadStruct") {
		t.Error("expected BadStruct in output")
	}
	if !strings.Contains(out, "Average") {
		t.Error("expected Average row")
	}
}

func TestTextFormatter_Empty(t *testing.T) {
	f := &formatter.TextFormatter{}
	out, err := f.Format(nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "No structs found") {
		t.Error("expected empty message")
	}
}

func TestJSONFormatter(t *testing.T) {
	f := &formatter.JSONFormatter{}
	out, err := f.Format(makeResults())
	if err != nil {
		t.Fatal(err)
	}

	var parsed struct {
		Results []struct {
			ID      string  `json:"id"`
			Name    string  `json:"name"`
			Package string  `json:"package"`
			Total   float64 `json:"total"`
		} `json:"results"`
		Summary struct {
			TotalStructs int     `json:"total_structs"`
			AverageScore float64 `json:"average_score"`
		} `json:"summary"`
	}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if parsed.Summary.TotalStructs != 2 {
		t.Errorf("expected 2 structs, got %d", parsed.Summary.TotalStructs)
	}
	if parsed.Summary.AverageScore <= 0 {
		t.Error("expected positive average score")
	}

	// Index results by name to assert the diff-oriented id/package fields.
	byName := make(map[string]struct {
		ID      string
		Package string
	})
	for _, r := range parsed.Results {
		byName[r.Name] = struct {
			ID      string
			Package string
		}{r.ID, r.Package}
	}

	// With a known package path, id is "<pkgPath>.<name>".
	if got := byName["GoodStruct"].ID; got != "example.com/pkg.GoodStruct" {
		t.Errorf("GoodStruct id: got %q, want %q", got, "example.com/pkg.GoodStruct")
	}
	if got := byName["GoodStruct"].Package; got != "example.com/pkg" {
		t.Errorf("GoodStruct package: got %q, want %q", got, "example.com/pkg")
	}

	// Without a package path, id falls back to "<file>:<name>" so that
	// distinct files never collapse to the same id.
	if got := byName["BadStruct"].ID; got != "bad.go:BadStruct" {
		t.Errorf("BadStruct fallback id: got %q, want %q", got, "bad.go:BadStruct")
	}
	if got := byName["BadStruct"].Package; got != "" {
		t.Errorf("BadStruct package: got %q, want empty", got)
	}
}

func TestJSONFormatter_Empty(t *testing.T) {
	f := &formatter.JSONFormatter{}
	out, err := f.Format(nil)
	if err != nil {
		t.Fatal(err)
	}
	var parsed struct {
		Summary struct {
			TotalStructs int `json:"total_structs"`
		} `json:"summary"`
	}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if parsed.Summary.TotalStructs != 0 {
		t.Errorf("expected 0 structs, got %d", parsed.Summary.TotalStructs)
	}
}
