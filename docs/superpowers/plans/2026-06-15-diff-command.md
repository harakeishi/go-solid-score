# diff サブコマンド実装計画

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** baseline JSON と head のスコアを安定IDで突き合わせ、回帰/改善/新規/削除を分類してレポートする `diff` サブコマンドを追加する（text/json/markdown 出力、報告のみ/CI失敗の切替付き）。

**Architecture:** 純関数 `differ.Diff()` を中核に、I/O・スコアリング・整形を周辺に分離する。base JSON のデコードは `formatter` が出力する形状の共有型を介し、head のスコアリングは `cmd.run()` のコアを切り出した共通関数を再利用する。出力整形は既存の `formatter` パッケージに追加する。

**Tech Stack:** Go 1.26, cobra（CLI）, encoding/json, 標準 testing。

---

## ファイル構成

- Create: `differ/differ.go` — 純粋な比較ロジック（`Snapshot`, `Status`, `Entry`, `Report`, `Options`, `Diff()`）
- Create: `differ/differ_test.go` — `Diff()` のテーブル駆動テスト
- Modify: `formatter/json.go` — `jsonResult` を公開型 `JSONResult` に抽出（共有契約）
- Create: `formatter/diff.go` — `differ.Report` の text/json/markdown レンダラ
- Create: `formatter/diff_test.go` — レンダラのテスト
- Create: `cmd/diff.go` — `diff` サブコマンド（base 読み込み・head 解析・分類・出力・exit code）
- Modify: `cmd/root.go` — スコアリングコアを `analyze()` に切り出し、`diff` コマンドを登録
- Create: `cmd/diff_test.go` — E2E テスト（base.json 書き出し→testdata 解析→exit code 検証）
- Create: `.github/workflows/solid-diff.yml` — PR コメント投稿サンプル
- Modify: `README.md` — diff サブコマンドと Actions の説明追加

---

## Task 1: `differ` パッケージ — 型と純関数 `Diff()`

**Files:**
- Create: `differ/differ.go`
- Test: `differ/differ_test.go`

- [ ] **Step 1: 失敗するテストを書く**

`differ/differ_test.go`:

```go
package differ_test

import (
	"testing"

	"github.com/harakeishi/go-solid-score/differ"
)

func snap(id string, total float64) differ.Snapshot {
	return differ.Snapshot{ID: id, Name: id, Package: "pkg", Total: total}
}

func statusOf(r differ.Report, id string) differ.Status {
	for _, e := range r.Entries {
		if e.ID == id {
			return e.Status
		}
	}
	return ""
}

func TestDiff_Classifications(t *testing.T) {
	base := []differ.Snapshot{
		snap("pkg.Reg", 72), // will drop a lot
		snap("pkg.Imp", 60), // will improve
		snap("pkg.Same", 80),
		snap("pkg.SmallDrop", 80), // drops within tolerance
		snap("pkg.Removed", 90),
	}
	head := []differ.Snapshot{
		snap("pkg.Reg", 58),       // -14 -> REGRESSED
		snap("pkg.Imp", 80),       // +20 -> IMPROVED
		snap("pkg.Same", 80),      // UNCHANGED
		snap("pkg.SmallDrop", 77), // -3 within maxDrop=5 -> UNCHANGED
		snap("pkg.NewLow", 45),    // new, below min -> NEW-LOW
		snap("pkg.NewOk", 90),     // new, no min issue -> NEW
	}

	r := differ.Diff(base, head, differ.Options{MaxDrop: 5, MinScore: 70})

	cases := map[string]differ.Status{
		"pkg.Reg":       differ.StatusRegressed,
		"pkg.Imp":       differ.StatusImproved,
		"pkg.Same":      differ.StatusUnchanged,
		"pkg.SmallDrop": differ.StatusUnchanged,
		"pkg.Removed":   differ.StatusRemoved,
		"pkg.NewLow":    differ.StatusNewLow,
		"pkg.NewOk":     differ.StatusNew,
	}
	for id, want := range cases {
		if got := statusOf(r, id); got != want {
			t.Errorf("%s: got %q, want %q", id, got, want)
		}
	}
	if !r.Regressed {
		t.Error("expected Regressed=true (has REGRESSED and NEW-LOW)")
	}
	if r.Counts[differ.StatusRegressed] != 1 {
		t.Errorf("regressed count: got %d, want 1", r.Counts[differ.StatusRegressed])
	}
}

func TestDiff_MaxDropBoundary(t *testing.T) {
	base := []differ.Snapshot{snap("pkg.A", 80), snap("pkg.B", 80)}
	head := []differ.Snapshot{snap("pkg.A", 75), snap("pkg.B", 74)} // -5 (==), -6 (>)

	r := differ.Diff(base, head, differ.Options{MaxDrop: 5})

	if got := statusOf(r, "pkg.A"); got != differ.StatusUnchanged {
		t.Errorf("drop == maxDrop must be UNCHANGED, got %q", got)
	}
	if got := statusOf(r, "pkg.B"); got != differ.StatusRegressed {
		t.Errorf("drop just over maxDrop must be REGRESSED, got %q", got)
	}
}

func TestDiff_MinScoreDisabled(t *testing.T) {
	head := []differ.Snapshot{snap("pkg.New", 10)}
	r := differ.Diff(nil, head, differ.Options{MaxDrop: 5, MinScore: 0})
	if got := statusOf(r, "pkg.New"); got != differ.StatusNew {
		t.Errorf("minScore=0 must yield NEW (not NEW-LOW), got %q", got)
	}
	if r.Regressed {
		t.Error("a lone NEW must not mark the report regressed")
	}
}
```

