package cmd

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

// writeBaseline writes a minimal baseline JSON with one target at the given total.
func writeBaseline(t *testing.T, dir, id, pkg, name string, total float64) string {
	t.Helper()
	content := `{"results":[{"id":"` + id + `","name":"` + name +
		`","package":"` + pkg + `","file":"x.go","line":1,` +
		`"srp":0,"ocp":0,"lsp":0,"isp":0,"dip":0,"total":` +
		floatStr(total) + `,"confidence":{}}],"summary":{"total_structs":1,"average_score":` +
		floatStr(total) + `}}`
	path := filepath.Join(dir, "base.json")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func floatStr(f float64) string {
	return strconv.FormatFloat(f, 'f', 1, 64)
}

func TestLoadBaseline(t *testing.T) {
	dir := t.TempDir()
	path := writeBaseline(t, dir, "pkg.Foo", "pkg", "Foo", 72.0)
	snaps, err := loadBaseline(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(snaps) != 1 || snaps[0].ID != "pkg.Foo" || snaps[0].Total != 72.0 {
		t.Errorf("unexpected snapshots: %+v", snaps)
	}
}

func TestLoadBaseline_Missing(t *testing.T) {
	if _, err := loadBaseline("/no/such/file.json"); err == nil {
		t.Error("expected error for missing baseline")
	}
}
