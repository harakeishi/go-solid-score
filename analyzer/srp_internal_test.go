package analyzer

import (
	"math"
	"testing"

	"github.com/harakeishi/go-solid-score/model"
)

func mi(name string, fields ...string) *model.MethodInfo {
	return &model.MethodInfo{Name: name, AccessedFields: fields}
}

// fset builds an own-field name set from the given field names.
func fset(names ...string) map[string]bool {
	s := make(map[string]bool, len(names))
	for _, n := range names {
		s[n] = true
	}
	return s
}

// convMethod builds a method with the given name, param type names, and return
// type names — enough to exercise the convention-method signature matcher.
func convMethod(name string, params, returns []string) *model.MethodInfo {
	m := &model.MethodInfo{Name: name}
	for _, p := range params {
		m.Params = append(m.Params, &model.ParamInfo{TypeName: p})
	}
	for _, r := range returns {
		m.Returns = append(m.Returns, &model.ReturnInfo{TypeName: r})
	}
	return m
}

func TestCalculateLSCC(t *testing.T) {
	tests := []struct {
		name      string
		methods   []*model.MethodInfo
		ownFields map[string]bool
		want      float64
		wantCount int
	}{
		{
			// 全メソッドが全フィールドを共有 = 最高凝集。
			// f1: 3 methods, f2: 3 methods. l=3,k=2.
			// (3*2 + 3*2) / (2*3*2) = 12/12 = 1.0
			name: "fully cohesive",
			methods: []*model.MethodInfo{
				mi("A", "f1", "f2"), mi("B", "f1", "f2"), mi("C", "f1", "f2"),
			},
			ownFields: fset("f1", "f2"),
			want:      1.0,
			wantCount: 3,
		},
		{
			// 各メソッドが別フィールドのみ = 共有なし。
			// f1:1, f2:1, f3:1. 分子 = 1*0*3 = 0 -> 0.0
			name: "no sharing",
			methods: []*model.MethodInfo{
				mi("A", "f1"), mi("B", "f2"), mi("C", "f3"),
			},
			ownFields: fset("f1", "f2", "f3"),
			want:      0.0,
			wantCount: 3,
		},
		{
			// 部分共有。f1: A,B (2 methods), f2: B,C (2 methods). l=3,k=2.
			// (2*1 + 2*1) / (2*3*2) = 4/12 = 0.3333...
			name: "partial sharing",
			methods: []*model.MethodInfo{
				mi("A", "f1"), mi("B", "f1", "f2"), mi("C", "f2"),
			},
			ownFields: fset("f1", "f2"),
			want:      1.0 / 3.0,
			wantCount: 3,
		},
		{
			name:      "single method is not applicable",
			methods:   []*model.MethodInfo{mi("A", "f1")},
			ownFields: fset("f1"),
			want:      0.0,
			wantCount: 0,
		},
		{
			name:      "no fields is not applicable",
			methods:   []*model.MethodInfo{mi("A"), mi("B")},
			ownFields: fset(),
			want:      0.0,
			wantCount: 0,
		},
		{
			name:      "no methods",
			methods:   nil,
			ownFields: fset("f1", "f2"),
			want:      0.0,
			wantCount: 0,
		},
		{
			// Warning 2 regression: three methods each read only EXTERNAL fields
			// (g1,g2 are not own fields). The unfiltered metric summed these into
			// the numerator against an own-field-only denominator and returned 2.0
			// (> 1, hiding low cohesion). Filtered to own fields, no field is
			// counted -> not applicable (0, count 0).
			name: "external fields do not inflate (was 2.0)",
			methods: []*model.MethodInfo{
				mi("A", "g1", "g2"), mi("B", "g1", "g2"), mi("C", "g1", "g2"),
			},
			ownFields: fset("own"),
			want:      0.0,
			wantCount: 0,
		},
		{
			// Warning 2 mixed: own f1 plus shared external g1. Only f1 (read by 2
			// methods) contributes. l=3,k=1 -> 2*1/(1*3*2)=1/3. g1's 3 accesses are
			// ignored, so LSCC stays <= 1.
			name: "external field ignored, own field counted",
			methods: []*model.MethodInfo{
				mi("A", "f1", "g1"), mi("B", "f1", "g1"), mi("C", "g1"),
			},
			ownFields: fset("f1"),
			want:      1.0 / 3.0,
			wantCount: 2,
		},
		{
			// Warning 1 regression: pure calculators read no own field. numerator=0
			// AND ownFieldMethodCount=0 -> the srp-cohesion rule's
			// own_field_access_method_count>=2 guard fails, so no false -40.
			name:      "no own-field access is not applicable, not low cohesion",
			methods:   []*model.MethodInfo{mi("Add"), mi("Mul")},
			ownFields: fset("total"),
			want:      0.0,
			wantCount: 0,
		},
		{
			// Genuinely low own-field cohesion is still measured (not silenced by
			// the fix): f1 read by A,C; f2 read only by B. l=3,k=2 ->
			// (2*1 + 0)/(2*3*2) = 2/12 = 0.1667. Count 3 (all touch an own field),
			// so the rule runs and the low band penalizes it.
			name: "low own cohesion still measured",
			methods: []*model.MethodInfo{
				mi("A", "f1"), mi("B", "f2"), mi("C", "f1"),
			},
			ownFields: fset("f1", "f2"),
			want:      2.0 / 12.0,
			wantCount: 3,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, gotCount := calculateLSCC(tt.methods, tt.ownFields)
			if math.Abs(got-tt.want) > 1e-9 {
				t.Errorf("calculateLSCC lscc = %v, want %v", got, tt.want)
			}
			if got > 1.0+1e-9 {
				t.Errorf("calculateLSCC lscc = %v exceeds 1.0", got)
			}
			if gotCount != tt.wantCount {
				t.Errorf("calculateLSCC ownFieldMethodCount = %v, want %v", gotCount, tt.wantCount)
			}
		})
	}
}