- [ ] **Step 2: テストが失敗することを確認**

Run: `go test ./differ/ -run TestDiff -v`
Expected: FAIL（`differ` パッケージが存在しない / コンパイルエラー）

- [ ] **Step 3: `differ/differ.go` を実装**

```go
// Package differ compares two sets of SOLID score snapshots (a baseline and a
// head) and classifies each target as regressed, improved, unchanged, new,
// new-low, or removed. The core Diff function is pure: it performs no I/O.
package differ

// Snapshot is the minimal projection of a scored target needed for diffing.
type Snapshot struct {
	ID      string
	Name    string
	Package string
	Total   float64
}

// Status is the classification of a target between base and head.
type Status string

const (
	StatusRegressed Status = "REGRESSED"
	StatusImproved  Status = "IMPROVED"
	StatusUnchanged Status = "UNCHANGED"
	StatusNew       Status = "NEW"
	StatusNewLow    Status = "NEW-LOW"
	StatusRemoved   Status = "REMOVED"
)

// Entry is the diff result for a single target. Base/Head are nil when the
// target is absent from that side (NEW/NEW-LOW have no Base; REMOVED has no Head).
type Entry struct {
	ID      string
	Name    string
	Package string
	Status  Status
	Base    *float64
	Head    *float64
}

// Diff returns the head total minus the base total. Only meaningful when both
// are present; callers guard via Status.
func (e *Entry) Diff() float64 {
	if e.Base == nil || e.Head == nil {
		return 0
	}
	return *e.Head - *e.Base
}

// Report is the full diff outcome.
type Report struct {
	Entries   []Entry
	Counts    map[Status]int
	Regressed bool // true if any REGRESSED or NEW-LOW exists
}

// Options tunes the classification thresholds.
type Options struct {
	MaxDrop  float64 // a total drop strictly greater than this is a regression
	MinScore float64 // a new target below this is NEW-LOW; 0 disables
}

// Diff compares base and head snapshots by ID and classifies each target.
// It is a pure function with no side effects.
func Diff(base, head []Snapshot, opts Options) Report {
	baseByID := make(map[string]Snapshot, len(base))
	for _, s := range base {
		baseByID[s.ID] = s
	}
	headByID := make(map[string]Snapshot, len(head))
	for _, s := range head {
		headByID[s.ID] = s
	}

	r := Report{Counts: make(map[Status]int)}

	// Targets present in head (both + new).
	for _, h := range head {
		hv := h.Total
		e := Entry{ID: h.ID, Name: h.Name, Package: h.Package, Head: &hv}
		if b, ok := baseByID[h.ID]; ok {
			bv := b.Total
			e.Base = &bv
			switch {
			case b.Total-h.Total > opts.MaxDrop:
				e.Status = StatusRegressed
			case h.Total > b.Total:
				e.Status = StatusImproved
			default:
				e.Status = StatusUnchanged
			}
		} else {
			if opts.MinScore > 0 && h.Total < opts.MinScore {
				e.Status = StatusNewLow
			} else {
				e.Status = StatusNew
			}
		}
		r.Entries = append(r.Entries, e)
		r.Counts[e.Status]++
	}

	// Targets only in base (removed).
	for _, b := range base {
		if _, ok := headByID[b.ID]; ok {
			continue
		}
		bv := b.Total
		e := Entry{ID: b.ID, Name: b.Name, Package: b.Package, Base: &bv, Status: StatusRemoved}
		r.Entries = append(r.Entries, e)
		r.Counts[e.Status]++
	}

	r.Regressed = r.Counts[StatusRegressed] > 0 || r.Counts[StatusNewLow] > 0
	return r
}
```

