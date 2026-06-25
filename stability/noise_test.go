package stability_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/harakeishi/go-solid-score/analyzer"
	"github.com/harakeishi/go-solid-score/config"
	"github.com/harakeishi/go-solid-score/parser"
	"github.com/harakeishi/go-solid-score/scorer"
	"github.com/harakeishi/go-solid-score/stability"
)

// fixture is a self-contained, stdlib-only module that exercises every
// principle's analyzer with the structures most likely to expose a precision
// bug: a multi-field struct whose methods touch different fields (SRP/LCOM4), a
// wide interface (ISP), a struct owning a concrete dependency (DIP), a concrete
// type switch (OCP), and a guarded panic (LSP). Keeping it stdlib-only lets each
// transformed copy load as a throwaway module with no network access.
var fixture = map[string]string{
	"go.mod": "module stabilitysample\n\ngo 1.26\n",
	"sample.go": `package sample

import (
	"fmt"
	"strings"
)

// Account aggregates two loosely related responsibilities so LCOM4 has real
// structure to measure; the methods deliberately touch disjoint field sets.
type Account struct {
	owner   string
	balance int
	logbuf  strings.Builder
	logSeq  int
}

func (a *Account) Deposit(n int) { a.balance += n }

func (a *Account) Balance() int { return a.balance }

func (a *Account) Owner() string { return a.owner }

func (a *Account) Log(msg string) {
	a.logSeq++
	a.logbuf.WriteString(msg)
}

func (a *Account) LogCount() int { return a.logSeq }

// Wide is a fat interface ISP should flag.
type Wide interface {
	A()
	B()
	C()
	D()
	E()
	F()
	G()
	H()
}

// Service owns a concrete dependency, the canonical DIP smell.
type Service struct {
	store *Account
}

func (s *Service) Run() { s.store.Deposit(1) }

// Classify branches on concrete types — an OCP smell.
type Classify struct{}

func (c Classify) Kind(v any) string {
	switch v.(type) {
	case int:
		return "int"
	case string:
		return "string"
	case fmt.Stringer:
		return "stringer"
	default:
		return "other"
	}
}

// Guarded panics on a precondition only; LSP should treat it as fail-fast.
type Guarded struct{ n int }

func (g *Guarded) Take(i int) int {
	if i < 0 {
		panic("negative index")
	}
	return g.n + i
}
`,
}

// scoreSource writes the given files into a fresh temp module and scores it with
// the default ruleset, exactly as the CLI would.
func scoreSource(t *testing.T, files map[string]string) []*scorer.ScoreResult {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatalf("writing %s: %v", name, err)
		}
	}

	pkgs, err := parser.ParseInDir(dir, []string{"./..."})
	if err != nil {
		t.Fatalf("parsing fixture module: %v", err)
	}

	cfg := config.Default()
	engine, err := cfg.Engine()
	if err != nil {
		t.Fatalf("building engine: %v", err)
	}
	analyzers := []analyzer.Analyzer{
		analyzer.NewRuleAnalyzer(analyzer.SRP, engine, nil),
		analyzer.NewRuleAnalyzer(analyzer.OCP, engine, nil),
		analyzer.NewRuleAnalyzer(analyzer.LSP, engine, nil),
		analyzer.NewRuleAnalyzer(analyzer.ISP, engine, nil),
		analyzer.NewRuleAnalyzer(analyzer.DIP, engine, cfg.DIP.Whitelist),
	}
	s := scorer.New(analyzers, cfg.Weights)

	var results []*scorer.ScoreResult
	for _, pkg := range pkgs {
		results = append(results, s.Score(pkg)...)
	}
	if len(results) == 0 {
		t.Fatal("fixture produced no scored targets")
	}
	return results
}

// transformFixture applies tr to every Go file in the fixture, leaving non-Go
// files (go.mod) untouched.
func transformFixture(t *testing.T, tr stability.Transform) map[string]string {
	t.Helper()
	out := make(map[string]string, len(fixture))
	for name, content := range fixture {
		if filepath.Ext(name) != ".go" {
			out[name] = content
			continue
		}
		got, err := tr.Apply([]byte(content))
		if err != nil {
			t.Fatalf("transform %s on %s: %v", tr.Name, name, err)
		}
		if string(got) == content {
			t.Fatalf("transform %s left %s byte-identical; it is not exercising anything", tr.Name, name)
		}
		out[name] = string(got)
	}
	return out
}

// TestNoiseFloor is the measurement: every semantics-preserving transform must
// leave every score unchanged at display resolution. The scorer is a
// deterministic static analysis, so the measured noise floor must be exactly
// zero — any divergence is a precision bug (the class of defect that the LCOM4
// stateless-method and field-access fixes addressed historically). The test
// both measures the floor and guards it, failing CI if a future change
// reintroduces surface-sensitivity.
func TestNoiseFloor(t *testing.T) {
	base := scoreSource(t, fixture)

	for _, tr := range stability.Transforms() {
		tr := tr
		t.Run(tr.Name, func(t *testing.T) {
			head := scoreSource(t, transformFixture(t, tr))
			divs := stability.Compare(base, head)
			if len(divs) != 0 {
				for _, d := range divs {
					t.Errorf("score moved under semantics-preserving %q: %s", tr.Name, d)
				}
			}
		})
	}
}

// TestCompareDetectsMovement is a self-check on the measurement instrument: if
// the scores really differ, Compare must report it. Without this, a Compare that
// silently returned nothing would make TestNoiseFloor vacuously pass.
func TestCompareDetectsMovement(t *testing.T) {
	base := scoreSource(t, fixture)
	// Mutate the design materially (remove a field's coupling) and confirm the
	// instrument notices at least one moved score.
	// Narrow the fat Wide interface to a single method: ISP should change.
	mutated := map[string]string{
		"go.mod":    fixture["go.mod"],
		"sample.go": replaceWideInterface(fixture["sample.go"]),
	}

	head := scoreSource(t, mutated)
	if divs := stability.Compare(base, head); len(divs) == 0 {
		t.Fatal("Compare reported no divergence for a materially changed design; the instrument is blind")
	}
}

// replaceWideInterface narrows the fat Wide interface to one method so ISP's
// verdict genuinely changes — a real design change, not a cosmetic one.
func replaceWideInterface(src string) string {
	const fat = `type Wide interface {
	A()
	B()
	C()
	D()
	E()
	F()
	G()
	H()
}`
	const thin = `type Wide interface {
	A()
}`
	return strings.Replace(src, fat, thin, 1)
}
