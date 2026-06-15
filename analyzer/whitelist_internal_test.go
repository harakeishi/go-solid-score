package analyzer

import "testing"

func TestCoreTypeName(t *testing.T) {
	cases := map[string]string{
		"string":                 "string",
		"*Foo":                   "Foo",
		"[]Foo":                  "Foo",
		"[]*Foo":                 "Foo",
		"...string":              "string",
		"chan error":             "error",
		"<-chan int":             "int",
		"map[string]string":      "string",
		"map[string]*Command":    "Command",
		"map[string][]*Tree":     "Tree",
		"map[Key]map[string]Bar": "Bar",
		"func(...)":              "func(...)",
	}
	for in, want := range cases {
		if got := coreTypeName(in); got != want {
			t.Errorf("coreTypeName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestIsWhitelistedContainers(t *testing.T) {
	whitelisted := []string{
		"[]string", "map[string]int", "chan error", "[]*time.Time", "map[string][]byte",
	}
	for _, ty := range whitelisted {
		if !isWhitelisted(ty, nil) {
			t.Errorf("isWhitelisted(%q) = false, want true", ty)
		}
	}
	if isWhitelisted("map[string]*sql.DB", nil) {
		t.Error("isWhitelisted(map[string]*sql.DB) = true, want false (sql.DB is concrete)")
	}
}

func TestIsSelfReference(t *testing.T) {
	if !isSelfReference("[]*Command", "Command") {
		t.Error("[]*Command should be a self-reference of Command")
	}
	if !isSelfReference("*Tree", "Tree") {
		t.Error("*Tree should be a self-reference of Tree")
	}
	if isSelfReference("*Other", "Tree") {
		t.Error("*Other should not be a self-reference of Tree")
	}
	if isSelfReference("*Tree", "") {
		t.Error("empty struct name should never match")
	}
}