- [ ] **Step 4: テストが通ることを確認**

Run: `go test ./differ/ -run TestDiff -v`
Expected: PASS（3テストとも）

- [ ] **Step 5: コミット**

```bash
git add differ/differ.go differ/differ_test.go
git commit -m "feat: add differ package with pure Diff classification"
```

---

## Task 2: `formatter` の JSON 結果型を共有型に抽出

**Files:**
- Modify: `formatter/json.go`（既存 `jsonResult`／`jsonOutput`／`jsonSummary`、`Format` 内の利用箇所）

base JSON のデコードと出力を1つの型に統一するため、private `jsonResult` を public `JSONResult` にリネームし、出力ラッパも公開する。これにより diff 側が同じ型でデコードできる。

- [ ] **Step 1: 既存テストが通る状態を確認（リネーム前の基準）**

Run: `go test ./formatter/ -v`
Expected: PASS（既存テスト）

- [ ] **Step 2: 型を公開名にリネーム**

`formatter/json.go` の型定義を以下に置き換える（フィールドは現状維持、名前のみ公開化）:

```go
// JSONOutput is the top-level JSON document emitted by JSONFormatter and is
// also the shape consumed when decoding a baseline for diffing.
type JSONOutput struct {
	Results []JSONResult `json:"results"`
	Summary JSONSummary  `json:"summary"`
}

// JSONResult is one scored target in JSON form. The stable id/package fields
// make it suitable as a diff baseline.
type JSONResult struct {
	ID         string             `json:"id"`
	Name       string             `json:"name"`
	Package    string             `json:"package"`
	File       string             `json:"file"`
	Line       int                `json:"line"`
	SRP        float64            `json:"srp"`
	OCP        float64            `json:"ocp"`
	LSP        float64            `json:"lsp"`
	ISP        float64            `json:"isp"`
	DIP        float64            `json:"dip"`
	Total      float64            `json:"total"`
	Confidence map[string]float64 `json:"confidence"`
}

// JSONSummary is the aggregate block of the JSON document.
type JSONSummary struct {
	TotalStructs int     `json:"total_structs"`
	AverageScore float64 `json:"average_score"`
}
```

- [ ] **Step 3: `Format` 内の利用箇所を更新**

`formatter/json.go` の `Format` で `jsonOutput{...}` → `JSONOutput{...}`、`make([]jsonResult, ...)` → `make([]JSONResult, ...)`、`jr := jsonResult{...}` → `jr := JSONResult{...}`、`jsonSummary{...}` → `JSONSummary{...}` に置換する。

- [ ] **Step 4: ビルドと既存テストを確認**

Run: `go build ./... && go test ./formatter/ -v`
Expected: PASS（既存の挙動は不変、JSON のキーも不変）

- [ ] **Step 5: コミット**

```bash
git add formatter/json.go
git commit -m "refactor: export formatter JSON result types for diff reuse"
```

---

## Task 3: head スコアリングコアを `cmd.analyze()` に切り出す

**Files:**
- Modify: `cmd/root.go`（`run` 内の parse→analyzers→score 部分、現状 50-91 行付近）

`run` と `diff` の両方から同じスコアリングを呼べるよう、純粋な処理を関数に切り出す。

- [ ] **Step 1: `analyze` 関数を追加**

`cmd/root.go` に以下の関数を追加する（import に既存の parser/analyzer/scorer/config を利用。新規 import は不要）:

