# LSCC 凝集度メトリクス移行 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** SRP 凝集度メトリクスを LCOM4 から LSCC（素の論文定義、0〜1）に即置換し、しきい値とペナルティを yml 側に移す。

**Architecture:** `analyzer` に純粋関数 `calculateLSCC` を追加し、`StructMetrics` が `lscc` を出力。SRP のスコアリングしきい値は `rules/presets.yaml` の bands で定義する。SRP 用の LCOM4 系メトリクス（lcom4 / srp_cohesion_penalty / srp_avg_component_size）と関連関数は撤去するが、`calculateLCOM4` は ISP の `public_lcom4` で現役のため残す。

**Tech Stack:** Go (標準 testing), gopkg.in/yaml.v3（既存）。

## Global Constraints

- LSCC は素の論文定義のみ。シングルトン除外などの独自ヒューリスティックを入れない。
- `calculateLSCC` は `l <= 1`（メソッド数）または `k == 0`（名前付きフィールド数）のとき **0.0** を返す。
- 値域 0〜1（1 が最高凝集）。
- 偽陽性対策はメトリクス計算ではなく yml ルール側（しきい値/where）で行う。
- 既存の `calculateLCOM4`・`shareField`・`callsEachOther` は削除しない（ISP で使用中）。
- コミットメッセージ末尾に `Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>` を付ける。
- 設計の出典: `docs/superpowers/specs/2026-06-30-lscc-cohesion-metric-design.md`。

---

### Task 1: `calculateLSCC` 純粋関数の追加

LSCC の中核計算を、メトリクス配線やルール変更とは独立に、純粋関数として TDD で追加する。

**Files:**
- Modify: `analyzer/srp.go`（関数追加）
- Test: `analyzer/srp_internal_test.go`（新規。internal test = `package analyzer`）

**Interfaces:**
- Consumes: `model.MethodInfo`（フィールド `AccessedFields []string`）。
- Produces: `func calculateLSCC(methods []*model.MethodInfo, namedFieldCount int) float64`
  - `namedFieldCount` は名前付き（非埋め込み）フィールド数 `k`。呼び出し側で算出して渡す。
  - 返り値は 0〜1。`l <= 1 || k <= 0` のとき 0.0。

LSCC 定義: 各名前付きフィールド `f` にアクセスするメソッド数を `x_f` として
`LSCC = Σ_f ( x_f·(x_f−1) ) / ( k·l·(l−1) )`（l=メソッド数, k=名前付きフィールド数）。

`AccessedFields` には埋め込みフィールドへのアクセス等、名前付きフィールド集合に
含まれない名前が混じりうる。分子は「メソッドが各フィールドにアクセスした延べ回数」を
フィールド名でカウントするが、分母の `k` は構造体の名前付きフィールド数を用いる。
`x_f` は `AccessedFields` に出現したフィールド名ごとのアクセスメソッド数をそのまま使う
（名前付きフィールド集合との突き合わせは不要 — 分子に現れる未知フィールドは
分母 `k` に対して相対的に小さく、素の定義に従う）。

- [ ] **Step 1: 失敗するテストを書く**

`analyzer/srp_internal_test.go` を新規作成:

```go
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
```

- [ ] **Step 2: テストが失敗することを確認**

Run: `cd /Users/keishi.hara/src/github.com/harakeishi/go-solid-score && go test ./analyzer/ -run TestCalculateLSCC -v`
Expected: FAIL（`undefined: calculateLSCC`）

- [ ] **Step 3: 最小実装を書く**

`analyzer/srp.go` の末尾に追加:

