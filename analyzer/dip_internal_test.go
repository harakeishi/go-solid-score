package analyzer

import (
	"testing"

	"github.com/harakeishi/go-solid-score/model"
)

func TestIsDataType(t *testing.T) {
	accessor := &model.MethodInfo{
		Name:    "Items",
		Returns: []*model.ReturnInfo{{TypeName: "[]*Item"}},
	}
	stringer := &model.MethodInfo{
		Name:    "String",
		Returns: []*model.ReturnInfo{{TypeName: "string"}},
	}
	// Returns nothing: exists for its effect, so it marks behavior even though
	// it calls nothing (e.g. a Run/Start method driving held collaborators).
	effect := &model.MethodInfo{Name: "Run"}
	// Delegates: calling anything while producing the result is behavior.
	delegator := &model.MethodInfo{
		Name:          "Describe",
		Returns:       []*model.ReturnInfo{{TypeName: "string"}},
		CalledMethods: []string{"ToUpper"},
	}

	cases := []struct {
		name string
		s    *model.StructInfo
		want bool
	}{
		{"no methods", &model.StructInfo{Name: "Record"}, true},
		{"accessors and convention methods only", &model.StructInfo{
			Name: "Doc", Methods: []*model.MethodInfo{accessor, stringer},
		}, true},
		{"effect method marks behavior", &model.StructInfo{
			Name: "Owner", Methods: []*model.MethodInfo{accessor, effect},
		}, false},
		{"delegating method marks behavior", &model.StructInfo{
			Name: "Tool", Methods: []*model.MethodInfo{delegator},
		}, false},
		{"embedding inherits behavior", &model.StructInfo{
			Name: "Embedder", Embeddings: []string{"Engine"},
		}, false},
	}
	for _, tc := range cases {
		if got := isDataType(tc.s); got != tc.want {
			t.Errorf("isDataType(%s) = %v, want %v", tc.name, got, tc.want)
		}
	}
}