```go
// analyze parses the given patterns and scores every target, applying the
// scoring rules from cfg. It is the shared core used by both `run` and `diff`.
func analyze(cfg *config.Config, patterns []string) ([]*scorer.ScoreResult, error) {
	pkgs, err := parser.Parse(patterns)
	if err != nil {
		return nil, fmt.Errorf("parsing packages: %w", err)
	}

	analyzers := []analyzer.Analyzer{
		analyzer.NewSRPAnalyzer(),
		analyzer.NewOCPAnalyzer(),
		analyzer.NewLSPAnalyzer(),
		analyzer.NewISPAnalyzer(),
		analyzer.NewDIPAnalyzer(cfg.DIP.Whitelist),
	}

	s := scorer.New(analyzers, cfg.Weights)
	var allResults []*scorer.ScoreResult
	for _, pkg := range pkgs {
		allResults = append(allResults, s.Score(pkg)...)
	}
	return allResults, nil
}
```

- [ ] **Step 2: `run` を `analyze` 利用に書き換え**

`cmd/root.go` の `run` 内、`// Parse` から score までのブロック（`pkgs, err := parser.Parse(...)` ～ `allResults` 構築ループ）を次の2行に置き換える:

```go
	allResults, err := analyze(cfg, patterns)
	if err != nil {
		return err
	}
```

（`analyze` が既に `fmt.Errorf("parsing packages: %w", ...)` を返すため、`run` 側のラップは不要。残りの format/threshold ロジックは現状のまま。）

- [ ] **Step 3: ビルドと既存テストを確認**

Run: `go build ./... && go test ./... 2>&1 | tail -12`
Expected: PASS（`go run . ./scorer/` の挙動は不変）

- [ ] **Step 4: 手動スモークで挙動不変を確認**

Run: `go run . -f json ./differ/ | head -5`
Expected: `"results":` を含む JSON が出る（regression なし）

- [ ] **Step 5: コミット**

```bash
git add cmd/root.go
git commit -m "refactor: extract analyze() core shared by run and diff"
```

---

## Task 4: `formatter` に diff レンダラを追加

**Files:**
- Create: `formatter/diff.go`
- Test: `formatter/diff_test.go`

- [ ] **Step 1: 失敗するテストを書く**

`formatter/diff_test.go`:

```go
package formatter_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/harakeishi/go-solid-score/differ"
	"github.com/harakeishi/go-solid-score/formatter"
)

func sampleReport() differ.Report {
	f := func(v float64) *float64 { return &v }
	return differ.Report{
		Entries: []differ.Entry{
			{ID: "pkg.Reg", Name: "Reg", Package: "pkg", Status: differ.StatusRegressed, Base: f(72), Head: f(58)},
			{ID: "pkg.New", Name: "New", Package: "pkg", Status: differ.StatusNewLow, Head: f(45)},
			{ID: "pkg.Same", Name: "Same", Package: "pkg", Status: differ.StatusUnchanged, Base: f(80), Head: f(80)},
		},
		Counts:    map[differ.Status]int{differ.StatusRegressed: 1, differ.StatusNewLow: 1, differ.StatusUnchanged: 1},
		Regressed: true,
	}
}

func TestFormatDiffText(t *testing.T) {
	out := formatter.FormatDiffText(sampleReport(), "base.json")
	if !strings.Contains(out, "REGRESSED") || !strings.Contains(out, "pkg.Reg") {
		t.Errorf("missing regressed line:\n%s", out)
	}
	if !strings.Contains(out, "-14.0") {
		t.Errorf("missing diff value:\n%s", out)
	}
	if !strings.Contains(out, "1 regressed") {
		t.Errorf("missing summary counts:\n%s", out)
	}
	// UNCHANGED should not be listed individually.
	if strings.Contains(out, "UNCHANGED  pkg.Same") {
		t.Errorf("UNCHANGED should be summarized, not listed:\n%s", out)
	}
}

func TestFormatDiffMarkdown(t *testing.T) {
	out := formatter.FormatDiffMarkdown(sampleReport())
	if !strings.Contains(out, "<!-- go-solid-score-diff -->") {
		t.Errorf("missing marker comment:\n%s", out)
	}
	if !strings.Contains(out, "REGRESSED") || !strings.Contains(out, "`pkg.Reg`") {
		t.Errorf("missing regressed row:\n%s", out)
	}
	if !strings.Contains(out, "<details>") {
		t.Errorf("missing details fold:\n%s", out)
	}
}

func TestFormatDiffJSON(t *testing.T) {
	out := formatter.FormatDiffJSON(sampleReport())
	var parsed struct {
		Results []struct {
			ID     string  `json:"id"`
			Status string  `json:"status"`
			Diff   float64 `json:"diff"`
		} `json:"results"`
		Summary struct {
			Regressed bool `json:"regressed"`
		} `json:"summary"`
	}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if !parsed.Summary.Regressed {
		t.Error("expected regressed=true in summary")
	}
	var foundReg bool
	for _, r := range parsed.Results {
		if r.ID == "pkg.Reg" {
			foundReg = true
			if r.Status != "REGRESSED" || r.Diff != -14 {
				t.Errorf("pkg.Reg: status=%s diff=%v", r.Status, r.Diff)
			}
		}
	}
	if !foundReg {
		t.Error("pkg.Reg not in JSON results")
	}
}
```

