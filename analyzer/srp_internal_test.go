package analyzer

import (
	"math"
	"testing"

	"github.com/harakeishi/go-solid-score/model"
)

func mi(name string, fields ...string) *model.MethodInfo {
	return &model.MethodInfo{Name: name, AccessedFields: fields}
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
		name        string
		methods     []*model.MethodInfo
		namedFields int
		want        float64
	}{
		{
			// 全メソッドが全フィールドを共有 = 最高凝集。
			// f1: 3 methods, f2: 3 methods. l=3,k=2.
			// (3*2 + 3*2) / (2*3*2) = 12/12 = 1.0
			name: "fully cohesive",
			methods: []*model.MethodInfo{
				mi("A", "f1", "f2"), mi("B", "f1", "f2"), mi("C", "f1", "f2"),
			},
			namedFields: 2,
			want:        1.0,
		},
		{
			// 各メソッドが別フィールドのみ = 共有なし。
			// f1:1, f2:1, f3:1. 分子 = 1*0*3 = 0 -> 0.0
			name: "no sharing",
			methods: []*model.MethodInfo{
				mi("A", "f1"), mi("B", "f2"), mi("C", "f3"),
			},
			namedFields: 3,
			want:        0.0,
		},
		{
			// 部分共有。f1: A,B (2 methods), f2: B,C (2 methods). l=3,k=2.
			// (2*1 + 2*1) / (2*3*2) = 4/12 = 0.3333...
			name: "partial sharing",
			methods: []*model.MethodInfo{
				mi("A", "f1"), mi("B", "f1", "f2"), mi("C", "f2"),
			},
			namedFields: 2,
			want:        1.0 / 3.0,
		},
		{
			name:        "single method is not applicable",
			methods:     []*model.MethodInfo{mi("A", "f1")},
			namedFields: 1,
			want:        0.0,
		},
		{
			name:        "no fields is not applicable",
			methods:     []*model.MethodInfo{mi("A"), mi("B")},
			namedFields: 0,
			want:        0.0,
		},
		{
			name:        "no methods",
			methods:     nil,
			namedFields: 2,
			want:        0.0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calculateLSCC(tt.methods, tt.namedFields)
			if math.Abs(got-tt.want) > 1e-9 {
				t.Errorf("calculateLSCC = %v, want %v", got, tt.want)
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
	got := calculateLSCC(methods, 1)
	if got != 0 {
		t.Errorf("calculateLSCC with excluded convention method = %v, want 0 (not applicable)", got)
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
	if got := calculateLSCC(cohesive, 1); math.Abs(got-1.0) > 1e-9 {
		t.Errorf("calculateLSCC (cohesive, Is excluded) = %v, want 1.0", got)
	}
}