func TestIsConventionMethod(t *testing.T) {
	tests := []struct {
		name string
		m    *model.MethodInfo
		want bool
	}{
		{"Is(error) bool", convMethod("Is", []string{"error"}, []string{"bool"}), true},
		{"As(any) bool", convMethod("As", []string{"any"}, []string{"bool"}), true},
		{"As(interface{}) bool", convMethod("As", []string{"interface{}"}, []string{"bool"}), true},
		{"Unwrap() error", convMethod("Unwrap", nil, []string{"error"}), true},
		{"Unwrap() []error", convMethod("Unwrap", nil, []string{"[]error"}), true},
		// Negatives: right name, wrong signature, or unrelated method.
		{"Error() string is not a convention method", convMethod("Error", nil, []string{"string"}), false},
		{"Is with wrong param", convMethod("Is", []string{"string"}, []string{"bool"}), false},
		{"Is with wrong return", convMethod("Is", []string{"error"}, []string{"error"}), false},
		{"Unwrap with a param", convMethod("Unwrap", []string{"error"}, []string{"error"}), false},
		{"unrelated method", convMethod("Calculate", []string{"float64"}, []string{"float64"}), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isConventionMethod(tt.m); got != tt.want {
				t.Errorf("isConventionMethod(%s) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

func TestCalculateLSCC_ExcludesConventionMethods(t *testing.T) {
	// A cohesive error type: Error() uses the sole field msg; Is(error) bool is a
	// convention method that touches no field. With Is excluded, only Error()
	// remains (l=1 after exclusion) -> not applicable -> 0.0 (no penalty). The
	// rule's where:[method_count>=2] also won't fire here, but the metric itself
	// must not report a spurious low-cohesion 0 from the raw 2-method set.
	methods := []*model.MethodInfo{
		{Name: "Error", AccessedFields: []string{"msg"}, Returns: []*model.ReturnInfo{{TypeName: "string"}}},
		convMethod("Is", []string{"error"}, []string{"bool"}),
	}
	// Without exclusion the raw set would be l=2,k=1: msg accessed by 1 method ->
	// numerator 0 -> LSCC 0 (false positive). With Is excluded, l=1 -> 0.0 too,
	// but that 0.0 is the "not applicable" value, not a low-cohesion verdict.
	got, gotCount := calculateLSCC(methods, fset("msg"))
	if got != 0 {
		t.Errorf("calculateLSCC with excluded convention method = %v, want 0 (not applicable)", got)
	}
	if gotCount != 0 {
		t.Errorf("ownFieldMethodCount = %v, want 0 (l=1 after exclusion)", gotCount)
	}

	// A genuinely cohesive type whose convention method would otherwise dilute
	// cohesion: two real methods share the field, plus an Is convention method.
	// With Is excluded, the two real methods over one field give full cohesion.
	cohesive := []*model.MethodInfo{
		{Name: "Error", AccessedFields: []string{"msg"}},
		{Name: "Message", AccessedFields: []string{"msg"}},
		convMethod("Is", []string{"error"}, []string{"bool"}),
	}
	// After excluding Is: l=2, k=1, msg accessed by 2 methods -> 2*1/(1*2*1)=1.0
	if got, count := calculateLSCC(cohesive, fset("msg")); math.Abs(got-1.0) > 1e-9 || count != 2 {
		t.Errorf("calculateLSCC (cohesive, Is excluded) = %v (count %v), want 1.0 (count 2)", got, count)
	}
}
