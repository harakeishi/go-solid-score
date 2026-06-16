package eval_test

import (
	"testing"

	"github.com/harakeishi/go-solid-score/analyzer"
	"github.com/harakeishi/go-solid-score/eval"
	"github.com/harakeishi/go-solid-score/model"
	"github.com/harakeishi/go-solid-score/scorer"
)

// TestCollectDocLabels_JoinsToScoreID is the guard against the harness's worst
// failure mode: a label ID that does not match the scorer's TargetID, which
// would make every label a silent miss. It builds a label from a struct's doc
// comment and a score for the same (pkgPath, name), then asserts the IDs join.
func TestCollectDocLabels_JoinsToScoreID(t *testing.T) {
	const pkgPath = "example.com/pkg"
	pkgs := []*model.PackageInfo{{
		PkgPath: pkgPath,
		Structs: []*model.StructInfo{{
			Name: "Foo",
			Doc:  "Foo does things.\nsolid:want ISP=violation reason=\"fat\"\n",
		}},
	}}

	labels, err := eval.CollectDocLabels(pkgs)
	if err != nil {
		t.Fatal(err)
	}
	if len(labels) != 1 {
		t.Fatalf("expected 1 label, got %d", len(labels))
	}
	got := labels[0]
	if got.Principle != analyzer.ISP || got.Expect != eval.Violation {
		t.Errorf("unexpected label: %+v", got)
	}

	// The label's ID must equal the scorer's TargetID for the same target.
	wantID := (&scorer.ScoreResult{TargetPkg: pkgPath, TargetName: "Foo"}).TargetID()
	if got.ID != wantID {
		t.Errorf("label ID %q does not match scorer TargetID %q — labels would never join", got.ID, wantID)
	}

	scored := eval.CollectScores([]*scorer.ScoreResult{{
		TargetPkg:  pkgPath,
		TargetName: "Foo",
		Scores:     map[analyzer.Principle]float64{analyzer.ISP: 10},
	}})
	if _, ok := scored[got.ID]; !ok {
		t.Errorf("collected score map has no entry for label ID %q", got.ID)
	}
}

// TestCollectDocLabels_Interfaces confirms interface doc comments are collected
// too, so interface labels are ready once interface scoring lands.
func TestCollectDocLabels_Interfaces(t *testing.T) {
	pkgs := []*model.PackageInfo{{
		PkgPath: "example.com/pkg",
		Interfaces: []*model.InterfaceInfo{{
			Name: "Fat",
			Doc:  "Fat is bloated.\nsolid:want ISP=violation\n",
		}},
	}}
	labels, err := eval.CollectDocLabels(pkgs)
	if err != nil {
		t.Fatal(err)
	}
	if len(labels) != 1 || labels[0].ID != "example.com/pkg.Fat" {
		t.Fatalf("expected one interface label with joined ID, got %+v", labels)
	}
}

// TestCollectDocLabels_BadLabel surfaces a malformed label with the offending
// type's ID, so a typo in a doc comment is diagnosable.
func TestCollectDocLabels_BadLabel(t *testing.T) {
	pkgs := []*model.PackageInfo{{
		PkgPath: "example.com/pkg",
		Structs: []*model.StructInfo{{
			Name: "Foo",
			Doc:  "solid:want NOPE=violation\n",
		}},
	}}
	_, err := eval.CollectDocLabels(pkgs)
	if err == nil {
		t.Fatal("expected error for unknown principle")
	}
}

// TestCollectDocLabels_NoLabels returns nothing (not an error) for types with
// no solid:want comment.
func TestCollectDocLabels_NoLabels(t *testing.T) {
	pkgs := []*model.PackageInfo{{
		PkgPath: "example.com/pkg",
		Structs: []*model.StructInfo{{Name: "Foo", Doc: "Foo is plain.\n"}},
	}}
	labels, err := eval.CollectDocLabels(pkgs)
	if err != nil {
		t.Fatal(err)
	}
	if len(labels) != 0 {
		t.Errorf("expected no labels, got %d", len(labels))
	}
}
