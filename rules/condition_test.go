package rules

import "testing"

func TestParseComparison(t *testing.T) {
	cases := []struct {
		in   string
		v    float64
		want bool
	}{
		{"> 40", 41, true},
		{"> 40", 40, false},
		{">= 40", 40, true},
		{"< 0.5", 0.4, true},
		{"<= 5", 6, false},
		{"== 0", 0, true},
		{"!= 0", 0, false},
		{">=0", 0, true}, // no space
	}
	for _, c := range cases {
		cmp, err := parseComparison(c.in)
		if err != nil {
			t.Fatalf("parseComparison(%q) error: %v", c.in, err)
		}
		if got := cmp.eval(c.v); got != c.want {
			t.Errorf("(%q).eval(%v) = %v, want %v", c.in, c.v, got, c.want)
		}
	}
}

func TestParseComparison_Errors(t *testing.T) {
	for _, in := range []string{"", "40", "~ 3", "> abc"} {
		if _, err := parseComparison(in); err == nil {
			t.Errorf("parseComparison(%q) expected error", in)
		}
	}
}

func TestParseWhere(t *testing.T) {
	w, err := parseWhere("structural_dep_total == 0")
	if err != nil {
		t.Fatal(err)
	}
	if w.metric != "structural_dep_total" {
		t.Errorf("metric = %q", w.metric)
	}
	if !w.cmp.eval(0) || w.cmp.eval(1) {
		t.Errorf("where eval wrong")
	}
}

func TestParseWhere_Errors(t *testing.T) {
	for _, in := range []string{"method_count", "method_count >=", "a b c d"} {
		if _, err := parseWhere(in); err == nil {
			t.Errorf("parseWhere(%q) expected error", in)
		}
	}
}