- [ ] **Step 2: テストが失敗することを確認**

Run: `go test ./formatter/ -run TestFormatDiff -v`
Expected: FAIL（`FormatDiffText` 等が未定義）

- [ ] **Step 3: `formatter/diff.go` を実装**

```go
package formatter

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/harakeishi/go-solid-score/differ"
)

// diffMarker is the leading HTML comment used by CI to find and update the
// previous PR comment.
const diffMarker = "<!-- go-solid-score-diff -->"

// statusOrder controls the display order of statuses in diff output.
var statusOrder = []differ.Status{
	differ.StatusRegressed, differ.StatusNewLow, differ.StatusImproved,
	differ.StatusNew, differ.StatusRemoved, differ.StatusUnchanged,
}

// statusEmoji maps a status to its Markdown marker.
var statusEmoji = map[differ.Status]string{
	differ.StatusRegressed: "🔻",
	differ.StatusNewLow:    "⚠️",
	differ.StatusImproved:  "🔺",
	differ.StatusNew:       "✨",
	differ.StatusRemoved:   "🗑",
	differ.StatusUnchanged: "▫️",
}

// sortedEntries returns entries grouped by statusOrder, then by ID.
func sortedEntries(r differ.Report) []differ.Entry {
	es := make([]differ.Entry, len(r.Entries))
	copy(es, r.Entries)
	rank := make(map[differ.Status]int, len(statusOrder))
	for i, s := range statusOrder {
		rank[s] = i
	}
	sort.SliceStable(es, func(i, j int) bool {
		if rank[es[i].Status] != rank[es[j].Status] {
			return rank[es[i].Status] < rank[es[j].Status]
		}
		return es[i].ID < es[j].ID
	})
	return es
}

// summaryLine renders "1 regressed, 1 new-low, ..." in a stable order.
func summaryLine(r differ.Report) string {
	labels := map[differ.Status]string{
		differ.StatusRegressed: "regressed", differ.StatusNewLow: "new-low",
		differ.StatusImproved: "improved", differ.StatusNew: "new",
		differ.StatusRemoved: "removed", differ.StatusUnchanged: "unchanged",
	}
	var parts []string
	for _, s := range statusOrder {
		parts = append(parts, fmt.Sprintf("%d %s", r.Counts[s], labels[s]))
	}
	return strings.Join(parts, ", ")
}

// FormatDiffText renders a human-readable diff report. UNCHANGED targets are
// summarized in the count line rather than listed.
func FormatDiffText(r differ.Report, basePath string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "go-solid-score diff (base: %s)\n", basePath)
	b.WriteString(strings.Repeat("=", 52) + "\n")
	for _, e := range sortedEntries(r) {
		switch e.Status {
		case differ.StatusUnchanged:
			continue
		case differ.StatusRegressed, differ.StatusImproved:
			fmt.Fprintf(&b, "%-10s %s  %.1f -> %.1f (%+.1f)\n",
				e.Status, e.ID, *e.Base, *e.Head, e.Diff())
		case differ.StatusNewLow:
			fmt.Fprintf(&b, "%-10s %s  %.1f (< min)\n", e.Status, e.ID, *e.Head)
		case differ.StatusNew:
			fmt.Fprintf(&b, "%-10s %s  %.1f\n", e.Status, e.ID, *e.Head)
		case differ.StatusRemoved:
			fmt.Fprintf(&b, "%-10s %s\n", e.Status, e.ID)
		}
	}
	b.WriteString(strings.Repeat("-", 52) + "\n")
	b.WriteString(summaryLine(r) + "\n")
	return b.String()
}

// FormatDiffMarkdown renders an octocov-style Markdown report for PR comments.
// Notable targets go in the top table; the full list is folded in <details>.
func FormatDiffMarkdown(r differ.Report) string {
	var b strings.Builder
	b.WriteString(diffMarker + "\n")
	b.WriteString("## go-solid-score\n\n")
	fmt.Fprintf(&b, "%s.\n\n", summaryLine(r))

	writeRow := func(e differ.Entry) {
		base, head, diff := "–", "–", "–"
		if e.Base != nil {
			base = fmt.Sprintf("%.1f", *e.Base)
		}
		if e.Head != nil {
			head = fmt.Sprintf("%.1f", *e.Head)
		}
		if e.Base != nil && e.Head != nil {
			diff = fmt.Sprintf("%+.1f", e.Diff())
		}
		fmt.Fprintf(&b, "| %s %s | `%s` | %s | %s | %s |\n",
			statusEmoji[e.Status], e.Status, e.ID, base, head, diff)
	}

	b.WriteString("| | target | base | head | diff |\n|--|--|--|--|--|\n")
	for _, e := range sortedEntries(r) {
		if e.Status == differ.StatusUnchanged {
			continue
		}
		writeRow(e)
	}

	if r.Counts[differ.StatusUnchanged] > 0 {
		fmt.Fprintf(&b, "\n<details><summary>All targets (incl. %d unchanged)</summary>\n\n",
			r.Counts[differ.StatusUnchanged])
		b.WriteString("| | target | base | head | diff |\n|--|--|--|--|--|\n")
		for _, e := range sortedEntries(r) {
			writeRow(e)
		}
		b.WriteString("\n</details>\n")
	}
	return b.String()
}

// diffJSONResult is the machine-readable per-target diff record.
type diffJSONResult struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Package string   `json:"package"`
	Status  string   `json:"status"`
	Base    *float64 `json:"base"`
	Head    *float64 `json:"head"`
	Diff    *float64 `json:"diff"`
}

// FormatDiffJSON renders the diff report as machine-readable JSON.
func FormatDiffJSON(r differ.Report) string {
	type summary struct {
		Counts    map[string]int `json:"counts"`
		Regressed bool           `json:"regressed"`
	}
	type doc struct {
		Results []diffJSONResult `json:"results"`
		Summary summary          `json:"summary"`
	}

	d := doc{Summary: summary{Counts: map[string]int{}, Regressed: r.Regressed}}
	for s, c := range r.Counts {
		d.Summary.Counts[string(s)] = c
	}
	for _, e := range sortedEntries(r) {
		jr := diffJSONResult{
			ID: e.ID, Name: e.Name, Package: e.Package, Status: string(e.Status),
			Base: e.Base, Head: e.Head,
		}
		if e.Base != nil && e.Head != nil {
			dv := e.Diff()
			jr.Diff = &dv
		}
		d.Results = append(d.Results, jr)
	}
	out, _ := json.MarshalIndent(d, "", "  ")
	return string(out) + "\n"
}
```

