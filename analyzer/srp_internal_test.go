package analyzer

import (
	"math"
	"testing"

	"github.com/harakeishi/go-solid-score/model"
)

func mi(name string, fields ...string) *model.MethodInfo {
	return &model.MethodInfo{Name: name, AccessedFields: fields}
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
