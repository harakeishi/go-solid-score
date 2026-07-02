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
	if !strings.Contains(out, "No targets found") {
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

// makeMixedResults returns one struct and one interface target so that
// section-splitting behavior can be exercised.
func makeMixedResults() []*scorer.ScoreResult {
	return []*scorer.ScoreResult{
		{
			TargetPkg:  "example.com/pkg",
			TargetName: "MyStruct",
			TargetFile: "s.go",
			TargetLine: 10,
			Scores: map[analyzer.Principle]float64{
				analyzer.SRP: 100, analyzer.OCP: 90, analyzer.LSP: 100, analyzer.ISP: 80, analyzer.DIP: 70,
			},
			Total: 88.0,
		},
		{
			TargetPkg:   "example.com/pkg",
			TargetName:  "MyIface",
			TargetFile:  "i.go",
			TargetLine:  5,
			IsInterface: true,
			// An interface is scored on ISP alone.
			Scores: map[analyzer.Principle]float64{analyzer.ISP: 100},
			Total:  100.0,
		},
	}
}

func TestTextFormatter_SeparatesInterfaceSection(t *testing.T) {
	f := &formatter.TextFormatter{}
	out, err := f.Format(makeMixedResults())
	if err != nil {
		t.Fatal(err)
	}
	// Both a Struct header and an Interface header must appear.
	if !strings.Contains(out, "Struct") {
		t.Error("expected a Struct section header")
	}
	if !strings.Contains(out, "Interface") {
		t.Error("expected an Interface section header")
	}
	// The interface must be listed after the struct section, in its own table.
	structIdx := strings.Index(out, "MyStruct")
	ifaceIdx := strings.Index(out, "MyIface")
	ifaceHdrIdx := strings.Index(out, "Interface")
	if structIdx < 0 || ifaceIdx < 0 {
		t.Fatalf("expected both targets in output:\n%s", out)
	}
	if ifaceIdx < ifaceHdrIdx {
		t.Errorf("interface row should appear under the Interface header, got:\n%s", out)
	}
}

func TestTextFormatter_StructOnlyHasNoInterfaceSection(t *testing.T) {
	f := &formatter.TextFormatter{}
	out, err := f.Format(makeResults()) // structs only
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "Interface") {
		t.Errorf("struct-only input should not emit an Interface section:\n%s", out)
	}
}

func TestTextFormatter_InterfaceOnlyHasNoStructSection(t *testing.T) {
	f := &formatter.TextFormatter{}
	only := makeMixedResults()[1:] // interface only
	out, err := f.Format(only)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "SRP") {
		t.Errorf("interface-only input should not emit the five-principle struct header:\n%s", out)
	}
	if !strings.Contains(out, "Interface") {
		t.Errorf("interface-only input should emit an Interface section:\n%s", out)
	}
}

func TestJSONFormatter_SummarySeparatesStructsAndInterfaces(t *testing.T) {
	f := &formatter.JSONFormatter{}
	out, err := f.Format(makeMixedResults()) // 1 struct (Total 88) + 1 interface (Total 100)
	if err != nil {
		t.Fatal(err)
	}
	var parsed struct {
		Summary struct {
			TotalStructs          int     `json:"total_structs"`
			AverageScore          float64 `json:"average_score"`
			TotalInterfaces       int     `json:"total_interfaces"`
			InterfaceAverageScore float64 `json:"interface_average_score"`
		} `json:"summary"`
	}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	s := parsed.Summary
	// total_structs must count structs only — not interfaces.
	if s.TotalStructs != 1 {
		t.Errorf("total_structs = %d, want 1 (interfaces excluded)", s.TotalStructs)
	}
	if s.TotalInterfaces != 1 {
		t.Errorf("total_interfaces = %d, want 1", s.TotalInterfaces)
	}
	// average_score must be the struct-only average, not the struct/interface blend.
	if s.AverageScore != 88.0 {
		t.Errorf("average_score = %.1f, want 88.0 (struct-only, not blended with interface)", s.AverageScore)
	}
	if s.InterfaceAverageScore != 100.0 {
		t.Errorf("interface_average_score = %.1f, want 100.0", s.InterfaceAverageScore)
	}
}

