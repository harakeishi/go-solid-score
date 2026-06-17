package eval

import (
	"testing"

	"github.com/harakeishi/go-solid-score/analyzer"
)

func TestParseDocLabels(t *testing.T) {
	tests := []struct {
		name string
		doc  string
		want []Label
	}{
		{
			name: "single violation with reason and split",
			doc:  `FatImpl has a bloated interface.` + "\n" + `solid:want ISP=violation reason="Lanza&Marinescu fat interface" split=test`,
			want: []Label{
				{Principle: analyzer.ISP, Expect: Violation, Reason: "Lanza&Marinescu fat interface", Split: SplitTest},
			},
		},
		{
			name: "ok defaults split to test",
			doc:  `solid:want SRP=ok reason="single cohesive responsibility"`,
			want: []Label{
				{Principle: analyzer.SRP, Expect: OK, Reason: "single cohesive responsibility", Split: SplitTest},
			},
		},
		{
			name: "multiple principles across lines",
			doc:  "solid:want ISP=violation reason=\"fat\"\nsolid:want DIP=ok reason=\"injected\" split=train",
			want: []Label{
				{Principle: analyzer.ISP, Expect: Violation, Reason: "fat", Split: SplitTest},
				{Principle: analyzer.DIP, Expect: OK, Reason: "injected", Split: SplitTrain},
			},
		},
		{
			name: "na is parsed",
			doc:  `solid:want SRP=na reason="facade, classification deferred"`,
			want: []Label{
				{Principle: analyzer.SRP, Expect: NA, Reason: "facade, classification deferred", Split: SplitTest},
			},
		},
		{
			name: "no solid:want line yields nothing",
			doc:  "just an ordinary doc comment\nwith two lines",
			want: nil,
		},
		{
			name: "reason optional",
			doc:  `solid:want LSP=violation`,
			want: []Label{
				{Principle: analyzer.LSP, Expect: Violation, Reason: "", Split: SplitTest},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseDocLabels(tt.doc)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("got %d labels, want %d: %+v", len(got), len(tt.want), got)
			}
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Errorf("label[%d] = %+v, want %+v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestParseDocLabels_Errors(t *testing.T) {
	tests := []struct {
		name string
		doc  string
	}{
		{"unknown principle", `solid:want XYZ=violation`},
		{"unknown expectation", `solid:want SRP=maybe`},
		{"missing equals", `solid:want SRP violation`},
		{"unknown split", `solid:want SRP=ok split=holdout`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := ParseDocLabels(tt.doc); err == nil {
				t.Errorf("expected error for %q, got nil", tt.doc)
			}
		})
	}
}