- [ ] **Step 4: テストが通ることを確認**

Run: `go test ./formatter/ -run TestFormatDiff -v`
Expected: PASS（3テストとも）

- [ ] **Step 5: コミット**

```bash
git add formatter/diff.go formatter/diff_test.go
git commit -m "feat: add text/markdown/json renderers for diff report"
```

---

## Task 5: `diff` サブコマンド

**Files:**
- Create: `cmd/diff.go`
- Modify: `cmd/root.go`（`newRootCmd` でサブコマンド登録）
- Test: `cmd/diff_test.go`

- [ ] **Step 1: `cmd/diff.go` を実装**

```go
package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/harakeishi/go-solid-score/config"
	"github.com/harakeishi/go-solid-score/differ"
	"github.com/harakeishi/go-solid-score/formatter"
	"github.com/harakeishi/go-solid-score/scorer"
	"github.com/spf13/cobra"
)

var (
	diffBase     string
	diffMaxDrop  float64
	diffMinScore float64
	diffFailOnReg bool
	diffFormat   string
)

func newDiffCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "diff [packages...]",
		Short: "Compare current SOLID scores against a baseline JSON",
		Long: "diff analyzes the given packages and compares each target's score " +
			"against a baseline JSON (produced earlier with `-f json`), reporting " +
			"regressions, improvements, new and removed targets.",
		Args: cobra.ArbitraryArgs,
		RunE: runDiff,
	}
	cmd.Flags().StringVar(&diffBase, "base", "", "Baseline JSON file to compare against (required)")
	cmd.Flags().Float64Var(&diffMaxDrop, "max-drop", 5.0, "A total drop greater than this is a regression")
	cmd.Flags().Float64Var(&diffMinScore, "min-score", 0, "A new target below this is NEW-LOW (0 disables)")
	cmd.Flags().BoolVar(&diffFailOnReg, "fail-on-regression", false, "Exit 1 if any regression or new-low exists")
	cmd.Flags().StringVarP(&diffFormat, "format", "f", "text", "Output format: text, json, markdown")
	_ = cmd.MarkFlagRequired("base")
	return cmd
}

// loadBaseline reads and decodes a baseline JSON file into snapshots.
func loadBaseline(path string) ([]differ.Snapshot, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading baseline: %w", err)
	}
	var doc formatter.JSONOutput
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parsing baseline JSON: %w", err)
	}
	snaps := make([]differ.Snapshot, 0, len(doc.Results))
	for _, r := range doc.Results {
		snaps = append(snaps, differ.Snapshot{
			ID: r.ID, Name: r.Name, Package: r.Package, Total: r.Total,
		})
	}
	return snaps, nil
}

// resultsToSnapshots projects scored results to diff snapshots.
func resultsToSnapshots(results []*scorer.ScoreResult) []differ.Snapshot {
	snaps := make([]differ.Snapshot, 0, len(results))
	for _, r := range results {
		snaps = append(snaps, differ.Snapshot{
			ID: r.TargetID(), Name: r.TargetName, Package: r.TargetPkg, Total: r.Total,
		})
	}
	return snaps
}

func runDiff(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	patterns := cfg.Paths
	if len(args) > 0 {
		patterns = args
	}

	base, err := loadBaseline(diffBase)
	if err != nil {
		return err
	}

	headResults, err := analyze(cfg, patterns)
	if err != nil {
		return err
	}
	head := resultsToSnapshots(headResults)

	report := differ.Diff(base, head, differ.Options{
		MaxDrop:  diffMaxDrop,
		MinScore: diffMinScore,
	})

	var out string
	switch diffFormat {
	case "json":
		out = formatter.FormatDiffJSON(report)
	case "markdown":
		out = formatter.FormatDiffMarkdown(report)
	default:
		out = formatter.FormatDiffText(report, diffBase)
	}
	if _, err := fmt.Fprint(os.Stdout, out); err != nil {
		return fmt.Errorf("writing output: %w", err)
	}

	if diffFailOnReg && report.Regressed {
		return fmt.Errorf("regression detected")
	}
	return nil
}
```