// makeDetailedResults returns one struct and one interface target carrying
// per-principle Details, for exercising verbose output.
func makeDetailedResults() []*scorer.ScoreResult {
	results := makeMixedResults()
	results[0].Details = map[analyzer.Principle][]string{
		analyzer.SRP: {"12 public methods (possible god class)"},
		analyzer.DIP: {"field db: *sql.DB (concrete dependency)"},
	}
	results[1].Details = map[analyzer.Principle][]string{
		analyzer.ISP: {"9 methods (fat interface)"},
	}
	return results
}

func TestTextFormatter_VerboseShowsDetails(t *testing.T) {
	f := &formatter.TextFormatter{Verbose: true}
	out, err := f.Format(makeDetailedResults())
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"SRP: 12 public methods (possible god class)",
		"DIP: field db: *sql.DB (concrete dependency)",
		"ISP: 9 methods (fat interface)",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("verbose output missing detail %q:\n%s", want, out)
		}
	}
	// SRP details must be listed before DIP details (fixed principle order).
	if srp, dip := strings.Index(out, "SRP: 12"), strings.Index(out, "DIP: field"); srp > dip {
		t.Errorf("details should follow principle order SRP..DIP, got:\n%s", out)
	}
}

func TestTextFormatter_NonVerboseHidesDetails(t *testing.T) {
	f := &formatter.TextFormatter{}
	out, err := f.Format(makeDetailedResults())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "god class") || strings.Contains(out, "fat interface") {
		t.Errorf("non-verbose output should not contain detail lines:\n%s", out)
	}
}

func TestJSONFormatter_VerboseIncludesDetails(t *testing.T) {
	f := &formatter.JSONFormatter{Verbose: true}
	out, err := f.Format(makeDetailedResults())
	if err != nil {
		t.Fatal(err)
	}
	var parsed struct {
		Results []struct {
			Name    string              `json:"name"`
			Details map[string][]string `json:"details"`
		} `json:"results"`
	}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	byName := make(map[string]map[string][]string)
	for _, r := range parsed.Results {
		byName[r.Name] = r.Details
	}
	if got := byName["MyStruct"]["SRP"]; len(got) != 1 || got[0] != "12 public methods (possible god class)" {
		t.Errorf("MyStruct SRP details = %v, want the god-class line", got)
	}
	if got := byName["MyIface"]["ISP"]; len(got) != 1 || got[0] != "9 methods (fat interface)" {
		t.Errorf("MyIface ISP details = %v, want the fat-interface line", got)
	}
}

func TestJSONFormatter_NonVerboseOmitsDetails(t *testing.T) {
	f := &formatter.JSONFormatter{}
	out, err := f.Format(makeDetailedResults())
	if err != nil {
		t.Fatal(err)
	}
	// The key must be absent entirely (omitempty), not present as null/empty,
	// so pre-existing baselines remain byte-compatible.
	if strings.Contains(out, `"details"`) {
		t.Errorf("non-verbose JSON should not contain a details key:\n%s", out)
	}
}

func TestJSONFormatter_EmitsIsInterface(t *testing.T) {
	f := &formatter.JSONFormatter{}
	out, err := f.Format(makeMixedResults())
	if err != nil {
		t.Fatal(err)
	}
	var parsed struct {
		Results []struct {
			Name        string   `json:"name"`
			IsInterface bool     `json:"is_interface"`
			SRP         *float64 `json:"srp"`
			ISP         *float64 `json:"isp"`
		} `json:"results"`
	}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	byName := make(map[string]struct {
		IsInterface bool
		SRP         *float64
		ISP         *float64
	})
	for _, r := range parsed.Results {
		byName[r.Name] = struct {
			IsInterface bool
			SRP         *float64
			ISP         *float64
		}{r.IsInterface, r.SRP, r.ISP}
	}
	if !byName["MyIface"].IsInterface {
		t.Error("MyIface should have is_interface=true")
	}
	if byName["MyStruct"].IsInterface {
		t.Error("MyStruct should have is_interface=false")
	}
	// Unevaluated principles on the interface remain null (distinct from 0.0).
	if byName["MyIface"].SRP != nil {
		t.Errorf("MyIface.srp should be null, got %v", *byName["MyIface"].SRP)
	}
	if byName["MyIface"].ISP == nil {
		t.Error("MyIface.isp should be present")
	}
}
