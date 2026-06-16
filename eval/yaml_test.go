package eval

import (
	"testing"

	"github.com/harakeishi/go-solid-score/analyzer"
)

func TestParseYAMLLabels(t *testing.T) {
	src := `
labels:
  - id: github.com/spf13/cobra.Command
    want:
      - principle: ISP
        expect: violation
        reason: "Lanza&Marinescu: wide self-imposed surface"
        split: test
      - principle: DIP
        expect: ok
        reason: "function fields are behavioural injection"
  - id: github.com/uber-go/zap/zapcore.jsonEncoder
    want:
      - principle: ISP
        expect: ok
        reason: "surface mandated by external zapcore.Encoder contract"
`
	got, err := ParseYAMLLabels([]byte(src))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []Label{
		{ID: "github.com/spf13/cobra.Command", Principle: analyzer.ISP, Expect: Violation, Reason: "Lanza&Marinescu: wide self-imposed surface", Split: SplitTest},
		{ID: "github.com/spf13/cobra.Command", Principle: analyzer.DIP, Expect: OK, Reason: "function fields are behavioural injection", Split: SplitTest},
		{ID: "github.com/uber-go/zap/zapcore.jsonEncoder", Principle: analyzer.ISP, Expect: OK, Reason: "surface mandated by external zapcore.Encoder contract", Split: SplitTest},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d labels, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("label[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestParseYAMLLabels_Errors(t *testing.T) {
	tests := []struct {
		name string
		src  string
	}{
		{"missing id", "labels:\n  - want:\n      - principle: ISP\n        expect: ok\n"},
		{"unknown principle", "labels:\n  - id: x.Y\n    want:\n      - principle: ZZZ\n        expect: ok\n"},
		{"unknown expect", "labels:\n  - id: x.Y\n    want:\n      - principle: ISP\n        expect: perhaps\n"},
		{"bad yaml", "labels: [unclosed"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := ParseYAMLLabels([]byte(tt.src)); err == nil {
				t.Errorf("expected error for %s, got nil", tt.name)
			}
		})
	}
}