注: `cfgFile` は `cmd/root.go` のパッケージ変数を再利用する。`diff` には専用の
`--config` フラグを足さず、root の `-c` は使えないため、diff 自身に config フラグを
持たせる必要がある。下の Step 2 で `cmd.Flags()` に `--config` を追加する。

- [ ] **Step 2: `diff` に config フラグを追加し、root に登録**

`cmd/diff.go` の `newDiffCmd` 内、`MarkFlagRequired` の直前に追加:

```go
	cmd.Flags().StringVarP(&cfgFile, "config", "c", ".go-solid-score.yaml", "Config file path")
```

`cmd/root.go` の `newRootCmd` の `return cmd` の直前に追加:

```go
	cmd.AddCommand(newDiffCmd())
```

- [ ] **Step 3: ビルドを確認**

Run: `go build ./...`
Expected: 成功（エラーなし）

- [ ] **Step 4: E2E テストを書く**

`cmd/diff_test.go`:

```go
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
```

ファイル先頭の import に `"strconv"` を追加すること。

- [ ] **Step 5: テストが通ることを確認**

Run: `go test ./cmd/ -v`
Expected: PASS

- [ ] **Step 6: 実バイナリでスモークテスト（回帰を意図的に作る）**

```bash
go run . -f json ./differ/ > /tmp/base.json
go run . diff --base /tmp/base.json ./differ/
echo "exit: $?"
```
Expected: `0 regressed` を含む text 出力、exit 0（同一コードなので回帰なし）