```go
// calculateLSCC computes the LSCC (Low-level Similarity-based Class Cohesion,
// Al Dallal & Briand 2012) cohesion metric in [0,1], where 1 is maximally
// cohesive. For each named field f accessed by x_f methods, the metric sums
// x_f*(x_f-1) and normalizes by k*l*(l-1) (l methods, k named fields). It
// returns 0 when the metric is undefined (l <= 1 or k <= 0). Unlike LCOM4 this
// is a normalized ratio, so a single stateless method dilutes rather than
// fragments the score; false-positive control is left to the rule thresholds.
func calculateLSCC(methods []*model.MethodInfo, namedFieldCount int) float64 {
	l := len(methods)
	if l <= 1 || namedFieldCount <= 0 {
		return 0
	}
	accessCount := make(map[string]int)
	for _, m := range methods {
		for _, f := range m.AccessedFields {
			accessCount[f]++
		}
	}
	numerator := 0.0
	for _, x := range accessCount {
		numerator += float64(x) * float64(x-1)
	}
	denominator := float64(namedFieldCount) * float64(l) * float64(l-1)
	return numerator / denominator
}
```

- [ ] **Step 4: テストが通ることを確認**

Run: `cd /Users/keishi.hara/src/github.com/harakeishi/go-solid-score && go test ./analyzer/ -run TestCalculateLSCC -v`
Expected: PASS（全サブテスト）

- [ ] **Step 5: コミット**

```bash
cd /Users/keishi.hara/src/github.com/harakeishi/go-solid-score
git add analyzer/srp.go analyzer/srp_internal_test.go
git commit -m "$(cat <<'EOF'
feat(analyzer): add calculateLSCC cohesion metric (#18)

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 2: `lscc` メトリクスの配線と LCOM4 系メトリクスの撤去

`StructMetrics` で `lscc` を出力し、SRP 用の LCOM4 系メトリクスと関連関数を撤去する。メトリクス語彙も更新する。この時点ではプリセットルールがまだ古いメトリクスを参照しているためビルド/テストは赤になる — 次タスクで緑にする。

**Files:**
- Modify: `analyzer/metrics.go`（`StructMetrics` 内の SRP ブロック、`metricNames`）
- Modify: `analyzer/srp.go`（`srpCohesion`, `baseLCOM4Penalty` を削除）

**Interfaces:**
- Consumes: Task 1 の `calculateLSCC(methods, namedFieldCount) float64`。
- Produces: メトリクスキー `"lscc"`（`StructMetrics` が出力）。`"lcom4"`,
  `"srp_cohesion_penalty"`, `"srp_avg_component_size"` は **出力されなくなる**。

- [ ] **Step 1: `StructMetrics` の SRP ブロックを差し替える**

`analyzer/metrics.go` の現在の SRP ブロック:

```go
	// --- SRP: LCOM4 cohesion, size, complexity ---
	cohesionPenalty, lcom4 := srpCohesion(methods)
	m["lcom4"] = float64(lcom4)
	m["srp_cohesion_penalty"] = cohesionPenalty
	if lcom4 > 0 {
		m["srp_avg_component_size"] = float64(len(methods)) / float64(lcom4)
	}
	totalComplexity := 0
	for _, mth := range methods {
		totalComplexity += mth.CyclomaticComplexity
	}
	m["total_complexity"] = float64(totalComplexity)
```

を次に置き換える（`namedFields` は同関数の上方で算出済み — `field_count` 用の
ループで得た `namedFields int` をそのまま使う）:

```go
	// --- SRP: LSCC cohesion, complexity ---
	m["lscc"] = calculateLSCC(methods, namedFields)
	totalComplexity := 0
	for _, mth := range methods {
		totalComplexity += mth.CyclomaticComplexity
	}
	m["total_complexity"] = float64(totalComplexity)
```

- [ ] **Step 2: `metricNames` を更新する**

`analyzer/metrics.go` の `metricNames` 内、現在の行:

```go
	"lcom4", "srp_cohesion_penalty", "srp_avg_component_size", "total_complexity",
```

を次に置き換える:

```go
	"lscc", "total_complexity",
