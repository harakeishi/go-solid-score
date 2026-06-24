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
		"atomic.Pointer[int]":    "atomic.Pointer",
		"*atomic.Pointer[*Foo]":  "atomic.Pointer",
		"[]atomic.Pointer[Bar]":  "atomic.Pointer",
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

// TestIsWhitelistedAtomicFamily guards against scoring sync/atomic value
// holders inconsistently. atomic.Uint64 etc. have a struct underlying type, so
// they are not recognized as value types and rely entirely on the whitelist;
// omitting any of the family makes equivalent concurrency primitives split
// between DIP=100 and DIP=0. The whole family must be whitelisted uniformly.
func TestIsWhitelistedAtomicFamily(t *testing.T) {
	family := []string{
		"atomic.Value", "atomic.Bool",
		"atomic.Int32", "atomic.Int64",
		"atomic.Uint32", "atomic.Uint64",
		"atomic.Uintptr",
		"atomic.Pointer", "atomic.Pointer[int]",
	}
	for _, ty := range family {
		if !isWhitelisted(ty, nil) {
			t.Errorf("isWhitelisted(%q) = false, want true (atomic family must be uniform)", ty)
		}
	}
}

// TestIsWhitelistedReflectFamily guards against scoring reflect value/metadata
// holders as concrete DIP dependencies. reflect.Value/StructField/Method have a
// struct underlying type (not basic), so IsValueType does not recognize them
// and they rely entirely on the whitelist — the same reasoning that put the
// sync/atomic family here. A pure-data struct holding reflect metadata must not
// be flagged as a DIP violation.
func TestIsWhitelistedReflectFamily(t *testing.T) {
	family := []string{
		"reflect.Value", "reflect.Type", "reflect.Kind",
		"reflect.StructField", "reflect.Method",
	}
	for _, ty := range family {
		if !isWhitelisted(ty, nil) {
			t.Errorf("isWhitelisted(%q) = false, want true (reflect metadata holders are value types)", ty)
		}
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