- [ ] **Step 7: markdown 出力を確認**

Run: `go run . diff --base /tmp/base.json -f markdown ./differ/ | head -5`
Expected: `<!-- go-solid-score-diff -->` で始まる Markdown

- [ ] **Step 8: コミット**

```bash
git add cmd/diff.go cmd/root.go cmd/diff_test.go
git commit -m "feat: add diff subcommand wiring base load, scoring, and output"
```

---

## Task 6: GitHub Actions サンプルと README

**Files:**
- Create: `.github/workflows/solid-diff.yml`
- Modify: `README.md`

- [ ] **Step 1: ワークフローサンプルを作成**

`.github/workflows/solid-diff.yml`:

```yaml
name: solid-score-diff

on:
  pull_request:

permissions:
  contents: read
  pull-requests: write

jobs:
  diff:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0

      - uses: actions/setup-go@v5
        with:
          go-version: stable

      - name: Install go-solid-score
        run: go install github.com/harakeishi/go-solid-score@latest

      - name: Score base (merge base)
        run: |
          git worktree add /tmp/base "${{ github.event.pull_request.base.sha }}"
          (cd /tmp/base && go-solid-score -f json ./...) > base.json

      - name: Diff against base
        id: diff
        run: |
          go-solid-score diff --base base.json -f markdown ./... > comment.md
          go-solid-score diff --base base.json ./...  # human-readable log

      - name: Comment on PR
        uses: marocchino/sticky-pull-request-comment@v2
        with:
          header: go-solid-score-diff
          path: comment.md
```

注: `sticky-pull-request-comment` は header をキーに既存コメントを更新する
（octocov の updatePrevious 相当）。CLI 出力の `<!-- go-solid-score-diff -->`
マーカーは header と独立して機能する。

- [ ] **Step 2: README に diff セクションを追記**

`README.md` の `## Stable Target IDs (for score diffing)` セクションの直後に以下を追加:

```markdown
## Diffing scores (regression gating)

Compare the current scores against a baseline to detect regressions:

\`\`\`bash
# 1. Capture a baseline (e.g. on the main branch)
go-solid-score -f json ./... > base.json

# 2. After making changes, diff against it
go-solid-score diff --base base.json ./...

# Fail CI when a target regresses or a new target is below a floor
go-solid-score diff --base base.json --min-score 70 --fail-on-regression ./...

# Markdown output for PR comments
go-solid-score diff --base base.json -f markdown ./... > comment.md
\`\`\`

Targets are matched by their stable `id`, so renames and file moves do not
produce false regressions. Each target is classified as REGRESSED, IMPROVED,
UNCHANGED, NEW, NEW-LOW, or REMOVED.

| Flag | Default | Meaning |
|------|---------|---------|
| \`--base\` | (required) | Baseline JSON to compare against |
| \`--max-drop\` | 5.0 | A total drop greater than this is a regression |
| \`--min-score\` | 0 | A new target below this is NEW-LOW (0 disables) |
| \`--fail-on-regression\` | false | Exit 1 on any regression or new-low |
| \`-f, --format\` | text | \`text\`, \`json\`, or \`markdown\` |

See [`.github/workflows/solid-diff.yml`](.github/workflows/solid-diff.yml) for a
PR-comment workflow (requires \`pull-requests: write\`; fork PRs fall back to a
job summary).
```

- [ ] **Step 3: ビルドと全テスト最終確認**

Run: `go build ./... && go vet ./... && go test ./... 2>&1 | tail -12`
Expected: 全 PASS

- [ ] **Step 4: 変更ファイルの gofmt 確認**

Run: `gofmt -l $(git diff --name-only HEAD~5 | grep '\.go$') 2>/dev/null; echo "checked"`
Expected: 何も出力されない（クリーン）

- [ ] **Step 5: コミット**

```bash
git add .github/workflows/solid-diff.yml README.md
git commit -m "docs: add diff CI workflow sample and README section"
```

---

## 完了条件

- `go-solid-score diff --base base.json ./...` が text/json/markdown を出力する
- 回帰時 `--fail-on-regression` で exit 1、未指定なら exit 0
- 安定 ID により、ファイル移動で誤検知しない
- `differ.Diff` が純関数としてテーブル駆動テストで網羅されている
- 全テスト・vet・gofmt がクリーン
- PR コメント用ワークフローサンプルが存在する