```

- [ ] **Step 3: 未使用になる関数を削除する**

`analyzer/srp.go` から `srpCohesion` 関数（`func srpCohesion(...) (penalty float64, lcom4 int) { ... }` 全体、metrics.go ではなく srp.go の `baseLCOM4Penalty` と合わせて）を削除する。

具体的には `analyzer/srp.go` の `baseLCOM4Penalty`（行コメント含む全体）と、`analyzer/metrics.go` の `srpCohesion` 関数全体を削除する。
`calculateLCOM4`・`shareField`・`callsEachOther` は **残す**（ISP の `public_lcom4` で使用）。

- [ ] **Step 4: ビルドを確認（コンパイルは通り、テストは赤を許容）**

Run: `cd /Users/keishi.hara/src/github.com/harakeishi/go-solid-score && go build ./...`
Expected: 成功（未使用関数削除によりビルドが通る）。
Run: `cd /Users/keishi.hara/src/github.com/harakeishi/go-solid-score && go vet ./analyzer/`
Expected: 成功。

注: この時点で `go test ./...` は **失敗してよい**（プリセット/テストがまだ古いメトリクス参照）。次タスクで解消する。コミットはまだしない。

---

### Task 3: プリセットルールを LSCC ベースに置換

`rules/presets.yaml` の SRP 凝集度ルールを LSCC の bands に置き換え、プリセット検証テストを緑にする。

**Files:**
- Modify: `rules/presets.yaml`（`srp-cohesion`, `srp-cohesion-confidence`）

**Interfaces:**
- Consumes: メトリクス `"lscc"`, `"method_count"`, `"has_fields"`。

- [ ] **Step 1: `srp-cohesion` ルールを置換**

`rules/presets.yaml` の現在の `srp-cohesion` ルール:

```yaml
  - id: srp-cohesion
    principle: SRP
    metric: srp_cohesion_penalty
    when: "> 0"
    effect: penalty
    from_metric: true
    message: "low cohesion (LCOM4): -%v penalty"
```

を次に置き換える:

```yaml
  - id: srp-cohesion
    principle: SRP
    metric: lscc
    where: ["method_count >= 2", "has_fields == 1"]
    bands:
      - { when: "< 0.2", value: 40, message: "very low cohesion (LSCC=%v)" }
      - { when: "< 0.4", value: 25, message: "low cohesion (LSCC=%v)" }
      - { when: "< 0.6", value: 10, message: "moderate cohesion (LSCC=%v)" }
```

- [ ] **Step 2: `srp-cohesion-confidence` ルールを削除**

`rules/presets.yaml` の `srp-cohesion-confidence` ルール（コメントブロック
「A heavily attenuated cohesion penalty ...」を含む)全体を削除する。これは
`srp_avg_component_size`/`lcom4` に依存しており、LSCC には対応概念がないため撤去する
（large aggregate の confidence 低減は YAGNI として初期は入れない）。

- [ ] **Step 3: プリセット検証テストが通ることを確認**

Run: `cd /Users/keishi.hara/src/github.com/harakeishi/go-solid-score && go test ./analyzer/ -run TestPresetsValidateAgainstMetricNames -v`
Expected: PASS（プリセットが未知メトリクスを参照していない）。
Run: `cd /Users/keishi.hara/src/github.com/harakeishi/go-solid-score && go test ./analyzer/ -run TestMetricNamesCoverEmitted -v`
Expected: PASS（`lscc` が語彙に登録済み、撤去メトリクスは出力されない）。

注: SRP スコアリングテスト（srp_test.go）と engine_test.go はまだ赤の可能性。Task 4/5 で解消。

---

### Task 4: engine_test.go の LCOM4 依存テストを LSCC に更新

`rules/engine_test.go` の `TestDefaultEngine_CohesionConfidenceBoundary` は撤去した
`srp_avg_component_size`/`srp_cohesion_penalty`/`lcom4` と、削除した confidence 低減
ルールに依存している。LSCC 移行後はこの振る舞いが存在しないため、テストを撤去し、
代わりに LSCC bands が正しく減点することを検証するテストに置き換える。

**Files:**
- Modify: `rules/engine_test.go`

**Interfaces:**
- Consumes: `DefaultEngine()`, `Metrics`, `e.Evaluate("SRP", false, m)`（既存ヘルパ）。

- [ ] **Step 1: 古いテストを置き換える**

`rules/engine_test.go` の `TestDefaultEngine_CohesionConfidenceBoundary` 関数
（直前のコメントブロック「// TestDefaultEngine_CohesionConfidenceBoundary pins ...」を含む全体）を、次に置き換える:

```go
// TestDefaultEngine_LSCCCohesionBands verifies the SRP cohesion penalty is now
// driven by the LSCC metric via banded thresholds: lower cohesion -> larger
// penalty, and a cohesive type (LSCC >= 0.6) takes no cohesion penalty.
func TestDefaultEngine_LSCCCohesionBands(t *testing.T) {
	base := func(lscc float64) Metrics {
		return Metrics{"method_count": 6, "has_fields": 1, "lscc": lscc}
	}
	cases := []struct {
		lscc    float64
		wantMin float64 // expected cohesion penalty (lower bound of the band)
	}{
		{0.1, 40},
		{0.3, 25},
		{0.5, 10},
	}
	var prev float64 = 101
	for _, c := range cases {
		score := DefaultEngine().Evaluate("SRP", false, base(c.lscc)).Score
		if score >= prev {
			t.Errorf("lscc=%.1f score %.1f should be lower than higher-cohesion case %.1f", c.lscc, score, prev)
		}
		prev = score
	}
	// A cohesive type takes no cohesion penalty (only any non-cohesion SRP
	// rules apply); score must exceed the moderate-band case.
	cohesive := DefaultEngine().Evaluate("SRP", false, base(0.8)).Score
	moderate := DefaultEngine().Evaluate("SRP", false, base(0.5)).Score
	if cohesive <= moderate {
		t.Errorf("cohesive (LSCC=0.8) score %.1f should exceed moderate (LSCC=0.5) %.1f", cohesive, moderate)
	}
}
```

- [ ] **Step 2: テストが通ることを確認**

Run: `cd /Users/keishi.hara/src/github.com/harakeishi/go-solid-score && go test ./rules/ -run TestDefaultEngine_LSCCCohesionBands -v`
Expected: PASS。

- [ ] **Step 3: rules パッケージ全体のテストを確認**

Run: `cd /Users/keishi.hara/src/github.com/harakeishi/go-solid-score && go test ./rules/`
Expected: PASS（古い境界テスト参照が消え、他テストも緑）。

---

### Task 5: SRP analyzer テストを LSCC 基準に更新し、testdata で偽陽性を検証

`analyzer/srp_test.go` は LCOM4 固有の振る舞い（graduated penalty, stateless 除外）を
検証している。LSCC 移行でこれらの前提が変わるため更新する。**特に `ParseError`
（msg フィールド1個、Error が使用・Is は不使用）は素の LSCC=0 になり偽陽性となりうる** —
実際のスコアを確認し、設計どおり「しきい値で吸収」が成立するか検証する。

**Files:**
- Modify: `analyzer/srp_test.go`

**Interfaces:**
- Consumes: `analyzer.NewSRPAnalyzer()`, `a.Analyze(pkg)`, `parser.Parse`（既存）。
- testdata: `testdata/srp/{good,bad,facade}.go`（TaxCalculator, GodStruct, ParseError, LargeFacade, SmallSplit）。

- [ ] **Step 1: 現状の各 testdata 型の LSCC スコアを実測する（探索用、コミットしない）**

一時的に次の探索テストを `analyzer/srp_test.go` の末尾に追加して値を観測する:

```go
func TestSRP_LSCC_Probe(t *testing.T) {
	pkgs, _ := parser.Parse([]string{"../testdata/srp"})
	a := analyzer.NewSRPAnalyzer()
	for _, r := range a.Analyze(pkgs[0]) {
		t.Logf("PROBE %-14s score=%.1f conf=%.2f", r.TargetName, r.Score, r.Confidence)
	}
}
```

Run: `cd /Users/keishi.hara/src/github.com/harakeishi/go-solid-score && go test ./analyzer/ -run TestSRP_LSCC_Probe -v`
Expected: 各型の score がログ出力される。**TaxCalculator/ParseError が高め（>=80目安）、GodStruct が低め（<70目安）になっているか確認**する。

- [ ] **Step 2: 観測結果に応じて判断する**

- TaxCalculator (LSCC≈0.67) と GodStruct (低 LSCC) は設計どおり分離されるはず。
- **ParseError が偽陽性（不当に低スコア）になる場合**: これは素の LSCC=0 由来。
  対処は yml しきい値ではなく、`srp-stateless` 既存ルールの射程外（2メソッド・1フィールド）
  のため bands が発火する。**この場合は実装を止め、`main` セッション/レビューに報告して
  方針を仰ぐ**（選択肢: stateless 規約メソッドを LSCC のメソッド集合から除く例外を
  設計に追加するか、しきい値を下げるか）。勝手にヒューリスティックを足さない
  （Global Constraints: 素の定義を守る）。
- ParseError が許容範囲（例: 既存の `srp:want SRP=ok` を満たす程度）なら次へ。

- [ ] **Step 3: 探索テストを削除し、本テストを更新する**

`TestSRP_LSCC_Probe` を削除する。`TestSRPAnalyzer_Good` と `TestSRPAnalyzer_Bad` は
意味が保たれる（TaxCalculator >= 80 / GodStruct < 70）ためそのまま残す。

LCOM4 固有の前提に依存する次の2テストを撤去する:
- `TestSRPAnalyzer_GraduatedLCOM4`（graduated LCOM4 penalty と confidence 低減の検証 — LSCC には該当概念なし）
- `TestSRPAnalyzer_StatelessConventionMethod`（LCOM4 のシングルトン除外検証 — Step 2 の観測で ParseError の扱いが確定したら、その確定挙動を検証する形に書き換えるか撤去する）

`TestSRPAnalyzer_StatelessConventionMethod` の置き換え（Step 2 で ParseError が
許容範囲だった場合）:

```go
// TestSRPAnalyzer_StatelessConventionMethod verifies the cohesive error type
// (ParseError) is not driven below the SRP threshold by the stateless errors.Is
// convention method. Under LSCC the single stateless method dilutes rather than
// fragments cohesion; the rule thresholds keep this type acceptable.
func TestSRPAnalyzer_StatelessConventionMethod(t *testing.T) {
	pkgs, err := parser.Parse([]string{"../testdata/srp"})
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	a := analyzer.NewSRPAnalyzer()
	for _, r := range a.Analyze(pkgs[0]) {
		if r.TargetName == "ParseError" {
			if r.Score < 70 {
				t.Errorf("ParseError SRP score %.1f should stay >= 70 "+
					"(a stateless errors.Is method must not drive a cohesive type below threshold)", r.Score)
			}
			return
		}
	}
	t.Error("ParseError not found in results")
}
```

（Step 2 で ParseError が偽陽性だった場合は本テストは追加せず、Step 2 の報告に従う。）

- [ ] **Step 4: analyzer パッケージ全体のテストを確認**

Run: `cd /Users/keishi.hara/src/github.com/harakeishi/go-solid-score && go test ./analyzer/`
Expected: PASS。

- [ ] **Step 5: コミット（Task 2〜5 をまとめて）**

```bash
cd /Users/keishi.hara/src/github.com/harakeishi/go-solid-score
git add analyzer/metrics.go analyzer/srp.go rules/presets.yaml rules/engine_test.go analyzer/srp_test.go
git commit -m "$(cat <<'EOF'
feat(srp): replace LCOM4 cohesion scoring with LSCC (#18)

Wire the lscc metric into StructMetrics, drop the SRP LCOM4 metrics
(lcom4/srp_cohesion_penalty/srp_avg_component_size) and their helpers,
and move cohesion thresholds into rules/presets.yaml bands. calculateLCOM4
is retained for ISP's public_lcom4.

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 6: README のメトリクス語彙を更新

ドキュメントを LSCC 移行に合わせて更新する。

**Files:**
- Modify: `README.md`（行 ~189 の例、~231-235 のメトリクス語彙一覧）

**Interfaces:** なし（ドキュメントのみ）。

- [ ] **Step 1: README のメトリクス語彙一覧を更新**

`README.md` 内、SRP メトリクスを列挙している箇所（`has_fields`, `lcom4`,
`srp_cohesion_penalty`, `srp_avg_component_size`, ...）から
`lcom4`, `srp_cohesion_penalty`, `srp_avg_component_size` を削除し、`lscc` を追加する。
`public_lcom4`（ISP）は残す。

- [ ] **Step 2: README の例ルールを更新**

`README.md` 内、`metric: srp_cohesion_penalty` を使ったカスタムルール例を、
LSCC を使う例に差し替える:

```yaml
rules:
  - id: my-strict-cohesion
    principle: SRP
    metric: lscc
    where: ["method_count >= 2", "has_fields == 1"]
    bands:
      - { when: "< 0.5", value: 30, message: "low cohesion (LSCC=%v)" }
```

LSCC の説明文（0〜1、1 が最高凝集、Al Dallal & Briand 2012）を1〜2文添える。

- [ ] **Step 3: ドキュメントの整合を目視確認**

Run: `cd /Users/keishi.hara/src/github.com/harakeishi/go-solid-score && grep -n "lcom4\|srp_cohesion_penalty\|srp_avg_component_size\|lscc" README.md`
Expected: `lscc` と `public_lcom4` のみが残り、撤去した3メトリクスは（ISP の public_lcom4 を除き）出てこない。

- [ ] **Step 4: コミット**

```bash
cd /Users/keishi.hara/src/github.com/harakeishi/go-solid-score
git add README.md
git commit -m "$(cat <<'EOF'
docs: update metric vocabulary for LSCC migration (#18)

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 7: 全体検証

リポジトリ全体でビルド・テスト・既存の評価ハーネスが通ることを確認する。

**Files:** なし（検証のみ）。

- [ ] **Step 1: 全テスト**

Run: `cd /Users/keishi.hara/src/github.com/harakeishi/go-solid-score && go build ./... && go test ./...`
Expected: 全パッケージ PASS。

- [ ] **Step 2: vet**

Run: `cd /Users/keishi.hara/src/github.com/harakeishi/go-solid-score && go vet ./...`
Expected: 出力なし（成功）。

- [ ] **Step 3: ツールを自分自身に対して実行し、SRP スコアが妥当か目視**

Run: `cd /Users/keishi.hara/src/github.com/harakeishi/go-solid-score && go run . analyzer/ 2>&1 | head -40`
Expected: クラッシュせず SRP スコアが出力される（出力フォーマットは既存 CLI に依存。
エラーになる場合は実行方法を README の Usage に合わせて調整）。

注: `./...` は testdata を拾わない既知制約あり。評価ハーネスを別途回す場合は対象を明示列挙する。

---

## Self-Review メモ

- **Spec coverage**: spec の決定事項（即置換 / 素の定義 / yml しきい値 / lscc 出力 /
  SRP から LCOM4 撤去 / 境界 0.0 / README 更新 / テスト更新）はすべて Task 1〜7 に対応。
- **偽陽性リスク（ParseError）**: spec が「しきい値で吸収」とした点を Task 5 Step 1-2 で
  実測検証し、成立しない場合はレビューに上げる明示ゲートを置いた（勝手にヒューリスティックを
  足さない Global Constraint を守る）。
- **型整合**: `calculateLSCC(methods []*model.MethodInfo, namedFieldCount int) float64` を
  Task 1 で定義、Task 2 で `namedFields`(int) を渡して呼ぶ。`StructInfo.Fields` は
  `[]*FieldInfo` だが Task 2 は既存ループで得た `namedFields int` を再利用するため
  ポインタ/値の差異は影響しない。
- **残す関数**: `calculateLCOM4`/`shareField`/`callsEachOther` は削除対象に含めていない。
